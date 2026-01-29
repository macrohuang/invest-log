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


def _table_has_column(cursor: sqlite3.Cursor, table: str, column: str) -> bool:
    cursor.execute(f"PRAGMA table_info({table})")
    return any(row["name"] == column for row in cursor.fetchall())


def _allocation_settings_has_asset_type_check(cursor: sqlite3.Cursor) -> bool:
    row = cursor.execute(
        "SELECT sql FROM sqlite_master WHERE type='table' AND name='allocation_settings'"
    ).fetchone()
    if not row or not row["sql"]:
        return False
    normalized = "".join(row["sql"].split()).lower()
    return "check(asset_type" in normalized


def _rebuild_allocation_settings(cursor: sqlite3.Cursor) -> None:
    cursor.execute("ALTER TABLE allocation_settings RENAME TO allocation_settings_old")
    cursor.execute("""
        CREATE TABLE allocation_settings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            currency TEXT NOT NULL CHECK(currency IN ('CNY', 'USD', 'HKD')),
            asset_type TEXT NOT NULL,
            min_percent REAL DEFAULT 0,
            max_percent REAL DEFAULT 100,
            UNIQUE(currency, asset_type)
        )
    """)
    cursor.execute("""
        INSERT INTO allocation_settings (id, currency, asset_type, min_percent, max_percent)
        SELECT id, currency, asset_type, min_percent, max_percent
        FROM allocation_settings_old
    """)
    cursor.execute("DROP TABLE allocation_settings_old")


