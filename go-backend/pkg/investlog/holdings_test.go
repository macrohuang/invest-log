package investlog

import (
	"strings"
	"testing"
)

func TestGetHoldings_Basic(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// No holdings initially
	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get empty holdings")
	if len(holdings) != 0 {
		t.Errorf("expected 0 holdings, got %d", len(holdings))
	}

	// Add a BUY transaction
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	holdings, err = core.GetHoldings("")
	assertNoError(t, err, "get holdings after BUY")
	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(holdings))
	}

	h := holdings[0]
	if h.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", h.Symbol)
	}
	assertFloatEquals(t, h.TotalShares, 100, "total shares")
	assertFloatEquals(t, h.TotalCost, 15000, "total cost")
	assertFloatEquals(t, h.AvgCost, 150, "average cost")
}

func TestGetHoldings_WeightedAverageCost(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Buy 100 shares at $150
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	// Buy 100 more shares at $160
	testBuyTransaction(t, core, "AAPL", 100, 160, "USD", "test-account")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get holdings")

	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(holdings))
	}

	h := holdings[0]
	// Total: 200 shares, cost: 100*150 + 100*160 = 31000
	// Avg: 31000/200 = 155
	assertFloatEquals(t, h.TotalShares, 200, "total shares")
	assertFloatEquals(t, h.TotalCost, 31000, "total cost")
	assertFloatEquals(t, h.AvgCost, 155, "weighted average cost")
}

func TestGetHoldings_CostAfterSell(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Buy 100 at $100 = $10000 cost
	testBuyTransaction(t, core, "AAPL", 100, 100, "USD", "test-account")

	// Sell 50 at $120 = $6000 proceeds
	testSellTransaction(t, core, "AAPL", 50, 120, "USD", "test-account")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get holdings")

	h := holdings[0]
	// After SELL: 50 shares remain
	// Cost: 10000 (buy) - 6000 (sell proceeds) = 4000
	// But the SQL shows SELL subtracts (total_amount - commission)
	// With no commission: cost = 10000 - 6000 = 4000
	assertFloatEquals(t, h.TotalShares, 50, "shares after sell")
	assertFloatEquals(t, h.TotalCost, 4000, "cost after sell")
	assertFloatEquals(t, h.AvgCost, 80, "avg cost after sell")
}

func TestGetHoldings_WithCommission(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Buy 100 at $100 with $10 commission
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "BUY",
		Quantity:        NewAmountFromInt(100),
		Price:           NewAmountFromInt(100),
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
		Commission:      NewAmountFromInt(10),
	})
	assertNoError(t, err, "buy with commission")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get holdings")

	h := holdings[0]
	// Cost = total_amount + commission = 10000 + 10 = 10010
	assertFloatEquals(t, h.TotalCost, 10010, "cost includes commission")
	assertFloatEquals(t, h.AvgCost, 100.1, "avg cost with commission")
}

func TestGetHoldings_SplitDoesNotAffectCost(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Buy 100 at $100 = $10000 cost
	testBuyTransaction(t, core, "AAPL", 100, 100, "USD", "test-account")

	// 2:1 split adds 100 shares
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "AAPL",
		TransactionType: "SPLIT",
		Quantity:        NewAmountFromInt(100), // Adds 100 shares
		Price:           NewAmountFromInt(0),
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "stock",
	})
	assertNoError(t, err, "split")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get holdings after split")

	h := holdings[0]
	// SPLIT: shares double but cost stays the same
	assertFloatEquals(t, h.TotalShares, 200, "shares after split")
	assertFloatEquals(t, h.TotalCost, 10000, "cost unchanged after split")
	assertFloatEquals(t, h.AvgCost, 50, "avg cost halved after split")
}

func TestGetHoldings_CashHandling(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add CASH via TRANSFER_IN
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "CASH",
		TransactionType: "TRANSFER_IN",
		Quantity:        NewAmountFromInt(10000),
		Price:           NewAmountFromInt(1),
		Currency:        "CNY",
		AccountID:       "test-account",
		AssetType:       "cash",
	})
	assertNoError(t, err, "add cash")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get holdings")

	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(holdings))
	}

	h := holdings[0]
	// For CASH: TotalCost = TotalShares, AvgCost = 1
	assertFloatEquals(t, h.TotalShares, 10000, "cash shares")
	assertFloatEquals(t, h.TotalCost, 10000, "cash cost equals shares")
	assertFloatEquals(t, h.AvgCost, 1, "cash avg cost is 1")
}

