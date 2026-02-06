package investlog

import (
	"database/sql"
	"fmt"
	"strings"
)

func initDatabase(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS accounts (
			account_id TEXT PRIMARY KEY,
			account_name TEXT NOT NULL,
			broker TEXT,
			account_type TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL UNIQUE,
			name TEXT,
			asset_type TEXT NOT NULL DEFAULT 'stock',
			sector TEXT,
			exchange TEXT,
			auto_update INTEGER DEFAULT 1
		)
	`); err != nil {
		return err
	}

	hasSymbolID, err := tableHasColumn(tx, "symbols", "id")
	if err != nil {
		return err
	}
	if !hasSymbolID {
		if err := migrateSymbols(tx); err != nil {
			return err
		}
	}
	if hasAutoUpdate, err := tableHasColumn(tx, "symbols", "auto_update"); err != nil {
		return err
	} else if !hasAutoUpdate {
		if err := exec(tx, "ALTER TABLE symbols ADD COLUMN auto_update INTEGER DEFAULT 1"); err != nil {
			return err
		}
	}
	if err := exec(tx, "DROP TRIGGER IF EXISTS trg_symbols_symbol_update"); err != nil {
		return err
	}

	if err := createTransactionsTable(tx); err != nil {
		return err
	}

	hasTxnSymbolID, err := tableHasColumn(tx, "transactions", "symbol_id")
	if err != nil {
		return err
	}
	hasTxnSymbol, err := tableHasColumn(tx, "transactions", "symbol")
	if err != nil {
		return err
	}
	hasTxnAssetType, err := tableHasColumn(tx, "transactions", "asset_type")
	if err != nil {
		return err
	}
	if !hasTxnSymbolID || hasTxnSymbol || hasTxnAssetType {
		if err := migrateTransactions(tx, hasTxnSymbol, hasTxnAssetType); err != nil {
			return err
		}
	} else {
		hasFK, err := transactionsHasForeignKeys(tx)
		if err != nil {
			return err
		}
		if !hasFK {
			if err := rebuildTransactionsWithForeignKeys(tx); err != nil {
				return err
			}
		}
	}

	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS allocation_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			currency TEXT NOT NULL CHECK(currency IN ('CNY', 'USD', 'HKD')),
			asset_type TEXT NOT NULL,
			min_percent REAL DEFAULT 0,
			max_percent REAL DEFAULT 100,
			UNIQUE(currency, asset_type)
		)
	`); err != nil {
		return err
	}

	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS exchange_rates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_currency TEXT NOT NULL CHECK(from_currency IN ('USD', 'HKD')),
			to_currency TEXT NOT NULL CHECK(to_currency = 'CNY'),
			rate REAL NOT NULL CHECK(rate > 0),
			source TEXT NOT NULL DEFAULT 'manual',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(from_currency, to_currency)
		)
	`); err != nil {
		return err
	}

	var exchangeRateCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM exchange_rates").Scan(&exchangeRateCount); err != nil {
		return err
	}
	if exchangeRateCount == 0 {
		defaults := []struct {
			FromCurrency string
			ToCurrency   string
			Rate         float64
		}{
			{FromCurrency: "USD", ToCurrency: "CNY", Rate: defaultUSDToCNYRate},
			{FromCurrency: "HKD", ToCurrency: "CNY", Rate: defaultHKDToCNYRate},
		}
		for _, item := range defaults {
			if _, err := tx.Exec(
				"INSERT INTO exchange_rates (from_currency, to_currency, rate, source) VALUES (?, ?, ?, ?)",
				item.FromCurrency,
				item.ToCurrency,
				item.Rate,
				"default",
			); err != nil {
				return err
			}
		}
	}

	hasAssetTypeCheck, err := allocationSettingsHasAssetTypeCheck(tx)
	if err != nil {
		return err
	}
	if hasAssetTypeCheck {
		if err := rebuildAllocationSettings(tx); err != nil {
			return err
		}
	}

	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS asset_types (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			label TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	var assetTypeCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM asset_types").Scan(&assetTypeCount); err != nil {
		return err
	}
	if assetTypeCount == 0 {
		defaults := []struct {
			Code  string
			Label string
		}{
			{"stock", "股票"},
			{"bond", "债券"},
			{"metal", "贵金属"},
			{"cash", "现金"},
		}
		for _, d := range defaults {
			if _, err := tx.Exec("INSERT INTO asset_types (code, label) VALUES (?, ?)", d.Code, d.Label); err != nil {
				return err
			}
		}
	}

	if err := exec(tx, `
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
	`); err != nil {
		return err
	}

	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS latest_prices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			currency TEXT NOT NULL,
			price REAL NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(symbol, currency)
		)
	`); err != nil {
		return err
	}

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_symbol_id ON transactions(symbol_id)",
		"CREATE INDEX IF NOT EXISTS idx_date ON transactions(transaction_date)",
		"CREATE INDEX IF NOT EXISTS idx_account ON transactions(account_id)",
		"CREATE INDEX IF NOT EXISTS idx_type ON transactions(transaction_type)",
		"CREATE INDEX IF NOT EXISTS idx_currency ON transactions(currency)",
		"CREATE INDEX IF NOT EXISTS idx_symbols_asset_type ON symbols(asset_type)",
	}
	for _, idx := range indexes {
		if err := exec(tx, idx); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func exec(tx *sql.Tx, query string) error {
	_, err := tx.Exec(query)
	return err
}

func createTransactionsTable(tx *sql.Tx) error {
	return exec(tx, `
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
			updated_at DATETIME,
			FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON UPDATE CASCADE ON DELETE RESTRICT,
			FOREIGN KEY(account_id) REFERENCES accounts(account_id) ON UPDATE CASCADE ON DELETE RESTRICT
		)
	`)
}

func tableExists(tx *sql.Tx, table string) (bool, error) {
	var name string
	err := tx.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func tableHasColumn(tx *sql.Tx, table, column string) (bool, error) {
	exists, err := tableExists(tx, table)
	if err != nil || !exists {
		return false, err
	}
	rows, err := tx.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func allocationSettingsHasAssetTypeCheck(tx *sql.Tx) (bool, error) {
	var sqlText sql.NullString
	if err := tx.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='allocation_settings'").Scan(&sqlText); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if !sqlText.Valid {
		return false, nil
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(sqlText.String), ""))
	return strings.Contains(normalized, "check(asset_type"), nil
}

func transactionsHasForeignKeys(tx *sql.Tx) (bool, error) {
	var sqlText sql.NullString
	if err := tx.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='transactions'").Scan(&sqlText); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if !sqlText.Valid {
		return false, nil
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(sqlText.String), ""))
	hasSymbolFK := strings.Contains(normalized, "foreignkey(symbol_id)referencessymbols")
	hasAccountFK := strings.Contains(normalized, "foreignkey(account_id)referencesaccounts")
	return hasSymbolFK && hasAccountFK, nil
}

