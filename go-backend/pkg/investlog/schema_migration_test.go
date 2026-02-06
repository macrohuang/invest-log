package investlog

import (
	"database/sql"
	"testing"
)

func openTestTx(t *testing.T) (*sql.DB, *sql.Tx) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		t.Fatalf("begin tx: %v", err)
	}
	return db, tx
}

func TestTableHelpers(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, "CREATE TABLE foo (id INTEGER, name TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	exists, err := tableExists(tx, "foo")
	if err != nil || !exists {
		t.Fatalf("tableExists foo: %v %v", exists, err)
	}
	exists, err = tableExists(tx, "missing")
	if err != nil || exists {
		t.Fatalf("tableExists missing: %v %v", exists, err)
	}

	has, err := tableHasColumn(tx, "foo", "id")
	if err != nil || !has {
		t.Fatalf("tableHasColumn id: %v %v", has, err)
	}
	has, err = tableHasColumn(tx, "foo", "nope")
	if err != nil || has {
		t.Fatalf("tableHasColumn nope: %v %v", has, err)
	}
}

func TestAllocationSettingsRebuild(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, `
		CREATE TABLE allocation_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			currency TEXT NOT NULL,
			asset_type TEXT NOT NULL CHECK(asset_type IN ('stock')),
			min_percent REAL,
			max_percent REAL
		)
	`); err != nil {
		t.Fatalf("create allocation_settings: %v", err)
	}
	if err := exec(tx, "INSERT INTO allocation_settings (currency, asset_type, min_percent, max_percent) VALUES ('USD','stock',10,20)"); err != nil {
		t.Fatalf("insert allocation_settings: %v", err)
	}

	hasCheck, err := allocationSettingsHasAssetTypeCheck(tx)
	if err != nil || !hasCheck {
		t.Fatalf("expected asset_type check")
	}
	if err := rebuildAllocationSettings(tx); err != nil {
		t.Fatalf("rebuildAllocationSettings: %v", err)
	}

	exists, err := tableExists(tx, "allocation_settings_old")
	if err != nil {
		t.Fatalf("tableExists old: %v", err)
	}
	if exists {
		t.Fatalf("expected allocation_settings_old to be dropped")
	}

	row := tx.QueryRow("SELECT COUNT(*) FROM allocation_settings")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count allocation_settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after rebuild, got %d", count)
	}
}

func TestInitDatabaseCreatesDefaultExchangeRates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := initDatabase(db); err != nil {
		t.Fatalf("initDatabase: %v", err)
	}

	row := db.QueryRow("SELECT COUNT(*) FROM exchange_rates")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count exchange_rates: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 default exchange rates, got %d", count)
	}
}

func TestAllocationSettingsHasAssetTypeCheckMissing(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	hasCheck, err := allocationSettingsHasAssetTypeCheck(tx)
	if err != nil {
		t.Fatalf("allocationSettingsHasAssetTypeCheck: %v", err)
	}
	if hasCheck {
		t.Fatalf("expected no check when table missing")
	}
}

func TestAllocationSettingsHasAssetTypeCheckFalse(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, `
		CREATE TABLE allocation_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			currency TEXT NOT NULL,
			asset_type TEXT NOT NULL,
			min_percent REAL,
			max_percent REAL
		)
	`); err != nil {
		t.Fatalf("create allocation_settings: %v", err)
	}
	hasCheck, err := allocationSettingsHasAssetTypeCheck(tx)
	if err != nil {
		t.Fatalf("allocationSettingsHasAssetTypeCheck: %v", err)
	}
	if hasCheck {
		t.Fatalf("expected no asset_type check")
	}
}

func TestRebuildAllocationSettingsError(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := rebuildAllocationSettings(tx); err == nil {
		t.Fatalf("expected error when allocation_settings missing")
	}
}

func TestMigrateSymbols(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, "CREATE TABLE symbols (symbol TEXT, name TEXT, asset_type TEXT)"); err != nil {
		t.Fatalf("create symbols: %v", err)
	}
	if err := exec(tx, "INSERT INTO symbols (symbol, name, asset_type) VALUES ('aapl','Apple',NULL)"); err != nil {
		t.Fatalf("insert symbols: %v", err)
	}

	if err := migrateSymbols(tx); err != nil {
		t.Fatalf("migrateSymbols: %v", err)
	}

	row := tx.QueryRow("SELECT symbol, asset_type FROM symbols")
	var symbol, assetType string
	if err := row.Scan(&symbol, &assetType); err != nil {
		t.Fatalf("scan symbols: %v", err)
	}
	if symbol != "AAPL" || assetType != "stock" {
		t.Fatalf("unexpected migrated symbol: %s %s", symbol, assetType)
	}
}

