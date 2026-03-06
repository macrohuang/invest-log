package investlog

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeAIModelAndContent(body []byte) (string, string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", "", fmt.Errorf("decode ai response: %w", err)
	}

	model := asString(raw["model"])
	if model == "" {
		model = asString(raw["modelVersion"])
	}
	if outputText := asString(raw["output_text"]); outputText != "" {
		return model, outputText, nil
	}

	if text := extractChoicesContent(raw["choices"]); text != "" {
		return model, text, nil
	}
	if text := extractCandidatesContent(raw["candidates"]); text != "" {
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

func extractCandidatesContent(value any) string {
	candidates, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, candidate := range candidates {
		candidateMap, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if text := extractText(candidateMap["content"]); text != "" {
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
		if parts, ok := typed["parts"].([]any); ok {
			var builder strings.Builder
			for _, part := range parts {
				partMap, ok := part.(map[string]any)
				if ok {
					if text := asString(partMap["text"]); text != "" {
						builder.WriteString(text)
						continue
					}
				}
				if text := extractText(part); text != "" {
					builder.WriteString(text)
				}
			}
			if builder.Len() > 0 {
				return strings.TrimSpace(builder.String())
			}
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

