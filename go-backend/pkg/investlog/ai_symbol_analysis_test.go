package investlog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Dimension stub responses used across tests.
const (
	stubMacroJSON         = `{"dimension":"macro","rating":"positive","confidence":"medium","key_points":["低利率环境有利"],"risks":["通胀压力"],"opportunities":["政策刺激"],"summary":"宏观环境整体有利"}`
	stubIndustryJSON      = `{"dimension":"industry","rating":"positive","confidence":"high","key_points":["行业增长强劲"],"risks":["竞争加剧"],"opportunities":["AI驱动增长"],"summary":"行业前景积极"}`
	stubCompanyJSON       = `{"dimension":"company","rating":"positive","confidence":"high","key_points":["营收稳健增长"],"risks":["估值偏高"],"opportunities":["新产品周期"],"summary":"基本面优良","valuation_assessment":"估值合理"}`
	stubInternationalJSON = `{"dimension":"international","rating":"neutral","confidence":"medium","key_points":["贸易关系稳定"],"risks":["地缘政治不确定"],"opportunities":["全球化布局"],"summary":"国际环境中性"}`
	stubSynthesisJSON     = `{"overall_rating":"buy","confidence":"medium","target_action":"increase","position_suggestion":"建议持有并适度加仓","overall_summary":"综合四维度分析看好","key_factors":["行业增长","基本面优良"],"risk_warnings":["估值偏高","地缘政治"],"action_items":[{"action":"适度加仓","rationale":"基本面支撑","priority":"medium"}],"time_horizon_notes":"中长期持有","disclaimer":"仅供参考，不构成投资建议"}`
)

// dimensionStubRouter inspects the system prompt to return the matching dimension JSON.
// The synthesis prompt is checked first because it also mentions dimension keywords like "宏观".
func dimensionStubRouter(_ context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
	sp := req.SystemPrompt
	switch {
	case strings.Contains(sp, "综合投资分析师"):
		return aiChatCompletionResult{Model: "mock", Content: stubSynthesisJSON}, nil
	case strings.Contains(sp, "宏观经济政策分析"):
		return aiChatCompletionResult{Model: "mock", Content: stubMacroJSON}, nil
	case strings.Contains(sp, "行业趋势分析"):
		return aiChatCompletionResult{Model: "mock", Content: stubIndustryJSON}, nil
	case strings.Contains(sp, "公司基本面分析"):
		return aiChatCompletionResult{Model: "mock", Content: stubCompanyJSON}, nil
	case strings.Contains(sp, "国际政治经济分析"):
		return aiChatCompletionResult{Model: "mock", Content: stubInternationalJSON}, nil
	default:
		return aiChatCompletionResult{}, errors.New("unknown dimension")
	}
}