func TestGetHoldings_FilterByAccount(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "account1", "Account 1")
	testAccount(t, core, "account2", "Account 2")

	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "account1")
	testBuyTransaction(t, core, "GOOGL", 50, 2000, "USD", "account2")

	// Filter by account1
	holdings, err := core.GetHoldings("account1")
	assertNoError(t, err, "get holdings for account1")
	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding for account1, got %d", len(holdings))
	}
	if holdings[0].Symbol != "AAPL" {
		t.Errorf("expected AAPL, got %s", holdings[0].Symbol)
	}

	// Filter by account2
	holdings, err = core.GetHoldings("account2")
	assertNoError(t, err, "get holdings for account2")
	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding for account2, got %d", len(holdings))
	}
	if holdings[0].Symbol != "GOOGL" {
		t.Errorf("expected GOOGL, got %s", holdings[0].Symbol)
	}
}

func TestGetHoldings_ZeroSharesExcluded(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Buy and sell all shares
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")
	testSellTransaction(t, core, "AAPL", 100, 160, "USD", "test-account")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get holdings")

	// Holdings with 0 shares should be excluded (HAVING total_shares > 0 OR total_cost != 0)
	// But cost after full sell may not be 0, so we need to check
	for _, h := range holdings {
		if h.Symbol == "AAPL" && h.TotalShares.IsZero() && h.TotalCost.IsZero() {
			t.Error("zero share/cost holdings should be excluded")
		}
	}
}

func TestGetHoldingsBySymbol(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")
	testBuyTransaction(t, core, "GOOGL", 10, 2000, "USD", "test-account")

	// Update prices for PnL calculation
	assertNoError(t, core.UpdateLatestPrice("AAPL", "USD", NewAmountFromInt(160)), "set AAPL price")
	assertNoError(t, core.UpdateLatestPrice("GOOGL", "USD", NewAmountFromInt(2100)), "set GOOGL price")

	result, err := core.GetHoldingsBySymbol()
	assertNoError(t, err, "get holdings by symbol")

	usdData, ok := result["USD"]
	if !ok {
		t.Fatal("expected USD currency in result")
	}

	if len(usdData.Symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(usdData.Symbols))
	}

	// Check total values
	// AAPL: 100 * 150 = 15000 cost, 100 * 160 = 16000 market value
	// GOOGL: 10 * 2000 = 20000 cost, 10 * 2100 = 21000 market value
	expectedTotalCost := 35000.0
	expectedTotalMarket := 37000.0
	assertFloatEquals(t, usdData.TotalCost, expectedTotalCost, "total cost")
	assertFloatEquals(t, usdData.TotalMarketValue, expectedTotalMarket, "total market value")
	assertFloatEquals(t, usdData.TotalPnL, 2000, "total PnL")

	// Check individual symbol PnL
	for _, s := range usdData.Symbols {
		if s.Symbol == "AAPL" {
			if s.UnrealizedPnL == nil {
				t.Error("AAPL should have unrealized PnL")
			} else {
				assertFloatEquals(t, *s.UnrealizedPnL, 1000, "AAPL PnL")
			}
		}
	}
}

func TestGetHoldingsByCurrency(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add stock and cash holdings
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "CASH",
		TransactionType: "TRANSFER_IN",
		Quantity:        NewAmountFromInt(5000),
		Price:           NewAmountFromInt(1),
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "cash",
	})
	assertNoError(t, err, "add cash")

	// Set price for AAPL
	assertNoError(t, core.UpdateLatestPrice("AAPL", "USD", NewAmountFromInt(150)), "set price")

	result, err := core.GetHoldingsByCurrency()
	assertNoError(t, err, "get holdings by currency")

	usdData, ok := result["USD"]
	if !ok {
		t.Fatal("expected USD in result")
	}

	// Total: AAPL 15000 + CASH 5000 = 20000
	assertFloatEquals(t, usdData.Total, 20000, "total value")

	// Check allocations
	if len(usdData.Allocations) == 0 {
		t.Fatal("expected allocations")
	}

	var stockAlloc, cashAlloc *AllocationEntry
	for i := range usdData.Allocations {
		if usdData.Allocations[i].AssetType == "stock" {
			stockAlloc = &usdData.Allocations[i]
		}
		if usdData.Allocations[i].AssetType == "cash" {
			cashAlloc = &usdData.Allocations[i]
		}
	}

	if stockAlloc != nil {
		// Stock: 15000/20000 = 75%
		assertFloatEquals(t, stockAlloc.Percent, 75, "stock allocation percent")
	}

	if cashAlloc != nil {
		// Cash: 5000/20000 = 25%
		assertFloatEquals(t, cashAlloc.Percent, 25, "cash allocation percent")
	}
}

