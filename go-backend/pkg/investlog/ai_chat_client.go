package investlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	openaioption "github.com/openai/openai-go/option"
)

const (
	defaultAIBaseURL      = "https://api.openai.com/v1"
	defaultGeminiBaseURL  = "https://generativelanguage.googleapis.com/v1beta"
	aiRequestTimeout      = 5 * time.Minute
	aiTotalRequestTimeout = 15 * time.Minute
	maxAIResponseBodySize = 2 << 20
	aiMaxOutputTokens     = 128000
	aiMaxInputTokens      = 200000
	aiAnthropicMaxTokens  = 8192
)

type aiChatCompletionRequest struct {
	EndpointURL  string
	APIKey       string
	Model        string
	SystemPrompt string
	UserPrompt   string
	Logger       *slog.Logger
	OnDelta      func(string)
}

type aiChatCompletionResult struct {
	Model   string
	Content string
}

var aiChatCompletion = requestAIChatCompletion
var aiChatCompletionStream = requestAIChatCompletionStream
var aiGeminiCompletion = requestAIByGeminiNative

// normalizeEnum validates and normalizes an enum value against an allowed set.
// Returns fallback if raw is empty, or an error if raw is not in allowed.
func normalizeEnum(raw, fallback string, allowed map[string]struct{}) (string, error) {
	if raw == "" {
		return fallback, nil
	}
	normalized := strings.ToLower(raw)
	if _, ok := allowed[normalized]; !ok {
		return "", fmt.Errorf("unsupported value: %s", raw)
	}
	return normalized, nil
}

// cleanupModelJSON strips markdown code fences and extracts the outermost JSON object
// from model-generated content that may contain surrounding noise.
func cleanupModelJSON(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.Join(lines, "\n")
		}
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}
	return strings.TrimSpace(trimmed)
}

func normalizeAIClientBaseURL(baseURL string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = defaultAIBaseURL
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid base_url scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid base_url host")
	}

	originalPath := strings.TrimRight(parsed.Path, "/")
	path := strings.ToLower(originalPath)
	trimmedPath := originalPath

	hasCompletionsSuffix := strings.HasSuffix(path, "/chat/completions")
	hasResponsesSuffix := strings.HasSuffix(path, "/responses")

	switch {
	case hasCompletionsSuffix:
		trimmedPath = strings.TrimRight(originalPath[:len(originalPath)-len("/chat/completions")], "/")
	case hasResponsesSuffix:
		trimmedPath = strings.TrimRight(originalPath[:len(originalPath)-len("/responses")], "/")
	}

	switch {
	case trimmedPath == "":
		if !hasCompletionsSuffix && !hasResponsesSuffix {
			trimmedPath = "/v1"
		}
	case strings.HasSuffix(strings.ToLower(trimmedPath), "/v1"):
		// keep path as-is
	case !hasCompletionsSuffix && !hasResponsesSuffix:
		trimmedPath += "/v1"
	}

	parsed.Path = trimmedPath
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return strings.TrimRight(parsed.String(), "/"), nil
}

func buildAICompletionsEndpoint(baseURL string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = defaultAIBaseURL
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")
	lower := strings.ToLower(trimmed)

	endpoint := ""
	switch {
	case strings.HasSuffix(lower, "/chat/completions"):
		endpoint = trimmed
	case strings.HasSuffix(lower, "/responses"):
		endpoint = trimmed
	case strings.HasSuffix(lower, "/v1"):
		endpoint = trimmed + "/chat/completions"
	case strings.Contains(lower, "perplexity.ai"):
		// Perplexity uses /chat/completions directly without /v1 prefix.
		endpoint = trimmed + "/chat/completions"
	default:
		endpoint = trimmed + "/v1/chat/completions"
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid base_url scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid base_url host")
	}
	return endpoint, nil
}

func requestAIChatCompletionStream(ctx context.Context, req aiChatCompletionRequest, onDelta func(string) error) (aiChatCompletionResult, error) {
	if onDelta == nil {
		return requestAIChatCompletion(ctx, req)
	}

	if isGeminiRequest(req.EndpointURL, req.Model) {
		return aiGeminiCompletion(ctx, req, onDelta)
	}

	if shouldUseAnthropicSDK(req.EndpointURL, req.Model) {
		return requestAIByAnthropic(ctx, req, onDelta)
	}

	result, err := requestAIChatCompletion(ctx, req)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	if result.Content != "" {
		if err := onDelta(result.Content); err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("stream callback failed: %w", err)
		}
	}
	return result, nil
}

