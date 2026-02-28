package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type streamHTTPResponse struct {
	status int
	body   string
	header http.Header
}

func doStreamRequest(t *testing.T, router http.Handler, method, path string, body any) streamHTTPResponse {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}

	server := httptest.NewServer(router)
	defer server.Close()

	req, err := http.NewRequest(method, server.URL+path, reqBody)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return streamHTTPResponse{
		status: resp.StatusCode,
		body:   string(respBody),
		header: resp.Header.Clone(),
	}
}

func TestAIHoldingsAnalysisStreamEndpoint_MissingKey(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPost, "/api/ai/holdings-analysis/stream", map[string]any{
		"base_url": "https://example.com/v1",
		"model":    "gpt-4o-mini",
		"currency": "USD",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing api_key: expected 400, got %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestAISymbolAnalysisStreamEndpoint_MissingKey(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPost, "/api/ai/symbol-analysis/stream", map[string]any{
		"base_url": "https://example.com/v1",
		"model":    "gpt-4o-mini",
		"symbol":   "AAPL",
		"currency": "USD",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing api_key: expected 400, got %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestAIAllocationAdviceStreamEndpoint_MissingKey(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPost, "/api/ai/allocation-advice/stream", map[string]any{
		"base_url": "https://example.com/v1",
		"model":    "gpt-4o-mini",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing api_key: expected 400, got %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestAIHoldingsAnalysisStreamEndpoint_RecorderEmitsErrorEvent(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPost, "/api/ai/holdings-analysis/stream", map[string]any{
		"base_url": "https://example.com/v1",
		"api_key":  "key",
		"model":    "gpt-4o-mini",
		"currency": "USD",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 stream envelope, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected error event, got body: %s", body)
	}
	if !strings.Contains(body, "\"ok\":false") {
		t.Fatalf("expected done=false marker, got body: %s", body)
	}
}

func TestAISymbolAnalysisStreamEndpoint_RecorderEmitsErrorEvent(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPost, "/api/ai/symbol-analysis/stream", map[string]any{
		"base_url": "https://example.com/v1",
		"api_key":  "key",
		"model":    "gpt-4o-mini",
		"symbol":   "AAPL",
		"currency": "USD",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 stream envelope, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected error event, got body: %s", body)
	}
	if !strings.Contains(body, "\"ok\":false") {
		t.Fatalf("expected done=false marker, got body: %s", body)
	}
}

func TestAIAllocationAdviceStreamEndpoint_RecorderEmitsErrorEvent(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPost, "/api/ai/allocation-advice/stream", map[string]any{
		"base_url": "https://example.com/v1",
		"api_key":  "key",
		"model":    "mock-model",
		"currencies": []string{
			"EUR",
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 stream envelope, got %d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected error event, got body: %s", body)
	}
	if !strings.Contains(body, "\"ok\":false") {
		t.Fatalf("expected done=false marker, got body: %s", body)
	}
}

func TestAIHoldingsAnalysisStreamEndpoint_Success(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "acc-stream",
		"account_name": "AI Stream Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         10,
		"price":            100,
		"currency":         "USD",
		"account_id":       "acc-stream",
		"asset_type":       "stock",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"mock-model","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"f1\"],\"recommendations\":[{\"symbol\":\"AAPL\",\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"长期持有\"}],\"disclaimer\":\"仅供参考\"}"}}]}`))
	}))
	defer server.Close()

	rr := doStreamRequest(t, router, http.MethodPost, "/api/ai/holdings-analysis/stream", map[string]any{
		"base_url":          server.URL,
		"api_key":           "key",
		"model":             "mock-model",
		"currency":          "USD",
		"risk_profile":      "balanced",
		"horizon":           "medium",
		"advice_style":      "balanced",
		"allow_new_symbols": true,
	})
	if rr.status != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.status, rr.body)
	}

	body := rr.body
	if !strings.Contains(body, "event: progress") {
		t.Fatalf("expected progress event, got body: %s", body)
	}
	if !strings.Contains(body, "event: delta") {
		t.Fatalf("expected delta event, got body: %s", body)
	}
	if !strings.Contains(body, "event: result") {
		t.Fatalf("expected result event, got body: %s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected done event, got body: %s", body)
	}
	if !strings.Contains(body, "overall_summary") {
		t.Fatalf("expected result payload in body: %s", body)
	}
}

func TestAISymbolAnalysisStreamEndpoint_Success(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "acc-sym-stream",
		"account_name": "Symbol Stream Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         10,
		"price":            150,
		"currency":         "USD",
		"account_id":       "acc-sym-stream",
		"asset_type":       "stock",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read ai request body: %v", err)
		}
		bodyStr := string(body)

		var content string
		switch {
		case strings.Contains(bodyStr, "\u5b8f\u89c2"):
			content = `{"dimension":"macro","rating":"positive","confidence":"medium","key_points":["低利率环境有利"],"risks":["通胀压力"],"opportunities":["政策刺激"],"summary":"宏观环境整体有利"}`
		case strings.Contains(bodyStr, "\u884c\u4e1a"):
			content = `{"dimension":"industry","rating":"positive","confidence":"high","key_points":["行业增长强劲"],"risks":["竞争加剧"],"opportunities":["AI驱动增长"],"summary":"行业前景积极"}`
		case strings.Contains(bodyStr, "\u516c\u53f8\u57fa\u672c\u9762"):
			content = `{"dimension":"company","rating":"positive","confidence":"high","key_points":["营收稳健增长"],"risks":["估值偏高"],"opportunities":["新产品周期"],"summary":"基本面优良","valuation_assessment":"估值合理"}`
		case strings.Contains(bodyStr, "\u56fd\u9645\u653f\u6cbb\u7ecf\u6d4e"):
			content = `{"dimension":"international","rating":"neutral","confidence":"medium","key_points":["贸易关系稳定"],"risks":["地缘政治不确定"],"opportunities":["全球化布局"],"summary":"国际环境中性"}`
		case strings.Contains(bodyStr, "\u7efc\u5408"):
			content = `{"overall_rating":"buy","confidence":"medium","target_action":"increase","position_suggestion":"建议持有","overall_summary":"综合看好","key_factors":["行业增长"],"risk_warnings":["估值偏高"],"action_items":[{"action":"适度加仓","rationale":"基本面支撑","priority":"medium"}],"time_horizon_notes":"中长期持有","disclaimer":"仅供参考"}`
		default:
			content = `{"dimension":"unknown","rating":"neutral","confidence":"low","key_points":[],"risks":[],"opportunities":[],"summary":"unknown"}`
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"model": "mock-model",
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rr := doStreamRequest(t, router, http.MethodPost, "/api/ai/symbol-analysis/stream", map[string]any{
		"base_url": server.URL,
		"api_key":  "key",
		"model":    "mock-model",
		"symbol":   "AAPL",
		"currency": "USD",
	})
	if rr.status != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.status, rr.body)
	}

	body := rr.body
	if !strings.Contains(body, "event: progress") {
		t.Fatalf("expected progress event, got body: %s", body)
	}
	if !strings.Contains(body, "event: result") {
		t.Fatalf("expected result event, got body: %s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected done event, got body: %s", body)
	}
	if !strings.Contains(body, "synthesis") {
		t.Fatalf("expected synthesis in stream result, got body: %s", body)
	}
}

func TestAIHoldingsAnalysisStreamEndpoint_NoHoldingsEmitsErrorEvent(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer server.Close()

	rr := doStreamRequest(t, router, http.MethodPost, "/api/ai/holdings-analysis/stream", map[string]any{
		"base_url": server.URL,
		"api_key":  "key",
		"model":    "mock-model",
	})
	if rr.status != http.StatusOK {
		t.Fatalf("expected 200 for stream envelope, got %d", rr.status)
	}

	body := rr.body
	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected error event, got body: %s", body)
	}
	if !strings.Contains(body, "\"ok\":false") {
		t.Fatalf("expected done=false marker, got body: %s", body)
	}
}

func TestAIAllocationAdviceStreamEndpoint_Success(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"mock-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"{\\\"summary\\\":\\\"ok\\\",\\\"rationale\\\":\\\"r\\\",\\\"allocations\\\":[\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"mock-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"{\\\"currency\\\":\\\"USD\\\",\\\"asset_type\\\":\\\"stock\\\",\\\"label\\\":\\\"股票\\\",\\\"min_percent\\\":10,\\\"max_percent\\\":30,\\\"rationale\\\":\\\"x\\\"}],\\\"disclaimer\\\":\\\"仅供参考\\\"}\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	rr := doStreamRequest(t, router, http.MethodPost, "/api/ai/allocation-advice/stream", map[string]any{
		"base_url":         server.URL,
		"api_key":          "key",
		"model":            "mock-model",
		"age_range":        "30s",
		"invest_goal":      "balanced",
		"risk_tolerance":   "balanced",
		"horizon":          "medium",
		"experience_level": "intermediate",
		"currencies":       []string{"USD"},
		"custom_prompt":    "偏好低回撤",
	})
	if rr.status != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rr.status, rr.body)
	}

	body := rr.body
	if !strings.Contains(body, "event: progress") {
		t.Fatalf("expected progress event, got body: %s", body)
	}
	if !strings.Contains(body, "event: delta") {
		t.Fatalf("expected delta event, got body: %s", body)
	}
	if !strings.Contains(body, "event: result") {
		t.Fatalf("expected result event, got body: %s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected done event, got body: %s", body)
	}
	if !strings.Contains(body, "allocations") {
		t.Fatalf("expected allocation payload in body: %s", body)
	}
}

func TestAIAllocationAdviceStreamEndpoint_InvalidCurrenciesEmitsErrorEvent(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doStreamRequest(t, router, http.MethodPost, "/api/ai/allocation-advice/stream", map[string]any{
		"base_url":         "https://example.com/v1",
		"api_key":          "key",
		"model":            "mock-model",
		"currencies":       []string{"EUR"},
		"age_range":        "30s",
		"invest_goal":      "balanced",
		"risk_tolerance":   "balanced",
		"horizon":          "medium",
		"experience_level": "intermediate",
	})
	if rr.status != http.StatusOK {
		t.Fatalf("expected 200 for stream envelope, got %d body=%s", rr.status, rr.body)
	}

	body := rr.body
	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected error event, got body: %s", body)
	}
	if !strings.Contains(body, "\"ok\":false") {
		t.Fatalf("expected done=false marker, got body: %s", body)
	}
}
