"""
Price Fetcher Module

Fetches latest asset prices from multiple sources with fallback support.
Supports A-shares, HK stocks, US stocks, and gold.

Data Sources (in order of priority):
1. pqquotation (roundrobin) - Primary source for A-shares
2. Tencent Finance API - Backup for A-shares and HK stocks
3. Sina Finance API - Backup for A-shares
4. Eastmoney Quote API - Backup for A-shares
5. Yahoo Finance (yfinance) - Good for US/HK stocks
"""

import re
import urllib.request
import json
from typing import Optional, Tuple, List, Callable
from time import monotonic
from threading import Lock

from logger_config import logger

try:
    import yfinance as yf
    YFINANCE_AVAILABLE = True
except ImportError:
    YFINANCE_AVAILABLE = False

try:
    import pqquotation
    PQQUOTATION_AVAILABLE = True
except ImportError:
    PQQUOTATION_AVAILABLE = False

_PQQUOTATION_CLIENT = None
_PRICE_CACHE = {}
_SERVICE_STATE = {}
_CACHE_LOCK = Lock()

# Cache and circuit breaker settings
_CACHE_TTL_SECONDS = 30
_SERVICE_FAIL_THRESHOLD = 3
_SERVICE_FAIL_WINDOW_SECONDS = 60
_SERVICE_COOLDOWN_SECONDS = 120


def _get_pqquotation_client():
    """Lazy init pqquotation client (roundrobin multi-source)."""
    global _PQQUOTATION_CLIENT
    if _PQQUOTATION_CLIENT is None and PQQUOTATION_AVAILABLE:
        _PQQUOTATION_CLIENT = pqquotation.use('roundrobin')
    return _PQQUOTATION_CLIENT


def _parse_price(value: Optional[object]) -> Optional[float]:
    try:
        if value is None:
            return None
        return float(value)
    except (TypeError, ValueError):
        return None


def _cache_key(symbol: str, currency: str, asset_type: str) -> str:
    return f"{symbol.upper()}|{currency}|{asset_type}"


def _get_cached_price(symbol: str, currency: str, asset_type: str) -> Optional[Tuple[float, str]]:
    key = _cache_key(symbol, currency, asset_type)
    now = monotonic()
    with _CACHE_LOCK:
        cached = _PRICE_CACHE.get(key)
        if not cached:
            return None
        price, ts, source = cached
        if now - ts <= _CACHE_TTL_SECONDS:
            return price, source
        return None


def _set_cached_price(symbol: str, currency: str, asset_type: str, price: float, source: str) -> None:
    key = _cache_key(symbol, currency, asset_type)
    with _CACHE_LOCK:
        _PRICE_CACHE[key] = (price, monotonic(), source)


def _service_available(service_name: str) -> bool:
    now = monotonic()
    state = _SERVICE_STATE.get(service_name)
    if not state:
        return True
    cooldown_until = state.get("cooldown_until", 0)
    return now >= cooldown_until


def _record_service_failure(service_name: str) -> None:
    now = monotonic()
    state = _SERVICE_STATE.setdefault(service_name, {
        "fail_count": 0,
        "first_fail_at": now,
        "cooldown_until": 0
    })

    # Reset window if expired
    if now - state["first_fail_at"] > _SERVICE_FAIL_WINDOW_SECONDS:
        state["fail_count"] = 0
        state["first_fail_at"] = now

    state["fail_count"] += 1
    if state["fail_count"] >= _SERVICE_FAIL_THRESHOLD:
        state["cooldown_until"] = now + _SERVICE_COOLDOWN_SECONDS


def _record_service_success(service_name: str) -> None:
    if service_name in _SERVICE_STATE:
        _SERVICE_STATE.pop(service_name, None)


def detect_symbol_type(symbol: str, currency: str) -> str:
    """Detect the type of symbol based on its format and currency."""
    symbol = symbol.upper()

    def _is_a_share_stock(code: str) -> bool:
        stock_prefixes = (
            "000", "001", "002", "003",  # SZ main board + SME
            "300", "301",                # SZ ChiNext
            "600", "601", "603", "605",  # SH main board
            "688", "689"                 # STAR Market
        )
        return any(code.startswith(p) for p in stock_prefixes)

    def _is_etf_or_lof(code: str) -> bool:
        return code.startswith(("5", "15", "16"))
    
    # A-shares: starts with SH/SZ or 6-digit number
    if symbol.startswith('SH') or symbol.startswith('SZ'):
        return 'a_share'
    if currency == 'CNY' and re.match(r'^\d{6}$', symbol):
        if _is_a_share_stock(symbol) or _is_etf_or_lof(symbol):
            return 'a_share'
        return 'fund'
    
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
# pqquotation Service (Primary for A-shares)
# ============================================================================

