package investlog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
