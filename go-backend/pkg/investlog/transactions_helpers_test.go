package investlog

import "testing"

func TestNullString(t *testing.T) {
	val := "x"
	if ns := nullString(&val); !ns.Valid || ns.String != "x" {
		t.Fatalf("expected valid nullString")
	}
	if ns := nullString(nil); ns.Valid {
		t.Fatalf("expected invalid nullString for nil")
	}
}

func TestGetCurrentSharesHelper(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct", "Account")
	_, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "AAA",
		TransactionType: "BUY",
		Quantity:        5,
		Price:           10,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction: %v", err)
	}

	shares, err := core.getCurrentShares("AAA", "USD", "acct")
	if err != nil {
		t.Fatalf("getCurrentShares: %v", err)
	}
	if shares != 5 {
		t.Fatalf("expected 5 shares, got %.2f", shares)
	}

	shares, err = core.getCurrentShares("MISSING", "USD", "acct")
	if err != nil {
		t.Fatalf("getCurrentShares missing: %v", err)
	}
	if shares != 0 {
		t.Fatalf("expected 0 shares, got %.2f", shares)
	}
}

func TestInsertTransactionTxError(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	tx, err := core.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_ = tx.Rollback()

	_, err = core.insertTransactionTx(tx, AddTransactionRequest{
		TransactionDate: "2024-01-01",
		TransactionType: "BUY",
		Quantity:        1,
		Price:           1,
		Currency:        "USD",
		AccountID:       "acct",
	}, 1, 1)
	if err == nil {
		t.Fatalf("expected error from insertTransactionTx")
	}
}

func TestGetTransactionOptionalFields(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct", "Account")
	accountName := "Account"
	notes := "note"
	tags := "tag"
	timeStr := "10:30:00"
	id, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		TransactionTime: &timeStr,
		Symbol:          "AAA",
		TransactionType: "BUY",
		Quantity:        1,
		Price:           10,
		Currency:        "USD",
		AccountID:       "acct",
		AccountName:     &accountName,
		Notes:           &notes,
		Tags:            &tags,
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction: %v", err)
	}

	tr, err := core.GetTransaction(id)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if tr.TransactionTime == nil || tr.AccountName == nil || tr.Notes == nil || tr.Tags == nil {
		t.Fatalf("expected optional fields to be set")
	}
}