def init_database(db_path: str = DB_PATH) -> None:
    """Initialize database with required tables and indexes."""
    conn = get_connection(db_path)
    cursor = conn.cursor()

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

    # Create symbols table (canonical symbol/asset_type source)
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS symbols (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            symbol TEXT NOT NULL UNIQUE,
            name TEXT,
            asset_type TEXT NOT NULL DEFAULT 'stock',
            sector TEXT,
            exchange TEXT,
            auto_update INTEGER DEFAULT 1
        )
    """)

    # Migrate symbols table if it lacks id
    if not _table_has_column(cursor, "symbols", "id"):
        cursor.execute("ALTER TABLE symbols RENAME TO symbols_old")
        old_has_name = _table_has_column(cursor, "symbols_old", "name")
        old_has_asset_type = _table_has_column(cursor, "symbols_old", "asset_type")
        old_has_sector = _table_has_column(cursor, "symbols_old", "sector")
        old_has_exchange = _table_has_column(cursor, "symbols_old", "exchange")
        old_has_auto_update = _table_has_column(cursor, "symbols_old", "auto_update")

        cursor.execute("""
            CREATE TABLE symbols (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                symbol TEXT NOT NULL UNIQUE,
                name TEXT,
                asset_type TEXT NOT NULL DEFAULT 'stock',
                sector TEXT,
                exchange TEXT,
                auto_update INTEGER DEFAULT 1
            )
        """)

        select_name = "name" if old_has_name else "NULL"
        select_asset_type = "COALESCE(asset_type, 'stock')" if old_has_asset_type else "'stock'"
        select_sector = "sector" if old_has_sector else "NULL"
        select_exchange = "exchange" if old_has_exchange else "NULL"
        select_auto_update = "auto_update" if old_has_auto_update else "1"

        cursor.execute(f"""
            INSERT INTO symbols (symbol, name, asset_type, sector, exchange, auto_update)
            SELECT UPPER(symbol), {select_name}, {select_asset_type}, {select_sector}, {select_exchange}, {select_auto_update}
            FROM symbols_old
        """)
        cursor.execute("DROP TABLE symbols_old")

    # Ensure auto_update column exists (legacy safety)
    if not _table_has_column(cursor, "symbols", "auto_update"):
        cursor.execute("ALTER TABLE symbols ADD COLUMN auto_update INTEGER DEFAULT 1")

    # Drop legacy trigger that referenced transactions.symbol
    cursor.execute("DROP TRIGGER IF EXISTS trg_symbols_symbol_update")

    # Create transactions table (use symbols.id)
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS transactions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            transaction_date DATE NOT NULL,
            transaction_time TIME,
            symbol_id INTEGER NOT NULL,
            transaction_type TEXT NOT NULL CHECK(transaction_type IN ('BUY', 'SELL', 'DIVIDEND', 'SPLIT', 'TRANSFER_IN', 'TRANSFER_OUT', 'ADJUST', 'INCOME')),
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

    # Migrate transactions table if schema is legacy
    has_symbol_id = _table_has_column(cursor, "transactions", "symbol_id")
    has_symbol = _table_has_column(cursor, "transactions", "symbol")
    has_asset_type = _table_has_column(cursor, "transactions", "asset_type")
    if (not has_symbol_id) or has_symbol or has_asset_type:
        cursor.execute("ALTER TABLE transactions RENAME TO transactions_old")

        old_has_symbol = _table_has_column(cursor, "transactions_old", "symbol")
        old_has_asset_type = _table_has_column(cursor, "transactions_old", "asset_type")

        if old_has_symbol:
            if old_has_asset_type:
                cursor.execute("""
                    INSERT OR IGNORE INTO symbols (symbol, asset_type)
                    SELECT DISTINCT UPPER(symbol), COALESCE(asset_type, 'stock')
                    FROM transactions_old
                """)
            else:
                cursor.execute("""
                    INSERT OR IGNORE INTO symbols (symbol, asset_type)
                    SELECT DISTINCT UPPER(symbol), 'stock'
                    FROM transactions_old
                """)

        cursor.execute("""
            CREATE TABLE transactions (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                transaction_date DATE NOT NULL,
                transaction_time TIME,
                symbol_id INTEGER NOT NULL,
                transaction_type TEXT NOT NULL CHECK(transaction_type IN ('BUY', 'SELL', 'DIVIDEND', 'SPLIT', 'TRANSFER_IN', 'TRANSFER_OUT', 'ADJUST', 'INCOME')),
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

        if old_has_symbol:
            cursor.execute("""
                INSERT INTO transactions (
                    transaction_date, transaction_time, symbol_id, transaction_type,
                    quantity, price, total_amount, commission, currency, account_id,
                    account_name, notes, tags, created_at, updated_at
                )
                SELECT
                    t.transaction_date, t.transaction_time, s.id, t.transaction_type,
                    t.quantity, t.price, t.total_amount, t.commission, t.currency, t.account_id,
                    t.account_name, t.notes, t.tags, t.created_at, t.updated_at
                FROM transactions_old t
                JOIN symbols s ON s.symbol = UPPER(t.symbol)
            """)
        else:
            cursor.execute("""
                INSERT INTO transactions (
                    transaction_date, transaction_time, symbol_id, transaction_type,
                    quantity, price, total_amount, commission, currency, account_id,
                    account_name, notes, tags, created_at, updated_at
                )
                SELECT
                    transaction_date, transaction_time, symbol_id, transaction_type,
                    quantity, price, total_amount, commission, currency, account_id,
                    account_name, notes, tags, created_at, updated_at
                FROM transactions_old
            """)

        cursor.execute("DROP TABLE transactions_old")

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
    # Remove legacy asset_type CHECK constraint if present
    if _allocation_settings_has_asset_type_check(cursor):
        _rebuild_allocation_settings(cursor)

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
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_symbol_id ON transactions(symbol_id)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_date ON transactions(transaction_date)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_account ON transactions(account_id)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_type ON transactions(transaction_type)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_currency ON transactions(currency)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_symbols_asset_type ON symbols(asset_type)")

    conn.commit()
    conn.close()


# ============================================================================
# Transaction CRUD Operations
# ============================================================================

def _asset_type_exists(cursor: sqlite3.Cursor, asset_type: str) -> bool:
    cursor.execute("SELECT 1 FROM asset_types WHERE code = ?", (asset_type.lower(),))
    return cursor.fetchone() is not None


def _ensure_symbol(cursor: sqlite3.Cursor, symbol: str, asset_type: Optional[str] = None) -> tuple[int, str, str]:
    """Ensure symbol exists in symbols table and return (symbol_id, symbol, asset_type)."""
    normalized_symbol = symbol.upper()
    normalized_asset_type = asset_type.lower() if asset_type else None

    if normalized_asset_type and not _asset_type_exists(cursor, normalized_asset_type):
        raise ValueError(f"Invalid asset_type: {normalized_asset_type}")

    cursor.execute("SELECT id, asset_type FROM symbols WHERE symbol = ?", (normalized_symbol,))
    row = cursor.fetchone()
    if row:
        current_asset_type = row["asset_type"]
        if normalized_asset_type and current_asset_type != normalized_asset_type:
            cursor.execute(
                "UPDATE symbols SET asset_type = ? WHERE id = ?",
                (normalized_asset_type, row["id"])
            )
            current_asset_type = normalized_asset_type
        return row["id"], normalized_symbol, current_asset_type

    insert_asset_type = normalized_asset_type or "stock"
    cursor.execute(
        "INSERT INTO symbols (symbol, asset_type) VALUES (?, ?)",
        (normalized_symbol, insert_asset_type)
    )
    return cursor.lastrowid, normalized_symbol, insert_asset_type


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
    symbol_id, symbol, asset_type = _ensure_symbol(cursor, symbol, asset_type)

    cursor.execute("""
        INSERT INTO transactions (
            transaction_date, transaction_time, symbol_id, transaction_type,
            quantity, price, total_amount, commission, currency,
            account_id, account_name, notes, tags
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    """, (
        transaction_date, transaction_time, symbol_id, transaction_type,
        quantity, price, total_amount, commission, currency,
        account_id, account_name, notes, tags
    ))

    transaction_id = cursor.lastrowid
    conn.commit()
    conn.close()

    # Cash linking logic
    if link_cash and transaction_type in ('BUY', 'SELL') and symbol != 'CASH':
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
    cursor.execute("""
        SELECT t.*, s.symbol AS symbol, s.name AS name, s.asset_type AS asset_type
        FROM transactions t
        JOIN symbols s ON s.id = t.symbol_id
        WHERE t.id = ?
    """, (transaction_id,))
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
        'quantity', 'price', 'commission', 'currency', 'account_id',
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

    conn = get_connection(db_path)
    cursor = conn.cursor()
    if 'symbol' in updates:
        symbol_id, _, _ = _ensure_symbol(cursor, updates['symbol'])
        updates['symbol_id'] = symbol_id
        updates.pop('symbol')

    set_clause = ", ".join(f"{k} = ?" for k in updates.keys())
    values = list(updates.values()) + [transaction_id]

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

    query = """
        SELECT t.*, s.symbol AS symbol, s.name AS name, s.asset_type AS asset_type
        FROM transactions t
        JOIN symbols s ON s.id = t.symbol_id
        WHERE 1=1
    """
    params = []

    if symbol:
        query += " AND s.symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND t.account_id = ?"
        params.append(account_id)
    if transaction_type:
        query += " AND t.transaction_type = ?"
        params.append(transaction_type)
    if currency:
        query += " AND t.currency = ?"
        params.append(currency)
    if year:
        query += " AND strftime('%Y', t.transaction_date) = ?"
        params.append(str(year))
    if start_date:
        query += " AND t.transaction_date >= ?"
        params.append(start_date)
    if end_date:
        query += " AND t.transaction_date <= ?"
        params.append(end_date)

    query += " ORDER BY t.transaction_date DESC, t.id DESC LIMIT ? OFFSET ?"
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
            s.symbol AS symbol,
            s.name AS name,
            t.account_id,
            t.currency,
            s.asset_type AS asset_type,
            SUM(CASE 
                WHEN t.transaction_type IN ('BUY', 'TRANSFER_IN', 'INCOME') THEN t.quantity
                WHEN t.transaction_type IN ('SELL', 'TRANSFER_OUT') THEN -t.quantity
                WHEN t.transaction_type IN ('SPLIT', 'ADJUST') THEN t.quantity
                ELSE 0
            END) as total_shares,
            SUM(CASE 
                WHEN t.transaction_type IN ('BUY', 'INCOME') THEN t.total_amount + t.commission
                WHEN t.transaction_type = 'SELL' THEN -(t.total_amount - t.commission)
                WHEN t.transaction_type = 'ADJUST' THEN t.total_amount
                ELSE 0
            END) as total_cost
        FROM transactions t
        JOIN symbols s ON s.id = t.symbol_id
    """
    params = []

    if account_id:
        query += " WHERE t.account_id = ?"
        params.append(account_id)

    query += " GROUP BY t.symbol_id, s.symbol, s.name, s.asset_type, t.account_id, t.currency HAVING total_shares > 0 OR total_cost != 0"

    cursor.execute(query, params)
    rows = cursor.fetchall()
    conn.close()

    holdings = []
    for row in rows:
        row_dict = dict(row)
        if row_dict.get('asset_type') == 'cash':
            # For cash, total value tracks the balance (no avg cost concept)
            row_dict['total_cost'] = row_dict['total_shares']
            row_dict['avg_cost'] = 1 if row_dict['total_shares'] != 0 else 0
        else:
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
    accounts_list = get_accounts(db_path=db_path)
    
    # Get auto_update status for symbols
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT symbol, auto_update FROM symbols")
    auto_update_map = {row['symbol']: row['auto_update'] for row in cursor.fetchall()}
    conn.close()
    
    # Build account name lookup
    account_names = {acc['account_id']: acc['account_name'] for acc in accounts_list}
    
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
            name = h.get('name')
            clean_name = name.strip() if isinstance(name, str) else ""
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
                'name': name,
                'display_name': clean_name if clean_name else symbol,
                'asset_type': h.get('asset_type', 'stock'),
                'asset_type_label': asset_type_labels.get(h.get('asset_type', 'stock'), '股票'),
                'auto_update': auto_update_map.get(symbol, 1),
                'account_id': h['account_id'],
                'account_name': account_names.get(h['account_id'], h['account_id']),
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
        
        # Group symbols by account for legend display
        by_account = {}
        for s in symbols_data:
            account_id = s['account_id']
            if account_id not in by_account:
                by_account[account_id] = {
                    'account_name': s['account_name'],
                    'symbols': []
                }
            by_account[account_id]['symbols'].append(s)
        
        result[curr] = {
            'total_cost': data['total_cost'],
            'total_market_value': total_market_value,
            'total_pnl': total_market_value - data['total_cost'],
            'symbols': symbols_data,
            'by_account': by_account
        }
    
    return result


def get_holdings_by_currency_and_account(db_path: str = DB_PATH) -> dict:
    """Get holdings grouped by currency, then by account, then by symbol with percentages."""
    holdings = get_holdings(db_path=db_path)
    latest_prices = get_all_latest_prices(db_path=db_path)
    asset_type_labels = get_asset_type_labels(db_path=db_path)
    accounts_list = get_accounts(db_path=db_path)
    
    # Build account name lookup
    account_names = {acc['account_id']: acc['account_name'] for acc in accounts_list}
    
    # Group by currency, then by account
    by_currency = {}
    for h in holdings:
        curr = h.get('currency', 'CNY')
        account_id = h.get('account_id', 'unknown')
        
        if curr not in by_currency:
            by_currency[curr] = {'total_market_value': 0, 'accounts': {}}
        
        if account_id not in by_currency[curr]['accounts']:
            by_currency[curr]['accounts'][account_id] = {
                'account_name': account_names.get(account_id, account_id),
                'total_market_value': 0,
                'symbols': []
            }
        
        by_currency[curr]['accounts'][account_id]['symbols'].append(h)
    
    # Calculate market values and percentages
    result = {}
    for curr, curr_data in by_currency.items():
        currency_total_market_value = 0
        accounts_result = {}
        
        for account_id, account_data in curr_data['accounts'].items():
            symbols_data = []
            account_total_market_value = 0
            
            for h in sorted(account_data['symbols'], key=lambda x: x['total_cost'], reverse=True):
                symbol = h['symbol']
                name = h.get('name')
                clean_name = name.strip() if isinstance(name, str) else ""
                shares = h['total_shares']
                avg_cost = h['avg_cost']
                cost_basis = h['total_cost']
                
                # Get latest price
                price_info = latest_prices.get((symbol, curr))
                latest_price = price_info['price'] if price_info else None
                
                # Calculate market value
                if latest_price is not None and shares > 0:
                    market_value = latest_price * shares
                else:
                    market_value = cost_basis
                
                account_total_market_value += market_value
                
                symbols_data.append({
                    'symbol': symbol,
                    'name': name,
                    'display_name': clean_name if clean_name else symbol,
                    'asset_type': h.get('asset_type', 'stock'),
                    'asset_type_label': asset_type_labels.get(h.get('asset_type', 'stock'), '股票'),
                    'market_value': market_value,
                    'total_shares': shares
                })
            
            # Calculate percentages within account
            for s in symbols_data:
                s['percent'] = round((s['market_value'] / account_total_market_value * 100) if account_total_market_value > 0 else 0, 2)
            
            currency_total_market_value += account_total_market_value
            
            accounts_result[account_id] = {
                'account_name': account_data['account_name'],
                'total_market_value': account_total_market_value,
                'symbols': symbols_data
            }
        
        result[curr] = {
            'total_market_value': currency_total_market_value,
            'accounts': accounts_result
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
            t.transaction_date,
            s.symbol AS symbol,
            s.name AS name,
            t.account_id,
            t.quantity,
            t.price,
            t.total_amount,
            t.commission
        FROM transactions t
        JOIN symbols s ON s.id = t.symbol_id
        WHERE t.transaction_type = 'SELL'
    """
    params = []

    if symbol:
        query += " AND s.symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND t.account_id = ?"
        params.append(account_id)
    if start_date:
        query += " AND t.transaction_date >= ?"
        params.append(start_date)
    if end_date:
        query += " AND t.transaction_date <= ?"
        params.append(end_date)

    query += " ORDER BY t.transaction_date"

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

    query = """
        SELECT t.*, s.symbol AS symbol, s.name AS name, s.asset_type AS asset_type
        FROM transactions t
        JOIN symbols s ON s.id = t.symbol_id
        WHERE t.transaction_type = 'DIVIDEND'
    """
    params = []

    if symbol:
        query += " AND s.symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND t.account_id = ?"
        params.append(account_id)
    if start_date:
        query += " AND t.transaction_date >= ?"
        params.append(start_date)
    if end_date:
        query += " AND t.transaction_date <= ?"
        params.append(end_date)

    query += " ORDER BY t.transaction_date"

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
# Default order for core asset types; dynamic asset types are appended after these.
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
    if currency not in CURRENCIES:
        return False
    if min_percent < 0 or max_percent > 100 or min_percent > max_percent:
        return False
    
    conn = get_connection(db_path)
    cursor = conn.cursor()
    normalized_asset_type = asset_type.lower()
    if not _asset_type_exists(cursor, normalized_asset_type):
        conn.close()
        return False
    
    try:
        cursor.execute("""
            INSERT INTO allocation_settings (currency, asset_type, min_percent, max_percent)
            VALUES (?, ?, ?, ?)
            ON CONFLICT(currency, asset_type) DO UPDATE SET
                min_percent = excluded.min_percent,
                max_percent = excluded.max_percent
        """, (currency, normalized_asset_type, min_percent, max_percent))
        conn.commit()
    except sqlite3.IntegrityError:
        conn.close()
        return False
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
    latest_prices = get_all_latest_prices(db_path=db_path)
    settings = get_allocation_settings(db_path=db_path)
    asset_types = get_asset_types(db_path=db_path)
    asset_type_labels = get_asset_type_labels(db_path=db_path)
    
    # Build settings lookup
    settings_map = {}
    for s in settings:
        asset_key = s['asset_type'].lower() if isinstance(s['asset_type'], str) else s['asset_type']
        key = (s['currency'], asset_key)
        settings_map[key] = {'min': s['min_percent'], 'max': s['max_percent']}
    
    # Group by currency
    by_currency = {}
    for h in holdings:
        curr = h.get('currency', 'CNY')
        if curr not in by_currency:
            by_currency[curr] = {'total': 0, 'by_asset_type': {}}
        
        # Get latest price
        symbol = h['symbol']
        shares = h['total_shares']
        cost_basis = h['total_cost']
        price_info = latest_prices.get((symbol, curr))
        latest_price = price_info['price'] if price_info else None
        
        # Calculate market value
        if latest_price is not None and shares > 0:
            market_value = latest_price * shares
        else:
            market_value = cost_basis  # Use cost if no latest price
        
        by_currency[curr]['total'] += market_value
        
        asset = h.get('asset_type') or 'stock'
        if isinstance(asset, str):
            asset = asset.lower()
        if asset not in by_currency[curr]['by_asset_type']:
            by_currency[curr]['by_asset_type'][asset] = 0
        by_currency[curr]['by_asset_type'][asset] += market_value
    
    asset_type_codes = [t['code'] for t in asset_types]
    holdings_types = {
        (h.get('asset_type', 'stock') or 'stock').lower()
        for h in holdings
        if h.get('asset_type') is None or isinstance(h.get('asset_type'), str)
    }
    missing_types = [t for t in holdings_types if t not in asset_type_codes]
    ordered_asset_types = (
        [t for t in ASSET_TYPES if t in asset_type_codes]
        + [t for t in asset_type_codes if t not in ASSET_TYPES]
        + sorted(missing_types)
    )

    # Calculate percentages and check warnings
    result = {}
    for curr, data in by_currency.items():
        result[curr] = {
            'total': data['total'],
            'allocations': []
        }
        for asset_type in ordered_asset_types:
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
                'label': asset_type_labels.get(asset_type, asset_type),
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


def update_symbol_auto_update(symbol: str, auto_update: int, db_path: str = DB_PATH) -> bool:
    """Update auto_update status for a symbol."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    symbol_id, _, _ = _ensure_symbol(cursor, symbol)
    cursor.execute("UPDATE symbols SET auto_update = ? WHERE id = ?", (auto_update, symbol_id))

    conn.commit()
    conn.close()
    return True


def get_symbol_metadata(symbol: str, db_path: str = DB_PATH) -> Optional[dict]:
    """Get metadata for a symbol."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT * FROM symbols WHERE symbol = ?", (symbol.upper(),))
    row = cursor.fetchone()
    conn.close()
    return dict(row) if row else None


