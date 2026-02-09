package investlog

import (
	"context"
	"encoding/json"
	"errors"
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
		APIKey: " k ",
		Model:  " m ",
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
