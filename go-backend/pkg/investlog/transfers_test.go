package investlog

import (
	"strings"
	"testing"
)

func TestTransfer_SameCurrency_Securities(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "acct-a")

	result, err := core.Transfer(TransferRequest{
		Symbol:        "AAPL",
		Quantity:      NewAmountFromInt(40),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
	})
	assertNoError(t, err, "Transfer")

	if result.TransferOutID == 0 || result.TransferInID == 0 {
		t.Fatalf("expected non-zero IDs, got out=%d in=%d", result.TransferOutID, result.TransferInID)
	}
	if !result.ExchangeRate.IsZero() {
		t.Errorf("same currency transfer should have zero exchange_rate, got %v", result.ExchangeRate)
	}

	// Verify source holdings: 60 shares remain, cost = 60*150 = 9000
	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "GetHoldings")

	var srcHolding, dstHolding *Holding
	for i := range holdings {
		if holdings[i].Symbol == "AAPL" && holdings[i].AccountID == "acct-a" {
			srcHolding = &holdings[i]
		}
		if holdings[i].Symbol == "AAPL" && holdings[i].AccountID == "acct-b" {
			dstHolding = &holdings[i]
		}
	}

	if srcHolding == nil {
		t.Fatal("source holding not found")
	}
	assertFloatEquals(t, srcHolding.TotalShares, 60, "source shares")
	assertFloatEquals(t, srcHolding.TotalCost, 9000, "source cost")
	assertFloatEquals(t, srcHolding.AvgCost, 150, "source avg cost")

	if dstHolding == nil {
		t.Fatal("destination holding not found")
	}
	assertFloatEquals(t, dstHolding.TotalShares, 40, "dest shares")
	assertFloatEquals(t, dstHolding.TotalCost, 6000, "dest cost")
	assertFloatEquals(t, dstHolding.AvgCost, 150, "dest avg cost")
}

func TestTransfer_SameCurrency_Cash(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")

	// Add cash to account A
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "CASH",
		TransactionType: "TRANSFER_IN",
		Quantity:        NewAmountFromInt(10000),
		Price:           NewAmountFromInt(1),
		AccountID:       "acct-a",
		AssetType:       "cash",
		Currency:        "CNY",
	})
	assertNoError(t, err, "seed cash")

	result, err := core.Transfer(TransferRequest{
		Symbol:        "CASH",
		Quantity:      NewAmountFromInt(3000),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "CNY",
		AssetType:     "cash",
	})
	assertNoError(t, err, "Transfer cash")

	if result.TransferOutID == 0 || result.TransferInID == 0 {
		t.Fatalf("expected non-zero IDs")
	}

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "GetHoldings")

	for _, h := range holdings {
		if h.Symbol == "CASH" && h.Currency == "CNY" && h.AccountID == "acct-a" {
			assertFloatEquals(t, h.TotalShares, 7000, "source cash")
		}
		if h.Symbol == "CASH" && h.Currency == "CNY" && h.AccountID == "acct-b" {
			assertFloatEquals(t, h.TotalShares, 3000, "dest cash")
		}
	}
}

func TestTransfer_CrossCurrency_Cash(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")

	// Set known exchange rate
	_, err := core.SetExchangeRate("USD", "CNY", 7.2, "manual")
	assertNoError(t, err, "set rate")

	// Seed USD cash
	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "CASH",
		TransactionType: "TRANSFER_IN",
		Quantity:        NewAmountFromInt(1000),
		Price:           NewAmountFromInt(1),
		AccountID:       "acct-a",
		AssetType:       "cash",
		Currency:        "USD",
	})
	assertNoError(t, err, "seed USD cash")

	result, err := core.Transfer(TransferRequest{
		Symbol:        "CASH",
		Quantity:      NewAmountFromInt(500),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
		ToCurrency:    "CNY",
		AssetType:     "cash",
	})
	assertNoError(t, err, "Transfer cross-currency cash")

	if !floatEquals(result.ExchangeRate.InexactFloat64(), 7.2, 0.001) {
		t.Errorf("expected exchange_rate=7.2, got %v", result.ExchangeRate)
	}

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "GetHoldings")

	for _, h := range holdings {
		if h.Symbol == "CASH" && h.Currency == "USD" && h.AccountID == "acct-a" {
			assertFloatEquals(t, h.TotalShares, 500, "source USD cash remaining")
		}
		if h.Symbol == "CASH" && h.Currency == "CNY" && h.AccountID == "acct-b" {
			// 500 USD * 7.2 = 3600 CNY
			assertFloatEquals(t, h.TotalShares, 3600, "dest CNY cash")
		}
	}
}

