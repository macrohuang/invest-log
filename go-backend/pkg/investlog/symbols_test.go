package investlog

import (
	"testing"
)

func TestGetSymbols(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Initially no symbols
	symbols, err := core.GetSymbols()
	assertNoError(t, err, "get symbols initially")
	if len(symbols) != 0 {
		t.Errorf("expected 0 symbols initially, got %d", len(symbols))
	}

	// Add transactions to create symbols
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")
	testBuyTransaction(t, core, "GOOGL", 10, 2000, "USD", "test-account")

	symbols, err = core.GetSymbols()
	assertNoError(t, err, "get symbols")
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}
}

func TestGetSymbolMetadata(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Create a symbol via transaction
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	symbol, err := core.GetSymbolMetadata("AAPL")
	assertNoError(t, err, "get symbol metadata")

	if symbol == nil {
		t.Fatal("expected symbol to exist")
	}
	if symbol.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", symbol.Symbol)
	}
	if symbol.AssetType != "stock" {
		t.Errorf("expected asset_type stock, got %s", symbol.AssetType)
	}
	// Default auto_update should be 1
	if symbol.AutoUpdate != 1 {
		t.Errorf("expected auto_update 1, got %d", symbol.AutoUpdate)
	}
}

func TestGetSymbolMetadata_CaseInsensitive(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Query with lowercase should work due to normalization
	symbol, err := core.GetSymbolMetadata("aapl")
	assertNoError(t, err, "get symbol with lowercase")

	if symbol == nil {
		t.Fatal("expected symbol to be found with lowercase query")
	}
	if symbol.Symbol != "AAPL" {
		t.Errorf("expected normalized symbol AAPL, got %s", symbol.Symbol)
	}
}

func TestGetSymbolMetadata_NonExistent(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	symbol, err := core.GetSymbolMetadata("NONEXISTENT")
	assertNoError(t, err, "get non-existent symbol")
	if symbol != nil {
		t.Error("expected nil for non-existent symbol")
	}
}

func TestUpdateSymbolMetadata(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	name := "Apple Inc."
	assetType := "stock"
	autoUpdate := 0
	sector := "Technology"
	exchange := "NASDAQ"

	updated, err := core.UpdateSymbolMetadata("AAPL", &name, &assetType, &autoUpdate, &sector, &exchange)
	assertNoError(t, err, "update symbol metadata")
	if !updated {
		t.Error("expected update to succeed")
	}

	// Verify changes
	symbol, err := core.GetSymbolMetadata("AAPL")
	assertNoError(t, err, "get updated symbol")

	if symbol.Name == nil || *symbol.Name != name {
		t.Errorf("expected name '%s', got %v", name, symbol.Name)
	}
	if symbol.AutoUpdate != autoUpdate {
		t.Errorf("expected auto_update %d, got %d", autoUpdate, symbol.AutoUpdate)
	}
	if symbol.Sector == nil || *symbol.Sector != sector {
		t.Errorf("expected sector '%s', got %v", sector, symbol.Sector)
	}
	if symbol.Exchange == nil || *symbol.Exchange != exchange {
		t.Errorf("expected exchange '%s', got %v", exchange, symbol.Exchange)
	}
}

func TestUpdateSymbolMetadata_PartialUpdate(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Update only name
	name := "Apple Inc."
	updated, err := core.UpdateSymbolMetadata("AAPL", &name, nil, nil, nil, nil)
	assertNoError(t, err, "partial update")
	if !updated {
		t.Error("expected update to succeed")
	}

	symbol, _ := core.GetSymbolMetadata("AAPL")
	if symbol.Name == nil || *symbol.Name != name {
		t.Errorf("expected name '%s'", name)
	}
	// Other fields should be unchanged
	if symbol.AssetType != "stock" {
		t.Error("asset_type should be unchanged")
	}
}

func TestUpdateSymbolMetadata_NonExistent(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	name := "Test"
	updated, err := core.UpdateSymbolMetadata("NONEXISTENT", &name, nil, nil, nil, nil)
	assertNoError(t, err, "update non-existent")
	if updated {
		t.Error("should not report updated for non-existent symbol")
	}
}

func TestUpdateSymbolAssetType(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")
	testBuyTransaction(t, core, "ETF01", 100, 50, "USD", "test-account")

	// Initially stock, change to bond
	updated, oldType, newType, err := core.UpdateSymbolAssetType("ETF01", "bond")
	assertNoError(t, err, "update asset type")
	if !updated {
		t.Error("expected update to succeed")
	}
	if oldType != "stock" {
		t.Errorf("expected old type 'stock', got '%s'", oldType)
	}
	if newType != "bond" {
		t.Errorf("expected new type 'bond', got '%s'", newType)
	}

	// Verify change
	symbol, _ := core.GetSymbolMetadata("ETF01")
	if symbol.AssetType != "bond" {
		t.Errorf("expected asset_type 'bond', got '%s'", symbol.AssetType)
	}
}

func TestUpdateSymbolAutoUpdate(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Default is 1, change to 0
	updated, err := core.UpdateSymbolAutoUpdate("AAPL", 0)
	assertNoError(t, err, "update auto_update")
	if !updated {
		t.Error("expected update to succeed")
	}

	symbol, _ := core.GetSymbolMetadata("AAPL")
	if symbol.AutoUpdate != 0 {
		t.Errorf("expected auto_update 0, got %d", symbol.AutoUpdate)
	}

	// Change back to 1
	updated, err = core.UpdateSymbolAutoUpdate("AAPL", 1)
	assertNoError(t, err, "update auto_update back")

	symbol, _ = core.GetSymbolMetadata("AAPL")
	if symbol.AutoUpdate != 1 {
		t.Errorf("expected auto_update 1, got %d", symbol.AutoUpdate)
	}
}

func TestSymbolNormalization(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Create with mixed case and spaces
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "  aapl  ",
		TransactionType: "BUY",
		Quantity:        100,
		Price:           150,
		Currency:        "USD",
		AccountID:       "test-account",
	})
	assertNoError(t, err, "add with unnormalized symbol")

	symbols, err := core.GetSymbols()
	assertNoError(t, err, "get symbols")

	if len(symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(symbols))
	}
	if symbols[0].Symbol != "AAPL" {
		t.Errorf("expected normalized symbol 'AAPL', got '%s'", symbols[0].Symbol)
	}
}
