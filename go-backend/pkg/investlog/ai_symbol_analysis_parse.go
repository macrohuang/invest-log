package investlog

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseSymbolDimensionResult(raw string) (*SymbolDimensionResult, error) {
	cleaned := cleanupModelJSON(raw)
	var result SymbolDimensionResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse dimension result: %w", err)
	}
	if strings.TrimSpace(result.Suggestion) == "" {
		var fallback struct {
			FrameworkID    string `json:"framework_id"`
			Suggestion     string `json:"suggestion"`
			Recommendation string `json:"recommendation"`
			Advice         string `json:"advice"`
		}
		if err := json.Unmarshal([]byte(cleaned), &fallback); err == nil {
			if strings.TrimSpace(result.Dimension) == "" {
				result.Dimension = strings.TrimSpace(fallback.FrameworkID)
			}
			result.Suggestion = firstNonEmptyString(fallback.Suggestion, fallback.Recommendation, fallback.Advice)
		}
	}
	return &result, nil
}

func parseSynthesisResult(raw string) (*SymbolSynthesisResult, error) {
	cleaned := cleanupModelJSON(raw)
	var result SymbolSynthesisResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse synthesis result: %w", err)
	}
	normalizeSynthesisResult(&result, nil)
	return &result, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
