package investlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildAICompletionsEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "empty uses default", input: "", want: "https://api.openai.com/v1/chat/completions"},
		{name: "base without v1", input: "https://example.com", want: "https://example.com/v1/chat/completions"},
		{name: "base with v1", input: "https://example.com/v1", want: "https://example.com/v1/chat/completions"},
		{name: "already completions", input: "https://example.com/v1/chat/completions", want: "https://example.com/v1/chat/completions"},
		{name: "responses endpoint", input: "https://example.com/v1/responses", want: "https://example.com/v1/responses"},
		{name: "missing scheme", input: "example.com/api", want: "https://example.com/api/v1/chat/completions"},
		{name: "invalid scheme", input: "ftp://example.com", wantErr: "invalid base_url scheme"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildAICompletionsEndpoint(tc.input)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error contains %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestAIRequestTimeoutIsFiveMinutes(t *testing.T) {
	t.Parallel()

	if aiRequestTimeout != 5*time.Minute {
		t.Fatalf("expected aiRequestTimeout to be 5m, got %s", aiRequestTimeout)
	}
}

func TestToResponsesEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "from chat completions", input: "https://example.com/v1/chat/completions", want: "https://example.com/v1/responses"},
		{name: "already responses", input: "https://example.com/v1/responses", want: "https://example.com/v1/responses"},
		{name: "from v1", input: "https://example.com/v1", want: "https://example.com/v1/responses"},
		{name: "unsupported path", input: "https://example.com/custom", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toResponsesEndpoint(tc.input)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestToAltEndpoints(t *testing.T) {
	t.Parallel()

	if got := toAltChatEndpoint("https://example.com/v1/chat/completions"); got != "https://example.com/chat/completions" {
		t.Fatalf("unexpected alt chat endpoint: %q", got)
	}
	if got := toAltChatEndpoint("https://example.com/chat/completions"); got != "https://example.com/v1/chat/completions" {
		t.Fatalf("unexpected alt chat endpoint: %q", got)
	}
	if got := toAltResponsesEndpoint("https://example.com/v1/responses"); got != "https://example.com/responses" {
		t.Fatalf("unexpected alt responses endpoint: %q", got)
	}
	if got := toAltResponsesEndpoint("https://example.com/responses"); got != "https://example.com/v1/responses" {
		t.Fatalf("unexpected alt responses endpoint: %q", got)
	}
}

func TestShouldFallbackToResponses(t *testing.T) {
	t.Parallel()

	if !shouldFallbackToResponses(errors.New("ai upstream error: input is required")) {
		t.Fatal("expected fallback for input is required")
	}
	if !shouldFallbackToResponses(errors.New("missing required parameter: input")) {
		t.Fatal("expected fallback for missing input parameter")
	}
	if shouldFallbackToResponses(errors.New("some other error")) {
		t.Fatal("did not expect fallback for unrelated error")
	}
}

func TestShouldFallbackToAltEndpoint(t *testing.T) {
	t.Parallel()

	if !shouldFallbackToAltEndpoint(errors.New("404 not found")) {
		t.Fatal("expected fallback for 404")
	}
	if !shouldFallbackToAltEndpoint(errors.New("unknown path")) {
		t.Fatal("expected fallback for unknown path")
	}
	if shouldFallbackToAltEndpoint(errors.New("bad request")) {
		t.Fatal("did not expect fallback for unrelated error")
	}
}

func TestIsTimeoutError(t *testing.T) {
	t.Parallel()

	if !isTimeoutError(context.DeadlineExceeded) {
		t.Fatal("expected timeout for context deadline")
	}
	if !isTimeoutError(errors.New("context deadline exceeded")) {
		t.Fatal("expected timeout for message")
	}
	if isTimeoutError(errors.New("bad request")) {
		t.Fatal("did not expect timeout for unrelated error")
	}
}

func TestNormalizeHoldingsAnalysisRequest(t *testing.T) {
	t.Parallel()

	_, err := normalizeHoldingsAnalysisRequest(HoldingsAnalysisRequest{Model: "gpt-4o"})
	if err == nil || !strings.Contains(err.Error(), "api_key is required") {
		t.Fatalf("expected api_key validation error, got %v", err)
	}

	_, err = normalizeHoldingsAnalysisRequest(HoldingsAnalysisRequest{APIKey: "k", Model: "m", Currency: "EUR"})
	if err == nil || !strings.Contains(err.Error(), "invalid currency") {
		t.Fatalf("expected currency validation error, got %v", err)
	}

	result, err := normalizeHoldingsAnalysisRequest(HoldingsAnalysisRequest{
		APIKey:         " k ",
		Model:          " m ",
		StrategyPrompt: "  偏好低波动高分红  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.APIKey != "k" {
		t.Fatalf("expected trimmed api key, got %q", result.APIKey)
	}
	if result.RiskProfile != "balanced" || result.Horizon != "medium" || result.AdviceStyle != "balanced" {
		t.Fatalf("expected default enums, got %+v", result)
	}
	if result.StrategyPrompt != "偏好低波动高分红" {
		t.Fatalf("expected trimmed strategy prompt, got %q", result.StrategyPrompt)
	}
}

func TestBuildHoldingsAnalysisUserPrompt_ContainsStrategyPrompt(t *testing.T) {
	t.Parallel()

	prompt, err := buildHoldingsAnalysisUserPrompt(&holdingsAnalysisPromptInput{
		Holdings: []holdingsAnalysisCurrencySnapshot{{
			Currency: "USD",
		}},
	}, HoldingsAnalysisRequest{
		RiskProfile:     "balanced",
		Horizon:         "medium",
		AdviceStyle:     "balanced",
		AllowNewSymbols: true,
		StrategyPrompt:  "优先控制回撤，不新增中概股",
	}, nil)
	if err != nil {
		t.Fatalf("buildHoldingsAnalysisUserPrompt failed: %v", err)
	}
	if !strings.Contains(prompt, "strategy_prompt") {
		t.Fatalf("expected strategy_prompt in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "优先控制回撤，不新增中概股") {
		t.Fatalf("expected strategy prompt value in prompt, got: %s", prompt)
	}
}

func TestParseHoldingsAnalysisResponse(t *testing.T) {
	t.Parallel()

	content := "```json\n{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"f1\"],\"recommendations\":[{\"action\":\"increase\",\"theory_tag\":\"Dalio\",\"rationale\":\"r\"}],\"disclaimer\":\"d\"}\n```"
	parsed, err := parseHoldingsAnalysisResponse(content)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.OverallSummary != "ok" {
		t.Fatalf("expected overall_summary=ok, got %q", parsed.OverallSummary)
	}

	_, err = parseHoldingsAnalysisResponse("not json")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestRequestAIChatCompletion(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if reqBody["model"] != "model-x" {
			t.Fatalf("expected model-x, got %v", reqBody["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-x","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"x\"],\"recommendations\":[{\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"wait\"}],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer ts.Close()

	result, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  ts.URL,
		APIKey:       "key",
		Model:        "model-x",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "model-x" {
		t.Fatalf("expected model-x, got %q", result.Model)
	}
	if !strings.Contains(result.Content, "overall_summary") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestRequestAIChatCompletion_FallbackToResponses(t *testing.T) {
	t.Parallel()

	sawResponses := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"input is required"}}`))
			return
		}
		if r.URL.Path != "/v1/responses" && r.URL.Path != "/responses" {
			t.Fatalf("expected fallback to /responses-like path, got %s", r.URL.Path)
		}
		sawResponses = true
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if reqBody["input"] == nil {
			t.Fatalf("expected input field in responses payload")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-y","output_text":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"x\"],\"recommendations\":[{\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"wait\"}],\"disclaimer\":\"d\"}"}`))
	}))
	defer ts.Close()

	result, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  ts.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-y",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "model-y" {
		t.Fatalf("expected model-y, got %q", result.Model)
	}
	if !strings.Contains(result.Content, "overall_summary") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if !sawResponses {
		t.Fatal("expected fallback request to responses endpoint")
	}
}

