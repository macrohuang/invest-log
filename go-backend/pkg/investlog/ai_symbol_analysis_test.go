package investlog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// Dimension stub responses used across tests.
const (
	stubMacroJSON         = `{"dimension":"macro","rating":"positive","confidence":"medium","key_points":["低利率环境有利"],"risks":["通胀压力"],"opportunities":["政策刺激"],"summary":"宏观环境整体有利"}`
	stubIndustryJSON      = `{"dimension":"industry","rating":"positive","confidence":"high","key_points":["行业增长强劲"],"risks":["竞争加剧"],"opportunities":["AI驱动增长"],"summary":"行业前景积极"}`
	stubCompanyJSON       = `{"dimension":"company","rating":"positive","confidence":"high","key_points":["营收稳健增长"],"risks":["估值偏高"],"opportunities":["新产品周期"],"summary":"基本面优良","valuation_assessment":"估值合理"}`
	stubInternationalJSON = `{"dimension":"international","rating":"neutral","confidence":"medium","key_points":["贸易关系稳定"],"risks":["地缘政治不确定"],"opportunities":["全球化布局"],"summary":"国际环境中性"}`
	stubSynthesisJSON     = `{"overall_rating":"buy","confidence":"medium","action_probability_percent":67,"target_action":"increase","position_suggestion":"建议持有并适度加仓","overall_summary":"结论：加仓，执行概率67%。仓位：建议持有并适度加仓。依据：行业增长；基本面优良。雷点：估值偏高。","key_factors":["行业增长","基本面优良"],"risk_warnings":["估值偏高","地缘政治"],"action_items":[{"action":"适度加仓","rationale":"基本面支撑","priority":"medium"}],"time_horizon_notes":"中长期持有","disclaimer":"仅供参考"}`
)

var percentInSentencePattern = regexp.MustCompile(`\d+%`)

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
		return aiChatCompletionResult{
			Model:   "mock",
			Content: buildGenericDimensionJSON(sp),
		}, nil
	}
}

func buildGenericDimensionJSON(systemPrompt string) string {
	frameworkID := "framework"
	for _, spec := range symbolFrameworkCatalog {
		if buildFrameworkSystemPrompt(spec) == systemPrompt {
			frameworkID = strings.TrimSpace(spec.ID)
			break
		}
	}
	if frameworkID == "" {
		frameworkID = "framework"
	}

	return fmt.Sprintf(
		`{"dimension":%q,"rating":"positive","confidence":"medium","key_points":["信号改善"],"risks":["波动仍在"],"opportunities":["建议继续跟踪仓位"],"summary":%q,"suggestion":"increase：按纪律分批加仓。"}`,
		frameworkID,
		frameworkID+" 框架分析完成",
	)
}

func firstSentence(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	for _, sep := range []string{"。", "！", "？", ".", "!", "?"} {
		if idx := strings.Index(summary, sep); idx >= 0 {
			return strings.TrimSpace(summary[:idx+len(sep)])
		}
	}
	return summary
}

