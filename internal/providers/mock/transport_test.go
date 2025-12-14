package mock

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/assagman/dsgo/internal/core"
)

func TestMockHTTP_Generate_WithScriptedTransport_NoBaseURL(t *testing.T) {
	st := NewScriptedTransport(HTTPResponseStep{Body: `{
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "hello"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
	}`})
	reset := SetHTTPTransport(st)
	defer reset()

	lm, err := core.NewLM(context.Background(), "mock/gpt-4o")
	if err != nil {
		t.Fatalf("NewLM: %v", err)
	}

	result, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "hi"}}, core.DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Content != "hello" {
		t.Fatalf("content=%q, want %q", result.Content, "hello")
	}

	reqs := st.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d, want 1", len(reqs))
	}
	if reqs[0].URL != "http://mock.local/chat/completions" {
		t.Fatalf("url=%q, want %q", reqs[0].URL, "http://mock.local/chat/completions")
	}
	if got := reqs[0].Header.Get("Authorization"); got != "Bearer test" {
		t.Fatalf("Authorization=%q, want %q", got, "Bearer test")
	}

	var payload map[string]any
	if err := json.Unmarshal(reqs[0].Body, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if got := payload["model"]; got != "gpt-4o" {
		t.Fatalf("payload.model=%v, want gpt-4o", got)
	}
}

func TestMockHTTP_Stream_WithScriptedTransport_NoBaseURL(t *testing.T) {
	st := NewScriptedTransport(HTTPResponseStep{Header: map[string][]string{"Content-Type": {"text/event-stream"}}, Body: "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":\"\"}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":\"\"}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n" +
		"data: [DONE]\n\n"})
	reset := SetHTTPTransport(st)
	defer reset()

	lm, err := core.NewLM(context.Background(), "mock/gpt-4o")
	if err != nil {
		t.Fatalf("NewLM: %v", err)
	}

	chunks, errs := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "stream"}}, core.DefaultGenerateOptions())

	var content string
	var last core.Chunk
	for ch := range chunks {
		content += ch.Content
		last = ch
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
	}

	if content != "Hello world" {
		t.Fatalf("content=%q, want %q", content, "Hello world")
	}
	if last.FinishReason != "stop" {
		t.Fatalf("finish=%q, want stop", last.FinishReason)
	}
	if last.Usage.TotalTokens != 3 {
		t.Fatalf("usage.total=%d, want 3", last.Usage.TotalTokens)
	}
}
