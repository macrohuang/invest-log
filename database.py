"""
Transaction Log Database Module

Provides SQLite database operations for stock/ETF transaction logging.
"""

import sqlite3
from datetime import datetime, date
from typing import Optional
from pathlib import Path

from config import DB_PATH


def get_connection(db_path: str = DB_PATH) -> sqlite3.Connection:
    """Get database connection with row factory enabled."""
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    return conn


def init_database(db_path: str = DB_PATH) -> None:
    """Initialize database with required tables and indexes."""
    conn = get_connection(db_path)
    cursor = conn.cursor()

    # Create transactions table
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS transactions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            transaction_date DATE NOT NULL,
            transaction_time TIME,
            symbol TEXT NOT NULL,
            transaction_type TEXT NOT NULL CHECK(transaction_type IN ('BUY', 'SELL', 'DIVIDEND', 'SPLIT', 'TRANSFER_IN', 'TRANSFER_OUT', 'ADJUST', 'INCOME')),
            asset_type TEXT DEFAULT 'stock' CHECK(asset_type IN ('stock', 'bond', 'metal', 'cash')),
            quantity REAL NOT NULL,
            price REAL NOT NULL,
            total_amount REAL NOT NULL,
            commission REAL DEFAULT 0,
            currency TEXT DEFAULT 'CNY' CHECK(currency IN ('CNY', 'USD', 'HKD')),
            account_id TEXT NOT NULL,
            account_name TEXT,
            notes TEXT,
            tags TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME
        )
    """)
    
    # Add asset_type column if not exists (for existing databases)
    try:
        cursor.execute("ALTER TABLE transactions ADD COLUMN asset_type TEXT DEFAULT 'stock'")
    except sqlite3.OperationalError:
        pass  # Column already exists

    # Create accounts table
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS accounts (
            account_id TEXT PRIMARY KEY,
            account_name TEXT NOT NULL,
            broker TEXT,
            account_type TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    """)

    # Create symbols table
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS symbols (
            symbol TEXT PRIMARY KEY,
            name TEXT,
            asset_type TEXT,
            sector TEXT,
            exchange TEXT
        )
    """)

    # Create allocation settings table
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS allocation_settings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            currency TEXT NOT NULL CHECK(currency IN ('CNY', 'USD', 'HKD')),
            asset_type TEXT NOT NULL,
            min_percent REAL DEFAULT 0,
            max_percent REAL DEFAULT 100,
            UNIQUE(currency, asset_type)
        )
    """)

    # Create asset types table (for dynamic asset type management)
    # Asset types are independent of currency - one type can be used with any currency
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS asset_types (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            code TEXT NOT NULL UNIQUE,
            label TEXT NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    """)

    # Initialize default asset types if table is empty
    cursor.execute("SELECT COUNT(*) FROM asset_types")
    if cursor.fetchone()[0] == 0:
        default_types = [
            ('stock', '股票'),
            ('bond', '债券'),
            ('metal', '贵金属'),
            ('cash', '现金'),
        ]
        cursor.executemany(
            "INSERT INTO asset_types (code, label) VALUES (?, ?)",
            default_types
        )

    # Create operation logs table
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS operation_logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            operation_type TEXT NOT NULL,
            symbol TEXT,
            currency TEXT,
            details TEXT,
            old_value REAL,
            new_value REAL,
            price_fetched REAL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    """)

    # Create latest prices table for tracking current market prices
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS latest_prices (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            symbol TEXT NOT NULL,
            currency TEXT NOT NULL,
            price REAL NOT NULL,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            UNIQUE(symbol, currency)
        )
    """)

    # Create indexes
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_symbol ON transactions(symbol)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_date ON transactions(transaction_date)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_account ON transactions(account_id)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_type ON transactions(transaction_type)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_currency ON transactions(currency)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_asset_type ON transactions(asset_type)")

    conn.commit()
    conn.close()


# ============================================================================
# Transaction CRUD Operations
# ============================================================================

def add_transaction(
    transaction_date: date,
    symbol: str,
    transaction_type: str,
    quantity: float,
    price: float,
    account_id: str,
    asset_type: str = "stock",
    transaction_time: Optional[str] = None,
    commission: float = 0,
    currency: str = "CNY",
    account_name: Optional[str] = None,
    notes: Optional[str] = None,
    tags: Optional[str] = None,
    total_amount_override: Optional[float] = None,
    link_cash: bool = False,
    db_path: str = DB_PATH
) -> int:
    """Add a new transaction and return its ID."""
    if transaction_type == 'INCOME':
        symbol = 'CASH'
        asset_type = 'cash'
        price = 1.0

    total_amount = total_amount_override if total_amount_override is not None else quantity * price
    conn = get_connection(db_path)
    cursor = conn.cursor()

    cursor.execute("""
        INSERT INTO transactions (
            transaction_date, transaction_time, symbol, transaction_type,
            asset_type, quantity, price, total_amount, commission, currency,
            account_id, account_name, notes, tags
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    """, (
        transaction_date, transaction_time, symbol.upper(), transaction_type,
        asset_type, quantity, price, total_amount, commission, currency,
        account_id, account_name, notes, tags
    ))

    transaction_id = cursor.lastrowid
    conn.commit()
    conn.close()

    # Cash linking logic
    if link_cash and transaction_type in ('BUY', 'SELL') and symbol.upper() != 'CASH':
        cash_transaction_type = 'SELL' if transaction_type == 'BUY' else 'BUY'
        # For BUY, we spend cash (SELL CASH). Amount spent = total_amount + commission.
        # For SELL, we receive cash (BUY CASH). Amount received = total_amount - commission.
        cash_amount = total_amount + commission if transaction_type == 'BUY' else total_amount - commission
        
        add_transaction(
            transaction_date=transaction_date,
            symbol='CASH',
            transaction_type=cash_transaction_type,
            quantity=cash_amount,
            price=1.0,
            account_id=account_id,
            asset_type='cash',
            transaction_time=transaction_time,
            currency=currency,
            account_name=account_name,
            notes=f"Linked to {transaction_type} {symbol.upper()}",
            link_cash=False,
            db_path=db_path
        )

    return transaction_id


def get_transaction(transaction_id: int, db_path: str = DB_PATH) -> Optional[dict]:
    """Get a transaction by ID."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT * FROM transactions WHERE id = ?", (transaction_id,))
    row = cursor.fetchone()
    conn.close()
    return dict(row) if row else None