func requestAIChatCompletion(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
	if isGeminiRequest(req.EndpointURL, req.Model) {
		return aiGeminiCompletion(ctx, req, nil)
	}

	if shouldUseAnthropicSDK(req.EndpointURL, req.Model) {
		return requestAIByAnthropic(ctx, req, nil)
	}

	logger := req.Logger
	if logger == nil {
		logger = slog.Default()
	}

	endpoint := strings.TrimSpace(req.EndpointURL)
	if strings.HasSuffix(strings.ToLower(endpoint), "/responses") {
		return requestAIByResponsesCandidates(ctx, req, endpoint)
	}

	chatCandidates := collectChatCandidates(endpoint)
	chatErrors := make([]string, 0, len(chatCandidates))
	sameEndpointErrors := []string{}
	allowResponsesFallback := false

	for _, candidate := range chatCandidates {
		logger.Info("ai analyze: try chat endpoint", "endpoint", candidate, "model", req.Model)
		chatResult, err := requestAIByChatCompletions(ctx, req, candidate)
		if err == nil {
			logger.Info("ai analyze: chat endpoint succeeded", "endpoint", candidate)
			return chatResult, nil
		}
		logger.Warn("ai analyze: chat endpoint failed", "endpoint", candidate, "err", err)
		chatErrors = append(chatErrors, fmt.Sprintf("%s -> %v", candidate, err))
		if shouldFallbackToResponses(err) || shouldFallbackToAltEndpoint(err) {
			allowResponsesFallback = true
		}

		if shouldFallbackToResponses(err) {
			logger.Info("ai analyze: try responses payload on same endpoint", "endpoint", candidate)
			sameEndpointResult, sameErr := requestAIByResponses(ctx, req, candidate)
			if sameErr == nil {
				logger.Info("ai analyze: same endpoint with responses payload succeeded", "endpoint", candidate)
				return sameEndpointResult, nil
			}
			logger.Warn("ai analyze: same endpoint with responses payload failed", "endpoint", candidate, "err", sameErr)
			sameEndpointErrors = append(sameEndpointErrors, fmt.Sprintf("%s -> %v", candidate, sameErr))

			logger.Info("ai analyze: try hybrid payload on same endpoint", "endpoint", candidate)
			hybridResult, hybridErr := requestAIByHybridPayload(ctx, req, candidate)
			if hybridErr == nil {
				logger.Info("ai analyze: same endpoint with hybrid payload succeeded", "endpoint", candidate)
				return hybridResult, nil
			}
			logger.Warn("ai analyze: same endpoint with hybrid payload failed", "endpoint", candidate, "err", hybridErr)
			sameEndpointErrors = append(sameEndpointErrors, fmt.Sprintf("%s(hybrid) -> %v", candidate, hybridErr))

			if isTimeoutError(sameErr) || isTimeoutError(hybridErr) {
				return aiChatCompletionResult{}, fmt.Errorf("ai upstream timeout on %s; try a faster model or retry later", candidate)
			}
		}
	}

	if allowResponsesFallback && !supportsResponsesFallbackModel(req.Model) {
		logger.Info("ai analyze: skip responses fallback for model", "model", req.Model)
		allowResponsesFallback = false
	}

	if !allowResponsesFallback {
		if len(sameEndpointErrors) > 0 {
			return aiChatCompletionResult{}, fmt.Errorf("chat completion failed: %s; same-endpoint responses attempts failed: %s", strings.Join(chatErrors, " | "), strings.Join(sameEndpointErrors, " | "))
		}
		return aiChatCompletionResult{}, fmt.Errorf("chat completion failed: %s", strings.Join(chatErrors, " | "))
	}

	responsesResult, err := requestAIByResponsesCandidates(ctx, req, endpoint)
	if err == nil {
		logger.Info("ai analyze: responses fallback succeeded")
		return responsesResult, nil
	}
	logger.Error("ai analyze: responses fallback failed", "err", err)

	if len(sameEndpointErrors) > 0 {
		return aiChatCompletionResult{}, fmt.Errorf("chat completion failed (%s); same-endpoint responses attempts failed (%s); responses fallback failed: %w", strings.Join(chatErrors, " | "), strings.Join(sameEndpointErrors, " | "), err)
	}
	return aiChatCompletionResult{}, fmt.Errorf("chat completion failed (%s); responses fallback failed: %w", strings.Join(chatErrors, " | "), err)
}

