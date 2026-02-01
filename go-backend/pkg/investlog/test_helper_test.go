package investlog

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestDB creates a temporary database for testing and returns a Core instance.
// The caller should defer cleanup() to remove the temp file.
func setupTestDB(t *testing.T) (*Core, func()) {
	t.Helper()

	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "investlog-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	core, err := Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open test db: %v", err)
	}

	cleanup := func() {
		core.Close()
		os.RemoveAll(tmpDir)
	}

	return core, cleanup
}

// testAccount creates a test account with given ID.
func testAccount(t *testing.T, core *Core, accountID, accountName string) {
	t.Helper()
	_, err := core.AddAccount(Account{
		AccountID:   accountID,
		AccountName: accountName,
	})
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}
}

// testBuyTransaction creates a BUY transaction for testing.
func testBuyTransaction(t *testing.T, core *Core, symbol string, qty, price float64, currency, accountID string) int64 {
	t.Helper()
	id, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          symbol,
		TransactionType: "BUY",
		Quantity:        qty,
		Price:           price,
		Currency:        currency,
		AccountID:       accountID,
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("failed to create test BUY transaction: %v", err)
	}
	return id
}

// testSellTransaction creates a SELL transaction for testing.
func testSellTransaction(t *testing.T, core *Core, symbol string, qty, price float64, currency, accountID string) int64 {
	t.Helper()
	id, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          symbol,
		TransactionType: "SELL",
		Quantity:        qty,
		Price:           price,
		Currency:        currency,
		AccountID:       accountID,
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("failed to create test SELL transaction: %v", err)
	}
	return id
}

// floatEquals checks if two floats are approximately equal.
func floatEquals(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// assertFloatEquals fails the test if the floats are not approximately equal.
func assertFloatEquals(t *testing.T, got, want float64, msg string) {
	t.Helper()
	if !floatEquals(got, want, 0.001) {
		t.Errorf("%s: got %.4f, want %.4f", msg, got, want)
	}
}

// assertNoError fails the test if err is not nil.
func assertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", msg, err)
	}
}

// assertError fails the test if err is nil.
func assertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

// assertContains checks if the string contains the substring.
func assertContains(t *testing.T, s, substr, msg string) {
	t.Helper()
	if len(s) == 0 || len(substr) == 0 {
		if len(substr) > 0 {
			t.Errorf("%s: string %q does not contain %q", msg, s, substr)
		}
		return
	}
	found := false
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("%s: string %q does not contain %q", msg, s, substr)
	}
}
