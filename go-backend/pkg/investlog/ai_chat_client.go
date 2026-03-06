package investlog

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	defaultAIBaseURL      = "https://api.aicodemirror.com/api/gemini"
	aiRequestTimeout      = 3 * time.Minute
	aiTotalRequestTimeout = 15 * time.Minute
	maxAIResponseBodySize = 2 << 20
	aiMaxOutputTokens     = 128000
	geminiMaxOutputTokens = 32768
	aiMaxInputTokens      = 200000
)

type aiChatCompletionRequest struct {
	EndpointURL         string
	APIKey              string
	Model               string
	SystemPrompt        string
	UserPrompt          string
	Logger              *slog.Logger
	OnDelta             func(string)
	UseGoogleSearchTool bool
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
