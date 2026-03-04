package investlog

import (
	"bufio"
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
	"sort"
	"strings"
	"time"
)

const (
	defaultAIBaseURL      = "https://api.openai.com/v1"
	aiRequestTimeout      = 5 * time.Minute
	aiTotalRequestTimeout = 15 * time.Minute
	maxAIResponseBodySize = 2 << 20
	aiMaxOutputTokens     = 128000
	aiMaxInputTokens      = 200000
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

func shouldUseGeminiAPI(endpoint, model string) bool {
	lowerEndpoint := strings.ToLower(strings.TrimSpace(endpoint))
	lowerModel := strings.ToLower(strings.TrimSpace(model))

	if strings.Contains(lowerEndpoint, "/v1beta/models/") ||
		strings.Contains(lowerEndpoint, ":streamgeneratecontent") ||
		strings.Contains(lowerEndpoint, "generativelanguage.googleapis.com") ||
		strings.Contains(lowerEndpoint, "/gemini") {
		return true
	}

	if strings.HasPrefix(lowerModel, "gemini") &&
		!strings.Contains(lowerEndpoint, "openai.com") &&
		!strings.Contains(lowerEndpoint, "perplexity.ai") {
		return true
	}

	return false
}

func normalizeGeminiModelName(model string) string {
	trimmed := strings.TrimSpace(model)
	trimmed = strings.TrimPrefix(trimmed, "models/")
	return strings.TrimSpace(trimmed)
}

func stripKnownAIEndpointSuffix(path string) string {
	trimmed := strings.TrimRight(path, "/")
	lower := strings.ToLower(trimmed)

	switch {
	case strings.HasSuffix(lower, "/v1/chat/completions"):
		return trimmed[:len(trimmed)-len("/v1/chat/completions")]
	case strings.HasSuffix(lower, "/chat/completions"):
		return trimmed[:len(trimmed)-len("/chat/completions")]
	case strings.HasSuffix(lower, "/v1/responses"):
		return trimmed[:len(trimmed)-len("/v1/responses")]
	case strings.HasSuffix(lower, "/responses"):
		return trimmed[:len(trimmed)-len("/responses")]
	case strings.HasSuffix(lower, "/v1"):
		return trimmed[:len(trimmed)-len("/v1")]
	default:
		return trimmed
	}
}

func buildGeminiStreamEndpoint(endpoint, model string) (string, error) {
	modelName := normalizeGeminiModelName(model)
	if modelName == "" {
		return "", errors.New("gemini model is required")
	}

	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return "", errors.New("gemini endpoint is required")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid gemini endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid gemini endpoint scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", errors.New("invalid gemini endpoint host")
	}

	basePath := stripKnownAIEndpointSuffix(parsed.Path)
	basePath = stripGeminiVersionSuffix(basePath)
	lowerBasePath := strings.ToLower(basePath)
	if idx := strings.Index(lowerBasePath, "/v1beta/models/"); idx >= 0 {
		basePath = basePath[:idx]
	}
	lowerBasePath = strings.ToLower(basePath)
	if idx := strings.Index(lowerBasePath, ":streamgeneratecontent"); idx >= 0 {
		basePath = basePath[:idx]
	}
	lowerBasePath = strings.ToLower(basePath)
	if idx := strings.Index(lowerBasePath, ":generatecontent"); idx >= 0 {
		basePath = basePath[:idx]
	}
	basePath = strings.TrimRight(basePath, "/")

	parsed.Path = basePath + "/v1beta/models/" + url.PathEscape(modelName) + ":streamGenerateContent"
	if !strings.HasPrefix(parsed.Path, "/") {
		parsed.Path = "/" + parsed.Path
	}
	query := parsed.Query()
	query.Set("alt", "sse")
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func stripGeminiVersionSuffix(path string) string {
	trimmed := strings.TrimRight(path, "/")
	lower := strings.ToLower(trimmed)

	switch {
	case strings.HasSuffix(lower, "/v1beta/openai"):
		return trimmed[:len(trimmed)-len("/v1beta/openai")]
	case strings.HasSuffix(lower, "/v1/openai"):
		return trimmed[:len(trimmed)-len("/v1/openai")]
	case strings.HasSuffix(lower, "/v1beta"):
		return trimmed[:len(trimmed)-len("/v1beta")]
	case strings.HasSuffix(lower, "/v1"):
		return trimmed[:len(trimmed)-len("/v1")]
	default:
		return trimmed
	}
}

func setAIAuthHeader(httpReq *http.Request, endpoint, model, apiKey string) {
	if shouldUseGeminiAPI(endpoint, model) {
		httpReq.Header.Set("x-goog-api-key", apiKey)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
}

func maskSecretForLog(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 6 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:3] + strings.Repeat("*", len(trimmed)-6) + trimmed[len(trimmed)-3:]
}

func maskHeaderValueForLog(key, value string) string {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	trimmedValue := strings.TrimSpace(value)

	switch lowerKey {
	case "x-goog-api-key", "x-api-key":
		return maskSecretForLog(trimmedValue)
	case "authorization":
		lowerValue := strings.ToLower(trimmedValue)
		if strings.HasPrefix(lowerValue, "bearer ") && len(trimmedValue) >= len("Bearer ") {
			token := strings.TrimSpace(trimmedValue[len("Bearer "):])
			return "Bearer " + maskSecretForLog(token)
		}
		return maskSecretForLog(trimmedValue)
	default:
		return trimmedValue
	}
}

func formatAIRequestForLog(httpReq *http.Request, body []byte) string {
	logPayload := map[string]any{
		"method": httpReq.Method,
		"url":    httpReq.URL.String(),
	}

	headerKeys := make([]string, 0, len(httpReq.Header))
	for key := range httpReq.Header {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)

	headers := make(map[string]string, len(httpReq.Header))
	for _, key := range headerKeys {
		values := httpReq.Header.Values(key)
		joined := strings.TrimSpace(strings.Join(values, ", "))
		headers[key] = maskHeaderValueForLog(key, joined)
	}
	logPayload["headers"] = headers

	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) > 0 {
		var decoded any
		if err := json.Unmarshal(trimmedBody, &decoded); err == nil {
			logPayload["body"] = decoded
		} else {
			logPayload["body_raw"] = string(trimmedBody)
		}
	}

	encoded, err := json.MarshalIndent(logPayload, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":"marshal ai request log failed","detail":%q}`, err.Error())
	}
	return string(encoded)
}

