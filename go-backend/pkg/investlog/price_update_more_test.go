package investlog

import "testing"

func TestUpdatePriceAndUpdateAllPrices(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	result, err := core.UpdatePrice("CASH", "USD", "cash")
	if err != nil {
		t.Fatalf("UpdatePrice: %v", err)
	}
	if result.Price == nil || result.Price.InexactFloat64() != 1.0 {
		t.Fatalf("expected cash price 1.0, got %v", result.Price)
	}
	latest, err := core.GetLatestPrice("CASH", "USD")
	if err != nil {
		t.Fatalf("GetLatestPrice: %v", err)
	}
	if latest == nil || latest.Price.InexactFloat64() != 1.0 {
		t.Fatalf("expected latest price 1.0, got %v", latest)
	}

	// No holdings for CNY should error.
	if _, _, err := core.UpdateAllPrices("CNY"); err == nil {
		t.Fatalf("expected error for missing currency")
	}

	testAccount(t, core, "acct", "Account")
	_, err = core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "CASH",
		TransactionType: "TRANSFER_IN",
		Quantity:        NewAmountFromInt(100),
		Price:           NewAmountFromInt(1),
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "cash",
	})
	if err != nil {
		t.Fatalf("AddTransaction: %v", err)
	}

	updated, errors, err := core.UpdateAllPrices("USD")
	if err != nil {
		t.Fatalf("UpdateAllPrices: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 updated, got %d", updated)
	}
	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %v", errors)
	}
}