func requestAIByResponsesCandidates(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	responseCandidates := collectResponsesCandidates(endpoint)
	errs := make([]string, 0, len(responseCandidates))
	for _, candidate := range responseCandidates {
		result, err := requestAIByResponses(ctx, req, candidate)
		if err == nil {
			return result, nil
		}
		errs = append(errs, fmt.Sprintf("%s -> %v", candidate, err))
	}
	return aiChatCompletionResult{}, fmt.Errorf("responses attempts failed: %s", strings.Join(errs, " | "))
}

func collectChatCandidates(endpoint string) []string {
	result := []string{}
	addUniqueString(&result, strings.TrimSpace(endpoint))
	addUniqueString(&result, toAltChatEndpoint(endpoint))
	return result
}

func collectResponsesCandidates(endpoint string) []string {
	chatCandidates := collectChatCandidates(endpoint)
	result := []string{}
	for _, candidate := range chatCandidates {
		responsesEndpoint := toResponsesEndpoint(candidate)
		addUniqueString(&result, responsesEndpoint)
		addUniqueString(&result, toAltResponsesEndpoint(responsesEndpoint))
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(endpoint)), "/responses") {
		addUniqueString(&result, strings.TrimSpace(endpoint))
		addUniqueString(&result, toAltResponsesEndpoint(endpoint))
	}
	return result
}

func addUniqueString(items *[]string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	for _, item := range *items {
		if item == trimmed {
			return
		}
	}
	*items = append(*items, trimmed)
}

func toAltChatEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/v1/chat/completions") {
		return trimmed[:len(trimmed)-len("/v1/chat/completions")] + "/chat/completions"
	}
	if strings.HasSuffix(lower, "/chat/completions") && !strings.HasSuffix(lower, "/v1/chat/completions") {
		return trimmed[:len(trimmed)-len("/chat/completions")] + "/v1/chat/completions"
	}
	return ""
}

func toAltResponsesEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/v1/responses") {
		return trimmed[:len(trimmed)-len("/v1/responses")] + "/responses"
	}
	if strings.HasSuffix(lower, "/responses") && !strings.HasSuffix(lower, "/v1/responses") {
		return trimmed[:len(trimmed)-len("/responses")] + "/v1/responses"
	}
	return ""
}

