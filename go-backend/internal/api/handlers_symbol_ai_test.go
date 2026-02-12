package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"investlog/pkg/investlog"
)

func TestSymbolAnalysisEndpoint_Success(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Seed account + BUY transaction.
	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "acc-sym",
		"account_name": "Symbol AI Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         10,
		"price":            150,
		"currency":         "USD",
		"account_id":       "acc-sym",
		"asset_type":       "stock",
	})

	// Mock AI server that returns dimension/synthesis JSON based on request body content.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read ai request body: %v", err)
		}
		bodyStr := string(body)

		var content string
		switch {
		case strings.Contains(bodyStr, "\u5b8f\u89c2"): // 宏观
			content = `{"dimension":"macro","rating":"positive","confidence":"medium","key_points":["低利率环境有利"],"risks":["通胀压力"],"opportunities":["政策刺激"],"summary":"宏观环境整体有利"}`
		case strings.Contains(bodyStr, "\u884c\u4e1a"): // 行业
			content = `{"dimension":"industry","rating":"positive","confidence":"high","key_points":["行业增长强劲"],"risks":["竞争加剧"],"opportunities":["AI驱动增长"],"summary":"行业前景积极"}`
		case strings.Contains(bodyStr, "\u516c\u53f8\u57fa\u672c\u9762"): // 公司基本面
			content = `{"dimension":"company","rating":"positive","confidence":"high","key_points":["营收稳健增长"],"risks":["估值偏高"],"opportunities":["新产品周期"],"summary":"基本面优良","valuation_assessment":"估值合理"}`
		case strings.Contains(bodyStr, "\u56fd\u9645\u653f\u6cbb\u7ecf\u6d4e"): // 国际政治经济
			content = `{"dimension":"international","rating":"neutral","confidence":"medium","key_points":["贸易关系稳定"],"risks":["地缘政治不确定"],"opportunities":["全球化布局"],"summary":"国际环境中性"}`
		case strings.Contains(bodyStr, "\u7efc\u5408"): // 综合
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

	rr := doRequest(router, http.MethodPost, "/api/ai/symbol-analysis", map[string]any{
		"base_url":        server.URL,
		"api_key":         "test-key",
		"model":           "mock-model",
		"symbol":          "AAPL",
		"currency":        "USD",
		"strategy_prompt": "长期持有科技股",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/ai/symbol-analysis: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	synthesis, ok := resp["synthesis"].(map[string]any)
	if !ok || synthesis == nil {
		t.Fatalf("expected synthesis in response, got %v", resp)
	}
	if synthesis["overall_rating"] == nil {
		t.Fatalf("expected synthesis.overall_rating, got %v", synthesis)
	}
}

func TestSymbolAnalysisEndpoint_MissingKey(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodPost, "/api/ai/symbol-analysis", map[string]any{
		"base_url": "https://example.com",
		"model":    "mock-model",
		"symbol":   "AAPL",
		"currency": "USD",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing api_key: expected 400, got %d, body: %s", rr.Code, rr.Body.String())
	}
}

func TestGetSymbolAnalysis_Empty(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	rr := doRequest(router, http.MethodGet, "/api/ai/symbol-analysis?symbol=AAPL&currency=USD", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/ai/symbol-analysis: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	// Should return null (no prior analysis).
	body := strings.TrimSpace(rr.Body.String())
	if body != "null" {
		t.Fatalf("expected null body for empty analysis, got: %s", body)
	}
}

func TestGetSymbolAnalysisHistory_Limit(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Seed account + BUY transaction.
	doRequest(router, http.MethodPost, "/api/accounts", map[string]any{
		"account_id":   "acc-hist",
		"account_name": "History Account",
	})
	doRequest(router, http.MethodPost, "/api/transactions", map[string]any{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         5,
		"price":            100,
		"currency":         "USD",
		"account_id":       "acc-hist",
		"asset_type":       "stock",
	})

	// Mock AI server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
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

	// Run analysis to populate history.
	rr := doRequest(router, http.MethodPost, "/api/ai/symbol-analysis", map[string]any{
		"base_url": server.URL,
		"api_key":  "test-key",
		"model":    "mock-model",
		"symbol":   "AAPL",
		"currency": "USD",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/ai/symbol-analysis: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	// GET history with limit=5.
	rr = doRequest(router, http.MethodGet, "/api/ai/symbol-analysis/history?symbol=AAPL&currency=USD&limit=5", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/ai/symbol-analysis/history: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var results []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	if len(results) < 1 {
		t.Fatalf("expected at least 1 history entry, got %d", len(results))
	}
}

// Ensure the types compile.
var (
	_ = investlog.SymbolAnalysisResult{}
	_ = investlog.SymbolSynthesisResult{}
)

// Suppress unused import warnings.
var (
	_ = bytes.NewBuffer
	_ = io.ReadAll
	_ = httptest.NewServer
)