func TestMigrateSymbolsMinimal(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, "CREATE TABLE symbols (symbol TEXT)"); err != nil {
		t.Fatalf("create symbols: %v", err)
	}
	if err := exec(tx, "INSERT INTO symbols (symbol) VALUES ('msft')"); err != nil {
		t.Fatalf("insert symbols: %v", err)
	}

	if err := migrateSymbols(tx); err != nil {
		t.Fatalf("migrateSymbols: %v", err)
	}
	row := tx.QueryRow("SELECT symbol, asset_type FROM symbols")
	var symbol, assetType string
	if err := row.Scan(&symbol, &assetType); err != nil {
		t.Fatalf("scan symbols: %v", err)
	}
	if symbol != "MSFT" || assetType != "stock" {
		t.Fatalf("unexpected migrated symbol: %s %s", symbol, assetType)
	}
}

func TestMigrateSymbolsError(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := migrateSymbols(tx); err == nil {
		t.Fatalf("expected error when symbols table missing")
	}
}

func TestMigrateTransactionsWithSymbol(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, `
		CREATE TABLE symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL UNIQUE,
			asset_type TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create symbols: %v", err)
	}

	if err := exec(tx, `
		CREATE TABLE transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_date DATE NOT NULL,
			transaction_time TIME,
			symbol TEXT,
			asset_type TEXT,
			transaction_type TEXT,
			quantity REAL,
			price REAL,
			total_amount REAL,
			commission REAL,
			currency TEXT,
			account_id TEXT,
			account_name TEXT,
			notes TEXT,
			tags TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`); err != nil {
		t.Fatalf("create old transactions: %v", err)
	}
	if err := exec(tx, "INSERT INTO transactions (transaction_date, symbol, asset_type, transaction_type, quantity, price, total_amount, currency, account_id) VALUES ('2024-01-01','aapl','stock','BUY',1,10,10,'USD','acct')"); err != nil {
		t.Fatalf("insert old tx: %v", err)
	}

	if err := migrateTransactions(tx, true, true); err != nil {
		t.Fatalf("migrateTransactions: %v", err)
	}

	row := tx.QueryRow("SELECT COUNT(*) FROM transactions")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count new transactions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 migrated transaction, got %d", count)
	}
}

func TestMigrateTransactionsWithSymbolNoAssetType(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, `
		CREATE TABLE symbols (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL UNIQUE,
			asset_type TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create symbols: %v", err)
	}

	if err := exec(tx, `
		CREATE TABLE transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_date DATE NOT NULL,
			transaction_time TIME,
			symbol TEXT,
			transaction_type TEXT,
			quantity REAL,
			price REAL,
			total_amount REAL,
			commission REAL,
			currency TEXT,
			account_id TEXT,
			account_name TEXT,
			notes TEXT,
			tags TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`); err != nil {
		t.Fatalf("create old transactions: %v", err)
	}
	if err := exec(tx, "INSERT INTO transactions (transaction_date, symbol, transaction_type, quantity, price, total_amount, currency, account_id) VALUES ('2024-01-01','msft','BUY',1,10,10,'USD','acct')"); err != nil {
		t.Fatalf("insert old tx: %v", err)
	}

	if err := migrateTransactions(tx, true, false); err != nil {
		t.Fatalf("migrateTransactions: %v", err)
	}

	row := tx.QueryRow("SELECT COUNT(*) FROM transactions")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count new transactions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 migrated transaction, got %d", count)
	}
}

func TestMigrateTransactionsError(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := migrateTransactions(tx, true, true); err == nil {
		t.Fatalf("expected error when transactions table missing")
	}
}

func TestMigrateTransactionsWithoutSymbol(t *testing.T) {
	db, tx := openTestTx(t)
	defer db.Close()
	defer tx.Rollback()

	if err := exec(tx, `
		CREATE TABLE transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_date DATE NOT NULL,
			transaction_time TIME,
			symbol_id INTEGER NOT NULL,
			transaction_type TEXT,
			quantity REAL,
			price REAL,
			total_amount REAL,
			commission REAL,
			currency TEXT,
			account_id TEXT,
			account_name TEXT,
			notes TEXT,
			tags TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`); err != nil {
		t.Fatalf("create old transactions: %v", err)
	}
	if err := exec(tx, "INSERT INTO transactions (transaction_date, symbol_id, transaction_type, quantity, price, total_amount, currency, account_id) VALUES ('2024-01-01',1,'BUY',1,10,10,'USD','acct')"); err != nil {
		t.Fatalf("insert old tx: %v", err)
	}

	if err := migrateTransactions(tx, false, false); err != nil {
		t.Fatalf("migrateTransactions: %v", err)
	}

	row := tx.QueryRow("SELECT COUNT(*) FROM transactions")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count new transactions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 migrated transaction, got %d", count)
	}
}
