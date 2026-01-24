"""
Price Fetcher Module

Fetches latest asset prices from multiple sources with fallback support.
Supports A-shares, HK stocks, US stocks, and gold.

Data Sources (in order of priority):
1. AKShare - Primary source for A-shares and Chinese markets
2. Yahoo Finance (yfinance) - Good for US/HK stocks
3. Sina Finance API - Backup for A-shares
4. Tencent Finance API - Backup for A-shares and HK stocks
"""

import re
import urllib.request
import json
from typing import Optional, Tuple, List, Callable
from datetime import datetime

from logger_config import logger

# Check available libraries
try:
    import akshare as ak
    AKSHARE_AVAILABLE = True
except ImportError:
    AKSHARE_AVAILABLE = False

try:
    import yfinance as yf
    YFINANCE_AVAILABLE = True
except ImportError:
    YFINANCE_AVAILABLE = False


def detect_symbol_type(symbol: str, currency: str) -> str:
    """Detect the type of symbol based on its format and currency."""
    symbol = symbol.upper()
    
    # A-shares: starts with SH/SZ or 6-digit number
    if symbol.startswith('SH') or symbol.startswith('SZ'):
        return 'a_share'
    if currency == 'CNY' and re.match(r'^\d{6}$', symbol):
        return 'a_share'
    
    # HK stocks: numeric codes (typically 5 digits with leading zeros)
    if currency == 'HKD' or re.match(r'^0\d{4}$', symbol):
        return 'hk_stock'
    
    # Gold
    if 'AU' in symbol or 'GOLD' in symbol.upper():
        return 'gold'
    
    # Cash
    if symbol.upper() == 'CASH':
        return 'cash'
    
    # US stocks: alphabetic symbols
    if currency == 'USD' or re.match(r'^[A-Z]+$', symbol):
        return 'us_stock'
    
    # Bonds
    if 'BOND' in symbol.upper():
        return 'bond'
    
    return 'unknown'


# ============================================================================
# AKShare Service (Primary)
# ============================================================================

def akshare_fetch_a_share(symbol: str) -> Optional[float]:
    """Fetch A-share price using AKShare."""
    if not AKSHARE_AVAILABLE:
        return None
    try:
        code = symbol.upper()
        if code.startswith('SH') or code.startswith('SZ'):
            code = code[2:]
        
        df = ak.stock_zh_a_spot_em()
        row = df[df['代码'] == code]
        if not row.empty:
            return float(row.iloc[0]['最新价'])
    except Exception as e:
        logger.debug(f"AKShare A-share error for {symbol}: {e}")
    return None


def akshare_fetch_hk_stock(symbol: str) -> Optional[float]:
    """Fetch HK stock price using AKShare."""
    if not AKSHARE_AVAILABLE:
        return None
    try:
        code = symbol.zfill(5)
        df = ak.stock_hk_spot_em()
        row = df[df['代码'] == code]
        if not row.empty:
            return float(row.iloc[0]['最新价'])
    except Exception as e:
        logger.debug(f"AKShare HK stock error for {symbol}: {e}")
    return None


def akshare_fetch_us_stock(symbol: str) -> Optional[float]:
    """Fetch US stock price using AKShare."""
    if not AKSHARE_AVAILABLE:
        return None
    try:
        df = ak.stock_us_spot_em()
        symbol_upper = symbol.upper()
        row = df[df['代码'] == symbol_upper]
        if row.empty:
            row = df[df['代码'].str.contains(symbol_upper, case=False, na=False)]
        if not row.empty:
            return float(row.iloc[0]['最新价'])
    except Exception as e:
        logger.debug(f"AKShare US stock error for {symbol}: {e}")
    return None


def akshare_fetch_gold() -> Optional[float]:
    """Fetch gold price using AKShare."""
    if not AKSHARE_AVAILABLE:
        return None
    try:
        df = ak.spot_golden_benchmark_sge()
        if not df.empty:
            return float(df.iloc[-1]['价格'])
    except Exception as e:
        logger.debug(f"AKShare gold error: {e}")
    return None


# ============================================================================
# Yahoo Finance Service (Backup 1)
# ============================================================================