def update_transaction(
    transaction_id: int,
    db_path: str = DB_PATH,
    **kwargs
) -> bool:
    """Update a transaction. Pass fields to update as keyword arguments."""
    if not kwargs:
        return False

    allowed_fields = {
        'transaction_date', 'transaction_time', 'symbol', 'transaction_type',
        'asset_type', 'quantity', 'price', 'commission', 'currency', 'account_id',
        'account_name', 'notes', 'tags'
    }

    updates = {k: v for k, v in kwargs.items() if k in allowed_fields}
    if not updates:
        return False

    # Recalculate total_amount if quantity or price changed
    if 'quantity' in updates or 'price' in updates:
        existing = get_transaction(transaction_id, db_path)
        if existing:
            quantity = updates.get('quantity', existing['quantity'])
            price = updates.get('price', existing['price'])
            updates['total_amount'] = quantity * price

    updates['updated_at'] = datetime.now().isoformat()

    set_clause = ", ".join(f"{k} = ?" for k in updates.keys())
    values = list(updates.values()) + [transaction_id]

    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute(f"UPDATE transactions SET {set_clause} WHERE id = ?", values)
    affected = cursor.rowcount
    conn.commit()
    conn.close()
    return affected > 0


def delete_transaction(transaction_id: int, db_path: str = DB_PATH) -> bool:
    """Delete a transaction by ID."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("DELETE FROM transactions WHERE id = ?", (transaction_id,))
    affected = cursor.rowcount
    conn.commit()
    conn.close()
    return affected > 0


# ============================================================================
# Query Functions
# ============================================================================

def get_transactions(
    symbol: Optional[str] = None,
    account_id: Optional[str] = None,
    transaction_type: Optional[str] = None,
    currency: Optional[str] = None,
    year: Optional[int] = None,
    start_date: Optional[date] = None,
    end_date: Optional[date] = None,
    limit: int = 100,
    offset: int = 0,
    db_path: str = DB_PATH
) -> list[dict]:
    """Query transactions with optional filters."""
    conn = get_connection(db_path)
    cursor = conn.cursor()

    query = "SELECT * FROM transactions WHERE 1=1"
    params = []

    if symbol:
        query += " AND symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND account_id = ?"
        params.append(account_id)
    if transaction_type:
        query += " AND transaction_type = ?"
        params.append(transaction_type)
    if currency:
        query += " AND currency = ?"
        params.append(currency)
    if year:
        query += " AND strftime('%Y', transaction_date) = ?"
        params.append(str(year))
    if start_date:
        query += " AND transaction_date >= ?"
        params.append(start_date)
    if end_date:
        query += " AND transaction_date <= ?"
        params.append(end_date)

    query += " ORDER BY transaction_date DESC, id DESC LIMIT ? OFFSET ?"
    params.extend([limit, offset])

    cursor.execute(query, params)
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


def get_holdings(account_id: Optional[str] = None, db_path: str = DB_PATH) -> list[dict]:
    """Calculate current holdings (total shares per symbol, grouped by currency and asset_type)."""
    conn = get_connection(db_path)
    cursor = conn.cursor()

    query = """
        SELECT 
            symbol,
            account_id,
            currency,
            asset_type,
            SUM(CASE 
                WHEN transaction_type IN ('BUY', 'TRANSFER_IN', 'INCOME') THEN quantity
                WHEN transaction_type IN ('SELL', 'TRANSFER_OUT') THEN -quantity
                WHEN transaction_type IN ('SPLIT', 'ADJUST') THEN quantity
                ELSE 0
            END) as total_shares,
            SUM(CASE 
                WHEN transaction_type IN ('BUY', 'INCOME') THEN total_amount + commission
                WHEN transaction_type = 'SELL' THEN -(total_amount - commission)
                WHEN transaction_type = 'ADJUST' THEN total_amount
                ELSE 0
            END) as total_cost
        FROM transactions
    """
    params = []

    if account_id:
        query += " WHERE account_id = ?"
        params.append(account_id)

    query += " GROUP BY symbol, account_id, currency, asset_type HAVING total_shares > 0 OR total_cost != 0"

    cursor.execute(query, params)
    rows = cursor.fetchall()
    conn.close()

    holdings = []
    for row in rows:
        row_dict = dict(row)
        if row_dict['total_shares'] > 0:
            row_dict['avg_cost'] = row_dict['total_cost'] / row_dict['total_shares']
        else:
            row_dict['avg_cost'] = 0
        holdings.append(row_dict)

    return holdings


def get_holdings_by_symbol(db_path: str = DB_PATH) -> dict:
    """Get holdings grouped by currency, then by symbol with percentages and P&L."""
    holdings = get_holdings(db_path=db_path)
    latest_prices = get_all_latest_prices(db_path=db_path)
    asset_type_labels = get_asset_type_labels(db_path=db_path)
    
    # Group by currency
    by_currency = {}
    for h in holdings:
        curr = h.get('currency', 'CNY')
        if curr not in by_currency:
            by_currency[curr] = {'total_cost': 0, 'total_market_value': 0, 'symbols': []}
        by_currency[curr]['total_cost'] += h['total_cost']
        by_currency[curr]['symbols'].append(h)
    
    # Calculate percentages and P&L
    result = {}
    for curr, data in by_currency.items():
        symbols_data = []
        total_market_value = 0
        
        for h in sorted(data['symbols'], key=lambda x: x['total_cost'], reverse=True):
            symbol = h['symbol']
            shares = h['total_shares']
            avg_cost = h['avg_cost']
            cost_basis = h['total_cost']
            
            # Get latest price
            price_info = latest_prices.get((symbol, curr))
            latest_price = price_info['price'] if price_info else None
            price_updated_at = price_info['updated_at'] if price_info else None
            
            # Calculate market value and P&L
            if latest_price is not None and shares > 0:
                market_value = latest_price * shares
                unrealized_pnl = market_value - cost_basis
                pnl_percent = (unrealized_pnl / cost_basis * 100) if cost_basis > 0 else 0
            else:
                market_value = cost_basis  # Use cost if no latest price
                unrealized_pnl = None
                pnl_percent = None
            
            total_market_value += market_value
            
            symbols_data.append({
                'symbol': symbol,
                'asset_type': h.get('asset_type', 'stock'),
                'asset_type_label': asset_type_labels.get(h.get('asset_type', 'stock'), '股票'),
                'account_id': h['account_id'],
                'total_shares': shares,
                'avg_cost': avg_cost,
                'cost_basis': cost_basis,
                'latest_price': latest_price,
                'price_updated_at': price_updated_at,
                'market_value': market_value,
                'unrealized_pnl': unrealized_pnl,
                'pnl_percent': round(pnl_percent, 2) if pnl_percent is not None else None
            })
        
        # Calculate percent based on market value
        for s in symbols_data:
            s['percent'] = round((s['market_value'] / total_market_value * 100) if total_market_value > 0 else 0, 2)
        
        result[curr] = {
            'total_cost': data['total_cost'],
            'total_market_value': total_market_value,
            'total_pnl': total_market_value - data['total_cost'],
            'symbols': symbols_data
        }
    
    return result


def adjust_asset_value(
    symbol: str,
    new_value: float,
    currency: str,
    account_id: str,
    asset_type: str = "stock",
    notes: Optional[str] = None,
    db_path: str = DB_PATH
) -> int:
    """Adjust asset value by creating an ADJUST transaction.
    
    The adjustment calculates the difference between current value and new value,
    then creates a transaction to record this change.
    """
    # Get current holding for this symbol
    holdings = get_holdings(db_path=db_path)
    current_value = 0
    current_shares = 0
    
    for h in holdings:
        if (h['symbol'] == symbol.upper() and 
            h['currency'] == currency and 
            h['account_id'] == account_id):
            current_value = h['total_cost']
            current_shares = h['total_shares']
            break
    
    # Calculate adjustment amount
    adjustment = new_value - current_value
    
    # Create adjustment transaction
    # quantity=0 means no share change, just value change
    # price stores the adjustment amount for reference
    # total_amount_override ensures the adjustment is recorded correctly
    return add_transaction(
        transaction_date=date.today(),
        symbol=symbol,
        transaction_type='ADJUST',
        quantity=0,
        price=adjustment,  # Store adjustment as price for display
        account_id=account_id,
        asset_type=asset_type,
        currency=currency,
        notes=notes or f"价值调整: {current_value:.2f} -> {new_value:.2f}",
        total_amount_override=adjustment,  # This ensures the value change is recorded
        db_path=db_path
    )


def get_realized_gains(
    symbol: Optional[str] = None,
    account_id: Optional[str] = None,
    start_date: Optional[date] = None,
    end_date: Optional[date] = None,
    db_path: str = DB_PATH
) -> list[dict]:
    """Get sell transactions for realized gain/loss analysis."""
    conn = get_connection(db_path)
    cursor = conn.cursor()

    query = """
        SELECT 
            transaction_date,
            symbol,
            account_id,
            quantity,
            price,
            total_amount,
            commission
        FROM transactions
        WHERE transaction_type = 'SELL'
    """
    params = []

    if symbol:
        query += " AND symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND account_id = ?"
        params.append(account_id)
    if start_date:
        query += " AND transaction_date >= ?"
        params.append(start_date)
    if end_date:
        query += " AND transaction_date <= ?"
        params.append(end_date)

    query += " ORDER BY transaction_date"

    cursor.execute(query, params)
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


def get_dividends(
    symbol: Optional[str] = None,
    account_id: Optional[str] = None,
    start_date: Optional[date] = None,
    end_date: Optional[date] = None,
    db_path: str = DB_PATH
) -> list[dict]:
    """Get dividend transactions."""
    conn = get_connection(db_path)
    cursor = conn.cursor()

    query = "SELECT * FROM transactions WHERE transaction_type = 'DIVIDEND'"
    params = []

    if symbol:
        query += " AND symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND account_id = ?"
        params.append(account_id)
    if start_date:
        query += " AND transaction_date >= ?"
        params.append(start_date)
    if end_date:
        query += " AND transaction_date <= ?"
        params.append(end_date)

    query += " ORDER BY transaction_date"

    cursor.execute(query, params)
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


# ============================================================================
# Account Operations
# ============================================================================

def add_account(
    account_id: str,
    account_name: str,
    broker: Optional[str] = None,
    account_type: Optional[str] = None,
    db_path: str = DB_PATH
) -> bool:
    """Add a new account."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    try:
        cursor.execute("""
            INSERT INTO accounts (account_id, account_name, broker, account_type)
            VALUES (?, ?, ?, ?)
        """, (account_id, account_name, broker, account_type))
        conn.commit()
        return True
    except sqlite3.IntegrityError:
        return False
    finally:
        conn.close()