func requestAIByChatCompletions(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, endpoint, req.Model, req.SystemPrompt, req.UserPrompt)

	baseURL, err := normalizeAIClientBaseURL(endpoint)
	if err != nil {
		return aiChatCompletionResult{}, err
	}

	client := openai.NewClient(
		openaioption.WithBaseURL(baseURL),
		openaioption.WithAPIKey(req.APIKey),
		openaioption.WithRequestTimeout(aiRequestTimeout),
		openaioption.WithMaxRetries(0),
	)

	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.SystemPrompt),
			openai.UserMessage(req.UserPrompt),
		},
		Model:               openai.ChatModel(req.Model),
		Temperature:         openai.Float(0.2),
		MaxCompletionTokens: openai.Int(aiMaxOutputTokens),
		MaxTokens:           openai.Int(aiMaxOutputTokens),
	})
	defer func() { _ = stream.Close() }()

	var (
		model                    = strings.TrimSpace(req.Model)
		builder                  strings.Builder
		streamFallbackErrMessage string
	)
	for stream.Next() {
		chunk := stream.Current()
		if strings.TrimSpace(chunk.Model) != "" {
			model = strings.TrimSpace(chunk.Model)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		builder.WriteString(delta)
		if req.OnDelta != nil {
			req.OnDelta(delta)
		}
	}

	if streamErr := stream.Err(); streamErr != nil {
		if isTimeoutError(streamErr) {
			return aiChatCompletionResult{}, fmt.Errorf("ai upstream timeout on %s; try a faster model or retry later", endpoint)
		}
		if shouldFallbackToNonStreaming(streamErr) {
			streamFallbackErrMessage = strings.TrimSpace(streamErr.Error())
			logger := req.Logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("ai analyze: chat stream decode failed, fallback to one-shot", "endpoint", endpoint, "err", streamErr)
		} else {
			var upstreamErr *openai.Error
			if errors.As(streamErr, &upstreamErr) {
				message := strings.TrimSpace(upstreamErr.Message)
				if message == "" {
					message = strings.TrimSpace(upstreamErr.RawJSON())
				}
				if message == "" {
					message = strings.TrimSpace(streamErr.Error())
				}
				return aiChatCompletionResult{}, fmt.Errorf("ai upstream error: %s", message)
			}
			return aiChatCompletionResult{}, fmt.Errorf("ai request failed: %w", streamErr)
		}
	}

	content := strings.TrimSpace(builder.String())
	if content != "" {
		if model == "" {
			model = req.Model
		}
		return aiChatCompletionResult{Model: model, Content: content}, nil
	}

	// Fallback for providers/tests that ignore stream and return one-shot JSON.
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.SystemPrompt),
			openai.UserMessage(req.UserPrompt),
		},
		Model:               openai.ChatModel(req.Model),
		Temperature:         openai.Float(0.2),
		MaxCompletionTokens: openai.Int(aiMaxOutputTokens),
		MaxTokens:           openai.Int(aiMaxOutputTokens),
	})
	if err != nil {
		if streamFallbackErrMessage != "" {
			return aiChatCompletionResult{}, fmt.Errorf("ai request failed: stream decode error: %s; one-shot retry failed: %w", streamFallbackErrMessage, err)
		}
		if isTimeoutError(err) {
			return aiChatCompletionResult{}, fmt.Errorf("ai upstream timeout on %s; try a faster model or retry later", endpoint)
		}
		var upstreamErr *openai.Error
		if errors.As(err, &upstreamErr) {
			message := strings.TrimSpace(upstreamErr.Message)
			if message == "" {
				message = strings.TrimSpace(upstreamErr.RawJSON())
			}
			if message == "" {
				message = strings.TrimSpace(err.Error())
			}
			return aiChatCompletionResult{}, fmt.Errorf("ai upstream error: %s", message)
		}
		return aiChatCompletionResult{}, fmt.Errorf("ai request failed: %w", err)
	}

	if resp != nil {
		if strings.TrimSpace(resp.Model) != "" {
			model = strings.TrimSpace(resp.Model)
		}
		if len(resp.Choices) > 0 {
			content = strings.TrimSpace(resp.Choices[0].Message.Content)
		}
	}
	if content == "" {
		return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
	}
	if req.OnDelta != nil {
		req.OnDelta(content)
	}

	return aiChatCompletionResult{Model: model, Content: content}, nil
}

func requestAIByResponses(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	payload := map[string]any{
		"model":             req.Model,
		"instructions":      req.SystemPrompt,
		"input":             req.UserPrompt,
		"temperature":       0.2,
		"stream":            false,
		"max_output_tokens": aiMaxOutputTokens,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
	}
	return requestAIByPayload(ctx, req, endpoint, payload)
}

func requestAIByHybridPayload(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	payload := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"input":                 req.UserPrompt,
		"instructions":          req.SystemPrompt,
		"temperature":           0.2,
		"stream":                false,
		"max_tokens":            aiMaxOutputTokens,
		"max_completion_tokens": aiMaxOutputTokens,
		"max_output_tokens":     aiMaxOutputTokens,
	}
	return requestAIByPayload(ctx, req, endpoint, payload)
}

func requestAIByPayload(ctx context.Context, req aiChatCompletionRequest, endpoint string, payload map[string]any) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, endpoint, req.Model, req.SystemPrompt, req.UserPrompt)

	body, err := json.Marshal(payload)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("marshal ai request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, aiRequestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("build ai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	respBody, err := executeAIRequest(httpReq, req.Logger)
	if err != nil {
		return aiChatCompletionResult{}, err
	}

	model, content, err := decodeAIModelAndContent(respBody)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	if content == "" {
		return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
	}
	return aiChatCompletionResult{Model: model, Content: content}, nil
}