def pqquotation_fetch_a_share(symbol: str) -> Optional[float]:
    """Fetch A-share price using pqquotation (multi-source round-robin)."""
    if not PQQUOTATION_AVAILABLE:
        return None
    try:
        code = symbol.upper()
        if code.startswith('SH') or code.startswith('SZ'):
            prefix = code[:2].lower()
            code = code[2:]
        else:
            prefix = 'sh' if code.startswith('6') else 'sz'

        client = _get_pqquotation_client()
        if client is None:
            return None

        data = client.real(code)
        if not data:
            data = client.real(prefix + code)
        if not data:
            data = client.real(f"{code}.{prefix.upper()}")

        # pqquotation may return {symbol: {now: price, ...}} or a single quote dict
        if isinstance(data, dict):
            if 'now' in data:
                return _parse_price(data.get('now'))

            key_candidates = [
                code,
                f"{prefix}{code}",
                f"{code}.{prefix.upper()}",
                f"{code}.{'SH' if prefix == 'sh' else 'SZ'}",
            ]
            for key in key_candidates:
                if key in data and isinstance(data[key], dict):
                    return _parse_price(data[key].get('now'))

            first = next(iter(data.values()), None)
            if isinstance(first, dict):
                return _parse_price(first.get('now'))
    except Exception as e:
        logger.debug(f"pqquotation A-share error for {symbol}: {e}")
    return None


# ============================================================================
# Eastmoney Quote API (A-share fallback)
# ============================================================================

def eastmoney_fetch_a_share(symbol: str) -> Optional[float]:
    """Fetch A-share price using Eastmoney quote API."""
    try:
        code = symbol.upper()
        if code.startswith('SH') or code.startswith('SZ'):
            market = 1 if code.startswith('SH') else 0
            code = code[2:]
        else:
            market = 1 if code.startswith('6') else 0

        if not re.match(r'^\d{6}$', code):
            return None

        secid = f"{market}.{code}"
        url = (
            "http://push2.eastmoney.com/api/qt/stock/get"
            f"?secid={secid}&fields=f43&ut=fa5fd1943c7b386f172d6893dbfba10b"
        )
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0",
            "Referer": "http://quote.eastmoney.com/"
        })
        with urllib.request.urlopen(req, timeout=10) as response:
            payload = response.read().decode("utf-8")
            data = json.loads(payload)
            price = _parse_price(data.get("data", {}).get("f43"))
            if price is None:
                return None
            if price > 1000:
                price = price / 100
            return price
    except Exception as e:
        logger.debug(f"Eastmoney A-share error for {symbol}: {e}")
    return None


# ============================================================================
# Eastmoney Fund API (CNY fund fallback)
# ============================================================================

def eastmoney_fetch_fund(symbol: str) -> Optional[float]:
    """Fetch fund price using Eastmoney fund API."""
    try:
        code = symbol.upper()
        if not re.match(r'^\d{6}$', code):
            return None
        url = f"http://fundgz.1234567.com.cn/js/{code}.js"
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0",
            "Referer": "http://fund.eastmoney.com/"
        })
        with urllib.request.urlopen(req, timeout=10) as response:
            payload = response.read().decode("utf-8")
            if "jsonpgz" not in payload:
                return None
            start = payload.find("(")
            end = payload.rfind(")")
            if start == -1 or end == -1 or end <= start:
                return None
            data = json.loads(payload[start + 1:end])
            if not isinstance(data, dict):
                return None
            price = _parse_price(data.get("gsz") or data.get("dwjz"))
            return price
    except Exception as e:
        logger.debug(f"Eastmoney fund error for {symbol}: {e}")
    return None


def eastmoney_fetch_fund_pingzhong(symbol: str) -> Optional[float]:
    """Fetch fund NAV using Eastmoney pingzhongdata API."""
    try:
        code = symbol.upper()
        if not re.match(r'^\d{6}$', code):
            return None
        url = f"http://fund.eastmoney.com/pingzhongdata/{code}.js"
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0",
            "Referer": "http://fund.eastmoney.com/"
        })
        with urllib.request.urlopen(req, timeout=10) as response:
            payload = response.read().decode("utf-8", errors="ignore")
            marker = "var Data_netWorthTrend ="
            start = payload.find(marker)
            if start == -1:
                return None
            bracket_start = payload.find("[", start)
            bracket_end = payload.find("];", bracket_start)
            if bracket_start == -1 or bracket_end == -1:
                return None
            raw = payload[bracket_start:bracket_end + 1]
            data = json.loads(raw)
            if not data:
                return None
            last = data[-1]
            if isinstance(last, dict):
                return _parse_price(last.get("y"))
    except Exception as e:
        logger.debug(f"Eastmoney pingzhongdata error for {symbol}: {e}")
    return None


