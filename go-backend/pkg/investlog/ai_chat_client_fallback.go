package investlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

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
