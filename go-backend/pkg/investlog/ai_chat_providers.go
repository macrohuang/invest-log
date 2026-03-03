package investlog

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	"google.golang.org/genai"
)

func requestAIByGeminiNative(ctx context.Context, req aiChatCompletionRequest, onDelta func(string) error) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, req.EndpointURL, req.Model, req.SystemPrompt, req.UserPrompt)

	if shouldFallbackToGeminiDefaultBaseURL(req.EndpointURL) {
		logger := req.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("gemini request uses openai default base url; fallback to gemini base url",
			"configured_endpoint", req.EndpointURL,
			"fallback_base_url", defaultGeminiBaseURL,
		)
	}

	clientConfig, err := buildGeminiClientConfig(req.EndpointURL, req.APIKey)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("create gemini client failed: %w", err)
	}

	requestConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: req.SystemPrompt}},
		},
		Temperature:     genai.Ptr(float32(0.2)),
		MaxOutputTokens: aiMaxOutputTokens,
		// 不强制设置 ResponseMIMEType，部分镜像不支持该参数会导致请求失败。
		// JSON 格式由 system prompt 引导输出。
	}
	contents := genai.Text(req.UserPrompt)

	if onDelta == nil {
		response, err := client.Models.GenerateContent(ctx, req.Model, contents, requestConfig)
		if err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("gemini generate content failed: %w", err)
		}
		content := strings.TrimSpace(response.Text())
		if content == "" {
			return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
		}
		if req.OnDelta != nil {
			req.OnDelta(content)
		}
		model := strings.TrimSpace(response.ModelVersion)
		if model == "" {
			model = req.Model
		}
		return aiChatCompletionResult{Model: model, Content: content}, nil
	}

	accumulated := ""
	model := strings.TrimSpace(req.Model)
	for response, err := range client.Models.GenerateContentStream(ctx, req.Model, contents, requestConfig) {
		if err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("gemini stream generate content failed: %w", err)
		}
		if response == nil {
			continue
		}

		if model == "" {
			model = strings.TrimSpace(response.ModelVersion)
		}

		chunkText := response.Text()
		if chunkText == "" {
			continue
		}
		delta := chunkText
		if strings.HasPrefix(chunkText, accumulated) {
			delta = chunkText[len(accumulated):]
		}
		if delta == "" {
			continue
		}

		accumulated += delta
		if req.OnDelta != nil {
			req.OnDelta(delta)
		}
		if err := onDelta(delta); err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("stream callback failed: %w", err)
		}
	}

	content := strings.TrimSpace(accumulated)
	if content == "" {
		return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
	}
	if model == "" {
		model = req.Model
	}
	return aiChatCompletionResult{Model: model, Content: content}, nil
}

func buildGeminiClientConfig(endpoint, apiKey string) (*genai.ClientConfig, error) {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if shouldFallbackToGeminiDefaultBaseURL(normalizedEndpoint) {
		normalizedEndpoint = defaultGeminiBaseURL
	}

	baseURL, apiVersion, err := parseGeminiBaseURLAndVersion(normalizedEndpoint)
	if err != nil {
		return nil, err
	}
	return &genai.ClientConfig{
		APIKey:  strings.TrimSpace(apiKey),
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			// SDK 拼接 URL 时会在 BaseURL 和 suffix 之间固定插入 "/"，
			// 若 BaseURL 本身有尾部斜杠则产生 "//"，导致镜像路由匹配失败（404）。
			BaseURL:    strings.TrimRight(baseURL, "/"),
			APIVersion: apiVersion,
		},
	}, nil
}

func shouldFallbackToGeminiDefaultBaseURL(endpoint string) bool {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return true
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.openai.com")
}

func parseGeminiBaseURLAndVersion(endpoint string) (string, string, error) {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		trimmed = defaultGeminiBaseURL
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("invalid gemini endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("invalid gemini endpoint scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", "", fmt.Errorf("invalid gemini endpoint host")
	}

	path := strings.Trim(parsed.Path, "/")
	segments := []string{}
	if path != "" {
		segments = strings.Split(path, "/")
	}

	apiVersion := "v1beta"
	prefixSegments := []string{}
	foundVersion := false
	for idx, segment := range segments {
		segmentLower := strings.ToLower(strings.TrimSpace(segment))
		if strings.HasPrefix(segmentLower, "v1") {
			apiVersion = segment
			prefixSegments = segments[:idx]
			foundVersion = true
			break
		}
	}
	if !foundVersion {
		prefixSegments = segments
	}

	basePath := strings.Trim(strings.Join(prefixSegments, "/"), "/")
	baseURL := fmt.Sprintf("%s://%s/", parsed.Scheme, parsed.Host)
	if basePath != "" {
		baseURL += basePath + "/"
	}
	return baseURL, apiVersion, nil
}