func TestTransfer_CrossCurrency_Securities(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")

	_, err := core.SetExchangeRate("USD", "CNY", 7.2, "manual")
	assertNoError(t, err, "set USD rate")
	_, err = core.SetExchangeRate("HKD", "CNY", 0.9, "manual")
	assertNoError(t, err, "set HKD rate")

	// Buy 100 shares at $150 USD
	testBuyTransaction(t, core, "STOCK", 100, 150, "USD", "acct-a")

	// Transfer 50 shares from USD account to HKD account
	result, err := core.Transfer(TransferRequest{
		Symbol:        "STOCK",
		Quantity:      NewAmountFromInt(50),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
		ToCurrency:    "HKD",
	})
	assertNoError(t, err, "Transfer cross-currency securities")

	// Exchange rate USD→HKD = 7.2/0.9 = 8.0
	expectedRate := 7.2 / 0.9
	if !floatEquals(result.ExchangeRate.InexactFloat64(), expectedRate, 0.001) {
		t.Errorf("expected exchange_rate=%.4f, got %.4f", expectedRate, result.ExchangeRate.InexactFloat64())
	}

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "GetHoldings")

	for _, h := range holdings {
		if h.Symbol == "STOCK" && h.AccountID == "acct-a" {
			// 50 shares remain, cost = 50*150 = 7500 USD
			assertFloatEquals(t, h.TotalShares, 50, "source shares")
			assertFloatEquals(t, h.TotalCost, 7500, "source cost")
		}
		if h.Symbol == "STOCK" && h.AccountID == "acct-b" {
			// 50 shares (quantity unchanged), cost = 7500 * 8.0 = 60000 HKD
			assertFloatEquals(t, h.TotalShares, 50, "dest shares")
			assertFloatEquals(t, h.TotalCost, 60000, "dest cost in HKD")
			assertFloatEquals(t, h.AvgCost, 1200, "dest avg cost in HKD") // 150*8=1200
		}
	}
}

func TestTransfer_WithCommission(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")
	testBuyTransaction(t, core, "AAPL", 100, 200, "USD", "acct-a")

	result, err := core.Transfer(TransferRequest{
		Symbol:        "AAPL",
		Quantity:      NewAmountFromInt(50),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
		Commission:    NewAmountFromInt(25),
	})
	assertNoError(t, err, "Transfer with commission")

	// Verify commission is on TRANSFER_OUT only
	outTxn, err := core.GetTransaction(result.TransferOutID)
	assertNoError(t, err, "GetTransaction out")
	assertFloatEquals(t, outTxn.Commission, 25, "out commission")

	inTxn, err := core.GetTransaction(result.TransferInID)
	assertNoError(t, err, "GetTransaction in")
	assertFloatEquals(t, inTxn.Commission, 0, "in commission")
}

func TestTransfer_InsufficientShares(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "acct-a")

	_, err := core.Transfer(TransferRequest{
		Symbol:        "AAPL",
		Quantity:      NewAmountFromInt(200), // only have 100
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
	})
	assertError(t, err, "insufficient shares")
	assertContains(t, err.Error(), "insufficient", "error message")
}