func rebuildTransactionsWithForeignKeys(tx *sql.Tx) error {
	if err := exec(tx, "ALTER TABLE transactions RENAME TO transactions_old"); err != nil {
		return err
	}
	oldHasSymbol, err := tableHasColumn(tx, "transactions_old", "symbol")
	if err != nil {
		return err
	}
	oldHasAssetType, err := tableHasColumn(tx, "transactions_old", "asset_type")
	if err != nil {
		return err
	}
	return rebuildTransactionsFromOld(tx, oldHasSymbol, oldHasAssetType)
}

func rebuildTransactionsFromOld(tx *sql.Tx, oldHasSymbol bool, oldHasAssetType bool) error {
	if err := ensureAccountsFromTransactions(tx); err != nil {
		return err
	}

	if oldHasSymbol {
		if oldHasAssetType {
			if err := exec(tx, `
				INSERT OR IGNORE INTO symbols (symbol, asset_type)
				SELECT DISTINCT UPPER(symbol), COALESCE(asset_type, 'stock')
				FROM transactions_old
			`); err != nil {
				return err
			}
		} else {
			if err := exec(tx, `
				INSERT OR IGNORE INTO symbols (symbol, asset_type)
				SELECT DISTINCT UPPER(symbol), 'stock'
				FROM transactions_old
			`); err != nil {
				return err
			}
		}
	} else {
		if err := ensureMissingSymbolsForTransactions(tx); err != nil {
			return err
		}
	}

	if err := createTransactionsTable(tx); err != nil {
		return err
	}

	if oldHasSymbol {
		if err := exec(tx, `
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
		`); err != nil {
			return err
		}
	} else {
		if err := exec(tx, `
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
		`); err != nil {
			return err
		}
	}

	return exec(tx, "DROP TABLE transactions_old")
}

