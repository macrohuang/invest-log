package investlog

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// Dimension stub responses used across tests.
const (
	stubMacroJSON         = `{"dimension":"macro","rating":"positive","confidence":"medium","key_points":["低利率环境有利"],"risks":["通胀压力"],"opportunities":["政策刺激"],"summary":"宏观环境整体有利"}`
	stubIndustryJSON      = `{"dimension":"industry","rating":"positive","confidence":"high","key_points":["行业增长强劲"],"risks":["竞争加剧"],"opportunities":["AI驱动增长"],"summary":"行业前景积极"}`
	stubCompanyJSON       = `{"dimension":"company","rating":"positive","confidence":"high","key_points":["营收稳健增长"],"risks":["估值偏高"],"opportunities":["新产品周期"],"summary":"基本面优良","valuation_assessment":"估值合理"}`
	stubInternationalJSON = `{"dimension":"international","rating":"neutral","confidence":"medium","key_points":["贸易关系稳定"],"risks":["地缘政治不确定"],"opportunities":["全球化布局"],"summary":"国际环境中性"}`
	stubSynthesisJSON     = `{"overall_rating":"buy","confidence":"medium","target_action":"increase","position_suggestion":"建议持有并适度加仓","overall_summary":"综合四维度分析看好","key_factors":["行业增长","基本面优良"],"risk_warnings":["估值偏高","地缘政治"],"action_items":[{"action":"适度加仓","rationale":"基本面支撑","priority":"medium"}],"time_horizon_notes":"中长期持有","disclaimer":"仅供参考，不构成投资建议"}`
)

func TestSymbolAnalysisTimeoutIsFifteenMinutes(t *testing.T) {
	t.Parallel()

	if symbolAnalysisTimeout != 15*time.Minute {
		t.Fatalf("expected symbolAnalysisTimeout to be 15m, got %s", symbolAnalysisTimeout)
	}
}

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

	origFetch := fetchExternalDataFn
	defer func() { fetchExternalDataFn = origFetch }()
	fetchExternalDataFn = func(_ context.Context, _, _ string, _ *slog.Logger) *symbolExternalData {
		return nil
	}

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

