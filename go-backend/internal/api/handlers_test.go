package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"investlog/pkg/investlog"
)

// setupTestRouter creates a test router with a temporary database.
func setupTestRouter(t *testing.T) (http.Handler, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	core, err := investlog.Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open test db: %v", err)
	}

	router := NewRouter(core)

	cleanup := func() {
		core.Close()
		os.RemoveAll(tmpDir)
	}

	return router, cleanup
}

// doRequest performs a request and returns the response.
func doRequest(router http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBytes)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// parseJSON parses the response body into a map.
func parseJSON(rr *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)
	return result
}

func TestHealthEndpoint(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, "GET", "/api/health", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	result := parseJSON(rr)
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}
}

func TestAccountsEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Get accounts (initially empty)
	rr := doRequest(router, "GET", "/api/accounts", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/accounts: expected 200, got %d", rr.Code)
	}

	// Add account
	rr = doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id":   "test-account",
		"account_name": "Test Account",
	})
	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/accounts: expected 200, got %d", rr.Code)
	}

	// Get accounts again
	rr = doRequest(router, "GET", "/api/accounts", nil)
	var accounts []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&accounts)
	if len(accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(accounts))
	}

	// Delete account
	rr = doRequest(router, "DELETE", "/api/accounts/test-account", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE /api/accounts: expected 200, got %d", rr.Code)
	}
}

func TestTransactionsEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Create account first
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id":   "test-account",
		"account_name": "Test Account",
	})

	// Add transaction
	rr := doRequest(router, "POST", "/api/transactions", map[string]interface{}{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         100,
		"price":            150,
		"currency":         "USD",
		"account_id":       "test-account",
		"asset_type":       "stock",
	})
	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/transactions: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	result := parseJSON(rr)
	txID := result["id"]
	if txID == nil {
		t.Error("expected transaction ID in response")
	}

	// Get transactions
	rr = doRequest(router, "GET", "/api/transactions?paged=1", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/transactions: expected 200, got %d", rr.Code)
	}

	var payload struct {
		Items []map[string]interface{} `json:"items"`
		Total int                      `json:"total"`
	}
	json.NewDecoder(rr.Body).Decode(&payload)
	if len(payload.Items) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(payload.Items))
	}
	if payload.Total != 1 {
		t.Errorf("expected total=1, got %d", payload.Total)
	}

	// Delete transaction
	rr = doRequest(router, "DELETE", "/api/transactions/1", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE /api/transactions: expected 200, got %d", rr.Code)
	}
}

func TestTransactionsEndpoints_ValidationErrors(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Create account first
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id":   "test-account",
		"account_name": "Test Account",
	})

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "missing symbol",
			body: map[string]interface{}{
				"transaction_type": "BUY",
				"quantity":         100,
				"price":            150,
				"currency":         "USD",
				"account_id":       "test-account",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid transaction type",
			body: map[string]interface{}{
				"symbol":           "AAPL",
				"transaction_type": "INVALID",
				"quantity":         100,
				"price":            150,
				"currency":         "USD",
				"account_id":       "test-account",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative quantity",
			body: map[string]interface{}{
				"symbol":           "AAPL",
				"transaction_type": "BUY",
				"quantity":         -100,
				"price":            150,
				"currency":         "USD",
				"account_id":       "test-account",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := doRequest(router, "POST", "/api/transactions", tt.body)
			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestHoldingsEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Setup: create account and transactions
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id":   "test-account",
		"account_name": "Test Account",
	})
	doRequest(router, "POST", "/api/transactions", map[string]interface{}{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         100,
		"price":            150,
		"currency":         "USD",
		"account_id":       "test-account",
		"asset_type":       "stock",
	})

	// Test holdings endpoint
	rr := doRequest(router, "GET", "/api/holdings", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/holdings: expected 200, got %d", rr.Code)
	}

	// Test holdings-by-currency
	rr = doRequest(router, "GET", "/api/holdings-by-currency", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/holdings-by-currency: expected 200, got %d", rr.Code)
	}

	// Test holdings-by-symbol
	rr = doRequest(router, "GET", "/api/holdings-by-symbol", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/holdings-by-symbol: expected 200, got %d", rr.Code)
	}

	// Test holdings-by-currency-account
	rr = doRequest(router, "GET", "/api/holdings-by-currency-account", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/holdings-by-currency-account: expected 200, got %d", rr.Code)
	}
}

func TestAssetTypesEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Get asset types (should have defaults)
	rr := doRequest(router, "GET", "/api/asset-types", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/asset-types: expected 200, got %d", rr.Code)
	}

	var types []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&types)
	if len(types) < 4 {
		t.Errorf("expected at least 4 default asset types, got %d", len(types))
	}

	// Add custom asset type
	rr = doRequest(router, "POST", "/api/asset-types", map[string]interface{}{
		"code":  "crypto",
		"label": "加密货币",
	})
	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/asset-types: expected 200, got %d", rr.Code)
	}

	// Delete custom asset type
	rr = doRequest(router, "DELETE", "/api/asset-types/crypto", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE /api/asset-types: expected 200, got %d", rr.Code)
	}
}

func TestAllocationSettingsEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Get allocation settings (initially empty)
	rr := doRequest(router, "GET", "/api/allocation-settings", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/allocation-settings: expected 200, got %d", rr.Code)
	}

	// Add allocation setting
	rr = doRequest(router, "PUT", "/api/allocation-settings", map[string]interface{}{
		"currency":    "USD",
		"asset_type":  "stock",
		"min_percent": 40,
		"max_percent": 60,
	})
	if rr.Code != http.StatusOK {
		t.Errorf("PUT /api/allocation-settings: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	// Get allocation settings again
	rr = doRequest(router, "GET", "/api/allocation-settings", nil)
	var settings []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&settings)
	if len(settings) != 1 {
		t.Errorf("expected 1 setting, got %d", len(settings))
	}

	// Delete allocation setting
	rr = doRequest(router, "DELETE", "/api/allocation-settings", map[string]interface{}{
		"currency":   "USD",
		"asset_type": "stock",
	})
	if rr.Code != http.StatusOK {
		t.Errorf("DELETE /api/allocation-settings: expected 200, got %d", rr.Code)
	}
}

func TestSymbolsEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Setup: create account and transaction to create a symbol
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id":   "test-account",
		"account_name": "Test Account",
	})
	doRequest(router, "POST", "/api/transactions", map[string]interface{}{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         100,
		"price":            150,
		"currency":         "USD",
		"account_id":       "test-account",
		"asset_type":       "stock",
	})

	// Get symbols
	rr := doRequest(router, "GET", "/api/symbols", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/symbols: expected 200, got %d", rr.Code)
	}

	// Update symbol
	rr = doRequest(router, "PUT", "/api/symbols/AAPL", map[string]interface{}{
		"name": "Apple Inc.",
	})
	if rr.Code != http.StatusOK {
		t.Errorf("PUT /api/symbols: expected 200, got %d", rr.Code)
	}

	// Update symbol asset type
	rr = doRequest(router, "POST", "/api/symbols/AAPL/asset-type", map[string]interface{}{
		"asset_type": "stock",
	})
	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/symbols/asset-type: expected 200, got %d", rr.Code)
	}

	// Update symbol auto-update
	rr = doRequest(router, "POST", "/api/symbols/AAPL/auto-update", map[string]interface{}{
		"auto_update": 0,
	})
	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/symbols/auto-update: expected 200, got %d", rr.Code)
	}
}

func TestPricesEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Setup: create account and transaction
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id":   "test-account",
		"account_name": "Test Account",
	})
	doRequest(router, "POST", "/api/transactions", map[string]interface{}{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         100,
		"price":            150,
		"currency":         "USD",
		"account_id":       "test-account",
		"asset_type":       "stock",
	})

	// Manual price update
	rr := doRequest(router, "POST", "/api/prices/manual", map[string]interface{}{
		"symbol":   "AAPL",
		"currency": "USD",
		"price":    160.50,
	})
	if rr.Code != http.StatusOK {
		t.Errorf("POST /api/prices/manual: expected 200, got %d", rr.Code)
	}

	// Note: We don't test /api/prices/update or /api/prices/update-all
	// as they depend on external API calls
}

func TestOperationLogsEndpoint(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Get operation logs
	rr := doRequest(router, "GET", "/api/operation-logs", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/operation-logs: expected 200, got %d", rr.Code)
	}
}

func TestPortfolioHistoryEndpoint(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Setup: create account and transactions
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id":   "test-account",
		"account_name": "Test Account",
	})
	doRequest(router, "POST", "/api/transactions", map[string]interface{}{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         100,
		"price":            150,
		"currency":         "USD",
		"account_id":       "test-account",
		"asset_type":       "stock",
	})

	// Get portfolio history
	rr := doRequest(router, "GET", "/api/portfolio-history", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/portfolio-history: expected 200, got %d", rr.Code)
	}
}

func TestCORS(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Test preflight request
	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Should allow localhost origin
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("OPTIONS /api/health: expected 200 or 204, got %d", rr.Code)
	}
}