func logAIRequestJSON(logger *slog.Logger, httpReq *http.Request, body []byte) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("ai request json", "request", formatAIRequestForLog(httpReq, body))
}

func requestAIChatCompletionStream(ctx context.Context, req aiChatCompletionRequest, onDelta func(string) error) (aiChatCompletionResult, error) {
	if onDelta == nil {
		return requestAIChatCompletion(ctx, req)
	}

	// Bridge the external onDelta callback into req.OnDelta so that
	// requestAIByChatCompletions relays SSE chunks through it.
	streamed := false
	var streamErr error
	bridged := req
	originalOnDelta := req.OnDelta
	bridged.OnDelta = func(delta string) {
		streamed = true
		if originalOnDelta != nil {
			originalOnDelta(delta)
		}
		if streamErr == nil {
			streamErr = onDelta(delta)
		}
	}

	result, err := requestAIChatCompletion(ctx, bridged)
	// Check callback error first — it takes priority.
	if streamErr != nil {
		return aiChatCompletionResult{}, fmt.Errorf("stream callback failed: %w", streamErr)
	}
	if err != nil {
		return aiChatCompletionResult{}, err
	}

	// If content wasn't delivered through streaming (e.g., responses/hybrid
	// fallback paths that don't call OnDelta), relay as a single chunk.
	if !streamed && result.Content != "" {
		if err := onDelta(result.Content); err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("stream callback failed: %w", err)
		}
	}
	return result, nil
}

