package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestExchangeRatesEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodGet, "/api/exchange-rates", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/exchange-rates: expected 200, got %d", rr.Code)
	}

	var rates []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&rates); err != nil {
		t.Fatalf("decode exchange rates response: %v", err)
	}
	if len(rates) < 2 {
		t.Fatalf("expected at least 2 rates, got %d", len(rates))
	}

	rr = doRequest(router, http.MethodPut, "/api/exchange-rates", map[string]any{
		"from_currency": "USD",
		"to_currency":   "CNY",
		"rate":          7.3,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/exchange-rates: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(router, http.MethodPut, "/api/exchange-rates", map[string]any{
		"from_currency": "EUR",
		"to_currency":   "CNY",
		"rate":          7.3,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT /api/exchange-rates invalid payload: expected 400, got %d", rr.Code)
	}

	rr = doRequest(router, http.MethodPost, "/api/exchange-rates/refresh", map[string]any{})
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Fatalf("POST /api/exchange-rates/refresh: expected 200 or 500, got %d", rr.Code)
	}
	if rr.Code == http.StatusOK {
		var payload map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatalf("decode refresh response: %v", err)
		}
		if _, ok := payload["updated"]; !ok {
			t.Fatalf("expected updated field in refresh response")
		}
	}
}