def get_symbols(db_path: str = DB_PATH) -> list[dict]:
    """Get all symbols for management."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("""
        SELECT id, symbol, name, asset_type, sector, exchange, auto_update
        FROM symbols
        ORDER BY symbol
    """)
    rows = cursor.fetchall()
    conn.close()
    return [dict(row) for row in rows]


def update_symbol_metadata(
    symbol: str,
    name: Optional[str] = None,
    asset_type: Optional[str] = None,
    auto_update: Optional[int] = None,
    sector: Optional[str] = None,
    exchange: Optional[str] = None,
    db_path: str = DB_PATH
) -> bool:
    """Update symbol metadata fields."""
    updates = []
    values = []

    if name is not None:
        clean_name = name.strip()
        updates.append("name = ?")
        values.append(clean_name if clean_name else None)

    if asset_type is not None:
        normalized_asset_type = asset_type.lower()
        conn = get_connection(db_path)
        cursor = conn.cursor()
        if not _asset_type_exists(cursor, normalized_asset_type):
            conn.close()
            raise ValueError(f"Invalid asset_type: {normalized_asset_type}")
        updates.append("asset_type = ?")
        values.append(normalized_asset_type)
        conn.close()

    if auto_update is not None:
        updates.append("auto_update = ?")
        values.append(1 if int(auto_update) else 0)

    if sector is not None:
        updates.append("sector = ?")
        values.append(sector.strip() if sector.strip() else None)

    if exchange is not None:
        updates.append("exchange = ?")
        values.append(exchange.strip() if exchange.strip() else None)

    if not updates:
        return False

    conn = get_connection(db_path)
    cursor = conn.cursor()
    values.append(symbol.upper())
    cursor.execute(f"UPDATE symbols SET {', '.join(updates)} WHERE symbol = ?", values)
    affected = cursor.rowcount
    conn.commit()
    conn.close()
    return affected > 0


def update_symbol_asset_type(symbol: str, asset_type: str, db_path: str = DB_PATH) -> tuple[bool, str, str]:
    """Update asset_type for a symbol. Returns (success, old_type, new_type)."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    normalized_symbol = symbol.upper()
    normalized_asset_type = asset_type.lower()

    if not _asset_type_exists(cursor, normalized_asset_type):
        conn.close()
        raise ValueError(f"Invalid asset_type: {normalized_asset_type}")

    row = cursor.execute(
        "SELECT id, asset_type FROM symbols WHERE symbol = ?",
        (normalized_symbol,)
    ).fetchone()
    if not row:
        conn.close()
        return False, "", ""

    current = (row["asset_type"] or "").lower()
    if current == normalized_asset_type:
        conn.close()
        return True, current, current

    cursor.execute(
        "UPDATE symbols SET asset_type = ? WHERE id = ?",
        (normalized_asset_type, row["id"])
    )
    conn.commit()
    conn.close()
    return True, current, normalized_asset_type


