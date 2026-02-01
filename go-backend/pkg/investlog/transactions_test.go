package investlog

import (
	"strings"
	"testing"
)

func TestAddTransaction_Basic(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Test basic BUY transaction
	id, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "BUY",
		Quantity:        100,
		Price:           150.0,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertNoError(t, err, "AddTransaction BUY")

	if id <= 0 {
		t.Errorf("expected positive transaction ID, got %d", id)
	}

	// Verify transaction was created
	tx, err := core.GetTransaction(id)
	assertNoError(t, err, "GetTransaction")

	if tx == nil {
		t.Fatal("transaction not found")
	}
	if tx.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", tx.Symbol)
	}
	if tx.Quantity != 100 {
		t.Errorf("expected quantity 100, got %f", tx.Quantity)
	}
	if tx.Price != 150.0 {
		t.Errorf("expected price 150, got %f", tx.Price)
	}
	assertFloatEquals(t, tx.TotalAmount, 15000.0, "total amount")
}

func TestAddTransaction_ValidationErrors(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	tests := []struct {
		name    string
		req     AddTransactionRequest
		wantErr string
	}{
		{
			name: "missing transaction type",
			req: AddTransactionRequest{
				Symbol:    "AAPL",
				Quantity:  100,
				Price:     150,
				Currency:  "USD",
				AccountID: "test-account",
			},
			wantErr: "transaction_type required",
		},
		{
			name: "invalid transaction type",
			req: AddTransactionRequest{
				Symbol:          "AAPL",
				TransactionType: "INVALID",
				Quantity:        100,
				Price:           150,
				Currency:        "USD",
				AccountID:       "test-account",
			},
			wantErr: "invalid transaction_type",
		},
		{
			name: "missing account ID",
			req: AddTransactionRequest{
				Symbol:          "AAPL",
				TransactionType: "BUY",
				Quantity:        100,
				Price:           150,
				Currency:        "USD",
			},
			wantErr: "account_id required",
		},
		{
			name: "invalid currency",
			req: AddTransactionRequest{
				Symbol:          "AAPL",
				TransactionType: "BUY",
				Quantity:        100,
				Price:           150,
				Currency:        "EUR",
				AccountID:       "test-account",
			},
			wantErr: "invalid currency",
		},
		{
			name: "missing symbol",
			req: AddTransactionRequest{
				TransactionType: "BUY",
				Quantity:        100,
				Price:           150,
				Currency:        "USD",
				AccountID:       "test-account",
			},
			wantErr: "symbol required",
		},
		{
			name: "negative quantity for BUY",
			req: AddTransactionRequest{
				Symbol:          "AAPL",
				TransactionType: "BUY",
				Quantity:        -100,
				Price:           150,
				Currency:        "USD",
				AccountID:       "test-account",
			},
			wantErr: "quantity must be positive",
		},
		{
			name: "zero quantity for BUY",
			req: AddTransactionRequest{
				Symbol:          "AAPL",
				TransactionType: "BUY",
				Quantity:        0,
				Price:           150,
				Currency:        "USD",
				AccountID:       "test-account",
			},
			wantErr: "quantity must be positive",
		},
		{
			name: "negative price",
			req: AddTransactionRequest{
				Symbol:          "AAPL",
				TransactionType: "BUY",
				Quantity:        100,
				Price:           -150,
				Currency:        "USD",
				AccountID:       "test-account",
			},
			wantErr: "price cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := core.AddTransaction(tt.req)
			assertError(t, err, tt.name)
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestAddTransaction_SellValidation(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Try to SELL without any holdings - should fail
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "SELL",
		Quantity:        100,
		Price:           150,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertError(t, err, "SELL without holdings")
	if !strings.Contains(err.Error(), "insufficient shares") {
		t.Errorf("expected 'insufficient shares' error, got %q", err.Error())
	}

	// Buy some shares first
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Try to SELL more than we have
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "SELL",
		Quantity:        150, // More than 100 we own
		Price:           160,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertError(t, err, "SELL exceeding holdings")
	if !strings.Contains(err.Error(), "insufficient shares") {
		t.Errorf("expected 'insufficient shares' error, got %q", err.Error())
	}

	// SELL valid amount should succeed
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "SELL",
		Quantity:        50,
		Price:           160,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertNoError(t, err, "valid SELL")

	// Verify remaining shares
	shares, err := core.getCurrentShares("AAPL", "USD", "test-account")
	assertNoError(t, err, "getCurrentShares")
	assertFloatEquals(t, shares, 50, "remaining shares")
}

func TestAddTransaction_TransferOutValidation(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Buy shares first
	testBuyTransaction(t, core, "TSLA", 50, 200, "USD", "test-account")

	// TRANSFER_OUT more than we have should fail
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "TSLA",
		TransactionType: "TRANSFER_OUT",
		Quantity:        100,
		Price:           200,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertError(t, err, "TRANSFER_OUT exceeding holdings")

	// Valid TRANSFER_OUT should succeed
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "TSLA",
		TransactionType: "TRANSFER_OUT",
		Quantity:        25,
		Price:           200,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertNoError(t, err, "valid TRANSFER_OUT")
}

func TestAddTransaction_AllTransactionTypes(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// BUY first to have shares for other operations
	testBuyTransaction(t, core, "TEST", 1000, 10, "CNY", "test-account")

	types := []struct {
		txType   string
		qty      float64
		price    float64
		wantPass bool
	}{
		{"SELL", 100, 11, true},
		{"DIVIDEND", 50, 0, true},
		{"SPLIT", 100, 0, true},          // Add 100 shares via split
		{"TRANSFER_IN", 50, 10, true},    // Transfer in 50 shares
		{"TRANSFER_OUT", 50, 10, true},   // Transfer out 50 shares
		{"ADJUST", 0, 100, true},         // Value adjustment
		{"INCOME", 1000, 1, true},        // Cash income (auto-sets symbol to CASH)
	}

	for _, tt := range types {
		t.Run(tt.txType, func(t *testing.T) {
			_, err := core.AddTransaction(AddTransactionRequest{
				Symbol:          "TEST",
				TransactionType: tt.txType,
				Quantity:        tt.qty,
				Price:           tt.price,
				Currency:        "CNY",
				AccountID:       "test-account",
				AssetType:       "stock",
			})
			if tt.wantPass {
				assertNoError(t, err, tt.txType)
			} else {
				assertError(t, err, tt.txType)
			}
		})
	}
}

func TestAddTransaction_CashLinking(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add initial cash via TRANSFER_IN
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "CASH",
		TransactionType: "TRANSFER_IN",
		Quantity:        100000,
		Price:           1,
		Currency:        "CNY",
		AccountID:       "test-account",
		AssetType:       "cash",
	})
	assertNoError(t, err, "add initial cash")

	// BUY with cash linking enabled
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "600000",
		TransactionType: "BUY",
		Quantity:        100,
		Price:           10,
		Currency:        "CNY",
		AccountID:       "test-account",
		AssetType:       "stock",
		Commission:      5,
		LinkCash:        true,
	})
	assertNoError(t, err, "BUY with cash linking")

	// Check that CASH was reduced (BUY creates a SELL on CASH)
	cashShares, err := core.getCurrentShares("CASH", "CNY", "test-account")
	assertNoError(t, err, "get CASH shares")

	// Initial 100000 - (100*10 + 5 commission) = 98995
	assertFloatEquals(t, cashShares, 98995, "CASH after BUY")

	// Now SELL with cash linking
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "600000",
		TransactionType: "SELL",
		Quantity:        50,
		Price:           12,
		Currency:        "CNY",
		AccountID:       "test-account",
		AssetType:       "stock",
		Commission:      5,
		LinkCash:        true,
	})
	assertNoError(t, err, "SELL with cash linking")

	// Check that CASH was increased (SELL creates a BUY on CASH)
	cashShares, err = core.getCurrentShares("CASH", "CNY", "test-account")
	assertNoError(t, err, "get CASH shares after SELL")

	// Previous 98995 + (50*12 - 5 commission) = 98995 + 595 = 99590
	assertFloatEquals(t, cashShares, 99590, "CASH after SELL")
}

