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
		{name: "empty uses default", input: "", want: defaultAIBaseURL + "/v1/chat/completions"},
		{name: "base without v1", input: "https://example.com", want: "https://example.com/v1/chat/completions"},
		{name: "base with v1", input: "https://example.com/v1", want: "https://example.com/v1/chat/completions"},
		{name: "already completions", input: "https://example.com/v1/chat/completions", want: "https://example.com/v1/chat/completions"},
		{name: "responses endpoint", input: "https://example.com/v1/responses", want: "https://example.com/v1/responses"},
		{name: "missing scheme", input: "example.com/api", want: "https://example.com/api/v1/chat/completions"},
		{name: "invalid scheme", input: "ftp://example.com", wantErr: "invalid base_url scheme"},
		{name: "perplexity base", input: "https://api.perplexity.ai", want: "https://api.perplexity.ai/chat/completions"},
		{name: "perplexity with trailing slash", input: "https://api.perplexity.ai/", want: "https://api.perplexity.ai/chat/completions"},
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

func TestAIRequestTimeoutIsThreeMinutes(t *testing.T) {
	t.Parallel()

	if aiRequestTimeout != 3*time.Minute {
		t.Fatalf("expected aiRequestTimeout to be 3m, got %s", aiRequestTimeout)
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

func TestShouldFallbackToNonStreaming(t *testing.T) {
	t.Parallel()

	if !shouldFallbackToNonStreaming(errors.New("unexpected end of JSON input")) {
		t.Fatal("expected fallback for json decode error")
	}
	if !shouldFallbackToNonStreaming(errors.New("decode json failed")) {
		t.Fatal("expected fallback for generic decode json error")
	}
	if shouldFallbackToNonStreaming(errors.New("bad request")) {
		t.Fatal("did not expect fallback for unrelated error")
	}
}

func TestSupportsResponsesFallbackModel(t *testing.T) {
	t.Parallel()

	if supportsResponsesFallbackModel("sonar-pro") {
		t.Fatal("expected sonar-pro to skip responses fallback")
	}
	if !supportsResponsesFallbackModel("gpt-4.1") {
		t.Fatal("expected gpt model to allow responses fallback")
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

func TestRequestAIChatCompletion_StreamDecodeFallbackToOneShot(t *testing.T) {
	t.Parallel()

	var streamCalls int
	var oneShotCalls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if reqBody["stream"] == true {
			streamCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"broken"`))
			return
		}

		oneShotCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-stream-fallback","choices":[{"message":{"content":"{\"overall_summary\":\"ok\"}"}}]}`))
	}))
	defer ts.Close()

	result, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  ts.URL + "/chat/completions",
		APIKey:       "key",
		Model:        "model-stream-fallback",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if streamCalls == 0 {
		t.Fatal("expected streaming request")
	}
	if oneShotCalls == 0 {
		t.Fatal("expected one-shot fallback request")
	}
	if result.Model != "model-stream-fallback" {
		t.Fatalf("unexpected model: %q", result.Model)
	}
	if !strings.Contains(result.Content, "overall_summary") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestRequestAIChatCompletion_SonarSkipsResponsesFallback(t *testing.T) {
	t.Parallel()

	var responsesCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/responses") {
			responsesCalls++
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"not found"}}`))
	}))
	defer server.Close()

	_, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/chat/completions",
		APIKey:       "key",
		Model:        "sonar-pro",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if responsesCalls != 0 {
		t.Fatalf("expected no responses fallback request, got %d", responsesCalls)
	}
	if strings.Contains(err.Error(), "responses fallback failed") {
		t.Fatalf("unexpected responses fallback error: %v", err)
	}
}

func TestRequestAIChatCompletionStream_ClaudeSSE(t *testing.T) {
	t.Parallel()

	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Fatalf("expected Authorization Bearer header, got %q", got)
		}

		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if reqBody["model"] != "claude-3-5-sonnet-20241022" {
			t.Fatalf("unexpected model: %v", reqBody["model"])
		}
		if reqBody["stream"] != true {
			t.Fatalf("expected stream=true, got: %v", reqBody["stream"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"model\":\"claude-3-5-sonnet-20241022\",\"choices\":[{\"delta\":{\"content\":\"{\\\"overall_summary\\\":\\\"ok\\\"\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"model\":\"claude-3-5-sonnet-20241022\",\"choices\":[{\"delta\":{\"content\":\",\\\"risk_level\\\":\\\"balanced\\\",\\\"key_findings\\\":[\\\"x\\\"],\\\"recommendations\\\":[],\\\"disclaimer\\\":\\\"d\\\"}\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	var chunks []string
	_, err := requestAIChatCompletionStream(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "claude-3-5-sonnet-20241022",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	}, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPath != "/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions endpoint, got %s", receivedPath)
	}
	joined := strings.Join(chunks, "")
	if joined == "" {
		t.Fatal("expected streamed chunks")
	}
	if !strings.Contains(joined, "overall_summary") {
		t.Fatalf("unexpected streamed content: %q", joined)
	}
}

func TestRequestAIChatCompletion_GeminiStreamGenerateContent(t *testing.T) {
	t.Parallel()

	var receivedPath string
	var receivedAlt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAlt = r.URL.Query().Get("alt")
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "gem-key" {
			t.Fatalf("expected x-goog-api-key header, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected empty Authorization header, got %q", got)
		}

		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := reqBody["model"]; ok {
			t.Fatalf("gemini payload should not include model field")
		}
		systemInstruction, ok := reqBody["systemInstruction"].(map[string]any)
		if !ok {
			t.Fatalf("expected systemInstruction, got %T", reqBody["systemInstruction"])
		}
		sysParts, ok := systemInstruction["parts"].([]any)
		if !ok || len(sysParts) == 0 {
			t.Fatalf("expected non-empty system_instruction parts")
		}
		sysPart0, ok := sysParts[0].(map[string]any)
		if !ok || sysPart0["text"] != "sys prompt" {
			t.Fatalf("unexpected system_instruction content: %v", sysParts[0])
		}
		contents, ok := reqBody["contents"].([]any)
		if !ok || len(contents) == 0 {
			t.Fatalf("expected non-empty contents field")
		}
		content0, ok := contents[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected contents[0] type: %T", contents[0])
		}
		contentParts, ok := content0["parts"].([]any)
		if !ok || len(contentParts) == 0 {
			t.Fatalf("expected non-empty contents parts")
		}
		contentPart0, ok := contentParts[0].(map[string]any)
		if !ok || contentPart0["text"] != "user prompt" {
			t.Fatalf("unexpected user content: %v", contentParts[0])
		}
		generationConfig, ok := reqBody["generationConfig"].(map[string]any)
		if !ok {
			t.Fatalf("expected generationConfig, got %T", reqBody["generationConfig"])
		}
		if generationConfig["temperature"] == nil {
			t.Fatalf("expected generationConfig.temperature, got %#v", generationConfig)
		}
		maxOutputTokens, ok := generationConfig["maxOutputTokens"].(float64)
		if !ok {
			t.Fatalf("expected numeric generationConfig.maxOutputTokens, got %#v", generationConfig)
		}
		if int(maxOutputTokens) != 32768 {
			t.Fatalf("expected safe Gemini maxOutputTokens 32768, got %v", maxOutputTokens)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"{\\\"overall_summary\\\":\\\"ok\\\"\"}]}}],\"modelVersion\":\"gemini-2.5-flash\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\",\\\"risk_level\\\":\\\"balanced\\\",\\\"key_findings\\\":[],\\\"recommendations\\\":[],\\\"disclaimer\\\":\\\"d\\\"}\"}]}}],\"modelVersion\":\"gemini-2.5-flash\"}\n\n"))
	}))
	defer server.Close()

	baseEndpoint, err := buildAICompletionsEndpoint(server.URL + "/api/gemini")
	if err != nil {
		t.Fatalf("build endpoint failed: %v", err)
	}

	result, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  baseEndpoint,
		APIKey:       "gem-key",
		Model:        "gemini-2.5-flash",
		SystemPrompt: "sys prompt",
		UserPrompt:   "user prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPath != "/api/gemini/v1beta/models/gemini-2.5-flash:streamGenerateContent" {
		t.Fatalf("unexpected gemini endpoint path: %s", receivedPath)
	}
	if receivedAlt != "sse" {
		t.Fatalf("expected alt=sse, got %q", receivedAlt)
	}
	if result.Model != "gemini-2.5-flash" {
		t.Fatalf("unexpected model: %q", result.Model)
	}
	if !strings.Contains(result.Content, "overall_summary") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestRequestAIChatCompletion_GeminiBaseWithV1beta(t *testing.T) {
	t.Parallel()

	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"{\\\"overall_summary\\\":\\\"ok\\\"}\"}]}}],\"modelVersion\":\"gemini-2.5-flash\"}\n\n"))
	}))
	defer server.Close()

	baseEndpoint, err := buildAICompletionsEndpoint(server.URL + "/api/gemini/v1beta")
	if err != nil {
		t.Fatalf("build endpoint failed: %v", err)
	}

	_, err = requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  baseEndpoint,
		APIKey:       "gem-key",
		Model:        "gemini-2.5-flash",
		SystemPrompt: "sys prompt",
		UserPrompt:   "user prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPath != "/api/gemini/v1beta/models/gemini-2.5-flash:streamGenerateContent" {
		t.Fatalf("unexpected gemini endpoint path: %s", receivedPath)
	}
}

func TestRequestAIChatCompletionStream_NonClaudeFallsBackToSingleChunk(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-fallback","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[],\"recommendations\":[],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer server.Close()

	var chunks []string
	result, err := requestAIChatCompletionStream(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL,
		APIKey:       "key",
		Model:        "model-fallback",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	}, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected one fallback chunk, got %d", len(chunks))
	}
	if result.Content != chunks[0] {
		t.Fatalf("expected chunk to equal result content, got chunk=%q content=%q", chunks[0], result.Content)
	}
}

func TestRequestAIChatCompletion_ClaudeNonStreamingUsesSDK(t *testing.T) {
	t.Parallel()

	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"claude-3-5-sonnet-20241022",
			"choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[],\"recommendations\":[],\"disclaimer\":\"d\"}"}}]
		}`))
	}))
	defer server.Close()

	result, err := requestAIChatCompletion(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "claude-3-5-sonnet-20241022",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPath != "/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions, got %s", receivedPath)
	}
	if !strings.Contains(result.Content, "overall_summary") {
		t.Fatalf("unexpected content: %q", result.Content)
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

func TestAnalyzeHoldingsStreamEndToEndWithStub(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-stream", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-stream")

	originalStream := aiChatCompletionStream
	defer func() { aiChatCompletionStream = originalStream }()

	fullContent := `{
		"overall_summary":"stream ok",
		"risk_level":"balanced",
		"key_findings":["x"],
		"recommendations":[{"symbol":"AAPL","action":"hold","theory_tag":"Buffett","rationale":"wait"}],
		"disclaimer":"仅供参考"
	}`

	aiChatCompletionStream = func(ctx context.Context, req aiChatCompletionRequest, onDelta func(string) error) (aiChatCompletionResult, error) {
		if err := onDelta(`{"overall_summary":"stream ok"`); err != nil {
			return aiChatCompletionResult{}, err
		}
		if err := onDelta(`,"risk_level":"balanced","key_findings":["x"],"recommendations":[{"symbol":"AAPL","action":"hold","theory_tag":"Buffett","rationale":"wait"}],"disclaimer":"仅供参考"}`); err != nil {
			return aiChatCompletionResult{}, err
		}
		return aiChatCompletionResult{
			Model:   "mock-stream-model",
			Content: fullContent,
		}, nil
	}

	var streamed []string
	result, err := core.AnalyzeHoldingsStream(HoldingsAnalysisRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "claude-3-5-sonnet-20241022",
		Currency:        "USD",
		RiskProfile:     "balanced",
		Horizon:         "medium",
		AdviceStyle:     "balanced",
		AllowNewSymbols: true,
	}, func(delta string) error {
		streamed = append(streamed, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("AnalyzeHoldingsStream failed: %v", err)
	}
	if len(streamed) != 2 {
		t.Fatalf("expected 2 streamed chunks, got %d", len(streamed))
	}
	if result.Model != "mock-stream-model" {
		t.Fatalf("unexpected model: %s", result.Model)
	}
	if result.ID <= 0 {
		t.Fatalf("expected saved result id, got %d", result.ID)
	}
}

func TestAnalyzeHoldings_IncludesSymbolRefs(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-ref", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-ref")

	_, err := core.db.Exec(
		`INSERT INTO symbol_analyses (symbol, currency, model, status, synthesis, completed_at)
		 VALUES (?, ?, ?, 'completed', ?, CURRENT_TIMESTAMP)`,
		"AAPL",
		"USD",
		"mock-symbol-model",
		`{"overall_rating":"buy","target_action":"hold","overall_summary":"这是一段用于持仓引用的标的分析摘要","disclaimer":"仅供参考"}`,
	)
	if err != nil {
		t.Fatalf("insert symbol analysis seed failed: %v", err)
	}

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()
	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
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
	if len(result.SymbolRefs) != 1 {
		t.Fatalf("expected 1 symbol ref, got %d", len(result.SymbolRefs))
	}
	if result.SymbolRefs[0].Symbol != "AAPL" {
		t.Fatalf("unexpected symbol ref: %+v", result.SymbolRefs[0])
	}
}

