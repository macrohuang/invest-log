package investlog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAIClientBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "empty uses default", input: "", want: "https://api.openai.com/v1"},
		{name: "base without v1", input: "https://example.com", want: "https://example.com/v1"},
		{name: "base with v1", input: "https://example.com/v1", want: "https://example.com/v1"},
		{name: "chat completions suffix", input: "https://example.com/v1/chat/completions", want: "https://example.com/v1"},
		{name: "chat completions suffix without v1", input: "https://example.com/chat/completions", want: "https://example.com"},
		{name: "responses suffix", input: "https://example.com/v1/responses", want: "https://example.com/v1"},
		{name: "missing scheme", input: "example.com/api", want: "https://example.com/api/v1"},
		{name: "invalid scheme", input: "ftp://example.com", wantErr: "invalid base_url scheme"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeAIClientBaseURL(tc.input)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error contains %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestRequestAIByChatCompletions_StreamingDelta(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"model-s\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"c1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"model-s\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	var streamed strings.Builder
	result, err := requestAIByChatCompletions(context.Background(), aiChatCompletionRequest{
		EndpointURL:  server.URL + "/v1/chat/completions",
		APIKey:       "key",
		Model:        "model-s",
		SystemPrompt: "sys",
		UserPrompt:   "user",
		OnDelta: func(delta string) {
			streamed.WriteString(delta)
		},
	}, server.URL+"/v1/chat/completions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Model != "model-s" {
		t.Fatalf("unexpected model: %s", result.Model)
	}
	if result.Content != "hello world" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if streamed.String() != "hello world" {
		t.Fatalf("unexpected streamed deltas: %q", streamed.String())
	}
}
