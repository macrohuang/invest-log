package investlog

import "testing"

func TestGetTransactionCountFilters(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct", "Account")

	_, err := core.AddTransaction(AddTransactionRequest{
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
	_, err = core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-02",
		Symbol:          "AAA",
		TransactionType: "SELL",
		Quantity:        1,
		Price:           5,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction 2: %v", err)
	}

	count, err := core.GetTransactionCount(TransactionFilter{})
	if err != nil {
		t.Fatalf("GetTransactionCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 transactions, got %d", count)
	}

	count, err = core.GetTransactionCount(TransactionFilter{Symbol: "AAA"})
	if err != nil {
		t.Fatalf("GetTransactionCount symbol: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 transactions for AAA, got %d", count)
	}

	count, err = core.GetTransactionCount(TransactionFilter{TransactionType: "SELL"})
	if err != nil {
		t.Fatalf("GetTransactionCount type: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 sell transaction, got %d", count)
	}

	count, err = core.GetTransactionCount(TransactionFilter{Year: 2024})
	if err != nil {
		t.Fatalf("GetTransactionCount year: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 transactions in 2024, got %d", count)
	}
}