func TestGetHoldingsAnalysisAndHistory_WithSeedData(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-history", "Main")
	testBuyTransaction(t, core, "MSFT", 5, 200, "USD", "acc-history")

	original := aiChatCompletion
	defer func() { aiChatCompletion = original }()
	aiChatCompletion = func(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
		return aiChatCompletionResult{
			Model: "mock-model",
			Content: `{
				"overall_summary":"history ok",
				"risk_level":"balanced",
				"key_findings":["x"],
				"recommendations":[{"symbol":"MSFT","action":"hold","theory_tag":"Buffett","rationale":"wait"}],
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

	latest, err := core.GetHoldingsAnalysis("USD")
	if err != nil {
		t.Fatalf("GetHoldingsAnalysis failed: %v", err)
	}
	if latest == nil || latest.Currency != "USD" {
		t.Fatalf("expected latest USD analysis, got %+v", latest)
	}

	history, err := core.GetHoldingsAnalysisHistory("USD", 10)
	if err != nil {
		t.Fatalf("GetHoldingsAnalysisHistory failed: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("expected non-empty history")
	}

	allHistory, err := core.GetHoldingsAnalysisHistory("", 10)
	if err != nil {
		t.Fatalf("GetHoldingsAnalysisHistory(all) failed: %v", err)
	}
	if len(allHistory) == 0 {
		t.Fatal("expected non-empty all-currency history")
	}
}

func TestAnalyzeHoldingsStream_NilCallback(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-nil", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-nil")

	originalStream := aiChatCompletionStream
	defer func() { aiChatCompletionStream = originalStream }()
	aiChatCompletionStream = func(ctx context.Context, req aiChatCompletionRequest, onDelta func(string) error) (aiChatCompletionResult, error) {
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

	result, err := core.AnalyzeHoldingsStream(HoldingsAnalysisRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "claude-3-5-sonnet-20241022",
		Currency:        "USD",
		RiskProfile:     "balanced",
		Horizon:         "medium",
		AdviceStyle:     "balanced",
		AllowNewSymbols: true,
	}, nil)
	if err != nil {
		t.Fatalf("AnalyzeHoldingsStream(nil callback) failed: %v", err)
	}
	if result == nil || result.OverallSummary == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAnalyzeHoldingsStream_CallbackReturnsError(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acc-callback", "Main")
	testBuyTransaction(t, core, "AAPL", 10, 100, "USD", "acc-callback")

	originalStream := aiChatCompletionStream
	defer func() { aiChatCompletionStream = originalStream }()
	aiChatCompletionStream = func(ctx context.Context, req aiChatCompletionRequest, onDelta func(string) error) (aiChatCompletionResult, error) {
		if err := onDelta("partial"); err != nil {
			return aiChatCompletionResult{}, err
		}
		return aiChatCompletionResult{}, nil
	}

	_, err := core.AnalyzeHoldingsStream(HoldingsAnalysisRequest{
		BaseURL:         "https://example.com/v1",
		APIKey:          "key",
		Model:           "claude-3-5-sonnet-20241022",
		Currency:        "USD",
		RiskProfile:     "balanced",
		Horizon:         "medium",
		AdviceStyle:     "balanced",
		AllowNewSymbols: true,
	}, func(delta string) error {
		return errors.New("stop from callback")
	})
	if err == nil || !strings.Contains(err.Error(), "stop from callback") {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestRequestAIChatCompletionStream_CallbackError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"model\":\"claude-3-5-sonnet-20241022\",\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	_, err := requestAIChatCompletionStream(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "claude-3-5-sonnet-20241022",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	}, func(delta string) error {
		return errors.New("stop stream")
	})
	if err == nil || !strings.Contains(err.Error(), "stream callback failed") {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestRequestAIChatCompletionStream_NilCallbackUsesDefaultPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"model-nil","choices":[{"message":{"content":"{\"overall_summary\":\"ok\",\"risk_level\":\"balanced\",\"key_findings\":[],\"recommendations\":[],\"disclaimer\":\"d\"}"}}]}`))
	}))
	defer server.Close()

	result, err := requestAIChatCompletionStream(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL,
		APIKey:       "key",
		Model:        "model-nil",
		SystemPrompt: "sys",
		UserPrompt:   "user",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "overall_summary") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestDecodeAIModelAndContent_FromOutputArray(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"m-output","output":[{"content":[{"type":"text","text":"hello from output"}]}]}`)
	model, content, err := decodeAIModelAndContent(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "m-output" {
		t.Fatalf("unexpected model: %s", model)
	}
	if content != "hello from output" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestExtractOutputContentAndExtractTextVariants(t *testing.T) {
	t.Parallel()

	output := []any{
		map[string]any{
			"content": []any{
				map[string]any{"text": "hello"},
			},
		},
	}
	if got := extractOutputContent(output); got != "hello" {
		t.Fatalf("unexpected output content: %q", got)
	}

	fallbackOutput := []any{
		map[string]any{
			"text": "fallback text",
		},
	}
	if got := extractOutputContent(fallbackOutput); got != "fallback text" {
		t.Fatalf("unexpected fallback output content: %q", got)
	}

	mixed := []any{
		"alpha",
		map[string]any{"value": "beta"},
		map[string]any{"content": "gamma"},
		map[string]any{"content": []any{map[string]any{"text": "delta"}}},
		map[string]any{"output_text": "epsilon"},
	}
	got := extractText(mixed)
	for _, want := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected extractText to contain %q, got %q", want, got)
		}
	}
}

func TestParseAIErrorMessage_TableDriven(t *testing.T) {
	t.Parallel()

	msg := parseAIErrorMessage([]byte(`{"error":{"message":"bad request"}}`))
	if msg != "bad request" {
		t.Fatalf("unexpected error message: %q", msg)
	}
	msg = parseAIErrorMessage([]byte(`{"message":"fallback msg"}`))
	if msg != "fallback msg" {
		t.Fatalf("unexpected fallback message: %q", msg)
	}
	msg = parseAIErrorMessage([]byte(`not-json`))
	if msg != "" {
		t.Fatalf("expected empty message for invalid json, got %q", msg)
	}
}

func TestAnyToStringAndNormalizeRecommendations(t *testing.T) {
	t.Parallel()

	if got := anyToString("abc"); got != "abc" {
		t.Fatalf("unexpected string conversion: %q", got)
	}
	if got := anyToString(float64(12.5)); got != "12.5" {
		t.Fatalf("unexpected float conversion: %q", got)
	}
	if got := anyToString(nil); got != "" {
		t.Fatalf("unexpected nil conversion: %q", got)
	}
	if got := anyToString(true); got != "true" {
		t.Fatalf("unexpected default conversion: %q", got)
	}

	normalized := normalizeRecommendations([]HoldingsAnalysisRecommendation{
		{
			Symbol:    " AAPL ",
			Action:    "",
			TheoryTag: "",
			Rationale: "",
			Priority:  " high ",
		},
	})
	if len(normalized) != 1 {
		t.Fatalf("unexpected normalized length: %d", len(normalized))
	}
	if normalized[0].Action != "hold" {
		t.Fatalf("expected default action hold, got %q", normalized[0].Action)
	}
	if normalized[0].TheoryTag != "Malkiel" {
		t.Fatalf("expected default theory, got %q", normalized[0].TheoryTag)
	}
	if normalized[0].Rationale == "" {
		t.Fatal("expected default rationale")
	}
	if normalized[0].Priority != "high" {
		t.Fatalf("expected trimmed priority, got %q", normalized[0].Priority)
	}
}

func TestGetHoldingsAnalysisHistory_EmptyResult(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	history, err := core.GetHoldingsAnalysisHistory("", 0)
	if err != nil {
		t.Fatalf("GetHoldingsAnalysisHistory failed: %v", err)
	}
	if history == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %d", len(history))
	}

	latest, err := core.GetHoldingsAnalysis("USD")
	if err != nil {
		t.Fatalf("GetHoldingsAnalysis failed: %v", err)
	}
	if latest != nil {
		t.Fatalf("expected nil latest analysis, got %+v", latest)
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

func TestFormatAIRequestForLog_MasksAuthHeadersAndKeepsBody(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"gemini-2.5-flash",
		"systemInstruction":{"parts":[{"text":"sys"}]},
		"contents":[{"parts":[{"text":"user"}]}]
	}`)
	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/v1beta/models/gemini:streamGenerateContent?alt=sse", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", "AIzaSySecretValue123456")
	req.Header.Set("Authorization", "Bearer sk-test-secret-key-123456")

	logged := formatAIRequestForLog(req, body)
	if strings.Contains(logged, "AIzaSySecretValue123456") {
		t.Fatalf("x-goog-api-key should be masked, got %s", logged)
	}
	if strings.Contains(logged, "sk-test-secret-key-123456") {
		t.Fatalf("authorization token should be masked, got %s", logged)
	}
	if !strings.Contains(logged, "\"X-Goog-Api-Key\"") {
		t.Fatalf("expected X-Goog-Api-Key in logged output: %s", logged)
	}
	if !strings.Contains(logged, "\"Authorization\"") {
		t.Fatalf("expected Authorization in logged output: %s", logged)
	}
	if !strings.Contains(logged, "\"systemInstruction\"") {
		t.Fatalf("expected request body in logged output: %s", logged)
	}
	if !strings.Contains(logged, "\"contents\"") {
		t.Fatalf("expected request body contents in logged output: %s", logged)
	}
	if strings.Contains(logged, "\n") {
		t.Fatalf("expected compact single-line JSON log, got %q", logged)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(logged), &decoded); err != nil {
		t.Fatalf("expected valid json log, got error: %v", err)
	}
	if _, ok := decoded["body"]; !ok {
		t.Fatalf("expected decoded body field in compact log: %v", decoded)
	}
}

func TestFormatAIRequestForLog_KeepsRawBodyCompact(t *testing.T) {
	t.Parallel()

	body := []byte("line1\nline2")
	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/v1/chat", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer sk-test-secret-key-123456")

	logged := formatAIRequestForLog(req, body)
	if strings.Contains(logged, "\n") {
		t.Fatalf("expected escaped newline in compact raw body log, got %q", logged)
	}
	if !strings.Contains(logged, "\"body_raw\":\"line1\\nline2\"") {
		t.Fatalf("expected escaped raw body in log output: %s", logged)
	}
	if strings.Contains(logged, "sk-test-secret-key-123456") {
		t.Fatalf("authorization token should be masked, got %s", logged)
	}
}
