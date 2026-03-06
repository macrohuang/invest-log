package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIAnalysisMethodsEndpoints(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	createResp := doRequest(router, http.MethodPost, "/api/ai-analysis-methods", map[string]any{
		"name":          "估值复盘",
		"system_prompt": "Analyze ${SYMBOL}",
		"user_prompt":   "Answer in ${LANGUAGE}",
	})
	if createResp.Code != http.StatusOK {
		t.Fatalf("POST /api/ai-analysis-methods: expected 200, got %d, body=%s", createResp.Code, createResp.Body.String())
	}

	var created map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created method: %v", err)
	}
	if created["name"] != "估值复盘" {
		t.Fatalf("unexpected name: %v", created["name"])
	}
	vars, ok := created["variables"].([]any)
	if !ok || len(vars) != 2 {
		t.Fatalf("unexpected variables: %#v", created["variables"])
	}

	listResp := doRequest(router, http.MethodGet, "/api/ai-analysis-methods", nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("GET /api/ai-analysis-methods: expected 200, got %d", listResp.Code)
	}

	updateResp := doRequest(router, http.MethodPut, "/api/ai-analysis-methods/1", map[string]any{
		"name":          "财报拆解",
		"system_prompt": "Analyze ${TICKER}",
		"user_prompt":   "Explain ${QUESTION}",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("PUT /api/ai-analysis-methods/1: expected 200, got %d, body=%s", updateResp.Code, updateResp.Body.String())
	}

	deleteResp := doRequest(router, http.MethodDelete, "/api/ai-analysis-methods/1", nil)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/ai-analysis-methods/1: expected 200, got %d, body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestAIAnalysisStreamEndpointSuccess(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	server := setupAIStubServer(t, `{"model":"mock-model","choices":[{"message":{"content":"Streamed analysis"}}]}`)
	defer server.Close()

	rr := doRequest(router, http.MethodPut, "/api/ai-settings", map[string]any{
		"base_url": server.URL,
		"model":    "mock-model",
		"api_key":  "test-key",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/ai-settings: expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	rr = doRequest(router, http.MethodPost, "/api/ai-analysis-methods", map[string]any{
		"name":          "股票速览",
		"system_prompt": "Analyze ${SYMBOL}",
		"user_prompt":   "Question ${QUESTION}",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/ai-analysis-methods: expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	streamResp := doStreamRequest(t, router, http.MethodPost, "/api/ai-analysis/stream", map[string]any{
		"method_id": 1,
		"variables": map[string]string{
			"SYMBOL":   "AAPL",
			"QUESTION": "增长如何",
		},
	})
	if streamResp.status != http.StatusOK {
		t.Fatalf("POST /api/ai-analysis/stream: expected 200, got %d, body=%s", streamResp.status, streamResp.body)
	}
	if !strings.Contains(streamResp.body, "event: progress") {
		t.Fatalf("expected progress event, got body=%s", streamResp.body)
	}
	if !strings.Contains(streamResp.body, "event: delta") {
		t.Fatalf("expected delta event, got body=%s", streamResp.body)
	}
	if !strings.Contains(streamResp.body, "event: result") {
		t.Fatalf("expected result event, got body=%s", streamResp.body)
	}
	if !strings.Contains(streamResp.body, "Streamed analysis") {
		t.Fatalf("expected final analysis text, got body=%s", streamResp.body)
	}

	historyResp := doRequest(router, http.MethodGet, "/api/ai-analysis/history?method_id=1&limit=10", nil)
	if historyResp.Code != http.StatusOK {
		t.Fatalf("GET /api/ai-analysis/history: expected 200, got %d, body=%s", historyResp.Code, historyResp.Body.String())
	}
}

func TestAIAnalysisRunDetailEndpointNotFound(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodGet, "/api/ai-analysis/runs/999", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/ai-analysis/runs/999: expected 404, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func setupAIStubServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}