func TestAnalyzeSymbol_EndToEnd(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()
	aiChatCompletion = dimensionStubRouter

	result, err := core.AnalyzeSymbol(SymbolAnalysisRequest{
		BaseURL:  "https://example.com/v1",
		APIKey:   "test-key",
		Model:    "mock-model",
		Symbol:   "AAPL",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("AnalyzeSymbol failed: %v", err)
	}

	// Status
	if result.Status != "completed" {
		t.Fatalf("expected status completed, got %s", result.Status)
	}
	// Symbol / Currency
	if result.Symbol != "AAPL" {
		t.Fatalf("expected symbol AAPL, got %s", result.Symbol)
	}
	if result.Currency != "USD" {
		t.Fatalf("expected currency USD, got %s", result.Currency)
	}

	// Dimensions
	expectedDims := map[string]string{
		"macro":         "positive",
		"industry":      "positive",
		"company":       "positive",
		"international": "neutral",
	}
	for dim, expectedRating := range expectedDims {
		d, ok := result.Dimensions[dim]
		if !ok {
			t.Fatalf("missing dimension %s", dim)
		}
		if d.Rating != expectedRating {
			t.Fatalf("dimension %s: expected rating %s, got %s", dim, expectedRating, d.Rating)
		}
	}

	// Company dimension should have valuation_assessment
	if result.Dimensions["company"].ValuationAssessment != "估值合理" {
		t.Fatalf("expected valuation_assessment=估值合理, got %q", result.Dimensions["company"].ValuationAssessment)
	}

	// Synthesis
	if result.Synthesis == nil {
		t.Fatal("expected synthesis to be non-nil")
	}
	if result.Synthesis.OverallRating != "buy" {
		t.Fatalf("expected overall_rating=buy, got %q", result.Synthesis.OverallRating)
	}
	if result.Synthesis.Confidence != "medium" {
		t.Fatalf("expected confidence=medium, got %s", result.Synthesis.Confidence)
	}
	if len(result.Synthesis.ActionItems) == 0 {
		t.Fatal("expected at least one action item")
	}
}

func TestAnalyzeSymbol_Validation(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name    string
		req     SymbolAnalysisRequest
		wantErr string
	}{
		{
			name:    "missing api_key",
			req:     SymbolAnalysisRequest{Model: "m", Symbol: "AAPL", Currency: "USD"},
			wantErr: "api_key is required",
		},
		{
			name:    "missing model",
			req:     SymbolAnalysisRequest{APIKey: "k", Symbol: "AAPL", Currency: "USD"},
			wantErr: "model is required",
		},
		{
			name:    "missing symbol",
			req:     SymbolAnalysisRequest{APIKey: "k", Model: "m", Currency: "USD"},
			wantErr: "symbol is required",
		},
		{
			name:    "empty currency",
			req:     SymbolAnalysisRequest{APIKey: "k", Model: "m", Symbol: "AAPL", Currency: ""},
			wantErr: "currency is required",
		},
		{
			name:    "invalid currency EUR",
			req:     SymbolAnalysisRequest{APIKey: "k", Model: "m", Symbol: "AAPL", Currency: "EUR"},
			wantErr: "invalid currency",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := core.AnalyzeSymbol(tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestAnalyzeSymbol_PartialFailure(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	// macro agent fails; the other 3 dimensions + synthesis succeed.
	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		sp := req.SystemPrompt
		if strings.Contains(sp, "宏观经济政策分析") {
			return aiChatCompletionResult{}, errors.New("macro agent timeout")
		}
		return dimensionStubRouter(ctx, req)
	}

	result, err := core.AnalyzeSymbol(SymbolAnalysisRequest{
		BaseURL:  "https://example.com/v1",
		APIKey:   "test-key",
		Model:    "mock-model",
		Symbol:   "AAPL",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("AnalyzeSymbol should succeed with partial failure, got: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}

	// macro should be missing, the other 3 should be present.
	if _, ok := result.Dimensions["macro"]; ok {
		t.Fatal("macro dimension should be absent after failure")
	}
	expectedPresent := []string{"industry", "company", "international"}
	for _, dim := range expectedPresent {
		if _, ok := result.Dimensions[dim]; !ok {
			t.Fatalf("expected dimension %s to be present", dim)
		}
	}

	if result.Synthesis == nil {
		t.Fatal("synthesis should still be present")
	}
}

func TestAnalyzeSymbol_AllFail(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		sp := req.SystemPrompt
		// Let synthesis through (won't be reached), fail all dimension agents.
		if strings.Contains(sp, "综合") {
			return dimensionStubRouter(ctx, req)
		}
		return aiChatCompletionResult{}, errors.New("agent failed")
	}

	_, err := core.AnalyzeSymbol(SymbolAnalysisRequest{
		BaseURL:  "https://example.com/v1",
		APIKey:   "test-key",
		Model:    "mock-model",
		Symbol:   "AAPL",
		Currency: "USD",
	})
	if err == nil {
		t.Fatal("expected error when all dimension agents fail")
	}
	if !strings.Contains(err.Error(), "too many dimension agents failed") {
		t.Fatalf("expected 'too many dimension agents failed' error, got: %v", err)
	}
}

func TestGetSymbolAnalysis(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert a completed row directly.
	_, err := core.db.Exec(
		`INSERT INTO symbol_analyses
		 (symbol, currency, model, status, macro_analysis, synthesis, created_at, completed_at)
		 VALUES (?, ?, ?, 'completed', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"AAPL", "USD", "test-model", stubMacroJSON, stubSynthesisJSON,
	)
	if err != nil {
		t.Fatalf("insert test row: %v", err)
	}

	result, err := core.GetSymbolAnalysis("aapl", "usd")
	if err != nil {
		t.Fatalf("GetSymbolAnalysis failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Symbol != "AAPL" {
		t.Fatalf("expected AAPL, got %s", result.Symbol)
	}
	if result.Currency != "USD" {
		t.Fatalf("expected USD, got %s", result.Currency)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}

	// Verify macro dimension parsed correctly.
	macro, ok := result.Dimensions["macro"]
	if !ok {
		t.Fatal("expected macro dimension")
	}
	if macro.Rating != "positive" {
		t.Fatalf("expected positive, got %s", macro.Rating)
	}
	if macro.Confidence != "medium" {
		t.Fatalf("expected medium confidence, got %s", macro.Confidence)
	}

	// Verify synthesis parsed correctly.
	if result.Synthesis == nil {
		t.Fatal("expected synthesis")
	}
	if result.Synthesis.OverallRating != "buy" {
		t.Fatalf("expected buy, got %s", result.Synthesis.OverallRating)
	}
	if result.Synthesis.Disclaimer != "仅供参考，不构成投资建议" {
		t.Fatalf("unexpected disclaimer: %s", result.Synthesis.Disclaimer)
	}

	// Non-existent symbol returns nil.
	missing, err := core.GetSymbolAnalysis("ZZZZ", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for non-existent symbol")
	}
}

func TestGetSymbolAnalysisHistory(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert 3 completed rows with distinct timestamps.
	for i := 0; i < 3; i++ {
		_, err := core.db.Exec(
			`INSERT INTO symbol_analyses
			 (symbol, currency, model, status, macro_analysis, synthesis, created_at, completed_at)
			 VALUES (?, ?, ?, 'completed', ?, ?, datetime('now', ?), datetime('now', ?))`,
			"AAPL", "USD", "model-v"+string(rune('1'+i)),
			stubMacroJSON, stubSynthesisJSON,
			// Offset each row by -i minutes so row 0 is most recent.
			"-"+string(rune('0'+i))+" minutes",
			"-"+string(rune('0'+i))+" minutes",
		)
		if err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}

	// Fetch all.
	results, err := core.GetSymbolAnalysisHistory("AAPL", "USD", 10)
	if err != nil {
		t.Fatalf("GetSymbolAnalysisHistory failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Most recent first: row 0 has model-v1 (created_at = now-0), row 1 = model-v2 (now-1), ...
	if results[0].Model != "model-v1" {
		t.Fatalf("expected most recent model-v1 first, got %s", results[0].Model)
	}
	if results[2].Model != "model-v3" {
		t.Fatalf("expected oldest model-v3 last, got %s", results[2].Model)
	}

	// Verify limit works.
	limited, err := core.GetSymbolAnalysisHistory("AAPL", "USD", 2)
	if err != nil {
		t.Fatalf("GetSymbolAnalysisHistory with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("expected 2 results with limit, got %d", len(limited))
	}

	// Non-existent symbol returns empty slice.
	empty, err := core.GetSymbolAnalysisHistory("ZZZZ", "USD", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty list, got %d", len(empty))
	}
}

func TestBuildSymbolContext(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")

	ctxJSON, err := core.buildSymbolContext("AAPL", "USD")
	if err != nil {
		t.Fatalf("buildSymbolContext failed: %v", err)
	}

	var ctx symbolContextData
	if err := json.Unmarshal([]byte(ctxJSON), &ctx); err != nil {
		t.Fatalf("unmarshal context: %v", err)
	}

	if ctx.Symbol != "AAPL" {
		t.Fatalf("expected symbol AAPL, got %s", ctx.Symbol)
	}
	if ctx.Currency != "USD" {
		t.Fatalf("expected currency USD, got %s", ctx.Currency)
	}
	if ctx.TotalShares != 10 {
		t.Fatalf("expected 10 shares, got %f", ctx.TotalShares)
	}
	if ctx.AvgCost != 100 {
		t.Fatalf("expected avg cost 100, got %f", ctx.AvgCost)
	}
	if ctx.CostBasis != 1000 {
		t.Fatalf("expected cost basis 1000, got %f", ctx.CostBasis)
	}

	// Symbol not held: should still succeed with minimal data.
	ctxJSON2, err := core.buildSymbolContext("MSFT", "USD")
	if err != nil {
		t.Fatalf("buildSymbolContext for unheld symbol failed: %v", err)
	}
	var ctx2 symbolContextData
	if err := json.Unmarshal([]byte(ctxJSON2), &ctx2); err != nil {
		t.Fatalf("unmarshal context2: %v", err)
	}
	if ctx2.Symbol != "MSFT" {
		t.Fatalf("expected MSFT, got %s", ctx2.Symbol)
	}
	if ctx2.TotalShares != 0 {
		t.Fatalf("expected 0 shares for unheld symbol, got %f", ctx2.TotalShares)
	}
}

func TestParseSymbolDimensionResult(t *testing.T) {
	t.Parallel()

	// Valid JSON
	parsed, err := parseSymbolDimensionResult(stubMacroJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dimension != "macro" {
		t.Fatalf("expected dimension macro, got %s", parsed.Dimension)
	}
	if parsed.Rating != "positive" {
		t.Fatalf("expected rating positive, got %s", parsed.Rating)
	}
	if parsed.Confidence != "medium" {
		t.Fatalf("expected confidence medium, got %s", parsed.Confidence)
	}
	if len(parsed.KeyPoints) != 1 || parsed.KeyPoints[0] != "低利率环境有利" {
		t.Fatalf("unexpected key_points: %v", parsed.KeyPoints)
	}
	if len(parsed.Risks) != 1 || parsed.Risks[0] != "通胀压力" {
		t.Fatalf("unexpected risks: %v", parsed.Risks)
	}

	// Malformed JSON
	_, err = parseSymbolDimensionResult("not valid json {{{")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse dimension result") {
		t.Fatalf("expected 'parse dimension result' in error, got: %v", err)
	}
}

func TestNormalizeSymbolAnalysisRequest(t *testing.T) {
	t.Parallel()

	// Trimming and uppercasing.
	req, err := normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{
		APIKey:         "  my-key  ",
		Model:          "  gpt-4o  ",
		Symbol:         "  aapl  ",
		Currency:       "  usd  ",
		StrategyPrompt: "  偏好低波动  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.APIKey != "my-key" {
		t.Fatalf("expected trimmed api key, got %q", req.APIKey)
	}
	if req.Model != "gpt-4o" {
		t.Fatalf("expected trimmed model, got %q", req.Model)
	}
	if req.Symbol != "AAPL" {
		t.Fatalf("expected uppercased symbol AAPL, got %q", req.Symbol)
	}
	if req.Currency != "USD" {
		t.Fatalf("expected uppercased currency USD, got %q", req.Currency)
	}
	if req.StrategyPrompt != "偏好低波动" {
		t.Fatalf("expected trimmed strategy prompt, got %q", req.StrategyPrompt)
	}

	// Missing api_key
	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{Model: "m", Symbol: "X", Currency: "USD"})
	if err == nil || !strings.Contains(err.Error(), "api_key is required") {
		t.Fatalf("expected api_key required error, got %v", err)
	}

	// Missing model
	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{APIKey: "k", Symbol: "X", Currency: "USD"})
	if err == nil || !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("expected model required error, got %v", err)
	}

	// Missing symbol
	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{APIKey: "k", Model: "m", Currency: "USD"})
	if err == nil || !strings.Contains(err.Error(), "symbol is required") {
		t.Fatalf("expected symbol required error, got %v", err)
	}

	// Empty currency
	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{APIKey: "k", Model: "m", Symbol: "X"})
	if err == nil || !strings.Contains(err.Error(), "currency is required") {
		t.Fatalf("expected currency required error, got %v", err)
	}

	// Invalid currency
	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{APIKey: "k", Model: "m", Symbol: "X", Currency: "EUR"})
	if err == nil || !strings.Contains(err.Error(), "invalid currency") {
		t.Fatalf("expected invalid currency error, got %v", err)
	}
}
