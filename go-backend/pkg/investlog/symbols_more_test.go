package investlog

import "testing"

func TestUpdateSymbolAssetTypeVariants(t *testing.T) {
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

	updated, oldType, newType, err := core.UpdateSymbolAssetType("AAA", "stock")
	if err != nil {
		t.Fatalf("UpdateSymbolAssetType: %v", err)
	}
	if !updated || oldType != "stock" || newType != "stock" {
		t.Fatalf("expected no-op update, got %v %s %s", updated, oldType, newType)
	}

	updated, _, _, err = core.UpdateSymbolAssetType("MISSING", "stock")
	if err != nil {
		t.Fatalf("UpdateSymbolAssetType missing: %v", err)
	}
	if updated {
		t.Fatalf("expected update false for missing symbol")
	}

	if _, _, _, err := core.UpdateSymbolAssetType("AAA", "invalid"); err == nil {
		t.Fatalf("expected error for invalid asset type")
	}
}

func TestGetSymbolsAndUpdateMetadata(t *testing.T) {
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

	updated, err := core.UpdateSymbolMetadata("AAA", stringPtr("Name"), stringPtr("stock"), intPtr(1), stringPtr("Tech"), stringPtr("NYSE"))
	if err != nil || !updated {
		t.Fatalf("UpdateSymbolMetadata: %v", err)
	}

	symbols, err := core.GetSymbols()
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}
	if len(symbols) == 0 || symbols[0].Name == nil || *symbols[0].Name != "Name" {
		t.Fatalf("expected symbol metadata to be set")
	}

	updated, err = core.UpdateSymbolMetadata("AAA", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSymbolMetadata empty: %v", err)
	}
	if updated {
		t.Fatalf("expected update false when no fields provided")
	}

	if sym, err := core.GetSymbolMetadata("MISSING"); err != nil || sym != nil {
		t.Fatalf("expected nil for missing symbol metadata")
	}
}

func intPtr(v int) *int {
	return &v
}