func TestTransfer_SameAccount(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")

	_, err := core.Transfer(TransferRequest{
		Symbol:        "AAPL",
		Quantity:      NewAmountFromInt(10),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-a",
		FromCurrency:  "USD",
	})
	assertError(t, err, "same account")
	assertContains(t, err.Error(), "different", "error message")
}

func TestTransfer_InvalidCurrency(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")

	_, err := core.Transfer(TransferRequest{
		Symbol:        "AAPL",
		Quantity:      NewAmountFromInt(10),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "EUR",
	})
	assertError(t, err, "invalid currency")
}

func TestTransfer_LinkedTransactionID(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "acct-a")

	result, err := core.Transfer(TransferRequest{
		Symbol:        "AAPL",
		Quantity:      NewAmountFromInt(30),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
	})
	assertNoError(t, err, "Transfer")

	outTxn, err := core.GetTransaction(result.TransferOutID)
	assertNoError(t, err, "GetTransaction out")
	if outTxn.LinkedTransactionID == nil || *outTxn.LinkedTransactionID != result.TransferInID {
		t.Errorf("out linked_id: want %d, got %v", result.TransferInID, outTxn.LinkedTransactionID)
	}

	inTxn, err := core.GetTransaction(result.TransferInID)
	assertNoError(t, err, "GetTransaction in")
	if inTxn.LinkedTransactionID == nil || *inTxn.LinkedTransactionID != result.TransferOutID {
		t.Errorf("in linked_id: want %d, got %v", result.TransferOutID, inTxn.LinkedTransactionID)
	}
}

func TestTransfer_DeleteCascade(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "acct-a")

	result, err := core.Transfer(TransferRequest{
		Symbol:        "AAPL",
		Quantity:      NewAmountFromInt(30),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
	})
	assertNoError(t, err, "Transfer")

	// Delete TRANSFER_OUT — should also delete TRANSFER_IN
	deleted, err := core.DeleteTransaction(result.TransferOutID)
	assertNoError(t, err, "DeleteTransaction")
	if !deleted {
		t.Fatal("expected delete to succeed")
	}

	outTxn, err := core.GetTransaction(result.TransferOutID)
	assertNoError(t, err, "GetTransaction out after delete")
	if outTxn != nil {
		t.Error("expected TRANSFER_OUT to be deleted")
	}

	inTxn, err := core.GetTransaction(result.TransferInID)
	assertNoError(t, err, "GetTransaction in after delete")
	if inTxn != nil {
		t.Error("expected TRANSFER_IN to be cascade deleted")
	}

	// Holdings should revert: 100 shares in acct-a, 0 in acct-b
	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "GetHoldings after delete")
	for _, h := range holdings {
		if h.Symbol == "AAPL" && h.AccountID == "acct-a" {
			assertFloatEquals(t, h.TotalShares, 100, "reverted shares")
		}
		if h.Symbol == "AAPL" && h.AccountID == "acct-b" {
			t.Error("expected no holdings in acct-b after cascade delete")
		}
	}
}

func TestTransfer_OldTransferNoCostImpact(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")

	// Create an old-style standalone TRANSFER_IN (no linked_transaction_id)
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "TRANSFER_IN",
		Quantity:        NewAmountFromInt(50),
		Price:           NewAmountFromInt(100),
		AccountID:       "acct-a",
		AssetType:       "stock",
		Currency:        "USD",
	})
	assertNoError(t, err, "old TRANSFER_IN")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "GetHoldings")

	for _, h := range holdings {
		if h.Symbol == "AAPL" && h.AccountID == "acct-a" {
			assertFloatEquals(t, h.TotalShares, 50, "old transfer shares")
			// Old TRANSFER_IN without linked_transaction_id: cost should be 0
			assertFloatEquals(t, h.TotalCost, 0, "old transfer cost should be zero")
		}
	}
}

