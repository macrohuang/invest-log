package investlog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"
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

// normalizeAIClientBaseURL normalizes a base URL for use with the OpenAI SDK client.
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

// buildAICompletionsEndpoint builds the full chat/completions endpoint URL from a base URL.
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

// cleanupModelJSON strips Markdown code fences and extracts the outermost JSON object
// from a model response string.
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
