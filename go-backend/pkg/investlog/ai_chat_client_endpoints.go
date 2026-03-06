package investlog

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

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
