package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPricesEndpoints_UpdateAndUpdateAll(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Update price for CASH should not hit external APIs.
	rr := doRequest(router, http.MethodPost, "/api/prices/update", map[string]any{
		"symbol":   "CASH",
		"currency": "USD",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/prices/update: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}
	var updateResp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&updateResp)
	if updateResp["price"] == nil {
		t.Fatalf("expected price in response, got %v", updateResp)
	}

	// Create account and cash transaction so update-all has a symbol to update.
	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "cash-account",
		"account_name": "Cash Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "CASH",
		"transaction_type": "TRANSFER_IN",
		"quantity":         100,
		"price":            1,
		"currency":         "USD",
		"account_id":       "cash-account",
		"asset_type":       "cash",
	})

	rr = doRequest(router, http.MethodPost, "/api/prices/update-all", map[string]any{
		"currency": "USD",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/prices/update-all: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}
	var updateAllResp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&updateAllResp)
	if updateAllResp["updated"] == nil {
		t.Fatalf("expected updated in response, got %v", updateAllResp)
	}

	// Missing currency should return 400.
	rr = doRequest(router, http.MethodPost, "/api/prices/update-all", map[string]any{
		"currency": "CNY",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/prices/update-all (missing currency): expected 400, got %d", rr.Code)
	}
}

func TestParseHelpers(t *testing.T) {
	if got := parseInt(""); got != 0 {
		t.Fatalf("parseInt empty: got %d", got)
	}
	if got := parseInt("10"); got != 10 {
		t.Fatalf("parseInt 10: got %d", got)
	}
	if got := parseInt("bad"); got != 0 {
		t.Fatalf("parseInt bad: got %d", got)
	}

	if got := parseIntDefault("", 5); got != 5 {
		t.Fatalf("parseIntDefault empty: got %d", got)
	}
	if got := parseIntDefault("9", 5); got != 9 {
		t.Fatalf("parseIntDefault 9: got %d", got)
	}
	if got := parseIntDefault("bad", 7); got != 7 {
		t.Fatalf("parseIntDefault bad: got %d", got)
	}

	value := "hello"
	ptr := ptrString(value)
	if ptr == nil || *ptr != value {
		t.Fatalf("ptrString: expected %q", value)
	}
}