def get_accounts(db_path: str = DB_PATH) -> list[dict]:
    """Get all accounts."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT * FROM accounts ORDER BY account_id")
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


def check_account_in_use(account_id: str, db_path: str = DB_PATH) -> bool:
    """Check if an account is in use (has transactions)."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT COUNT(*) FROM transactions WHERE account_id = ?", (account_id,))
    count = cursor.fetchone()[0]
    conn.close()
    return count > 0


def delete_account(account_id: str, db_path: str = DB_PATH) -> tuple[bool, str]:
    """Delete an account if not in use."""
    if check_account_in_use(account_id, db_path):
        return False, "Cannot delete: transactions exist for this account"
    
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("DELETE FROM accounts WHERE account_id = ?", (account_id,))
    affected = cursor.rowcount
    conn.commit()
    conn.close()
    
    if affected > 0:
        return True, "Account deleted"
    return False, "Account not found"


# ============================================================================
# Allocation Settings Operations
# ============================================================================

CURRENCIES = ['CNY', 'USD', 'HKD']
ASSET_TYPES = ['stock', 'bond', 'metal', 'cash']
ASSET_TYPE_LABELS = {
    'stock': '股票',
    'bond': '债券',
    'metal': '贵金属',
    'cash': '现金'
}


def get_allocation_settings(currency: Optional[str] = None, db_path: str = DB_PATH) -> list[dict]:
    """Get allocation settings, optionally filtered by currency."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    
    if currency:
        cursor.execute("SELECT * FROM allocation_settings WHERE currency = ?", (currency,))
    else:
        cursor.execute("SELECT * FROM allocation_settings ORDER BY currency, asset_type")
    
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


def set_allocation_setting(
    currency: str,
    asset_type: str,
    min_percent: float,
    max_percent: float,
    db_path: str = DB_PATH
) -> bool:
    """Set allocation range for a currency/asset_type combination."""
    if currency not in CURRENCIES or asset_type not in ASSET_TYPES:
        return False
    if min_percent < 0 or max_percent > 100 or min_percent > max_percent:
        return False
    
    conn = get_connection(db_path)
    cursor = conn.cursor()
    
    cursor.execute("""
        INSERT INTO allocation_settings (currency, asset_type, min_percent, max_percent)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(currency, asset_type) DO UPDATE SET
            min_percent = excluded.min_percent,
            max_percent = excluded.max_percent
    """, (currency, asset_type, min_percent, max_percent))
    
    conn.commit()
    conn.close()
    return True


def delete_allocation_setting(currency: str, asset_type: str, db_path: str = DB_PATH) -> bool:
    """Delete an allocation setting."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute(
        "DELETE FROM allocation_settings WHERE currency = ? AND asset_type = ?",
        (currency, asset_type)
    )
    affected = cursor.rowcount
    conn.commit()
    conn.close()
    return affected > 0