func sortedDimensionKeys(dimensions map[string]*SymbolDimensionResult) []string {
	keys := make([]string, 0, len(dimensions))
	for k := range dimensions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertSynthesisHardConstraints(t *testing.T, synthesis *SymbolSynthesisResult) {
	t.Helper()
	if synthesis == nil {
		t.Fatal("expected synthesis")
	}
	if synthesis.ActionProbability <= 0 || synthesis.ActionProbability > 100 {
		t.Fatalf("expected numeric action_probability_percent in (0,100], got %v", synthesis.ActionProbability)
	}

	first := firstSentence(synthesis.OverallSummary)
	if !strings.Contains(first, "执行概率") || !percentInSentencePattern.MatchString(first) {
		t.Fatalf("expected first sentence to include explicit probability number, got: %s", first)
	}
	hasAction := strings.Contains(first, "加仓") || strings.Contains(first, "减仓") || strings.Contains(first, "持有")
	if !hasAction {
		t.Fatalf("expected first sentence contains action conclusion, got: %s", first)
	}

	lowerSummary := strings.ToLower(synthesis.OverallSummary)
	for _, fuzzy := range []string{"看情况", "视情况", "it depends"} {
		checkTarget := synthesis.OverallSummary
		if fuzzy == "it depends" {
			checkTarget = lowerSummary
		}
		if strings.Contains(checkTarget, fuzzy) {
			t.Fatalf("summary must not contain fuzzy phrase %q, got: %s", fuzzy, synthesis.OverallSummary)
		}
	}

	disclaimerLen := utf8.RuneCountInString(strings.TrimSpace(synthesis.Disclaimer))
	if disclaimerLen == 0 {
		t.Fatal("expected non-empty disclaimer")
	}
	if disclaimerLen > 16 {
		t.Fatalf("expected disclaimer <=16 chars, got %d: %s", disclaimerLen, synthesis.Disclaimer)
	}
}

func TestDimensionAgentPool_HasNineDistinctFrameworkIDs(t *testing.T) {
	t.Parallel()

	if len(symbolFrameworkCatalog) != 9 {
		t.Fatalf("expected framework pool size 9, got %d", len(symbolFrameworkCatalog))
	}

	seen := make(map[string]struct{}, len(symbolFrameworkCatalog))
	for _, spec := range symbolFrameworkCatalog {
		id := strings.TrimSpace(spec.ID)
		if id == "" {
			t.Fatal("framework id should not be empty")
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("framework id must be unique, got duplicate: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestAnalyzeSymbol_SelectsStableThreeFrameworks(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-framework", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-framework")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()
	aiChatCompletion = dimensionStubRouter

	origFetch := fetchExternalDataFn
	defer func() { fetchExternalDataFn = origFetch }()
	fetchExternalDataFn = func(_ context.Context, _, _ string, _ *slog.Logger) *symbolExternalData {
		return nil
	}

	req := SymbolAnalysisRequest{
		BaseURL:  "https://example.com/v1",
		APIKey:   "test-key",
		Model:    "mock-model",
		Symbol:   "AAPL",
		Currency: "USD",
	}

	firstResult, err := core.AnalyzeSymbol(req)
	if err != nil {
		t.Fatalf("AnalyzeSymbol first run failed: %v", err)
	}
	secondResult, err := core.AnalyzeSymbol(req)
	if err != nil {
		t.Fatalf("AnalyzeSymbol second run failed: %v", err)
	}

	if got := len(firstResult.Dimensions); got != 3 {
		t.Fatalf("expected first run to select 3 frameworks, got %d", got)
	}
	if got := len(secondResult.Dimensions); got != 3 {
		t.Fatalf("expected second run to select 3 frameworks, got %d", got)
	}

	firstKeys := sortedDimensionKeys(firstResult.Dimensions)
	secondKeys := sortedDimensionKeys(secondResult.Dimensions)
	if strings.Join(firstKeys, ",") != strings.Join(secondKeys, ",") {
		t.Fatalf("expected stable framework selection, first=%v second=%v", firstKeys, secondKeys)
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

	if got := len(result.Dimensions); got != 3 {
		t.Fatalf("expected exactly 3 selected frameworks, got %d", got)
	}
	for frameworkID, framework := range result.Dimensions {
		if strings.TrimSpace(frameworkID) == "" {
			t.Fatal("framework key should not be empty")
		}
		if framework == nil {
			t.Fatalf("framework %s result is nil", frameworkID)
		}
		if strings.TrimSpace(framework.Summary) == "" {
			t.Fatalf("framework %s should contain analysis summary", frameworkID)
		}
		if strings.TrimSpace(framework.Suggestion) == "" {
			t.Fatalf("framework %s should include suggestion", frameworkID)
		}
	}

	// Synthesis
	assertSynthesisHardConstraints(t, result.Synthesis)
	if result.Synthesis.OverallRating != "buy" {
		t.Fatalf("expected overall_rating=buy, got %q", result.Synthesis.OverallRating)
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
	hasFrameworkDelta := false
	for frameworkID := range result.Dimensions {
		if strings.Contains(text, "["+frameworkID+"]") {
			hasFrameworkDelta = true
			break
		}
	}
	if !hasFrameworkDelta {
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

	var failedOnce int32
	// Let one framework fail once; remaining frameworks + synthesis should still complete.
	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		if !strings.Contains(req.SystemPrompt, "综合投资分析师") && atomic.CompareAndSwapInt32(&failedOnce, 0, 1) {
			return aiChatCompletionResult{}, errors.New("framework timeout")
		}
		return dimensionStubRouter(ctx, req)
	}

	_, err := core.AnalyzeSymbol(SymbolAnalysisRequest{
		BaseURL:  "https://example.com/v1",
		APIKey:   "test-key",
		Model:    "mock-model",
		Symbol:   "AAPL",
		Currency: "USD",
	})
	if err == nil {
		t.Fatal("expected failure when one of three selected frameworks fails")
	}
	if !strings.Contains(err.Error(), "framework analyses insufficient") {
		t.Fatalf("expected framework insufficiency error, got: %v", err)
	}
}

func TestAnalyzeSymbol_SynthesisUsesPositionAndPreferences(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	if _, err := core.SetAllocationSetting("USD", "stock", 20, 30); err != nil {
		t.Fatalf("SetAllocationSetting failed: %v", err)
	}

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

	mustInclude := []string{
		`"position_percent"`,
		`"allocation_min_percent"`,
		`"allocation_max_percent"`,
		`"allocation_status"`,
		`"risk_profile":"aggressive"`,
		`"horizon":"long"`,
		`"advice_style":"aggressive"`,
		`"strategy_prompt"`,
		"成长股高波动策略",
	}
	for _, want := range mustInclude {
		if !strings.Contains(synthesisPrompt, want) {
			t.Fatalf("expected synthesis prompt to contain %q, got: %s", want, synthesisPrompt)
		}
	}
	if !strings.Contains(synthesisPrompt, `"total_shares"`) && !strings.Contains(synthesisPrompt, `"holdings_quantity"`) {
		t.Fatalf("expected synthesis prompt to include holdings quantity context, got: %s", synthesisPrompt)
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
	if !strings.Contains(err.Error(), "framework analyses insufficient") {
		t.Fatalf("expected framework insufficiency error, got: %v", err)
	}
}

func TestGetSymbolAnalysis(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert a completed row directly.
	_, err := core.db.Exec(
		`INSERT INTO symbol_analyses
		 (symbol, currency, model, status, macro_analysis, industry_analysis, company_analysis, international_analysis, synthesis, created_at, completed_at)
		 VALUES (?, ?, ?, 'completed', ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"AAPL", "USD", "test-model", stubMacroJSON, stubIndustryJSON, stubCompanyJSON, stubInternationalJSON, stubSynthesisJSON,
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

	if got := len(result.Dimensions); got == 0 {
		t.Fatal("expected at least one framework analysis in result")
	}
	for frameworkID, framework := range result.Dimensions {
		if strings.TrimSpace(frameworkID) == "" {
			t.Fatal("framework id should not be empty")
		}
		if framework == nil || strings.TrimSpace(framework.Summary) == "" {
			t.Fatalf("framework %s should preserve summary", frameworkID)
		}
	}

	// Regression: key synthesis fields must not be dropped when loading.
	assertSynthesisHardConstraints(t, result.Synthesis)
	if result.Synthesis.OverallRating != "buy" {
		t.Fatalf("expected buy, got %s", result.Synthesis.OverallRating)
	}
	if result.Synthesis.TargetAction != "increase" {
		t.Fatalf("expected target_action increase, got %s", result.Synthesis.TargetAction)
	}
	if len(result.Synthesis.ActionItems) == 0 {
		t.Fatal("expected action items")
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
			 (symbol, currency, model, status, macro_analysis, industry_analysis, company_analysis, international_analysis, synthesis, created_at, completed_at)
			 VALUES (?, ?, ?, 'completed', ?, ?, ?, ?, ?, datetime('now', ?), datetime('now', ?))`,
			"AAPL", "USD", "model-v"+string(rune('1'+i)),
			stubMacroJSON, stubIndustryJSON, stubCompanyJSON, stubInternationalJSON, stubSynthesisJSON,
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
	for i := range results {
		if len(results[i].Dimensions) == 0 {
			t.Fatalf("history item %d missing framework analyses", i)
		}
		assertSynthesisHardConstraints(t, results[i].Synthesis)
		if strings.TrimSpace(results[i].Synthesis.TargetAction) == "" {
			t.Fatalf("history item %d missing target_action", i)
		}
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

	if _, err := core.SetAllocationSetting("USD", "stock", 40, 60); err != nil {
		t.Fatalf("SetAllocationSetting failed: %v", err)
	}

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
	if ctx.AllocationMinPercent != 40 {
		t.Fatalf("expected allocation min 40, got %f", ctx.AllocationMinPercent)
	}
	if ctx.AllocationMaxPercent != 60 {
		t.Fatalf("expected allocation max 60, got %f", ctx.AllocationMaxPercent)
	}
	if ctx.AllocationStatus != "above_target" {
		t.Fatalf("expected allocation status above_target, got %s", ctx.AllocationStatus)
	}

	// aiJSON should keep lightweight fields and avoid account identity fields.
	aiJSON, err := ctx.aiJSON()
	if err != nil {
		t.Fatalf("aiJSON failed: %v", err)
	}
	for _, want := range []string{
		`"symbol"`,
		`"avg_cost"`,
		`"position_percent"`,
		`"allocation_max_percent"`,
		`"allocation_status"`,
	} {
		if !strings.Contains(aiJSON, want) {
			t.Fatalf("expected aiJSON to contain %s, got: %s", want, aiJSON)
		}
	}
	for _, forbidden := range []string{`"account_name"`, `"account_names"`} {
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
		"action_probability_percent":67,
		"target_action":"increase",
		"position_suggestion":"把仓位从12%提到15%",
		"overall_summary":"好的，综合看下来还要看情况，视情况，it depends。",
		"key_factors":["盈利增速改善","订单能见度提升"],
		"risk_warnings":["估值偏高"],
		"action_items":[],
		"time_horizon_notes":"中长期",
		"disclaimer":"仅供参考，不构成投资建议，请独立判断并注意风险。"
	}`

	parsed, err := parseSynthesisResult(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	wants := []string{
		"加仓，执行概率67%。",
		"仓位：把仓位从12%提到15%。",
		"依据：盈利增速改善；订单能见度提升。",
		"雷点：估值偏高。",
	}
	for _, want := range wants {
		if !strings.Contains(parsed.OverallSummary, want) {
			t.Fatalf("expected summary to contain %q, got: %s", want, parsed.OverallSummary)
		}
	}

	assertSynthesisHardConstraints(t, parsed)

	if strings.Contains(parsed.OverallSummary, "看情况") ||
		strings.Contains(parsed.OverallSummary, "视情况") ||
		strings.Contains(strings.ToLower(parsed.OverallSummary), "it depends") {
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

	summary := normalizeSynthesisSummary(result, []string{"dcf", "dynamic_moat", "relative_valuation"})
	if len([]rune(summary)) > 210 {
		t.Fatalf("expected summary <= 210 runes, got %d: %s", len([]rune(summary)), summary)
	}
	if first := firstSentence(summary); !strings.Contains(first, "执行概率") {
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

	normalizeSynthesisResult(result, ctx, []string{"dcf", "dynamic_moat", "relative_valuation"})

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

func TestNormalizeSynthesisResult_EnforcesShortDisclaimer(t *testing.T) {
	t.Parallel()

	result := &SymbolSynthesisResult{
		Confidence:         "high",
		TargetAction:       "hold",
		PositionSuggestion: "维持",
		KeyFactors:         []string{"盈利稳定"},
		RiskWarnings:       []string{"估值波动"},
		Disclaimer:         "仅供参考，不构成投资建议，请结合自身风险承受能力独立判断。",
	}

	normalizeSynthesisResult(result, nil, []string{"dcf", "dynamic_moat", "relative_valuation"})

	if got := utf8.RuneCountInString(strings.TrimSpace(result.Disclaimer)); got > 16 {
		t.Fatalf("expected disclaimer <=16 chars after normalization, got %d: %s", got, result.Disclaimer)
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
