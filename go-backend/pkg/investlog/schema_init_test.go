package investlog

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestInitDatabaseWithLegacySchema(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "legacy.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Legacy symbols table without id/auto_update.
	if _, err := db.Exec("CREATE TABLE symbols (symbol TEXT, name TEXT)"); err != nil {
		t.Fatalf("create legacy symbols: %v", err)
	}
	if _, err := db.Exec("INSERT INTO symbols (symbol, name) VALUES ('aapl','Apple')"); err != nil {
		t.Fatalf("insert legacy symbols: %v", err)
	}

	// Legacy transactions table with symbol/asset_type.
	if _, err := db.Exec(`
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
		t.Fatalf("create legacy transactions: %v", err)
	}
	if _, err := db.Exec("INSERT INTO transactions (transaction_date, symbol, asset_type, transaction_type, quantity, price, total_amount, currency, account_id) VALUES ('2024-01-01','aapl','stock','BUY',1,10,10,'USD','acct')"); err != nil {
		t.Fatalf("insert legacy transactions: %v", err)
	}

	// Legacy allocation_settings with asset_type check.
	if _, err := db.Exec(`
		CREATE TABLE allocation_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			currency TEXT NOT NULL,
			asset_type TEXT NOT NULL CHECK(asset_type IN ('stock')),
			min_percent REAL,
			max_percent REAL
		)
	`); err != nil {
		t.Fatalf("create legacy allocation_settings: %v", err)
	}
	if _, err := db.Exec("INSERT INTO allocation_settings (currency, asset_type, min_percent, max_percent) VALUES ('USD','stock',10,20)"); err != nil {
		t.Fatalf("insert legacy allocation_settings: %v", err)
	}

	if err := initDatabase(db); err != nil {
		t.Fatalf("initDatabase: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	if hasID, err := tableHasColumn(tx, "symbols", "id"); err != nil || !hasID {
		t.Fatalf("expected symbols id column")
	}
	if hasSymbolID, err := tableHasColumn(tx, "transactions", "symbol_id"); err != nil || !hasSymbolID {
		t.Fatalf("expected transactions symbol_id column")
	}
}

func TestInitDatabaseClosedDB(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_ = db.Close()
	if err := initDatabase(db); err == nil {
		t.Fatalf("expected error on closed db")
	}
}