func ensureAccountsFromTransactions(tx *sql.Tx) error {
	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS accounts (
			account_id TEXT PRIMARY KEY,
			account_name TEXT NOT NULL,
			broker TEXT,
			account_type TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	return exec(tx, `
		INSERT OR IGNORE INTO accounts (account_id, account_name)
		SELECT DISTINCT
			account_id,
			COALESCE(NULLIF(TRIM(account_name), ''), account_id)
		FROM transactions_old
	`)
}

func ensureMissingSymbolsForTransactions(tx *sql.Tx) error {
	if err := exec(tx, `
		CREATE TABLE IF NOT EXISTS symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL UNIQUE,
			name TEXT,
			asset_type TEXT NOT NULL DEFAULT 'stock',
			sector TEXT,
			exchange TEXT,
			auto_update INTEGER DEFAULT 1
		)
	`); err != nil {
		return err
	}
	return exec(tx, `
		INSERT OR IGNORE INTO symbols (id, symbol, asset_type)
		SELECT DISTINCT
			t.symbol_id,
			printf('MISSING_%d', t.symbol_id),
			'stock'
		FROM transactions_old t
		LEFT JOIN symbols s ON s.id = t.symbol_id
		WHERE s.id IS NULL
	`)
}

func rebuildAllocationSettings(tx *sql.Tx) error {
	if err := exec(tx, "ALTER TABLE allocation_settings RENAME TO allocation_settings_old"); err != nil {
		return err
	}
	if err := exec(tx, `
		CREATE TABLE allocation_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			currency TEXT NOT NULL CHECK(currency IN ('CNY', 'USD', 'HKD')),
			asset_type TEXT NOT NULL,
			min_percent REAL DEFAULT 0,
			max_percent REAL DEFAULT 100,
			UNIQUE(currency, asset_type)
		)
	`); err != nil {
		return err
	}
	if err := exec(tx, `
		INSERT INTO allocation_settings (id, currency, asset_type, min_percent, max_percent)
		SELECT id, currency, asset_type, min_percent, max_percent
		FROM allocation_settings_old
	`); err != nil {
		return err
	}
	return exec(tx, "DROP TABLE allocation_settings_old")
}

func migrateSymbols(tx *sql.Tx) error {
	if err := exec(tx, "ALTER TABLE symbols RENAME TO symbols_old"); err != nil {
		return err
	}
	oldHasName, err := tableHasColumn(tx, "symbols_old", "name")
	if err != nil {
		return err
	}
	oldHasAssetType, err := tableHasColumn(tx, "symbols_old", "asset_type")
	if err != nil {
		return err
	}
	oldHasSector, err := tableHasColumn(tx, "symbols_old", "sector")
	if err != nil {
		return err
	}
	oldHasExchange, err := tableHasColumn(tx, "symbols_old", "exchange")
	if err != nil {
		return err
	}
	oldHasAutoUpdate, err := tableHasColumn(tx, "symbols_old", "auto_update")
	if err != nil {
		return err
	}

	if err := exec(tx, `
		CREATE TABLE symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL UNIQUE,
			name TEXT,
			asset_type TEXT NOT NULL DEFAULT 'stock',
			sector TEXT,
			exchange TEXT,
			auto_update INTEGER DEFAULT 1
		)
	`); err != nil {
		return err
	}

	selectName := "NULL"
	if oldHasName {
		selectName = "name"
	}
	selectAssetType := "'stock'"
	if oldHasAssetType {
		selectAssetType = "COALESCE(asset_type, 'stock')"
	}
	selectSector := "NULL"
	if oldHasSector {
		selectSector = "sector"
	}
	selectExchange := "NULL"
	if oldHasExchange {
		selectExchange = "exchange"
	}
	selectAutoUpdate := "1"
	if oldHasAutoUpdate {
		selectAutoUpdate = "auto_update"
	}

	insert := fmt.Sprintf(`
		INSERT INTO symbols (symbol, name, asset_type, sector, exchange, auto_update)
		SELECT UPPER(symbol), %s, %s, %s, %s, %s
		FROM symbols_old
	`, selectName, selectAssetType, selectSector, selectExchange, selectAutoUpdate)
	if err := exec(tx, insert); err != nil {
		return err
	}
	return exec(tx, "DROP TABLE symbols_old")
}

func migrateTransactions(tx *sql.Tx, oldHasSymbol bool, oldHasAssetType bool) error {
	if err := exec(tx, "ALTER TABLE transactions RENAME TO transactions_old"); err != nil {
		return err
	}

	return rebuildTransactionsFromOld(tx, oldHasSymbol, oldHasAssetType)
}
