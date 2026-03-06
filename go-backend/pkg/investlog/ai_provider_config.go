package investlog

import (
	"fmt"
	"net/url"
	"strings"
)

const defaultAIModel = "gemini-2.5-flash"

func isGeminiModel(model string) bool {
	trimmed := strings.TrimSpace(strings.TrimPrefix(model, "models/"))
	return strings.HasPrefix(strings.ToLower(trimmed), "gemini")
}

func normalizeAIModel(model string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(model, "models/"))
	if !isGeminiModel(trimmed) {
		return defaultAIModel
	}
	return trimmed
}

func normalizeAIModelName(model string) string {
	return normalizeAIModel(model)
}

func normalizeAIBaseURL(baseURL string) string {
	canonical, err := canonicalizeAIBaseURL(baseURL)
	if err != nil {
		return defaultAIBaseURL
	}
	return canonical
}

func canonicalizeAIBaseURL(baseURL string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = defaultAIBaseURL
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

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

	hostLower := strings.ToLower(parsed.Host)
	if strings.Contains(hostLower, "openai.com") || strings.Contains(hostLower, "perplexity.ai") {
		return defaultAIBaseURL, nil
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""

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
	parsed.Path = strings.TrimRight(basePath, "/")

	return strings.TrimRight(parsed.String(), "/"), nil
}
