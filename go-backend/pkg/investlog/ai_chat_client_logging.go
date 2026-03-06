package investlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

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
