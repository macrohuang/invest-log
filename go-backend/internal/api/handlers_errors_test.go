package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doRawRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestTransactionDeleteErrors(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodDelete, "/api/transactions/notanint", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", rr.Code)
	}

	rr = doRequest(router, http.MethodDelete, "/api/transactions/999", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing transaction, got %d", rr.Code)
	}
}

func TestPriceEndpointsErrors(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRawRequest(router, http.MethodPost, "/api/prices/update", "{bad}")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", rr.Code)
	}

	rr = doRequest(router, http.MethodPost, "/api/prices/update", map[string]any{
		"symbol":   "???",
		"currency": "CNY",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown symbol, got %d", rr.Code)
	}

	rr = doRawRequest(router, http.MethodPost, "/api/prices/manual", "{bad}")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad manual JSON, got %d", rr.Code)
	}

	rr = doRawRequest(router, http.MethodPost, "/api/prices/update-all", "{bad}")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad update-all JSON, got %d", rr.Code)
	}
}

func TestAllocationSettingErrors(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Invalid asset type should error.
	rr := doRequest(router, http.MethodPut, "/api/allocation-settings", map[string]any{
		"currency":    "USD",
		"asset_type":  "invalid",
		"min_percent": 10,
		"max_percent": 20,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid asset type, got %d", rr.Code)
	}

	// Deleting non-existent setting should 404.
	rr = doRequest(router, http.MethodDelete, "/api/allocation-settings", map[string]any{
		"currency":   "USD",
		"asset_type": "stock",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing setting, got %d", rr.Code)
	}
}

func TestAssetTypeErrors(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Missing fields.
	rr := doRequest(router, http.MethodPost, "/api/asset-types", map[string]any{
		"code": "",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid asset type, got %d", rr.Code)
	}

	// Duplicate asset type.
	rr = doRequest(router, http.MethodPost, "/api/asset-types", map[string]any{
		"code":  "crypto",
		"label": "Crypto",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for add asset type, got %d", rr.Code)
	}
	rr = doRequest(router, http.MethodPost, "/api/asset-types", map[string]any{
		"code":  "crypto",
		"label": "Crypto",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate asset type, got %d", rr.Code)
	}

	// Deleting in-use asset type should fail.
	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "acct",
		"account_name": "Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         1,
		"price":            10,
		"currency":         "USD",
		"account_id":       "acct",
		"asset_type":       "stock",
	})
	rr = doRequest(router, http.MethodDelete, "/api/asset-types/stock", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for in-use asset type, got %d", rr.Code)
	}

	// Deleting missing asset type should 400.
	rr = doRequest(router, http.MethodDelete, "/api/asset-types/does-not-exist", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing asset type, got %d", rr.Code)
	}
}

func TestSymbolUpdateErrors(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPut, "/api/symbols/UNKNOWN", map[string]any{
		"name": "Unknown",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing symbol, got %d", rr.Code)
	}

	rr = doRequest(router, http.MethodPost, "/api/symbols/UNKNOWN/asset-type", map[string]any{
		"asset_type": "stock",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing symbol asset type, got %d", rr.Code)
	}
}

func TestAccountDeleteError(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodDelete, "/api/accounts/missing", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing account, got %d", rr.Code)
	}
}
