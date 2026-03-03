package investlog

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
)

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