func TestRequestAIChatCompletion_FallbackToAltChatPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"not found"}}`))
			return
		}
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-z","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"x\"],\"recommendations\":[{\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"wait\"}],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer server.Close()

	result, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-z",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "model-z" {
		t.Fatalf("expected model-z, got %q", result.Model)
	}
}

func TestRequestAIChatCompletion_SameEndpointResponsesPayload(t *testing.T) {
	t.Parallel()

	sawHybridPayload := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if reqBody["messages"] != nil && reqBody["input"] == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"input is required"}}`))
			return
		}
		if reqBody["input"] != nil && reqBody["messages"] != nil {
			sawHybridPayload = true
		}
		if reqBody["input"] == nil && reqBody["messages"] == nil {
			t.Fatalf("expected input or messages")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-r","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"x\"],\"recommendations\":[{\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"wait\"}],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer server.Close()

	result, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-r",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "model-r" {
		t.Fatalf("expected model-r, got %q", result.Model)
	}
	if !sawHybridPayload {
		t.Fatal("expected same endpoint with hybrid payload")
	}
}

func TestRequestAIChatCompletion_ReturnsFriendlyTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if reqBody["messages"] != nil && reqBody["input"] == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"input is required"}}`))
			return
		}
		if reqBody["input"] != nil {
			time.Sleep(80 * time.Millisecond)
			return
		}
		time.Sleep(80 * time.Millisecond)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := requestAIChatCompletion(ctx, aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-timeout",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "ai upstream timeout") {
		t.Fatalf("expected friendly timeout message, got %v", err)
	}
}

func TestRequestAIChatCompletion_DebugLogsPrompt(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-debug","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"x\"],\"recommendations\":[{\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"wait\"}],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer server.Close()

	_, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-debug",
		SystemPrompt: "system prompt for debug",
		UserPrompt:   "user prompt for debug",
		Logger:       logger,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "ai request prompt") {
		t.Fatalf("expected debug prompt log, got %q", logs)
	}
	if !strings.Contains(logs, "system_prompt=\"system prompt for debug\"") {
		t.Fatalf("expected system prompt in debug log, got %q", logs)
	}
	if !strings.Contains(logs, "user_prompt=\"user prompt for debug\"") {
		t.Fatalf("expected user prompt in debug log, got %q", logs)
	}
}

func TestRequestAIChatCompletion_InfoLevelSkipsPromptLog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-info","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"x\"],\"recommendations\":[{\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"wait\"}],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer server.Close()

	_, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-info",
		SystemPrompt: "system prompt for info",
		UserPrompt:   "user prompt for info",
		Logger:       logger,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := buf.String()
	if strings.Contains(logs, "ai request prompt") {
		t.Fatalf("did not expect debug prompt log at info level, got %q", logs)
	}
	if strings.Contains(logs, "system_prompt=") || strings.Contains(logs, "user_prompt=") {
		t.Fatalf("did not expect prompt fields at info level, got %q", logs)
	}
}

func TestRequestAIChatCompletion_DebugLogsRawResponse(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-raw","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[\"x\"],\"recommendations\":[{\"action\":\"hold\",\"theory_tag\":\"Buffett\",\"rationale\":\"wait\"}],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer server.Close()

	_, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-raw",
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Logger:       logger,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "ai request prompt") {
		t.Fatalf("expected debug prompt log, got %q", logs)
	}
	if !strings.Contains(logs, "model=model-raw") {
		t.Fatalf("expected model in debug log, got %q", logs)
	}
}

func TestRequestAIByChatCompletions_DebugLogsRawResponseOnError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"provider raw detail"}}`))
	}))
	defer server.Close()

	_, err := requestAIByChatCompletions(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL,
		APIKey:       "key",
		Model:        "model-raw-err",
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Logger:       logger,
	}, server.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "provider raw detail") {
		t.Fatalf("expected upstream error detail, got %v", err)
	}

	logs := buf.String()
	if !strings.Contains(logs, "ai request prompt") {
		t.Fatalf("expected debug prompt log, got %q", logs)
	}
}

