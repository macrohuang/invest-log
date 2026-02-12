package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"investlog/pkg/investlog"
)

func TestAIHoldingsAnalysisEndpoint(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()
	strategyPrompt := "优先控制回撤，不新增中概股"

	// Seed minimal holdings.
	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "acc-ai",
		"account_name": "AI Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         10,
		"price":            100,
		"currency":         "USD",
		"account_id":       "acc-ai",
		"asset_type":       "stock",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read ai request body: %v", err)
		}
		if !bytes.Contains(body, []byte(strategyPrompt)) {
			t.Fatalf("expected strategy prompt in ai request body, got: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"mock-model","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"f1\"],\"recommendations\":[{\"symbol\":\"AAPL\",\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"长期持有\"}],\"disclaimer\":\"仅供参考\"}"}}]}`))
	}))
	defer server.Close()

	rr := doRequest(router, http.MethodPost, "/api/ai/holdings-analysis", map[string]any{
		"base_url":          server.URL,
		"api_key":           "key",
		"model":             "mock-model",
		"currency":          "USD",
		"risk_profile":      "balanced",
		"horizon":           "medium",
		"advice_style":      "balanced",
		"allow_new_symbols": true,
		"strategy_prompt":   strategyPrompt,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/ai/holdings-analysis: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["overall_summary"] == nil {
		t.Fatalf("expected overall_summary, got %v", resp)
	}

	// Missing api_key should fail.
	rr = doRequest(router, http.MethodPost, "/api/ai/holdings-analysis", map[string]any{
		"base_url": server.URL,
		"model":    "mock-model",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing api_key: expected 400, got %d", rr.Code)
	}
}

func TestAIHoldingsAnalysisEndpoint_DefaultAllowNewSymbols(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "acc-default",
		"account_name": "Default Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "MSFT",
		"transaction_type": "BUY",
		"quantity":         5,
		"price":            200,
		"currency":         "USD",
		"account_id":       "acc-default",
		"asset_type":       "stock",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"f\"],\"recommendations\":[{\"action\":\"add\",\"theory_tag\":\"Dalio\",\"rationale\":\"提升分散\"}],\"disclaimer\":\"仅供参考\"}"}}]}`))
	}))
	defer server.Close()

	// omit allow_new_symbols, should default to true via handler.
	rr := doRequest(router, http.MethodPost, "/api/ai/holdings-analysis", map[string]any{
		"base_url":     server.URL,
		"api_key":      "key",
		"model":        "mock",
		"currency":     "USD",
		"risk_profile": "balanced",
		"horizon":      "medium",
		"advice_style": "balanced",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAIHoldingsAnalysisEndpoint_PropagatesCoreError(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// no holdings => should return bad request from core
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer server.Close()

	rr := doRequest(router, http.MethodPost, "/api/ai/holdings-analysis", map[string]any{
		"base_url": server.URL,
		"api_key":  "key",
		"model":    "mock",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no holdings, got %d", rr.Code)
	}
}

func TestAIHoldingsAnalysisHandlerDecodesUnknownField(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// decodeJSON disallows unknown fields.
	rr := doRequest(router, http.MethodPost, "/api/ai/holdings-analysis", map[string]any{
		"base_url": "https://example.com",
		"api_key":  "key",
		"model":    "model",
		"unknown":  "x",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field, got %d", rr.Code)
	}
}

var _ = investlog.HoldingsAnalysisResult{}