func requestAIChatCompletion(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
	logger := req.Logger
	if logger == nil {
		logger = slog.Default()
	}

	endpoint := strings.TrimSpace(req.EndpointURL)
	if shouldUseGeminiAPI(endpoint, req.Model) {
		geminiEndpoint, err := buildGeminiStreamEndpoint(endpoint, req.Model)
		if err != nil {
			return aiChatCompletionResult{}, err
		}
		logger.Info("ai analyze: use gemini stream endpoint", "endpoint", geminiEndpoint, "model", req.Model)
		return requestAIByGeminiStream(ctx, req, geminiEndpoint)
	}

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

	logger := req.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Build streaming request payload.
	payload := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"temperature":           0.2,
		"stream":                true,
		"max_completion_tokens": aiMaxOutputTokens,
		"max_tokens":            aiMaxOutputTokens,
	}
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
	setAIAuthHeader(httpReq, endpoint, req.Model, req.APIKey)
	logAIRequestJSON(logger, httpReq, body)

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("ai request failed: %w", err)
	}
	defer resp.Body.Close()

	// Non-2xx: extract error message.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBodySize))
		logAIRawResponseDebug(logger, endpoint, resp.StatusCode, respBody)
		message := parseAIErrorMessage(respBody)
		if message == "" {
			message = strings.TrimSpace(string(respBody))
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return aiChatCompletionResult{}, fmt.Errorf("ai upstream error: %s", message)
	}

	contentType := resp.Header.Get("Content-Type")

	// SSE streaming response.
	if strings.Contains(contentType, "text/event-stream") {
		model := strings.TrimSpace(req.Model)
		fullContent, parsedModel, err := parseSSEStream(resp.Body, func(m, delta string) error {
			if m != "" {
				model = m
			}
			if req.OnDelta != nil {
				req.OnDelta(delta)
			}
			return nil
		})
		if err != nil {
			if isTimeoutError(err) {
				return aiChatCompletionResult{}, fmt.Errorf("ai upstream timeout on %s; try a faster model or retry later", endpoint)
			}
			// Stream decode failed — try one-shot fallback.
			logger.Warn("ai analyze: chat stream decode failed, fallback to one-shot", "endpoint", endpoint, "err", err)
		} else {
			content := strings.TrimSpace(fullContent)
			if content != "" {
				if parsedModel != "" {
					model = parsedModel
				}
				if model == "" {
					model = req.Model
				}
				return aiChatCompletionResult{Model: model, Content: content}, nil
			}
		}
	} else if strings.Contains(contentType, "application/json") {
		// Non-streaming JSON response (some providers ignore stream:true).
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBodySize))
		if err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("read ai response: %w", err)
		}
		logAIRawResponseDebug(logger, endpoint, resp.StatusCode, respBody)
		model, content, err := decodeAIModelAndContent(respBody)
		if err == nil && content != "" {
			if req.OnDelta != nil {
				req.OnDelta(content)
			}
			return aiChatCompletionResult{Model: model, Content: content}, nil
		}
		// JSON decode failed or content empty — fall through to one-shot retry.
		if err != nil {
			logger.Warn("ai analyze: json response decode failed, fallback to one-shot", "endpoint", endpoint, "err", err)
		}
	}

	// Fallback: retry as non-streaming one-shot.
	payload["stream"] = false
	return requestAIByPayload(ctx, req, endpoint, payload)
}

func buildGeminiStreamPayload(req aiChatCompletionRequest) map[string]any {
	payload := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": req.UserPrompt},
				},
			},
		},
	}

	if strings.TrimSpace(req.SystemPrompt) != "" {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]string{
				{"text": req.SystemPrompt},
			},
		}
	}

	return payload
}

func requestAIByGeminiStream(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, endpoint, req.Model, req.SystemPrompt, req.UserPrompt)

	logger := req.Logger
	if logger == nil {
		logger = slog.Default()
	}

	payload := buildGeminiStreamPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("marshal gemini request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, aiRequestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("build gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	setAIAuthHeader(httpReq, endpoint, req.Model, req.APIKey)
	logAIRequestJSON(logger, httpReq, body)

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("ai request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBodySize))
		logAIRawResponseDebug(logger, endpoint, resp.StatusCode, respBody)
		message := parseAIErrorMessage(respBody)
		if message == "" {
			message = strings.TrimSpace(string(respBody))
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return aiChatCompletionResult{}, fmt.Errorf("ai upstream error: %s", message)
	}

	contentType := resp.Header.Get("Content-Type")
	model := strings.TrimSpace(req.Model)
	if strings.Contains(contentType, "text/event-stream") {
		fullContent, parsedModel, err := parseSSEStream(resp.Body, func(m, delta string) error {
			if m != "" {
				model = m
			}
			if req.OnDelta != nil {
				req.OnDelta(delta)
			}
			return nil
		})
		if err != nil {
			if isTimeoutError(err) {
				return aiChatCompletionResult{}, fmt.Errorf("ai upstream timeout on %s; try a faster model or retry later", endpoint)
			}
			return aiChatCompletionResult{}, err
		}
		content := strings.TrimSpace(fullContent)
		if content == "" {
			return aiChatCompletionResult{}, errors.New("ai response content is empty")
		}
		if parsedModel != "" {
			model = parsedModel
		}
		if model == "" {
			model = req.Model
		}
		return aiChatCompletionResult{Model: model, Content: content}, nil
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBodySize))
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("read ai response: %w", err)
	}
	logAIRawResponseDebug(logger, endpoint, resp.StatusCode, respBody)

	decodedModel, content, err := decodeAIModelAndContent(respBody)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	if content == "" {
		return aiChatCompletionResult{}, errors.New("ai response content is empty")
	}
	if decodedModel != "" {
		model = decodedModel
	}
	if req.OnDelta != nil {
		req.OnDelta(content)
	}
	return aiChatCompletionResult{Model: model, Content: content}, nil
}