func TestAddTransaction_SymbolNormalization(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add transaction with lowercase symbol
	id, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "aapl",
		TransactionType: "BUY",
		Quantity:        100,
		Price:           150,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertNoError(t, err, "add with lowercase symbol")

	// Verify symbol was normalized to uppercase
	tx, err := core.GetTransaction(id)
	assertNoError(t, err, "get transaction")
	if tx.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", tx.Symbol)
	}
}

func TestAddTransaction_TotalAmountOverride(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Provide explicit total amount that differs from qty*price
	totalAmt := 1600.0
	id, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "BUY",
		Quantity:        100,
		Price:           15, // 100 * 15 = 1500, but we override to 1600
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
		TotalAmount:     &totalAmt,
	})
	assertNoError(t, err, "add with total amount override")

	tx, err := core.GetTransaction(id)
	assertNoError(t, err, "get transaction")
	assertFloatEquals(t, tx.TotalAmount, 1600, "overridden total amount")
}

func TestAddTransaction_DefaultCurrency(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add transaction without currency - should default to CNY
	id, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "600000",
		TransactionType: "BUY",
		Quantity:        100,
		Price:           10,
		AccountID:       "test-account",
	})
	assertNoError(t, err, "add without currency")

	tx, err := core.GetTransaction(id)
	assertNoError(t, err, "get transaction")
	if tx.Currency != "CNY" {
		t.Errorf("expected default currency CNY, got %s", tx.Currency)
	}
}

