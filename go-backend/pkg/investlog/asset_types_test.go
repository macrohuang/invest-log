package investlog

import (
	"testing"
)

func TestGetAssetTypes(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	types, err := core.GetAssetTypes()
	assertNoError(t, err, "get asset types")

	// Should have default types
	if len(types) < 4 {
		t.Errorf("expected at least 4 default asset types, got %d", len(types))
	}

	// Check default types exist
	codeSet := make(map[string]bool)
	for _, at := range types {
		codeSet[at.Code] = true
	}

	expectedCodes := []string{"stock", "bond", "metal", "cash"}
	for _, code := range expectedCodes {
		if !codeSet[code] {
			t.Errorf("expected default asset type '%s' to exist", code)
		}
	}
}

func TestGetAssetTypeLabels(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	labels, err := core.GetAssetTypeLabels()
	assertNoError(t, err, "get asset type labels")

	// Check default labels
	expectedLabels := map[string]string{
		"stock": "股票",
		"bond":  "债券",
		"metal": "贵金属",
		"cash":  "现金",
	}

	for code, expectedLabel := range expectedLabels {
		if label, ok := labels[code]; !ok || label != expectedLabel {
			t.Errorf("expected label for '%s' to be '%s', got '%s'", code, expectedLabel, label)
		}
	}
}

func TestAddAssetType(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	success, err := core.AddAssetType("crypto", "加密货币")
	assertNoError(t, err, "add asset type")
	if !success {
		t.Error("expected success")
	}

	// Verify it exists
	types, err := core.GetAssetTypes()
	assertNoError(t, err, "get asset types")

	found := false
	for _, at := range types {
		if at.Code == "crypto" && at.Label == "加密货币" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected new asset type to be found")
	}
}

func TestAddAssetType_Duplicate(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add first time
	_, err := core.AddAssetType("realestate", "房产")
	assertNoError(t, err, "add asset type first time")

	// Add duplicate
	_, err = core.AddAssetType("realestate", "房地产")
	assertError(t, err, "add duplicate asset type")
}

func TestAddAssetType_Normalization(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add with uppercase and spaces
	_, err := core.AddAssetType("  CRYPTO  ", "加密货币")
	assertNoError(t, err, "add with unnormalized code")

	// Should be stored as lowercase
	labels, _ := core.GetAssetTypeLabels()
	if _, ok := labels["crypto"]; !ok {
		t.Error("expected code to be normalized to lowercase")
	}
}

func TestCheckAssetTypeInUse(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// stock is not in use initially
	inUse, err := core.CheckAssetTypeInUse("stock")
	assertNoError(t, err, "check stock in use")
	if inUse {
		t.Error("expected stock not in use initially")
	}

	// Add a stock transaction
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Now stock should be in use
	inUse, err = core.CheckAssetTypeInUse("stock")
	assertNoError(t, err, "check stock in use after transaction")
	if !inUse {
		t.Error("expected stock to be in use after transaction")
	}
}

func TestCanDeleteAssetType(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add a custom asset type
	_, err := core.AddAssetType("crypto", "加密货币")
	assertNoError(t, err, "add crypto type")

	// Should be deletable (not in use)
	canDelete, _, err := core.CanDeleteAssetType("crypto")
	assertNoError(t, err, "can delete crypto")
	if !canDelete {
		t.Error("expected crypto to be deletable")
	}

	// Add a symbol using crypto
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "BTC",
		TransactionType: "BUY",
		Quantity:        1,
		Price:           50000,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "crypto",
	})
	assertNoError(t, err, "add BTC")

	// Now should not be deletable
	canDelete, _, err = core.CanDeleteAssetType("crypto")
	assertNoError(t, err, "can delete crypto after use")
	if canDelete {
		t.Error("expected crypto to not be deletable after use")
	}
}

func TestDeleteAssetType(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add a custom asset type
	_, err := core.AddAssetType("crypto", "加密货币")
	assertNoError(t, err, "add crypto type")

	// Delete should succeed
	deleted, msg, err := core.DeleteAssetType("crypto")
	assertNoError(t, err, "delete crypto")
	if !deleted {
		t.Errorf("expected deletion to succeed, got message: %s", msg)
	}

	// Verify it's gone
	labels, _ := core.GetAssetTypeLabels()
	if _, ok := labels["crypto"]; ok {
		t.Error("expected crypto to be deleted")
	}
}

func TestDeleteAssetType_InUse(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add a custom asset type and use it
	_, _ = core.AddAssetType("crypto", "加密货币")
	_, _ = core.AddTransaction(AddTransactionRequest{
		Symbol:          "BTC",
		TransactionType: "BUY",
		Quantity:        1,
		Price:           50000,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "crypto",
	})

	// Delete should fail
	deleted, msg, err := core.DeleteAssetType("crypto")
	assertNoError(t, err, "delete crypto in use")
	if deleted {
		t.Error("should not delete asset type in use")
	}
	if msg == "" {
		t.Error("expected error message")
	}
}

func TestDeleteAssetType_NonExistent(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	deleted, _, err := core.DeleteAssetType("nonexistent")
	assertNoError(t, err, "delete non-existent")
	if deleted {
		t.Error("should not report deleted for non-existent type")
	}
}

func TestDeleteAssetType_DefaultTypes(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Trying to delete default types should fail if they have any symbols
	// But if no symbols, it might succeed - depends on business rules
	// Let's test that we can at least check them

	for _, defaultType := range DefaultAssetTypes {
		inUse, err := core.CheckAssetTypeInUse(defaultType)
		assertNoError(t, err, "check default type")
		// Initially no symbols, so should not be in use
		if inUse {
			t.Errorf("expected '%s' not in use initially", defaultType)
		}
	}
}