// sseChunk represents one SSE chunk in the OpenAI chat completions streaming format.
type sseChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type geminiSSEChunk struct {
	ModelVersion string `json:"modelVersion"`
	Candidates   []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// parseSSEStream reads an SSE stream in OpenAI/Gemini-compatible formats and
// calls onChunk for each content delta. It returns the accumulated content and
// the last seen model identifier.
func parseSSEStream(body io.Reader, onChunk func(model, delta string) error) (string, string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var (
		builder strings.Builder
		model   string
	)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines, event lines, and comment lines.
		if line == "" || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") && !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimPrefix(data, "data:")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			break
		}

		chunkModel, delta, handled := extractOpenAIStyleSSEChunk(data)
		if !handled {
			chunkModel, delta, handled = extractGeminiStyleSSEChunk(data)
		}
		if !handled {
			slog.Default().Warn("ai sse: failed to parse chunk", "data", data)
			continue
		}
		if chunkModel != "" {
			model = chunkModel
		}
		if delta == "" {
			continue
		}

		builder.WriteString(delta)
		if err := onChunk(model, delta); err != nil {
			return builder.String(), model, fmt.Errorf("stream callback failed: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return builder.String(), model, fmt.Errorf("ai sse read error: %w", err)
	}

	return builder.String(), model, nil
}

func extractOpenAIStyleSSEChunk(data string) (string, string, bool) {
	var chunk sseChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", "", false
	}

	model := strings.TrimSpace(chunk.Model)
	if len(chunk.Choices) == 0 {
		return model, "", model != ""
	}
	return model, chunk.Choices[0].Delta.Content, true
}

func extractGeminiStyleSSEChunk(data string) (string, string, bool) {
	var chunk geminiSSEChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", "", false
	}

	model := strings.TrimSpace(chunk.ModelVersion)
	if len(chunk.Candidates) == 0 {
		return model, "", model != ""
	}

	var builder strings.Builder
	for _, candidate := range chunk.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text == "" {
				continue
			}
			builder.WriteString(part.Text)
		}
	}

	return model, builder.String(), true
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
	setAIAuthHeader(httpReq, endpoint, req.Model, req.APIKey)
	logAIRequestJSON(req.Logger, httpReq, body)

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
	if model == "" {
		model = asString(raw["modelVersion"])
	}
	if outputText := asString(raw["output_text"]); outputText != "" {
		return model, outputText, nil
	}

	if text := extractChoicesContent(raw["choices"]); text != "" {
		return model, text, nil
	}
	if text := extractCandidatesContent(raw["candidates"]); text != "" {
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

func extractCandidatesContent(value any) string {
	candidates, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, candidate := range candidates {
		candidateMap, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if text := extractText(candidateMap["content"]); text != "" {
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
		if parts, ok := typed["parts"].([]any); ok {
			var builder strings.Builder
			for _, part := range parts {
				partMap, ok := part.(map[string]any)
				if ok {
					if text := asString(partMap["text"]); text != "" {
						builder.WriteString(text)
						continue
					}
				}
				if text := extractText(part); text != "" {
					builder.WriteString(text)
				}
			}
			if builder.Len() > 0 {
				return strings.TrimSpace(builder.String())
			}
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
