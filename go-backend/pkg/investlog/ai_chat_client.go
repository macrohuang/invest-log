package investlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAIBaseURL      = "https://api.aicodemirror.com/api/gemini"
	aiRequestTimeout      = 5 * time.Minute
	aiTotalRequestTimeout = 15 * time.Minute
	maxAIResponseBodySize = 2 << 20
	aiMaxOutputTokens     = 128000
	geminiMaxOutputTokens = 32768
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
		"generationConfig": map[string]any{
			"temperature":     0.2,
			"maxOutputTokens": geminiMaxOutputTokens,
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