def get_holdings_by_currency(db_path: str = DB_PATH) -> dict:
    """Get holdings grouped by currency, then by asset_type with percentages."""
    holdings = get_holdings(db_path=db_path)
    settings = get_allocation_settings(db_path=db_path)
    
    # Build settings lookup
    settings_map = {}
    for s in settings:
        key = (s['currency'], s['asset_type'])
        settings_map[key] = {'min': s['min_percent'], 'max': s['max_percent']}
    
    # Group by currency
    by_currency = {}
    for h in holdings:
        curr = h.get('currency', 'CNY')
        if curr not in by_currency:
            by_currency[curr] = {'total': 0, 'by_asset_type': {}}
        by_currency[curr]['total'] += h['total_cost']
        
        asset = h.get('asset_type', 'stock')
        if asset not in by_currency[curr]['by_asset_type']:
            by_currency[curr]['by_asset_type'][asset] = 0
        by_currency[curr]['by_asset_type'][asset] += h['total_cost']
    
    # Calculate percentages and check warnings
    result = {}
    for curr, data in by_currency.items():
        result[curr] = {
            'total': data['total'],
            'allocations': []
        }
        for asset_type in ASSET_TYPES:
            amount = data['by_asset_type'].get(asset_type, 0)
            percent = (amount / data['total'] * 100) if data['total'] > 0 else 0
            
            # Check against settings
            setting = settings_map.get((curr, asset_type), {'min': 0, 'max': 100})
            warning = None
            if percent < setting['min']:
                warning = f"低于最小配置 {setting['min']}%"
            elif percent > setting['max']:
                warning = f"超过最大配置 {setting['max']}%"
            
            result[curr]['allocations'].append({
                'asset_type': asset_type,
                'label': ASSET_TYPE_LABELS[asset_type],
                'amount': amount,
                'percent': round(percent, 2),
                'min_percent': setting['min'],
                'max_percent': setting['max'],
                'warning': warning
            })
    
    return result