def update_symbol_auto_update(symbol: str, auto_update: int, db_path: str = DB_PATH) -> bool:
    """Update auto_update status for a symbol."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    symbol_id, _, _ = _ensure_symbol(cursor, symbol)
    cursor.execute("UPDATE symbols SET auto_update = ? WHERE id = ?", (auto_update, symbol_id))

    conn.commit()
    conn.close()
    return True


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
    
    # Check if any symbols use this asset type
    cursor.execute("""
        SELECT COUNT(*) FROM symbols WHERE asset_type = ?
    """, (code.lower(),))
    count = cursor.fetchone()[0]
    conn.close()
    
    if count > 0:
        return False, f"Cannot delete: {count} symbols use this asset type"
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
    
    query = """
        SELECT COUNT(*)
        FROM transactions t
        JOIN symbols s ON s.id = t.symbol_id
        WHERE 1=1
    """
    params = []
    
    if symbol:
        query += " AND s.symbol = ?"
        params.append(symbol.upper())
    if account_id:
        query += " AND t.account_id = ?"
        params.append(account_id)
    if transaction_type:
        query += " AND t.transaction_type = ?"
        params.append(transaction_type)
    if currency:
        query += " AND t.currency = ?"
        params.append(currency)
    if year:
        query += " AND strftime('%Y', t.transaction_date) = ?"
        params.append(str(year))
    
    cursor.execute(query, params)
    count = cursor.fetchone()[0]
    conn.close()
    return count
def check_asset_type_in_use(code: str, db_path: str = DB_PATH) -> bool:
    """Check if an asset type is in use (has holdings)."""
    conn = get_connection(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT COUNT(*) FROM symbols WHERE asset_type = ?", (code.lower(),))
    count = cursor.fetchone()[0]
    conn.close()
    return count > 0

if __name__ == "__main__":
    # Example usage
    init_database()
    print("Database initialized successfully.")
