package investlog

import "testing"

func TestAddTransactionVariants(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct", "Account")

	// INCOME should map to CASH.
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionType: "INCOME",
		Quantity:        NewAmountFromInt(100),
		Currency:        "USD",
		AccountID:       "acct",
	}); err != nil {
		t.Fatalf("AddTransaction INCOME: %v", err)
	}

	// DIVIDEND with total amount override.
	total := NewAmount(123.45)
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-02",
		Symbol:          "AAA",
		TransactionType: "DIVIDEND",
		Quantity:        Amount{},
		Price:           Amount{},
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
		TotalAmount:     &total,
	}); err != nil {
		t.Fatalf("AddTransaction DIVIDEND: %v", err)
	}

	// SPLIT with negative quantity.
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-03",
		Symbol:          "AAA",
		TransactionType: "SPLIT",
		Quantity:        NewAmount(-2),
		Price:           Amount{},
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	}); err != nil {
		t.Fatalf("AddTransaction SPLIT: %v", err)
	}

	// ADJUST with custom total amount.
	adj := NewAmount(10.0)
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-04",
		Symbol:          "AAA",
		TransactionType: "ADJUST",
		Quantity:        Amount{},
		Price:           adj,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
		TotalAmount:     &adj,
	}); err != nil {
		t.Fatalf("AddTransaction ADJUST: %v", err)
	}

	// BUY with LinkCash should create a cash transaction too.
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-05",
		Symbol:          "BBB",
		TransactionType: "BUY",
		Quantity:        NewAmountFromInt(1),
		Price:           NewAmountFromInt(10),
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
		LinkCash:        true,
	}); err != nil {
		t.Fatalf("AddTransaction BUY LinkCash: %v", err)
	}
	count, err := core.GetTransactionCount(TransactionFilter{})
	if err != nil {
		t.Fatalf("GetTransactionCount: %v", err)
	}
	if count < 2 {
		t.Fatalf("expected cash-linked transaction, got %d", count)
	}

	// TRANSFER_OUT without holdings should error.
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-06",
		Symbol:          "CCC",
		TransactionType: "TRANSFER_OUT",
		Quantity:        NewAmountFromInt(1),
		Price:           NewAmountFromInt(1),
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	}); err == nil {
		t.Fatalf("expected error for insufficient shares")
	}
}