func logAIPromptDebug(logger *slog.Logger, endpoint, model, systemPrompt, userPrompt string) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Debug("ai request prompt",
		"endpoint", strings.TrimSpace(endpoint),
		"model", strings.TrimSpace(model),
		"system_prompt", systemPrompt,
		"user_prompt", userPrompt,
	)
}

func logAIRawResponseDebug(logger *slog.Logger, endpoint string, statusCode int, body []byte) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Debug("ai raw response",
		"endpoint", strings.TrimSpace(endpoint),
		"status_code", statusCode,
		"body_bytes", len(body),
		"raw_body", string(body),
	)
}

func decodeAIModelAndContent(body []byte) (string, string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", "", fmt.Errorf("decode ai response: %w", err)
	}

	model := asString(raw["model"])
	if outputText := asString(raw["output_text"]); outputText != "" {
		return model, outputText, nil
	}

	if text := extractChoicesContent(raw["choices"]); text != "" {
		return model, text, nil
	}
	if text := extractOutputContent(raw["output"]); text != "" {
		return model, text, nil
	}
	if text := extractText(raw["content"]); text != "" {
		return model, text, nil
	}

	return model, "", fmt.Errorf("ai response content is empty")
}

func extractChoicesContent(value any) string {
	choices, ok := value.([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	first, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	message, ok := first["message"].(map[string]any)
	if ok {
		if text := extractText(message["content"]); text != "" {
			return text
		}
	}
	return extractText(first["text"])
}

func extractOutputContent(value any) string {
	outputs, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, output := range outputs {
		outputMap, ok := output.(map[string]any)
		if !ok {
			continue
		}
		if text := extractText(outputMap["content"]); text != "" {
			return text
		}
		if text := extractText(outputMap["text"]); text != "" {
			return text
		}
	}
	return ""
}

func extractText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := extractText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		if text := asString(typed["text"]); text != "" {
			return text
		}
		if text := asString(typed["value"]); text != "" {
			return text
		}
		if text := asString(typed["content"]); text != "" {
			return text
		}
		if text := extractText(typed["content"]); text != "" {
			return text
		}
		if text := extractText(typed["output_text"]); text != "" {
			return text
		}
	}
	return ""
}

func asString(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func executeAIRequest(httpReq *http.Request, logger *slog.Logger) ([]byte, error) {
	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBodySize))
	if err != nil {
		return nil, fmt.Errorf("read ai response: %w", err)
	}

	logAIRawResponseDebug(logger, httpReq.URL.String(), resp.StatusCode, respBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := parseAIErrorMessage(respBody)
		if message == "" {
			message = strings.TrimSpace(string(respBody))
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("ai upstream error: %s", message)
	}

	return respBody, nil
}

func shouldFallbackToResponses(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "input is required") || strings.Contains(message, "missing required parameter: input")
}

func shouldFallbackToAltEndpoint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") || strings.Contains(message, "404") || strings.Contains(message, "unknown path")
}

func shouldFallbackToNonStreaming(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unexpected end of json input") {
		return true
	}
	if strings.Contains(message, "invalid character") && strings.Contains(message, "json") {
		return true
	}
	if strings.Contains(message, "decode") && strings.Contains(message, "json") {
		return true
	}
	return false
}

func supportsResponsesFallbackModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return true
	}
	// Perplexity sonar models are chat-completions-only and reject /responses.
	return !strings.HasPrefix(normalized, "sonar")
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "context deadline exceeded") || strings.Contains(message, "timeout")
}

func toResponsesEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/responses") {
		return trimmed
	}
	if strings.HasSuffix(lower, "/chat/completions") {
		return trimmed[:len(trimmed)-len("/chat/completions")] + "/responses"
	}
	if strings.HasSuffix(lower, "/v1") {
		return trimmed + "/responses"
	}
	return ""
}

func parseAIErrorMessage(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if strings.TrimSpace(payload.Error.Message) != "" {
		return strings.TrimSpace(payload.Error.Message)
	}
	return strings.TrimSpace(payload.Message)
}