def yahoo_fetch_stock(symbol: str, currency: str) -> Optional[float]:
    """Fetch stock price using Yahoo Finance."""
    if not YFINANCE_AVAILABLE:
        return None
    try:
        # Convert symbol format for Yahoo Finance
        yahoo_symbol = symbol.upper()
        
        if currency == 'CNY':
            # A-shares: 600xxx -> 600xxx.SS, 000xxx -> 000xxx.SZ
            code = yahoo_symbol
            if yahoo_symbol.startswith('SH'):
                code = yahoo_symbol[2:]
            elif yahoo_symbol.startswith('SZ'):
                code = yahoo_symbol[2:]
            
            if code.startswith('6'):
                yahoo_symbol = f"{code}.SS"
            else:
                yahoo_symbol = f"{code}.SZ"
        elif currency == 'HKD':
            # HK stocks: add .HK suffix
            code = symbol.zfill(4)
            yahoo_symbol = f"{code}.HK"
        # USD stocks use symbol as-is
        
        ticker = yf.Ticker(yahoo_symbol)
        hist = ticker.history(period="1d")
        if not hist.empty:
            return float(hist['Close'].iloc[-1])
    except Exception as e:
        logger.debug(f"Yahoo Finance error for {symbol}: {e}")
    return None


def yahoo_fetch_gold() -> Optional[float]:
    """Fetch gold price using Yahoo Finance (per oz, need to convert to per gram)."""
    if not YFINANCE_AVAILABLE:
        return None
    try:
        ticker = yf.Ticker("GC=F")  # Gold futures
        hist = ticker.history(period="1d")
        if not hist.empty:
            price_per_oz = float(hist['Close'].iloc[-1])
            # Convert to CNY per gram (approximate)
            # 1 oz = 31.1035 grams, use approximate USD/CNY rate
            return round(price_per_oz / 31.1035 * 7.2, 2)
    except Exception as e:
        logger.debug(f"Yahoo Finance gold error: {e}")
    return None


# ============================================================================
# Sina Finance API (Backup 2)
# ============================================================================

def sina_fetch_a_share(symbol: str) -> Optional[float]:
    """Fetch A-share price using Sina Finance API."""
    try:
        code = symbol.upper()
        if code.startswith('SH') or code.startswith('SZ'):
            prefix = code[:2].lower()
            code = code[2:]
        else:
            # Determine prefix based on code
            prefix = 'sh' if code.startswith('6') else 'sz'
        
        url = f"http://hq.sinajs.cn/list={prefix}{code}"
        req = urllib.request.Request(url)
        req.add_header('Referer', 'http://finance.sina.com.cn')
        
        with urllib.request.urlopen(req, timeout=10) as response:
            data = response.read().decode('gbk')
            # Parse: var hq_str_sh600000="name,open,prev_close,current,high,low,...";
            if '="' in data:
                parts = data.split('="')[1].split(',')
                if len(parts) > 3 and parts[3]:
                    return float(parts[3])
    except Exception as e:
        logger.debug(f"Sina Finance error for {symbol}: {e}")
    return None


def sina_fetch_hk_stock(symbol: str) -> Optional[float]:
    """Fetch HK stock price using Sina Finance API."""
    try:
        code = symbol.zfill(5)
        url = f"http://hq.sinajs.cn/list=hk{code}"
        req = urllib.request.Request(url)
        req.add_header('Referer', 'http://finance.sina.com.cn')
        
        with urllib.request.urlopen(req, timeout=10) as response:
            data = response.read().decode('gbk')
            if '="' in data:
                parts = data.split('="')[1].split(',')
                if len(parts) > 6 and parts[6]:
                    return float(parts[6])
    except Exception as e:
        logger.debug(f"Sina Finance HK error for {symbol}: {e}")
    return None


def sina_fetch_us_stock(symbol: str) -> Optional[float]:
    """Fetch US stock price using Sina Finance API."""
    try:
        url = f"http://hq.sinajs.cn/list=gb_{symbol.lower()}"
        req = urllib.request.Request(url)
        req.add_header('Referer', 'http://finance.sina.com.cn')
        
        with urllib.request.urlopen(req, timeout=10) as response:
            data = response.read().decode('gbk')
            if '="' in data:
                parts = data.split('="')[1].split(',')
                if len(parts) > 1 and parts[1]:
                    return float(parts[1])
    except Exception as e:
        logger.debug(f"Sina Finance US error for {symbol}: {e}")
    return None


# ============================================================================
# Tencent Finance API (Backup 3)
# ============================================================================

def tencent_fetch_a_share(symbol: str) -> Optional[float]:
    """Fetch A-share price using Tencent Finance API."""
    try:
        code = symbol.upper()
        if code.startswith('SH') or code.startswith('SZ'):
            prefix = code[:2].lower()
            code = code[2:]
        else:
            prefix = 'sh' if code.startswith('6') else 'sz'
        
        url = f"http://qt.gtimg.cn/q={prefix}{code}"
        
        with urllib.request.urlopen(url, timeout=10) as response:
            data = response.read().decode('gbk')
            # Parse: v_sh600000="1~name~600000~current~...";
            if '~' in data:
                parts = data.split('~')
                if len(parts) > 3 and parts[3]:
                    return float(parts[3])
    except Exception as e:
        logger.debug(f"Tencent Finance error for {symbol}: {e}")
    return None


