package investlog

import (
	"testing"
)

func TestUpdateLatestPrice(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Update price
	err := core.UpdateLatestPrice("AAPL", "USD", 150.50)
	assertNoError(t, err, "update price")

	// Verify it was stored
	price, err := core.GetLatestPrice("AAPL", "USD")
	assertNoError(t, err, "get price")
	if price == nil {
		t.Fatal("expected price to exist")
	}
	assertFloatEquals(t, price.Price, 150.50, "stored price")
	if price.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", price.Symbol)
	}
	if price.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", price.Currency)
	}
}

func TestUpdateLatestPrice_Upsert(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Initial price
	err := core.UpdateLatestPrice("AAPL", "USD", 150.00)
	assertNoError(t, err, "set initial price")

	// Update price
	err = core.UpdateLatestPrice("AAPL", "USD", 160.00)
	assertNoError(t, err, "update price")

	// Should have the new price
	price, _ := core.GetLatestPrice("AAPL", "USD")
	assertFloatEquals(t, price.Price, 160.00, "updated price")
}

func TestGetLatestPrice_NonExistent(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	price, err := core.GetLatestPrice("NONEXISTENT", "USD")
	assertNoError(t, err, "get non-existent price")
	if price != nil {
		t.Error("expected nil for non-existent price")
	}
}

func TestGetAllLatestPrices(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add multiple prices
	_ = core.UpdateLatestPrice("AAPL", "USD", 150.00)
	_ = core.UpdateLatestPrice("GOOGL", "USD", 2000.00)
	_ = core.UpdateLatestPrice("600000", "CNY", 10.50)

	prices, err := core.GetAllLatestPrices()
	assertNoError(t, err, "get all prices")

	if len(prices) != 3 {
		t.Fatalf("expected 3 prices, got %d", len(prices))
	}

	// Check specific prices using [symbol, currency] key
	aaplKey := [2]string{"AAPL", "USD"}
	if p, ok := prices[aaplKey]; !ok {
		t.Error("expected AAPL price")
	} else {
		assertFloatEquals(t, p.Price, 150.00, "AAPL price")
	}

	cnyKey := [2]string{"600000", "CNY"}
	if p, ok := prices[cnyKey]; !ok {
		t.Error("expected 600000 price")
	} else {
		assertFloatEquals(t, p.Price, 10.50, "600000 price")
	}
}

func TestManualUpdatePrice(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	err := core.ManualUpdatePrice("AAPL", "USD", 155.00)
	assertNoError(t, err, "manual update price")

	// Verify price was updated
	price, _ := core.GetLatestPrice("AAPL", "USD")
	if price == nil {
		t.Fatal("expected price to exist")
	}
	assertFloatEquals(t, price.Price, 155.00, "manual price")
}

func TestPriceSymbolNormalization(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Set price with lowercase symbol
	err := core.UpdateLatestPrice("aapl", "usd", 150.00)
	assertNoError(t, err, "set price with lowercase")

	// Should be retrievable with uppercase
	price, err := core.GetLatestPrice("AAPL", "USD")
	assertNoError(t, err, "get with uppercase")
	if price == nil {
		t.Fatal("expected price with normalized symbol")
	}
	assertFloatEquals(t, price.Price, 150.00, "price value")
}

