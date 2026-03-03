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

	openai "github.com/openai/openai-go"
	openaioption "github.com/openai/openai-go/option"
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
