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
)

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
