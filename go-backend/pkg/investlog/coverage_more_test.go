package investlog

import "testing"

func TestGetTransactionAndDelete(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct", "Account")
	id, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "AAA",
		TransactionType: "BUY",
		Quantity:        1,
		Price:           10,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction: %v", err)
	}

	tx, err := core.GetTransaction(id)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if tx == nil || tx.Symbol != "AAA" {
		t.Fatalf("unexpected transaction: %+v", tx)
	}

	if missing, err := core.GetTransaction(99999); err != nil || missing != nil {
		t.Fatalf("expected nil for missing transaction, got %v %v", missing, err)
	}

	deleted, err := core.DeleteTransaction(id)
	if err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}
	if !deleted {
		t.Fatalf("expected delete true")
	}
	deleted, err = core.DeleteTransaction(id)
	if err != nil {
		t.Fatalf("DeleteTransaction second: %v", err)
	}
	if deleted {
		t.Fatalf("expected delete false for missing")
	}
}

func TestGetTransactionsFilters(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct", "Account")

	add := func(date, symbol, ttype, currency string) {
		t.Helper()
		_, err := core.AddTransaction(AddTransactionRequest{
			TransactionDate: date,
			Symbol:          symbol,
			TransactionType: ttype,
			Quantity:        1,
			Price:           10,
			Currency:        currency,
			AccountID:       "acct",
			AssetType:       "stock",
		})
		if err != nil {
			t.Fatalf("AddTransaction: %v", err)
		}
	}

	add("2024-01-01", "AAA", "BUY", "USD")
	add("2024-02-01", "AAA", "SELL", "USD")
	add("2024-03-01", "BBB", "BUY", "HKD")
	add("2025-01-01", "AAA", "BUY", "USD")

	res, err := core.GetTransactions(TransactionFilter{Symbol: "AAA"})
	if err != nil || len(res) != 3 {
		t.Fatalf("GetTransactions symbol: %v len=%d", err, len(res))
	}

	res, err = core.GetTransactions(TransactionFilter{Currency: "HKD"})
	if err != nil || len(res) != 1 {
		t.Fatalf("GetTransactions currency: %v len=%d", err, len(res))
	}

	res, err = core.GetTransactions(TransactionFilter{Year: 2024})
	if err != nil || len(res) != 3 {
		t.Fatalf("GetTransactions year: %v len=%d", err, len(res))
	}

	res, err = core.GetTransactions(TransactionFilter{StartDate: "2024-02-01", EndDate: "2024-03-01"})
	if err != nil || len(res) != 2 {
		t.Fatalf("GetTransactions date range: %v len=%d", err, len(res))
	}

	res, err = core.GetTransactions(TransactionFilter{Limit: 1, Offset: 1})
	if err != nil || len(res) != 1 {
		t.Fatalf("GetTransactions limit/offset: %v len=%d", err, len(res))
	}
}

func TestEnsureSymbolVariants(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	tx, err := core.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, _, _, err = core.ensureSymbol(tx, "AAA", stringPtr("invalid"))
	if err == nil {
		_ = tx.Rollback()
		t.Fatalf("expected error for invalid asset type")
	}
	_ = tx.Rollback()

	tx, err = core.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, _, _, err = core.ensureSymbol(tx, "AAA", stringPtr("stock"))
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("ensureSymbol stock: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	tx, err = core.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, _, assetType, err := core.ensureSymbol(tx, "AAA", stringPtr("bond"))
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("ensureSymbol bond: %v", err)
	}
	if assetType != "bond" {
		_ = tx.Rollback()
		t.Fatalf("expected updated asset type bond, got %s", assetType)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestAccountAndAllocationValidation(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	inUse, err := core.CheckAccountInUse("acct")
	if err != nil {
		t.Fatalf("CheckAccountInUse: %v", err)
	}
	if inUse {
		t.Fatalf("expected account not in use")
	}

	testAccount(t, core, "acct", "Account")
	_, err = core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "AAA",
		TransactionType: "BUY",
		Quantity:        1,
		Price:           10,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction: %v", err)
	}
	inUse, err = core.CheckAccountInUse("acct")
	if err != nil {
		t.Fatalf("CheckAccountInUse after tx: %v", err)
	}
	if !inUse {
		t.Fatalf("expected account in use")
	}

	if _, err := core.SetAllocationSetting("ABC", "stock", 0, 10); err == nil {
		t.Fatalf("expected invalid currency error")
	}
	if _, err := core.SetAllocationSetting("USD", "stock", 80, 20); err == nil {
		t.Fatalf("expected invalid percent error")
	}
	if _, err := core.SetAllocationSetting("USD", "", 0, 10); err == nil {
		t.Fatalf("expected asset_type required error")
	}
	if _, err := core.SetAllocationSetting("USD", "invalid", 0, 10); err == nil {
		t.Fatalf("expected invalid asset_type error")
	}

	ok, err := core.SetAllocationSetting("USD", "stock", 10, 20)
	if err != nil || !ok {
		t.Fatalf("SetAllocationSetting: %v", err)
	}
	deleted, err := core.DeleteAllocationSetting("USD", "stock")
	if err != nil || !deleted {
		t.Fatalf("DeleteAllocationSetting: %v", err)
	}
	deleted, err = core.DeleteAllocationSetting("USD", "stock")
	if err != nil {
		t.Fatalf("DeleteAllocationSetting second: %v", err)
	}
	if deleted {
		t.Fatalf("expected delete false for missing")
	}
}

func TestCoreCloseNil(t *testing.T) {
	var c *Core
	if err := c.Close(); err != nil {
		t.Fatalf("Close nil: %v", err)
	}
}