func TestTransfer_MissingFields(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name    string
		req     TransferRequest
		wantErr string
	}{
		{
			name:    "missing symbol",
			req:     TransferRequest{Quantity: NewAmountFromInt(10), FromAccountID: "a", ToAccountID: "b", FromCurrency: "USD"},
			wantErr: "symbol required",
		},
		{
			name:    "zero quantity",
			req:     TransferRequest{Symbol: "AAPL", Quantity: Amount{}, FromAccountID: "a", ToAccountID: "b", FromCurrency: "USD"},
			wantErr: "quantity must be positive",
		},
		{
			name:    "missing from_account",
			req:     TransferRequest{Symbol: "AAPL", Quantity: NewAmountFromInt(10), ToAccountID: "b", FromCurrency: "USD"},
			wantErr: "from_account_id required",
		},
		{
			name:    "missing to_account",
			req:     TransferRequest{Symbol: "AAPL", Quantity: NewAmountFromInt(10), FromAccountID: "a", FromCurrency: "USD"},
			wantErr: "to_account_id required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := core.Transfer(tc.req)
			assertError(t, err, tc.name)
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestTransfer_CostBasisInHoldings(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct-a", "Account A")
	testAccount(t, core, "acct-b", "Account B")

	// Buy 100 shares at $100, then 100 shares at $200 → avg cost = $150
	testBuyTransaction(t, core, "STOCK", 100, 100, "USD", "acct-a")
	testBuyTransaction(t, core, "STOCK", 100, 200, "USD", "acct-a")

	// Transfer 80 shares: cost transferred = 80 * 150 = 12000
	_, err := core.Transfer(TransferRequest{
		Symbol:        "STOCK",
		Quantity:      NewAmountFromInt(80),
		FromAccountID: "acct-a",
		ToAccountID:   "acct-b",
		FromCurrency:  "USD",
	})
	assertNoError(t, err, "Transfer")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "GetHoldings")

	for _, h := range holdings {
		if h.Symbol == "STOCK" && h.AccountID == "acct-a" {
			// 120 shares remain, cost = 30000 - 12000 = 18000
			assertFloatEquals(t, h.TotalShares, 120, "source shares")
			assertFloatEquals(t, h.TotalCost, 18000, "source cost")
			assertFloatEquals(t, h.AvgCost, 150, "source avg cost unchanged")
		}
		if h.Symbol == "STOCK" && h.AccountID == "acct-b" {
			assertFloatEquals(t, h.TotalShares, 80, "dest shares")
			assertFloatEquals(t, h.TotalCost, 12000, "dest cost")
			assertFloatEquals(t, h.AvgCost, 150, "dest avg cost matches source")
		}
	}
}

func TestGetExchangeRate_CrossPair(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := core.SetExchangeRate("USD", "CNY", 7.2, "manual")
	assertNoError(t, err, "set USD rate")
	_, err = core.SetExchangeRate("HKD", "CNY", 0.9, "manual")
	assertNoError(t, err, "set HKD rate")

	// Same currency
	rate, err := core.GetExchangeRate("USD", "USD")
	assertNoError(t, err, "same currency")
	assertFloatEquals(t, rate, 1.0, "same currency rate")

	// USD→CNY
	rate, err = core.GetExchangeRate("USD", "CNY")
	assertNoError(t, err, "USD→CNY")
	assertFloatEquals(t, rate, 7.2, "USD→CNY rate")

	// CNY→USD
	rate, err = core.GetExchangeRate("CNY", "USD")
	assertNoError(t, err, "CNY→USD")
	assertFloatEquals(t, rate, 1.0/7.2, "CNY→USD rate")

	// USD→HKD (cross pair)
	rate, err = core.GetExchangeRate("USD", "HKD")
	assertNoError(t, err, "USD→HKD")
	assertFloatEquals(t, rate, 7.2/0.9, "USD→HKD rate")

	// HKD→USD
	rate, err = core.GetExchangeRate("HKD", "USD")
	assertNoError(t, err, "HKD→USD")
	assertFloatEquals(t, rate, 0.9/7.2, "HKD→USD rate")
}