# ============================================================================
# Operation Logs
# ============================================================================

def add_operation_log(
    operation_type: str,
    symbol: Optional[str] = None,
    currency: Optional[str] = None,
    details: Optional[str] = None,
    old_value: Optional[float] = None,
    new_value: Optional[float] = None,
    price_fetched: Optional[float] = None,
    db_path: str = DB_PATH
) -> int:
    """Add an operation log entry."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    
    cursor.execute("""
        INSERT INTO operation_logs (operation_type, symbol, currency, details, old_value, new_value, price_fetched)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    """, (operation_type, symbol, currency, details, old_value, new_value, price_fetched))
    
    log_id = cursor.lastrowid
    conn.commit()
    conn.close()
    return log_id


def get_operation_logs(
    limit: int = 50,
    offset: int = 0,
    db_path: str = DB_PATH
) -> list[dict]:
    """Get operation logs."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute(
        "SELECT * FROM operation_logs ORDER BY created_at DESC LIMIT ? OFFSET ?",
        (limit, offset)
    )
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


# ============================================================================
# Latest Prices
# ============================================================================

def update_latest_price(
    symbol: str,
    currency: str,
    price: float,
    db_path: str = DB_PATH
) -> bool:
    """Update or insert latest price for a symbol."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    
    cursor.execute("""
        INSERT INTO latest_prices (symbol, currency, price, updated_at)
        VALUES (?, ?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(symbol, currency) DO UPDATE SET
            price = excluded.price,
            updated_at = CURRENT_TIMESTAMP
    """, (symbol.upper(), currency, price))
    
    conn.commit()
    conn.close()
    return True


def get_latest_price(symbol: str, currency: str, db_path: str = DB_PATH) -> Optional[dict]:
    """Get latest price for a symbol."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute(
        "SELECT * FROM latest_prices WHERE symbol = ? AND currency = ?",
        (symbol.upper(), currency)
    )
    row = cursor.fetchone()
    conn.close()
    return dict(row) if row else None


