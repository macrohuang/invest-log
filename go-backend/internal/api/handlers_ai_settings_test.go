package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAISettingsEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodGet, "/api/ai-settings", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/ai-settings: expected 200, got %d", rr.Code)
	}

	var defaults map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&defaults); err != nil {
		t.Fatalf("decode defaults: %v", err)
	}
	if got := defaults["base_url"]; got != "https://api.openai.com/v1" {
		t.Fatalf("unexpected default base_url: %v", got)
	}
	if got := defaults["risk_profile"]; got != "balanced" {
		t.Fatalf("unexpected default risk_profile: %v", got)
	}
	if got := defaults["horizon"]; got != "medium" {
		t.Fatalf("unexpected default horizon: %v", got)
	}
	if got := defaults["advice_style"]; got != "balanced" {
		t.Fatalf("unexpected default advice_style: %v", got)
	}
	if got := defaults["allow_new_symbols"]; got != true {
		t.Fatalf("unexpected default allow_new_symbols: %v", got)
	}

	rr = doRequest(router, http.MethodPut, "/api/ai-settings", map[string]any{
		"base_url":          "https://example.com/v1/",
		"model":             "gpt-4.1-mini",
		"risk_profile":      "aggressive",
		"horizon":           "long",
		"advice_style":      "conservative",
		"allow_new_symbols": false,
		"strategy_prompt":   "focus on cashflow",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/ai-settings: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(router, http.MethodGet, "/api/ai-settings", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/ai-settings after PUT: expected 200, got %d", rr.Code)
	}

	var saved map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&saved); err != nil {
		t.Fatalf("decode saved: %v", err)
	}
	if got := saved["base_url"]; got != "https://example.com/v1" {
		t.Fatalf("unexpected persisted base_url: %v", got)
	}
	if got := saved["model"]; got != "gpt-4.1-mini" {
		t.Fatalf("unexpected persisted model: %v", got)
	}
	if got := saved["risk_profile"]; got != "aggressive" {
		t.Fatalf("unexpected persisted risk_profile: %v", got)
	}
	if got := saved["horizon"]; got != "long" {
		t.Fatalf("unexpected persisted horizon: %v", got)
	}
	if got := saved["advice_style"]; got != "conservative" {
		t.Fatalf("unexpected persisted advice_style: %v", got)
	}
	if got := saved["allow_new_symbols"]; got != false {
		t.Fatalf("unexpected persisted allow_new_symbols: %v", got)
	}
	if got := saved["strategy_prompt"]; got != "focus on cashflow" {
		t.Fatalf("unexpected persisted strategy_prompt: %v", got)
	}
}

func TestAISettingsEndpointsInvalidPayload(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPut, "/api/ai-settings", map[string]any{
		"allow_new_symbols": "yes",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT /api/ai-settings with invalid payload: expected 400, got %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestAISettingsEndpointDefaultAllowNewSymbolsWhenOmitted(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPut, "/api/ai-settings", map[string]any{
		"model": "gpt-4o-mini",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/ai-settings: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(router, http.MethodGet, "/api/ai-settings", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/ai-settings: expected 200, got %d", rr.Code)
	}

	var got map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if got["allow_new_symbols"] != true {
		t.Fatalf("expected allow_new_symbols=true when omitted, got %v", got["allow_new_symbols"])
	}
}

func TestAISettingsEndpointsDBClosed(t *testing.T) {
	router, cleanup := setupClosedRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodGet, "/api/ai-settings", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("GET /api/ai-settings on closed db: expected 500, got %d", rr.Code)
	}

	rr = doRequest(router, http.MethodPut, "/api/ai-settings", map[string]any{
		"model": "m",
	})
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("PUT /api/ai-settings on closed db: expected 500, got %d", rr.Code)
	}
}