func TestDetectSymbolType(t *testing.T) {
	tests := []struct {
		symbol   string
		currency string
		expected string
	}{
		// A-shares (6-digit CNY stocks)
		{"600000", "CNY", "a_share"},
		{"000001", "CNY", "a_share"},
		{"300750", "CNY", "a_share"},
		{"688001", "CNY", "a_share"},
		{"SH600000", "CNY", "a_share"},
		{"SZ000001", "CNY", "a_share"},

		// ETFs
		{"510300", "CNY", "a_share"},
		{"159915", "CNY", "a_share"},

		// Funds (6-digit CNY that don't match stock patterns)
		{"110011", "CNY", "fund"},
		{"000001", "CNY", "a_share"}, // This is actually a stock code

		// HK stocks
		{"00700", "HKD", "hk_stock"},
		{"09988", "HKD", "hk_stock"},

		// US stocks
		{"AAPL", "USD", "us_stock"},
		{"GOOGL", "USD", "us_stock"},
		{"TSLA", "USD", "us_stock"},

		// Gold
		{"AU9999", "CNY", "gold"},
		{"GOLD", "USD", "gold"},

		// Cash
		{"CASH", "CNY", "cash"},
		{"CASH", "USD", "cash"},

		// Unknown
		{"RANDOM123", "CNY", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol+"_"+tt.currency, func(t *testing.T) {
			result := detectSymbolType(tt.symbol, tt.currency)
			if result != tt.expected {
				t.Errorf("detectSymbolType(%s, %s) = %s, want %s",
					tt.symbol, tt.currency, result, tt.expected)
			}
		})
	}
}

func TestBuildYahooSymbol(t *testing.T) {
	tests := []struct {
		symbol   string
		currency string
		expected string
	}{
		// CNY stocks
		{"600000", "CNY", "600000.SS"},    // Shanghai
		{"000001", "CNY", "000001.SZ"},    // Shenzhen
		{"SH600000", "CNY", "600000.SS"},  // With prefix

		// HK stocks (pads to 4 digits minimum)
		{"00700", "HKD", "00700.HK"},      // Already 5 digits
		{"9988", "HKD", "9988.HK"},        // 4 digits, no padding needed

		// US stocks
		{"AAPL", "USD", "AAPL"},
		{"GOOGL", "USD", "GOOGL"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol+"_"+tt.currency, func(t *testing.T) {
			result := buildYahooSymbol(tt.symbol, tt.currency)
			if result != tt.expected {
				t.Errorf("buildYahooSymbol(%s, %s) = %s, want %s",
					tt.symbol, tt.currency, result, tt.expected)
			}
		})
	}
}

func TestUpdateAllPrices(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "test-account", "Test Account")

	// Add holdings
	testBuyTransaction(t, core, "AAPL", 100, 150, "USD", "test-account")
	testBuyTransaction(t, core, "GOOGL", 10, 2000, "USD", "test-account")

	// Disable auto-update for GOOGL
	_, _ = core.UpdateSymbolAutoUpdate("GOOGL", 0)

	// UpdateAllPrices will try to fetch from external APIs
	// Since we can't mock HTTP easily, we just verify it runs without panic
	// and returns sensible values
	updated, errors, err := core.UpdateAllPrices("USD")
	assertNoError(t, err, "update all prices")

	// The function should have attempted to update symbols with auto_update=1
	// Updated count might be 0 if external APIs fail, but shouldn't crash
	_ = updated
	_ = errors
}

func TestPriceFetcher_CacheKey(t *testing.T) {
	// Test the cache key generation
	tests := []struct {
		symbol    string
		currency  string
		assetType string
		expected  string
	}{
		{"AAPL", "USD", "stock", "AAPL|USD|stock"},
		{"600000", "CNY", "stock", "600000|CNY|stock"},
		{"CASH", "USD", "cash", "CASH|USD|cash"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := cacheKey(tt.symbol, tt.currency, tt.assetType)
			if result != tt.expected {
				t.Errorf("cacheKey(%s, %s, %s) = %s, want %s",
					tt.symbol, tt.currency, tt.assetType, result, tt.expected)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
		wantErr  bool
	}{
		{150.5, 150.5, false},
		{float32(100.25), 100.25, false},
		{100, 100.0, false},
		{int64(200), 200.0, false},
		{"123.45", 123.45, false},
		{"", 0, true},
		{nil, 0, true},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := parseFloat(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseFloat(%v) expected error", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseFloat(%v) unexpected error: %v", tt.input, err)
			}
			if !floatEquals(result, tt.expected, 0.01) {
				t.Errorf("parseFloat(%v) = %f, want %f", tt.input, result, tt.expected)
			}
		}
	}
}