def get_all_latest_prices(db_path: str = DB_PATH) -> dict:
    """Get all latest prices as a lookup dict."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT symbol, currency, price, updated_at FROM latest_prices")
    rows = cursor.fetchall()
    conn.close()
    
    # Return as dict keyed by (symbol, currency)
    return {(row['symbol'], row['currency']): row for row in rows}


# ============================================================================
# Asset Type Management
# ============================================================================

def get_asset_types(db_path: str = DB_PATH) -> list[dict]:
    """Get all asset types from database."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT * FROM asset_types ORDER BY code")
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


def get_asset_type_labels(db_path: str = DB_PATH) -> dict:
    """Get asset type code to label mapping."""
    types = get_asset_types(db_path)
    return {t['code']: t['label'] for t in types}


def add_asset_type(
    code: str,
    label: str,
    db_path: str = DB_PATH
) -> bool:
    """Add a new asset type."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    try:
        cursor.execute(
            "INSERT INTO asset_types (code, label) VALUES (?, ?)",
            (code.lower(), label)
        )
        conn.commit()
        return True
    except sqlite3.IntegrityError:
        return False
    finally:
        conn.close()


def can_delete_asset_type(code: str, db_path: str = DB_PATH) -> tuple[bool, str]:
    """Check if an asset type can be deleted (no holdings exist)."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    
    # Check if any holdings exist with this asset type
    cursor.execute("""
        SELECT COUNT(*) FROM transactions WHERE asset_type = ?
    """, (code.lower(),))
    count = cursor.fetchone()[0]
    conn.close()
    
    if count > 0:
        return False, f"Cannot delete: {count} transactions exist with this asset type"
    return True, "Can be deleted"