func TestGetHoldingsByCurrency_AllocationWarnings(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Set allocation rules: stock should be 60-80%
	_, err := core.SetAllocationSetting("USD", "stock", 60, 80)
	assertNoError(t, err, "set allocation setting")

	// Add holdings that violate the rule
	testBuyTransaction(t, core, "AAPL", 100, 100, "USD", "test-account")
	assertNoError(t, core.UpdateLatestPrice("AAPL", "USD", NewAmountFromInt(100)), "set price")

	// Stock is 100% (exceeds max 80%)
	result, err := core.GetHoldingsByCurrency()
	assertNoError(t, err, "get holdings by currency")

	usdData := result["USD"]
	for _, alloc := range usdData.Allocations {
		if alloc.AssetType == "stock" {
			if alloc.Warning == nil {
				t.Error("expected warning for stock exceeding max allocation")
			} else if !strings.Contains(*alloc.Warning, "超过最大配置") {
				t.Errorf("expected warning containing '超过最大配置', got %s", *alloc.Warning)
			}
		}
	}
}

func TestAdjustAssetValue(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Start with a metal holding
	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "GOLD",
		TransactionType: "BUY",
		Quantity:        NewAmountFromInt(10),
		Price:           NewAmountFromInt(500),
		Currency:        "CNY",
		AccountID:       "test-account",
		AssetType:       "metal",
	})
	assertNoError(t, err, "buy gold")

	// Current value: 10 * 500 = 5000
	holdings, _ := core.GetHoldings("")
	var currentValue Amount
	for _, h := range holdings {
		if h.Symbol == "GOLD" {
			currentValue = h.TotalCost
			break
		}
	}
	assertFloatEquals(t, currentValue, 5000, "initial gold value")

	// Adjust to new value 6000
	_, err = core.AdjustAssetValue("GOLD", NewAmountFromInt(6000), "CNY", "test-account", "metal", nil)
	assertNoError(t, err, "adjust value")

	// Check adjusted value
	holdings, _ = core.GetHoldings("")
	for _, h := range holdings {
		if h.Symbol == "GOLD" {
			assertFloatEquals(t, h.TotalCost, 6000, "adjusted gold value")
			break
		}
	}
}

func TestGetHoldingsByCurrencyAndAccount(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "account1", "Account 1")
	testAccount(t, core, "account2", "Account 2")

	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "account1")
	testBuyTransaction(t, core, "GOOGL", 10, 2000, "USD", "account2")

	assertNoError(t, core.UpdateLatestPrice("AAPL", "USD", NewAmountFromInt(150)), "set AAPL price")
	assertNoError(t, core.UpdateLatestPrice("GOOGL", "USD", NewAmountFromInt(2000)), "set GOOGL price")

	result, err := core.GetHoldingsByCurrencyAndAccount()
	assertNoError(t, err, "get holdings by currency and account")

	usdData, ok := result["USD"]
	if !ok {
		t.Fatal("expected USD in result")
	}

	// Check that we have both accounts
	if len(usdData.Accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(usdData.Accounts))
	}

	account1Data, ok := usdData.Accounts["account1"]
	if !ok {
		t.Fatal("expected account1 in result")
	}
	assertFloatEquals(t, account1Data.TotalMarketValue, 15000, "account1 market value")

	account2Data, ok := usdData.Accounts["account2"]
	if !ok {
		t.Fatal("expected account2 in result")
	}
	assertFloatEquals(t, account2Data.TotalMarketValue, 20000, "account2 market value")
}

func TestGetHoldings_MultipleAssetTypes(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add different asset types
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")

	_, err := core.AddTransaction(AddTransactionRequest{
		Symbol:          "BOND01",
		TransactionType: "BUY",
		Quantity:        NewAmountFromInt(50),
		Price:           NewAmountFromInt(100),
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "bond",
	})
	assertNoError(t, err, "buy bond")

	_, err = core.AddTransaction(AddTransactionRequest{
		Symbol:          "GOLD",
		TransactionType: "BUY",
		Quantity:        NewAmountFromInt(10),
		Price:           NewAmountFromInt(500),
		Currency:        "USD",
		AccountID:       "test-account",
		AssetType:       "metal",
	})
	assertNoError(t, err, "buy gold")

	holdings, err := core.GetHoldings("")
	assertNoError(t, err, "get holdings")

	if len(holdings) != 3 {
		t.Fatalf("expected 3 holdings, got %d", len(holdings))
	}

	assetTypes := make(map[string]bool)
	for _, h := range holdings {
		assetTypes[h.AssetType] = true
	}

	if !assetTypes["stock"] || !assetTypes["bond"] || !assetTypes["metal"] {
		t.Error("expected all asset types to be present")
	}
}