func TestAnalyzeSymbolWithStream_EmitsDelta(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-stream", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-stream")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	origFetch := fetchExternalDataFn
	defer func() { fetchExternalDataFn = origFetch }()
	fetchExternalDataFn = func(_ context.Context, _, _ string, _ *slog.Logger) *symbolExternalData {
		return nil
	}

	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		if req.OnDelta != nil {
			req.OnDelta("delta")
		}
		return dimensionStubRouter(ctx, req)
	}

	var streamed strings.Builder
	result, err := core.AnalyzeSymbolWithStream(SymbolAnalysisRequest{
		BaseURL:  "https://example.com/v1",
		APIKey:   "test-key",
		Model:    "mock-model",
		Symbol:   "AAPL",
		Currency: "USD",
	}, func(delta string) {
		streamed.WriteString(delta)
		streamed.WriteString("\n")
	})
	if err != nil {
		t.Fatalf("AnalyzeSymbolWithStream failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	text := streamed.String()
	hasDimensionDelta := strings.Contains(text, "[macro]") ||
		strings.Contains(text, "[industry]") ||
		strings.Contains(text, "[company]") ||
		strings.Contains(text, "[international]")
	if !hasDimensionDelta {
		t.Fatalf("expected at least one dimension delta prefix, got: %s", text)
	}
	if !strings.Contains(text, "[synthesis]") {
		t.Fatalf("expected synthesis delta prefix, got: %s", text)
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

	origFetch := fetchExternalDataFn
	defer func() { fetchExternalDataFn = origFetch }()
	fetchExternalDataFn = func(_ context.Context, _, _ string, _ *slog.Logger) *symbolExternalData {
		return nil
	}

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

func TestAnalyzeSymbol_SynthesisUsesPositionAndPreferences(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testAccount(t, core, "acc-2", "IRA")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")
	testBuyTransaction(t, core, "AAPL", 5, 120, "USD", "acc-2")
	testBuyTransaction(t, core, "MSFT", 20, 50, "USD", "acc-1")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	origFetch := fetchExternalDataFn
	defer func() { fetchExternalDataFn = origFetch }()
	fetchExternalDataFn = func(_ context.Context, _, _ string, _ *slog.Logger) *symbolExternalData {
		return nil
	}

	var synthesisPrompt string
	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		if strings.Contains(req.SystemPrompt, "综合投资分析师") {
			synthesisPrompt = req.UserPrompt
			return aiChatCompletionResult{Model: "mock", Content: stubSynthesisJSON}, nil
		}
		return dimensionStubRouter(ctx, req)
	}

	_, err := core.AnalyzeSymbol(SymbolAnalysisRequest{
		BaseURL:        "https://example.com/v1",
		APIKey:         "test-key",
		Model:          "mock-model",
		Symbol:         "AAPL",
		Currency:       "USD",
		RiskProfile:    "aggressive",
		Horizon:        "long",
		AdviceStyle:    "aggressive",
		StrategyPrompt: "成长股高波动策略",
	})
	if err != nil {
		t.Fatalf("AnalyzeSymbol failed: %v", err)
	}

	// Only allowed fields should appear in the synthesis prompt.
	allowed := []string{
		`"position_percent"`,
		`"avg_cost"`,
		`"risk_profile":"aggressive"`,
		`"horizon":"long"`,
		`"advice_style":"aggressive"`,
	}
	for _, want := range allowed {
		if !strings.Contains(synthesisPrompt, want) {
			t.Fatalf("expected synthesis prompt to contain %q, got: %s", want, synthesisPrompt)
		}
	}

	// Forbidden fields must NOT appear in the prompt sent to AI.
	forbidden := []string{
		`"total_shares"`,
		`"cost_basis"`,
		`"latest_price"`,
		`"market_value"`,
		`"currency_total_market_value"`,
		`"account_name"`,
		`"account_names"`,
		"成长股高波动策略",
	}
	for _, bad := range forbidden {
		if strings.Contains(synthesisPrompt, bad) {
			t.Fatalf("synthesis prompt must NOT contain %q, got: %s", bad, synthesisPrompt)
		}
	}
}

func TestAnalyzeSymbol_AllFail(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	origFetch := fetchExternalDataFn
	defer func() { fetchExternalDataFn = origFetch }()
	fetchExternalDataFn = func(_ context.Context, _, _ string, _ *slog.Logger) *symbolExternalData {
		return nil
	}

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
	testAccount(t, core, "acc-2", "IRA")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")
	testBuyTransaction(t, core, "AAPL", 5, 120, "USD", "acc-2")
	testBuyTransaction(t, core, "MSFT", 20, 50, "USD", "acc-1")

	ctx, err := core.buildSymbolContext("AAPL", "USD")
	if err != nil {
		t.Fatalf("buildSymbolContext failed: %v", err)
	}

	if ctx.Symbol != "AAPL" {
		t.Fatalf("expected symbol AAPL, got %s", ctx.Symbol)
	}
	if ctx.Currency != "USD" {
		t.Fatalf("expected currency USD, got %s", ctx.Currency)
	}
	if ctx.TotalShares != 15 {
		t.Fatalf("expected 15 shares across accounts, got %f", ctx.TotalShares)
	}
	if ctx.AvgCost != 106.67 {
		t.Fatalf("expected avg cost 106.67, got %f", ctx.AvgCost)
	}
	if ctx.CostBasis != 1600 {
		t.Fatalf("expected cost basis 1600, got %f", ctx.CostBasis)
	}
	if ctx.PositionPercent <= 60 || ctx.PositionPercent >= 62 {
		t.Fatalf("expected position percent around 61.54, got %f", ctx.PositionPercent)
	}
	if len(ctx.AccountNames) != 2 {
		t.Fatalf("expected 2 account names, got %v", ctx.AccountNames)
	}
	if !(contains(ctx.AccountNames, "Main") && contains(ctx.AccountNames, "IRA")) {
		t.Fatalf("expected account names to include Main and IRA, got %v", ctx.AccountNames)
	}

	// aiJSON should only contain allowed fields.
	aiJSON, err := ctx.aiJSON()
	if err != nil {
		t.Fatalf("aiJSON failed: %v", err)
	}
	// Allowed fields that have non-zero values must be present.
	for _, want := range []string{`"symbol"`, `"avg_cost"`, `"position_percent"`} {
		if !strings.Contains(aiJSON, want) {
			t.Fatalf("expected aiJSON to contain %s, got: %s", want, aiJSON)
		}
	}
	// Forbidden fields must be absent.
	for _, forbidden := range []string{`"currency"`, `"total_shares"`, `"cost_basis"`, `"latest_price"`, `"market_value"`, `"account_name"`, `"account_names"`} {
		if strings.Contains(aiJSON, forbidden) {
			t.Fatalf("aiJSON must NOT contain %s, got: %s", forbidden, aiJSON)
		}
	}

	// Symbol not held: should still succeed with minimal data.
	ctx2, err := core.buildSymbolContext("NVDA", "USD")
	if err != nil {
		t.Fatalf("buildSymbolContext for unheld symbol failed: %v", err)
	}
	if ctx2.Symbol != "NVDA" {
		t.Fatalf("expected NVDA, got %s", ctx2.Symbol)
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

func TestNormalizeSynthesisProbability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		confidence  string
		probability float64
		want        float64
	}{
		{name: "use provided probability", confidence: "high", probability: 67.123, want: 67.12},
		{name: "fallback high confidence", confidence: "high", probability: 0, want: 72},
		{name: "fallback medium confidence", confidence: "medium", probability: 0, want: 58},
		{name: "fallback low confidence", confidence: "low", probability: 0, want: 42},
		{name: "fallback for out of range", confidence: "high", probability: 132, want: 72},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSynthesisProbability(tc.confidence, tc.probability)
			if got != tc.want {
				t.Fatalf("expected %f, got %f", tc.want, got)
			}
		})
	}
}

