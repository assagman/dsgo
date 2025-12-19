package mock

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/assagman/dsgo/internal/core"
)

func TestMockHTTP_Generate(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path=%s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer testkey" {
			t.Errorf("Authorization=%q, want %q", got, "Bearer testkey")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type=%q, want application/json", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := body["model"]; got != "gpt-4o" {
			t.Fatalf("model=%v, want gpt-4o", got)
		}
		msgs, ok := body["messages"].([]any)
		if !ok || len(msgs) == 0 {
			t.Fatalf("messages=%T/%v, want non-empty array", body["messages"], body["messages"])
		}
		if got := body["max_tokens"]; got != float64(123) {
			t.Fatalf("max_tokens=%v, want 123", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
		}`))
	}))
	defer server.Close()

	t.Setenv("DSGO_MOCK_BASE_URL", server.URL)
	t.Setenv("DSGO_MOCK_API_KEY", "testkey")

	lm, err := core.NewLM(context.Background(), "mock/gpt-4o")
	if err != nil {
		t.Fatalf("NewLM: %v", err)
	}

	opts := core.DefaultGenerateOptions()
	opts.MaxTokens = 123

	result, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "hi"}}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Content != "hello" {
		t.Fatalf("content=%q, want %q", result.Content, "hello")
	}
	if result.Usage.TotalTokens != 5 {
		t.Fatalf("totalTokens=%d, want 5", result.Usage.TotalTokens)
	}
}

func TestMockHTTP_Stream(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Two content chunks + one usage chunk.
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":\"\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":\"\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	t.Setenv("DSGO_MOCK_BASE_URL", server.URL)
	t.Setenv("DSGO_MOCK_API_KEY", "testkey")

	lm, err := core.NewLM(context.Background(), "mock/gpt-4o")
	if err != nil {
		t.Fatalf("NewLM: %v", err)
	}

	chunks, errs := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "stream"}}, core.DefaultGenerateOptions())

	var got strings.Builder
	var last core.Chunk
	for ch := range chunks {
		got.WriteString(ch.Content)
		last = ch
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
	}

	if got.String() != "Hello world" {
		t.Fatalf("content=%q, want %q", got.String(), "Hello world")
	}
	if last.FinishReason != "stop" {
		t.Fatalf("finish=%q, want stop", last.FinishReason)
	}
	if last.Usage.TotalTokens != 3 {
		t.Fatalf("usage.total=%d, want 3", last.Usage.TotalTokens)
	}
}

func TestMockHTTP_buildRequest_ToolChoiceRequired(t *testing.T) {
	m := &mockHTTP{model: "gpt-4o"}

	tool := core.NewTool("test_tool", "A test tool", nil)
	tool.AddParameter("q", "string", "Query", true)

	req := m.buildRequest([]core.Message{{Role: "user", Content: "hi"}}, &core.GenerateOptions{
		Tools:      []core.Tool{*tool},
		ToolChoice: "required",
	})

	if req["tool_choice"] != "required" {
		t.Fatalf("tool_choice=%v, want required", req["tool_choice"])
	}
}

func TestMockHTTP_convertTool_AdditionalPropertiesFalse(t *testing.T) {
	tool := core.NewTool("test_tool", "A test tool", nil)
	tool.AddParameter("q", "string", "Query", true)

	converted := convertTool(tool)
	fn, _ := converted["function"].(map[string]any)
	params, _ := fn["parameters"].(map[string]any)
	if params["additionalProperties"] != false {
		t.Fatalf("additionalProperties=%v, want false", params["additionalProperties"])
	}
}