func TestGetTransactions_Filtering(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "account1", "Account 1")
	testAccount(t, core, "account2", "Account 2")

	// Create transactions for filtering tests
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "account1")
	testBuyTransaction(t, core, "GOOGL", 50, 2000, "USD", "account1")
	testBuyTransaction(t, core, "600000", 200, 10, "CNY", "account2")

	tests := []struct {
		name      string
		filter    TransactionFilter
		wantCount int
	}{
		{
			name:      "all transactions",
			filter:    TransactionFilter{},
			wantCount: 3,
		},
		{
			name:      "filter by symbol",
			filter:    TransactionFilter{Symbol: "AAPL"},
			wantCount: 1,
		},
		{
			name:      "filter by account",
			filter:    TransactionFilter{AccountID: "account1"},
			wantCount: 2,
		},
		{
			name:      "filter by currency",
			filter:    TransactionFilter{Currency: "USD"},
			wantCount: 2,
		},
		{
			name:      "filter by transaction type",
			filter:    TransactionFilter{TransactionType: "BUY"},
			wantCount: 3,
		},
		{
			name:      "combined filters",
			filter:    TransactionFilter{Currency: "USD", AccountID: "account1"},
			wantCount: 2,
		},
		{
			name:      "limit results",
			filter:    TransactionFilter{Limit: 2},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs, err := core.GetTransactions(tt.filter)
			assertNoError(t, err, tt.name)
			if len(txs) != tt.wantCount {
				t.Errorf("expected %d transactions, got %d", tt.wantCount, len(txs))
			}
		})
	}
}

func TestGetTransactionCount(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Create multiple transactions
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")
	testBuyTransaction(t, core, "GOOGL", 50, 2000, "USD", "test-account")
	testBuyTransaction(t, core, "600000", 200, 10, "CNY", "test-account")

	count, err := core.GetTransactionCount(TransactionFilter{})
	assertNoError(t, err, "count all")
	if count != 3 {
		t.Errorf("expected 3 transactions, got %d", count)
	}

	count, err = core.GetTransactionCount(TransactionFilter{Currency: "USD"})
	assertNoError(t, err, "count USD")
	if count != 2 {
		t.Errorf("expected 2 USD transactions, got %d", count)
	}
}

func TestDeleteTransaction(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	id := testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Delete the transaction
	deleted, err := core.DeleteTransaction(id)
	assertNoError(t, err, "delete transaction")
	if !deleted {
		t.Error("expected transaction to be deleted")
	}

	// Verify it's gone
	tx, err := core.GetTransaction(id)
	assertNoError(t, err, "get deleted transaction")
	if tx != nil {
		t.Error("expected transaction to be nil after deletion")
	}

	// Delete non-existent transaction
	deleted, err = core.DeleteTransaction(99999)
	assertNoError(t, err, "delete non-existent")
	if deleted {
		t.Error("expected false for non-existent transaction")
	}
}

func TestGetCurrentShares(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Initially no shares
	shares, err := core.getCurrentShares("AAPL", "USD", "test-account")
	assertNoError(t, err, "initial shares")
	assertFloatEquals(t, shares, 0, "initial shares should be 0")

	// BUY 100
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	shares, err = core.getCurrentShares("AAPL", "USD", "test-account")
	assertNoError(t, err, "after BUY")
	assertFloatEquals(t, shares, 100, "shares after BUY")

	// SELL 30
	testSellTransaction(t, core, "AAPL", 30, 160, "USD", "test-account")

	shares, err = core.getCurrentShares("AAPL", "USD", "test-account")
	assertNoError(t, err, "after SELL")
	assertFloatEquals(t, shares, 70, "shares after SELL")

	// SPLIT adds 70 more shares
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "SPLIT",
		Quantity:        70,
		Price:           0,
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertNoError(t, err, "SPLIT")

	shares, err = core.getCurrentShares("AAPL", "USD", "test-account")
	assertNoError(t, err, "after SPLIT")
	assertFloatEquals(t, shares, 140, "shares after SPLIT")
}

func TestGetCurrentShares_MultiAccountMultiCurrency(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "account1", "Account 1")
	testAccount(t, core, "account2", "Account 2")

	// Same symbol, different accounts
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "account1")
	testBuyTransaction(t, core, "AAPL", 200, 155, "USD", "account2")

	shares1, err := core.getCurrentShares("AAPL", "USD", "account1")
	assertNoError(t, err, "account1 shares")
	assertFloatEquals(t, shares1, 100, "account1 should have 100")

	shares2, err := core.getCurrentShares("AAPL", "USD", "account2")
	assertNoError(t, err, "account2 shares")
	assertFloatEquals(t, shares2, 200, "account2 should have 200")

	// Same symbol, same account, different currency
	testBuyTransaction(t, core, "HSBC", 50, 40, "HKD", "account1")
	testBuyTransaction(t, core, "HSBC", 30, 5, "USD", "account1")

	sharesHKD, err := core.getCurrentShares("HSBC", "HKD", "account1")
	assertNoError(t, err, "HSBC HKD shares")
	assertFloatEquals(t, sharesHKD, 50, "HSBC HKD should have 50")

	sharesUSD, err := core.getCurrentShares("HSBC", "USD", "account1")
	assertNoError(t, err, "HSBC USD shares")
	assertFloatEquals(t, sharesUSD, 30, "HSBC USD should have 30")
}