def tencent_fetch_hk_stock(symbol: str) -> Optional[float]:
    """Fetch HK stock price using Tencent Finance API."""
    try:
        code = symbol.zfill(5)
        url = f"http://qt.gtimg.cn/q=hk{code}"
        
        with urllib.request.urlopen(url, timeout=10) as response:
            data = response.read().decode('gbk')
            if '~' in data:
                parts = data.split('~')
                if len(parts) > 3 and parts[3]:
                    return float(parts[3])
    except Exception as e:
        logger.debug(f"Tencent Finance HK error for {symbol}: {e}")
    return None


def tencent_fetch_us_stock(symbol: str) -> Optional[float]:
    """Fetch US stock price using Tencent Finance API."""
    try:
        url = f"http://qt.gtimg.cn/q=us{symbol.upper()}"
        
        with urllib.request.urlopen(url, timeout=10) as response:
            data = response.read().decode('gbk')
            if '~' in data:
                parts = data.split('~')
                if len(parts) > 3 and parts[3]:
                    return float(parts[3])
    except Exception as e:
        logger.debug(f"Tencent Finance US error for {symbol}: {e}")
    return None


# ============================================================================
# Main Fetch Function with Fallback
# ============================================================================

def fetch_price(symbol: str, currency: str, asset_type: str = 'stock') -> Tuple[Optional[float], str]:
    """
    Fetch the latest price for a symbol with multi-service fallback.
    
    Tries multiple data sources in order:
    1. AKShare (primary)
    2. Yahoo Finance
    3. Sina Finance API
    4. Tencent Finance API
    
    Returns:
        Tuple of (price, message)
        - price: The latest price, or None if all services failed
        - message: Success or error message
    """
    symbol_type = detect_symbol_type(symbol, currency)
    logger.info(f"Fetching price for {symbol} {currency} (type: {symbol_type})")
    
    if symbol_type == 'bond':
        return None, "债券价格暂不支持自动获取"
    
    if symbol_type == 'cash':
        return 1.0, "现金价格固定为 1.0"
    
    if symbol_type == 'unknown':
        return None, f"无法识别标的类型: {symbol}"
    
    # Define fetch functions for each symbol type and service
    fetch_attempts: List[Tuple[str, Callable]] = []
    
    if symbol_type == 'a_share':
        fetch_attempts = [
            ("AKShare", lambda: akshare_fetch_a_share(symbol)),
            ("Yahoo Finance", lambda: yahoo_fetch_stock(symbol, currency)),
            ("Sina Finance", lambda: sina_fetch_a_share(symbol)),
            ("Tencent Finance", lambda: tencent_fetch_a_share(symbol)),
        ]
    elif symbol_type == 'hk_stock':
        fetch_attempts = [
            ("AKShare", lambda: akshare_fetch_hk_stock(symbol)),
            ("Yahoo Finance", lambda: yahoo_fetch_stock(symbol, currency)),
            ("Sina Finance", lambda: sina_fetch_hk_stock(symbol)),
            ("Tencent Finance", lambda: tencent_fetch_hk_stock(symbol)),
        ]
    elif symbol_type == 'us_stock':
        fetch_attempts = [
            ("AKShare", lambda: akshare_fetch_us_stock(symbol)),
            ("Yahoo Finance", lambda: yahoo_fetch_stock(symbol, currency)),
            ("Sina Finance", lambda: sina_fetch_us_stock(symbol)),
            ("Tencent Finance", lambda: tencent_fetch_us_stock(symbol)),
        ]
    elif symbol_type == 'gold':
        fetch_attempts = [
            ("AKShare", akshare_fetch_gold),
            ("Yahoo Finance", yahoo_fetch_gold),
        ]
    
    # Try each service in order
    errors = []
    for service_name, fetch_func in fetch_attempts:
        try:
            logger.info(f"Trying {service_name} for {symbol}...")
            price = fetch_func()
            if price is not None:
                logger.info(f"Successfully fetched from {service_name}: {price}")
                return price, f"价格获取成功 (来源: {service_name})"
            else:
                errors.append(f"{service_name}: 未获取到数据")
        except Exception as e:
            error_msg = f"{service_name}: {str(e)}"
            errors.append(error_msg)
            logger.warning(f"Service failed - {error_msg}")
    
    # All services failed
    error_summary = "; ".join(errors) if errors else "所有数据源均不可用"
    logger.error(f"All price services failed for {symbol}: {error_summary}")
    return None, f"价格获取失败: {error_summary}"



