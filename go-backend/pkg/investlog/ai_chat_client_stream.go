package investlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

type sseChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type geminiSSEChunk struct {
	ModelVersion string `json:"modelVersion"`
	Candidates   []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// parseSSEStream reads an SSE stream in OpenAI/Gemini-compatible formats and
// calls onChunk for each content delta. It returns the accumulated content and
// the last seen model identifier.
func parseSSEStream(body io.Reader, onChunk func(model, delta string) error) (string, string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var (
		builder strings.Builder
		model   string
	)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines, event lines, and comment lines.
		if line == "" || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") && !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimPrefix(data, "data:")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			break
		}

		chunkModel, delta, handled := extractOpenAIStyleSSEChunk(data)
		if !handled {
			chunkModel, delta, handled = extractGeminiStyleSSEChunk(data)
		}
		if !handled {
			slog.Default().Warn("ai sse: failed to parse chunk", "data", data)
			continue
		}
		if chunkModel != "" {
			model = chunkModel
		}
		if delta == "" {
			continue
		}

		builder.WriteString(delta)
		if err := onChunk(model, delta); err != nil {
			return builder.String(), model, fmt.Errorf("stream callback failed: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return builder.String(), model, fmt.Errorf("ai sse read error: %w", err)
	}

	return builder.String(), model, nil
}

func extractOpenAIStyleSSEChunk(data string) (string, string, bool) {
	var chunk sseChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", "", false
	}

	model := strings.TrimSpace(chunk.Model)
	if len(chunk.Choices) == 0 {
		return model, "", model != ""
	}
	return model, chunk.Choices[0].Delta.Content, true
}

func extractGeminiStyleSSEChunk(data string) (string, string, bool) {
	var chunk geminiSSEChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", "", false
	}

	model := strings.TrimSpace(chunk.ModelVersion)
	if len(chunk.Candidates) == 0 {
		return model, "", model != ""
	}

	var builder strings.Builder
	for _, candidate := range chunk.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text == "" {
				continue
			}
			builder.WriteString(part.Text)
		}
	}

	return model, builder.String(), true
}