def delete_asset_type(code: str, db_path: str = DB_PATH) -> tuple[bool, str]:
    """Delete an asset type if no holdings exist."""
    can_delete, message = can_delete_asset_type(code, db_path)
    if not can_delete:
        return False, message
    
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("DELETE FROM asset_types WHERE code = ?", (code.lower(),))
    affected = cursor.rowcount
    conn.commit()
    conn.close()
    
    if affected > 0:
        return True, "Asset type deleted"
    return False, "Asset type not found"


def get_transaction_count(
    symbol: Optional[str] = None,
    account_id: Optional[str] = None,
    transaction_type: Optional[str] = None,
    currency: Optional[str] = None,
    year: Optional[int] = None,
    db_path: str = DB_PATH
) -> int:
    """Get total count of transactions matching filters."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    
    query = "SELECT COUNT(*) FROM transactions WHERE 1=1"
    params = []
    
    if symbol:
        query += " AND symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND account_id = ?"
        params.append(account_id)
    if transaction_type:
        query += " AND transaction_type = ?"
        params.append(transaction_type)
    if currency:
        query += " AND currency = ?"
        params.append(currency)
    if year:
        query += " AND strftime('%Y', transaction_date) = ?"
        params.append(str(year))
    
    cursor.execute(query, params)
    count = cursor.fetchone()[0]
    conn.close()
    return count
def check_asset_type_in_use(code: str, db_path: str = DB_PATH) -> bool:
    """Check if an asset type is in use (has holdings)."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT COUNT(*) FROM transactions WHERE asset_type = ?", (code.lower(),))
    count = cursor.fetchone()[0]
    conn.close()
    return count > 0

if __name__ == "__main__":
    # Example usage
    init_database()
    print("Database initialized successfully.")