func TestDecodeAIModelAndContent(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"m1","content":[{"text":"hello"}]}`)
	model, content, err := decodeAIModelAndContent(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "m1" || content != "hello" {
		t.Fatalf("unexpected decode result: model=%q content=%q", model, content)
	}
}

func TestAnalyzeHoldingsEndToEndWithStub(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		if !strings.Contains(req.EndpointURL, "/chat/completions") {
			return aiChatCompletionResult{}, errors.New("bad endpoint")
		}
		return aiChatCompletionResult{
			Model: "mock-model",
			Content: `{
				"overall_summary":"组合较集中，建议增强分散化",
				"risk_level":"balanced",
				"key_findings":["单一标的集中度偏高"],
				"recommendations":[{"symbol":"AAPL","action":"reduce","theory_tag":"Malkiel","rationale":"降低非系统性风险","target_weight":"<20%","priority":"high"}],
				"disclaimer":"仅供参考"
			}`,
		}, nil
	}

	result, err := core.AnalyzeHoldings(HoldingsAnalysisRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "mock-model",
		Currency:        "USD",
		RiskProfile:     "balanced",
		Horizon:         "medium",
		AdviceStyle:     "balanced",
		AllowNewSymbols: true,
	})
	if err != nil {
		t.Fatalf("AnalyzeHoldings failed: %v", err)
	}
	if result.Currency != "USD" {
		t.Fatalf("expected USD, got %s", result.Currency)
	}
	if len(result.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
	if result.Recommendations[0].Action != "reduce" {
		t.Fatalf("unexpected action: %s", result.Recommendations[0].Action)
	}
}

func TestAnalyzeHoldings_UsesFifteenMinuteOverallTimeout(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-1", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-1")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	var remainingAtCall time.Duration
	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return aiChatCompletionResult{}, errors.New("analysis context missing deadline")
		}
		remainingAtCall = time.Until(deadline)
		return aiChatCompletionResult{
			Model: "mock-model",
			Content: `{
				"overall_summary":"ok",
				"risk_level":"balanced",
				"key_findings":["x"],
				"recommendations":[{"symbol":"AAPL","action":"hold","theory_tag":"Buffett","rationale":"wait"}],
				"disclaimer":"仅供参考"
			}`,
		}, nil
	}

	_, err := core.AnalyzeHoldings(HoldingsAnalysisRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "mock-model",
		Currency:        "USD",
		RiskProfile:     "balanced",
		Horizon:         "medium",
		AdviceStyle:     "balanced",
		AllowNewSymbols: true,
	})
	if err != nil {
		t.Fatalf("AnalyzeHoldings failed: %v", err)
	}
	if remainingAtCall < 14*time.Minute {
		t.Fatalf("expected overall timeout close to 15m, got remaining %s", remainingAtCall)
	}
	if remainingAtCall > 15*time.Minute+5*time.Second {
		t.Fatalf("unexpectedly large timeout, got remaining %s", remainingAtCall)
	}
}

func TestAnalyzeHoldingsWithStream_EmitsDelta(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-stream", "Stream Account")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-stream")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()

	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		if req.OnDelta == nil {
			t.Fatal("expected onDelta callback")
		}
		req.OnDelta("第一段")
		req.OnDelta("第二段")

		return aiChatCompletionResult{
			Model: "mock-stream-model",
			Content: `{
				"overall_summary":"ok",
				"risk_level":"balanced",
				"key_findings":["x"],
				"recommendations":[{"symbol":"AAPL","action":"hold","theory_tag":"Buffett","rationale":"wait"}],
				"disclaimer":"仅供参考"
			}`,
		}, nil
	}

	var streamed strings.Builder
	result, err := core.AnalyzeHoldingsWithStream(HoldingsAnalysisRequest{
		BaseURL: "https://example.com/v1",
		APIKey:  "key",
		Model:   "mock-stream-model",
	}, func(delta string) {
		streamed.WriteString(delta)
	})
	if err != nil {
		t.Fatalf("AnalyzeHoldingsWithStream failed: %v", err)
	}

	if streamed.String() != "第一段第二段" {
		t.Fatalf("unexpected streamed deltas: %q", streamed.String())
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Model != "mock-stream-model" {
		t.Fatalf("unexpected model: %s", result.Model)
	}
}

func TestGetHoldingsAnalysisAndHistory(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	latest, err := core.GetHoldingsAnalysis("USD")
	if err != nil {
		t.Fatalf("GetHoldingsAnalysis empty failed: %v", err)
	}
	if latest != nil {
		t.Fatalf("expected nil latest analysis, got %+v", latest)
	}

	firstID, err := core.saveHoldingsAnalysis(&HoldingsAnalysisResult{
		Currency:       "USD",
		Model:          "m-usd",
		AnalysisType:   "adhoc",
		RiskLevel:      "balanced",
		OverallSummary: "usd summary",
		KeyFindings:    []string{"f1"},
		Recommendations: []HoldingsAnalysisRecommendation{
			{Symbol: "AAPL", Action: "hold", TheoryTag: "Buffett", Rationale: "长期持有"},
		},
		Disclaimer: "仅供参考",
		SymbolRefs: []HoldingsSymbolRef{
			{Symbol: "AAPL", ID: 11, Rating: "buy", Action: "increase", Summary: "summary", CreatedAt: "2026-01-01T00:00:00+08:00"},
		},
	})
	if err != nil {
		t.Fatalf("save first holdings analysis failed: %v", err)
	}

	if _, err := core.saveHoldingsAnalysis(&HoldingsAnalysisResult{
		Currency:        "CNY",
		Model:           "m-cny",
		AnalysisType:    "weekly",
		RiskLevel:       "conservative",
		OverallSummary:  "cny summary",
		KeyFindings:     []string{"f2"},
		Recommendations: []HoldingsAnalysisRecommendation{},
		Disclaimer:      "仅供参考",
	}); err != nil {
		t.Fatalf("save second holdings analysis failed: %v", err)
	}

	if _, err := core.db.Exec(
		`INSERT INTO holdings_analyses
			(currency, model, analysis_type, risk_level, overall_summary, key_findings, recommendations, disclaimer, symbol_refs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"USD", "m-bad", "adhoc", "balanced", "bad json", "{invalid", "{invalid", "仅供参考", "{invalid",
	); err != nil {
		t.Fatalf("insert malformed holdings analysis failed: %v", err)
	}

	latest, err = core.GetHoldingsAnalysis(" usd ")
	if err != nil {
		t.Fatalf("GetHoldingsAnalysis failed: %v", err)
	}
	if latest == nil {
		t.Fatal("expected non-nil latest analysis")
	}
	if latest.Currency != "USD" {
		t.Fatalf("expected normalized currency USD, got %s", latest.Currency)
	}

	usdHistory, err := core.GetHoldingsAnalysisHistory("USD", 10)
	if err != nil {
		t.Fatalf("GetHoldingsAnalysisHistory USD failed: %v", err)
	}
	if len(usdHistory) < 2 {
		t.Fatalf("expected at least 2 USD analyses, got %d", len(usdHistory))
	}

	foundSaved := false
	foundMalformed := false
	for _, item := range usdHistory {
		if item.ID == firstID {
			foundSaved = true
			if len(item.SymbolRefs) != 1 {
				t.Fatalf("expected symbol refs for saved row, got %+v", item.SymbolRefs)
			}
		}
		if item.OverallSummary == "bad json" {
			foundMalformed = true
			if item.KeyFindings == nil || len(item.KeyFindings) != 0 {
				t.Fatalf("expected malformed findings fallback to empty slice, got %+v", item.KeyFindings)
			}
			if item.Recommendations == nil || len(item.Recommendations) != 0 {
				t.Fatalf("expected malformed recommendations fallback to empty slice, got %+v", item.Recommendations)
			}
		}
	}
	if !foundSaved {
		t.Fatalf("expected to find saved row id=%d in USD history", firstID)
	}
	if !foundMalformed {
		t.Fatal("expected to find malformed row in USD history")
	}

	allHistory, err := core.GetHoldingsAnalysisHistory("", 0)
	if err != nil {
		t.Fatalf("GetHoldingsAnalysisHistory all failed: %v", err)
	}
	if allHistory == nil {
		t.Fatal("expected non-nil all history slice")
	}
	if len(allHistory) < 3 {
		t.Fatalf("expected at least 3 analyses, got %d", len(allHistory))
	}
}

