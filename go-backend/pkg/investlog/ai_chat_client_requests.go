package investlog

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

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