func TestParseSynthesisResult_UsesDirectSummaryTemplate(t *testing.T) {
	t.Parallel()

	raw := `{
		"overall_rating":"buy",
		"confidence":"high",
		"target_action":"increase",
		"position_suggestion":"把仓位从12%提到15%",
		"overall_summary":"好的，综合看下来还要看情况。",
		"key_factors":["盈利增速改善","订单能见度提升"],
		"risk_warnings":["估值偏高"],
		"action_items":[],
		"time_horizon_notes":"中长期",
		"disclaimer":"仅供参考，不构成投资建议"
	}`

	parsed, err := parseSynthesisResult(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	wants := []string{
		"结论：加仓，执行概率72%。",
		"仓位：把仓位从12%提到15%。",
		"依据：盈利增速改善；订单能见度提升。",
		"雷点：估值偏高。",
	}
	for _, want := range wants {
		if !strings.Contains(parsed.OverallSummary, want) {
			t.Fatalf("expected summary to contain %q, got: %s", want, parsed.OverallSummary)
		}
	}

	if strings.Contains(parsed.OverallSummary, "看情况") {
		t.Fatalf("summary should not contain fuzzy phrase, got: %s", parsed.OverallSummary)
	}
}

func TestNormalizeSynthesisSummary_TruncatesLength(t *testing.T) {
	t.Parallel()

	result := SymbolSynthesisResult{
		TargetAction:       "hold",
		Confidence:         "medium",
		ActionProbability:  59,
		PositionSuggestion: strings.Repeat("仓位控制", 40),
		KeyFactors: []string{
			strings.Repeat("强信号", 30),
			strings.Repeat("边际改善", 30),
		},
		RiskWarnings: []string{strings.Repeat("波动风险", 30)},
	}

	summary := normalizeSynthesisSummary(result)
	if len([]rune(summary)) > 200 {
		t.Fatalf("expected summary <= 200 runes, got %d: %s", len([]rune(summary)), summary)
	}
	if !strings.HasPrefix(summary, "结论：") {
		t.Fatalf("expected direct-answer prefix, got: %s", summary)
	}
}

func TestNormalizeSynthesisPositionSuggestion_WithContextTriplet(t *testing.T) {
	t.Parallel()

	result := SymbolSynthesisResult{
		TargetAction:       "increase",
		PositionSuggestion: "建议加一点",
	}
	ctx := &symbolContextData{
		PositionPercent:      12.34,
		AllocationMinPercent: 15,
		AllocationMaxPercent: 25,
	}

	got := normalizeSynthesisPositionSuggestion(result, ctx)
	wants := []string{
		"当前占比12.34%",
		"目标区间15.00%-25.00%",
		"差值-2.66%",
		"动作：加仓",
		"执行：建议加一点",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("expected position suggestion contain %q, got: %s", want, got)
		}
	}
}