func TestDecodeAIModelAndContent_VariousShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		wantModel   string
		wantContent string
		wantErr     string
	}{
		{
			name:        "output_text field",
			body:        `{"model":"m-output-text","output_text":"final text"}`,
			wantModel:   "m-output-text",
			wantContent: "final text",
		},
		{
			name:        "choices message content parts",
			body:        `{"model":"m-choices","choices":[{"message":{"content":[{"text":" part1 "},{"value":"part2"},{"content":"part3"}]}}]}`,
			wantModel:   "m-choices",
			wantContent: "part1\npart2\npart3",
		},
		{
			name:        "output content array",
			body:        `{"model":"m-output","output":[{"content":[{"text":"alpha"},{"content":"beta"}]}]}`,
			wantModel:   "m-output",
			wantContent: "alpha\nbeta",
		},
		{
			name:        "content output_text fallback",
			body:        `{"model":"m-content","content":{"output_text":["x","y"]}}`,
			wantModel:   "m-content",
			wantContent: "x\ny",
		},
		{
			name:    "empty payload",
			body:    `{"model":"m-empty"}`,
			wantErr: "ai response content is empty",
		},
		{
			name:    "invalid json",
			body:    `{`,
			wantErr: "decode ai response",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			model, content, err := decodeAIModelAndContent([]byte(tc.body))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error contains %q, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if model != tc.wantModel {
				t.Fatalf("unexpected model: got %q want %q", model, tc.wantModel)
			}
			if content != tc.wantContent {
				t.Fatalf("unexpected content: got %q want %q", content, tc.wantContent)
			}
		})
	}
}

func TestParseAIErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "nested error message", body: `{"error":{"message":" provider detail "}}`, want: "provider detail"},
		{name: "top level message", body: `{"message":" top message "}`, want: "top message"},
		{name: "invalid json", body: `{`, want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := parseAIErrorMessage([]byte(tc.body)); got != tc.want {
				t.Fatalf("unexpected message: got %q want %q", got, tc.want)
			}
		})
	}
}
