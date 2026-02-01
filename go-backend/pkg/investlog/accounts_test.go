package investlog

import (
	"testing"
)

func TestAddAccount(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	success, err := core.AddAccount(Account{
		AccountID:   "test-account",
		AccountName: "Test Account",
	})
	assertNoError(t, err, "add account")
	if !success {
		t.Error("expected success to be true")
	}

	// Verify account exists
	accounts, err := core.GetAccounts()
	assertNoError(t, err, "get accounts")
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}

	acc := accounts[0]
	if acc.AccountID != "test-account" {
		t.Errorf("expected account_id 'test-account', got %s", acc.AccountID)
	}
	if acc.AccountName != "Test Account" {
		t.Errorf("expected account_name 'Test Account', got %s", acc.AccountName)
	}
}

func TestAddAccount_WithOptionalFields(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	broker := "Interactive Brokers"
	accountType := "Margin"

	success, err := core.AddAccount(Account{
		AccountID:   "ibkr-account",
		AccountName: "IBKR Margin",
		Broker:      &broker,
		AccountType: &accountType,
	})
	assertNoError(t, err, "add account with optional fields")
	if !success {
		t.Error("expected success")
	}

	accounts, err := core.GetAccounts()
	assertNoError(t, err, "get accounts")

	acc := accounts[0]
	if acc.Broker == nil || *acc.Broker != broker {
		t.Errorf("expected broker '%s', got %v", broker, acc.Broker)
	}
	if acc.AccountType == nil || *acc.AccountType != accountType {
		t.Errorf("expected account_type '%s', got %v", accountType, acc.AccountType)
	}
}

func TestAddAccount_Duplicate(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add first account
	_, err := core.AddAccount(Account{
		AccountID:   "test-account",
		AccountName: "Test Account",
	})
	assertNoError(t, err, "add first account")

	// Try to add duplicate
	_, err = core.AddAccount(Account{
		AccountID:   "test-account",
		AccountName: "Another Name",
	})
	// SQLite UNIQUE constraint should cause an error
	assertError(t, err, "add duplicate account")
}

func TestGetAccounts_Ordering(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add accounts in non-alphabetical order
	testAccount(t, core, "charlie", "Charlie")
	testAccount(t, core, "alpha", "Alpha")
	testAccount(t, core, "bravo", "Bravo")

	accounts, err := core.GetAccounts()
	assertNoError(t, err, "get accounts")

	// Should be ordered by account_id
	if len(accounts) != 3 {
		t.Fatalf("expected 3 accounts, got %d", len(accounts))
	}
	if accounts[0].AccountID != "alpha" {
		t.Errorf("expected first account 'alpha', got %s", accounts[0].AccountID)
	}
	if accounts[1].AccountID != "bravo" {
		t.Errorf("expected second account 'bravo', got %s", accounts[1].AccountID)
	}
	if accounts[2].AccountID != "charlie" {
		t.Errorf("expected third account 'charlie', got %s", accounts[2].AccountID)
	}
}

func TestCheckAccountInUse(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Account not in use initially
	inUse, err := core.CheckAccountInUse("test-account")
	assertNoError(t, err, "check account in use")
	if inUse {
		t.Error("expected account not in use initially")
	}

	// Add a transaction
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Account should now be in use
	inUse, err = core.CheckAccountInUse("test-account")
	assertNoError(t, err, "check account in use after transaction")
	if !inUse {
		t.Error("expected account to be in use after transaction")
	}
}

func TestDeleteAccount(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Delete should succeed when no transactions
	deleted, msg, err := core.DeleteAccount("test-account")
	assertNoError(t, err, "delete account")
	if !deleted {
		t.Errorf("expected account to be deleted, got message: %s", msg)
	}

	// Verify account is gone
	accounts, err := core.GetAccounts()
	assertNoError(t, err, "get accounts")
	if len(accounts) != 0 {
		t.Error("expected 0 accounts after deletion")
	}
}

func TestDeleteAccount_InUse(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add a transaction to make account in use
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Delete should fail
	deleted, msg, err := core.DeleteAccount("test-account")
	assertNoError(t, err, "delete in-use account")
	if deleted {
		t.Error("should not delete account with transactions")
	}
	if msg == "" {
		t.Error("expected error message for in-use account")
	}

	// Account should still exist
	accounts, err := core.GetAccounts()
	assertNoError(t, err, "get accounts")
	if len(accounts) != 1 {
		t.Error("expected account to still exist")
	}
}

func TestDeleteAccount_NonExistent(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	deleted, _, err := core.DeleteAccount("non-existent")
	assertNoError(t, err, "delete non-existent")
	// Should return false for non-existent account
	if deleted {
		t.Error("should not report deleted for non-existent account")
	}
}