func TestNormalizeSynthesisPositionSuggestion_DefaultRange(t *testing.T) {
	t.Parallel()

	result := SymbolSynthesisResult{TargetAction: "hold"}
	ctx := &symbolContextData{PositionPercent: 40}

	got := normalizeSynthesisPositionSuggestion(result, ctx)
	if !strings.Contains(got, "目标区间0.00%-100.00%") {
		t.Fatalf("expected default target range, got: %s", got)
	}
	if !strings.Contains(got, "差值0.00%（在区间内）") {
		t.Fatalf("expected in-range delta, got: %s", got)
	}
}

func TestNormalizeSynthesisResult_RewritesSummaryAndPositionByContext(t *testing.T) {
	t.Parallel()

	result := &SymbolSynthesisResult{
		Confidence:         "medium",
		TargetAction:       "reduce",
		PositionSuggestion: "看情况",
		OverallSummary:     "还要看情况",
		KeyFactors:         []string{"盈利下修"},
		RiskWarnings:       []string{"波动加大"},
	}
	ctx := &symbolContextData{
		PositionPercent:      31.2,
		AllocationMinPercent: 10,
		AllocationMaxPercent: 20,
	}

	normalizeSynthesisResult(result, ctx)

	if !strings.Contains(result.PositionSuggestion, "当前占比31.20%") {
		t.Fatalf("expected rewritten position suggestion, got: %s", result.PositionSuggestion)
	}
	if !strings.Contains(result.OverallSummary, "仓位：当前占比31.20%") {
		t.Fatalf("expected summary to embed rewritten position, got: %s", result.OverallSummary)
	}
	if strings.Contains(result.OverallSummary, "看情况") {
		t.Fatalf("summary should remove fuzzy phrase, got: %s", result.OverallSummary)
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
		RiskProfile:    "  AGGRESSIVE  ",
		Horizon:        "  LONG  ",
		AdviceStyle:    "  conservative  ",
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
	if req.RiskProfile != "aggressive" {
		t.Fatalf("expected normalized risk_profile aggressive, got %q", req.RiskProfile)
	}
	if req.Horizon != "long" {
		t.Fatalf("expected normalized horizon long, got %q", req.Horizon)
	}
	if req.AdviceStyle != "conservative" {
		t.Fatalf("expected normalized advice_style conservative, got %q", req.AdviceStyle)
	}

	defaults, err := normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{
		APIKey:   "k",
		Model:    "m",
		Symbol:   "X",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("unexpected error for defaults: %v", err)
	}
	if defaults.RiskProfile != "balanced" {
		t.Fatalf("expected default risk_profile balanced, got %q", defaults.RiskProfile)
	}
	if defaults.Horizon != "medium" {
		t.Fatalf("expected default horizon medium, got %q", defaults.Horizon)
	}
	if defaults.AdviceStyle != "balanced" {
		t.Fatalf("expected default advice_style balanced, got %q", defaults.AdviceStyle)
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

	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{APIKey: "k", Model: "m", Symbol: "X", Currency: "USD", RiskProfile: "boom"})
	if err == nil || !strings.Contains(err.Error(), "invalid risk_profile") {
		t.Fatalf("expected invalid risk_profile error, got %v", err)
	}

	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{APIKey: "k", Model: "m", Symbol: "X", Currency: "USD", Horizon: "daytrade"})
	if err == nil || !strings.Contains(err.Error(), "invalid horizon") {
		t.Fatalf("expected invalid horizon error, got %v", err)
	}

	_, err = normalizeSymbolAnalysisRequest(SymbolAnalysisRequest{APIKey: "k", Model: "m", Symbol: "X", Currency: "USD", AdviceStyle: "wild"})
	if err == nil || !strings.Contains(err.Error(), "invalid advice_style") {
		t.Fatalf("expected invalid advice_style error, got %v", err)
	}
}