def eastmoney_fetch_fund_lsjz(symbol: str) -> Optional[float]:
    """Fetch fund NAV using Eastmoney LSJZ API (latest net value)."""
    try:
        code = symbol.upper()
        if not re.match(r'^\d{6}$', code):
            return None
        url = (
            "http://fund.eastmoney.com/f10/F10DataApi.aspx"
            f"?type=lsjz&code={code}&page=1&per=1"
        )
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0",
            "Referer": "http://fund.eastmoney.com/"
        })
        with urllib.request.urlopen(req, timeout=10) as response:
            payload = response.read().decode("utf-8", errors="ignore")
            match = re.search(r"<td[^>]*>\d{4}-\d{2}-\d{2}</td>\s*<td[^>]*>([\d.]+)</td>", payload)
            if match:
                return _parse_price(match.group(1))
    except Exception as e:
        logger.debug(f"Eastmoney LSJZ error for {symbol}: {e}")
    return None


# ============================================================================
# Yahoo Finance Service (Backup)
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

    cached = _get_cached_price(symbol, currency, asset_type)
    if cached:
        price, source = cached
        return price, f"价格获取成功 (缓存, 来源: {source})"
    
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
            ("pqquotation", lambda: pqquotation_fetch_a_share(symbol)),
            ("Tencent Finance", lambda: tencent_fetch_a_share(symbol)),
            ("Sina Finance", lambda: sina_fetch_a_share(symbol)),
            ("Eastmoney", lambda: eastmoney_fetch_a_share(symbol)),
            ("Eastmoney Fund", lambda: eastmoney_fetch_fund(symbol)),
            ("Yahoo Finance", lambda: yahoo_fetch_stock(symbol, currency)),
        ]
    elif symbol_type == 'fund':
        fetch_attempts = [
            ("Eastmoney Fund GZ", lambda: eastmoney_fetch_fund(symbol)),
            ("Eastmoney Fund PZ", lambda: eastmoney_fetch_fund_pingzhong(symbol)),
            ("Eastmoney Fund LSJZ", lambda: eastmoney_fetch_fund_lsjz(symbol)),
            ("Eastmoney", lambda: eastmoney_fetch_a_share(symbol)),
        ]
    elif symbol_type == 'hk_stock':
        fetch_attempts = [
            ("Yahoo Finance", lambda: yahoo_fetch_stock(symbol, currency)),
            ("Sina Finance", lambda: sina_fetch_hk_stock(symbol)),
            ("Tencent Finance", lambda: tencent_fetch_hk_stock(symbol)),
        ]
    elif symbol_type == 'us_stock':
        fetch_attempts = [
            ("Yahoo Finance", lambda: yahoo_fetch_stock(symbol, currency)),
            ("Sina Finance", lambda: sina_fetch_us_stock(symbol)),
            ("Tencent Finance", lambda: tencent_fetch_us_stock(symbol)),
        ]
    elif symbol_type == 'gold':
        fetch_attempts = [
            ("Yahoo Finance", yahoo_fetch_gold),
        ]
    
    # Try each service in order
    errors = []
    for service_name, fetch_func in fetch_attempts:
        if not _service_available(service_name):
            errors.append(f"{service_name}: 熔断冷却中")
            continue
        try:
            logger.info(f"Trying {service_name} for {symbol}...")
            price = fetch_func()
            if price is not None:
                logger.info(f"Successfully fetched from {service_name}: {price}")
                _record_service_success(service_name)
                _set_cached_price(symbol, currency, asset_type, price, service_name)
                return price, f"价格获取成功 (来源: {service_name})"
            else:
                errors.append(f"{service_name}: 未获取到数据")
                _record_service_failure(service_name)
        except Exception as e:
            error_msg = f"{service_name}: {str(e)}"
            errors.append(error_msg)
            _record_service_failure(service_name)
            logger.warning(f"Service failed - {error_msg}")
    
    # All services failed
    error_summary = "; ".join(errors) if errors else "所有数据源均不可用"
    logger.error(f"All price services failed for {symbol}: {error_summary}")
    return None, f"价格获取失败: {error_summary}"