func isGeminiRequest(endpointURL, model string) bool {
	modelLower := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(modelLower, "gemini") {
		return true
	}

	endpointLower := strings.ToLower(strings.TrimSpace(endpointURL))
	if endpointLower == "" {
		return false
	}
	if strings.Contains(endpointLower, "generativelanguage.googleapis.com") {
		return true
	}
	if strings.Contains(endpointLower, "/gemini") {
		return true
	}
	return false
}

func shouldUseAnthropicSDK(endpointURL, model string) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(model)), "claude") {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(endpointURL)), "anthropic")
}

func buildAnthropicBaseURL(endpoint string) (string, error) {
	trimmed := strings.TrimSpace(endpoint)
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

	path := strings.TrimRight(parsed.Path, "/")
	lowerPath := strings.ToLower(path)
	suffixes := []string{
		"/v1/chat/completions",
		"/chat/completions",
		"/v1/responses",
		"/responses",
		"/v1/messages",
		"/messages",
		"/v1",
	}
	for {
		matched := false
		for _, suffix := range suffixes {
			if strings.HasSuffix(lowerPath, suffix) {
				path = path[:len(path)-len(suffix)]
				lowerPath = strings.ToLower(path)
				matched = true
				break
			}
		}
		if !matched {
			break
		}
	}

	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	baseURL := strings.TrimRight(parsed.String(), "/")
	if baseURL == "" {
		return "", fmt.Errorf("invalid base_url")
	}
	return baseURL, nil
}

func requestAIByAnthropic(ctx context.Context, req aiChatCompletionRequest, onDelta func(string) error) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, req.EndpointURL, req.Model, req.SystemPrompt, req.UserPrompt)

	baseURL, err := buildAnthropicBaseURL(req.EndpointURL)
	if err != nil {
		return aiChatCompletionResult{}, err
	}

	client := anthropic.NewClient(
		anthropicoption.WithBaseURL(baseURL),
		anthropicoption.WithAPIKey(req.APIKey),
	)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(strings.TrimSpace(req.Model)),
		MaxTokens: aiAnthropicMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.UserPrompt)),
		},
	}
	if systemPrompt := strings.TrimSpace(req.SystemPrompt); systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: systemPrompt}}
	}

	if onDelta == nil {
		message, err := client.Messages.New(ctx, params)
		if err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("ai request failed: %w", err)
		}
		if message == nil {
			return aiChatCompletionResult{}, fmt.Errorf("ai response is empty")
		}
		content := strings.TrimSpace(extractAnthropicMessageContent(message.Content))
		if content == "" {
			return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
		}
		model := strings.TrimSpace(string(message.Model))
		if model == "" {
			model = strings.TrimSpace(req.Model)
		}
		return aiChatCompletionResult{Model: model, Content: content}, nil
	}

	stream := client.Messages.NewStreaming(ctx, params)
	var content strings.Builder
	for stream.Next() {
		event := stream.Current()

		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			deltaVariant, ok := eventVariant.Delta.AsAny().(anthropic.TextDelta)
			if !ok {
				continue
			}
			if deltaVariant.Text == "" {
				continue
			}
			content.WriteString(deltaVariant.Text)
			if err := onDelta(deltaVariant.Text); err != nil {
				return aiChatCompletionResult{}, fmt.Errorf("stream callback failed: %w", err)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("ai request failed: %w", err)
	}

	finalContent := strings.TrimSpace(content.String())
	if finalContent == "" {
		return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "claude"
	}
	return aiChatCompletionResult{Model: model, Content: finalContent}, nil
}

func extractAnthropicMessageContent(contentBlocks []anthropic.ContentBlockUnion) string {
	parts := make([]string, 0, len(contentBlocks))
	for _, block := range contentBlocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
