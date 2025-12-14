package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
	"github.com/assagman/dsgo/internal/providers/mock"
)

func newScriptedMockLMWithTransport(t *testing.T, steps ...mock.HTTPResponseStep) (dsgo.LM, *mock.ScriptedTransport) {
	t.Helper()

	transport := mock.NewScriptedTransport(steps...)
	reset := mock.SetHTTPTransport(transport)
	t.Cleanup(reset)

	lm, err := dsgo.NewLM(context.Background(), "mock/gpt-4o-mini")
	if err != nil {
		t.Fatalf("Failed to create mock LM: %v", err)
	}
	return lm, transport
}

func TestMockLM_Generate_Success(t *testing.T) {
	lm, _ := newScriptedMockLMWithTransport(t, mock.HTTPResponseStep{
		StatusCode: http.StatusOK,
		Body:       fixtures.OpenAIChatCompletionJSON("hello", "stop", 10, 5),
	})

	res, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "hi"}}, dsgo.DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if res.Content != "hello" {
		t.Fatalf("Content = %q, want %q", res.Content, "hello")
	}
	if res.FinishReason != "stop" {
		t.Fatalf("FinishReason = %q, want %q", res.FinishReason, "stop")
	}
	if res.Usage.PromptTokens != 10 || res.Usage.CompletionTokens != 5 || res.Usage.TotalTokens != 15 {
		t.Fatalf("Unexpected usage: %+v", res.Usage)
	}
	if res.Metadata == nil {
		t.Fatal("Expected Metadata to be non-nil")
	}
	if mockFlag, ok := res.Metadata["mock"].(bool); !ok || !mockFlag {
		t.Fatalf("Expected Metadata['mock']=true, got %v", res.Metadata["mock"])
	}
}

func TestMockLM_GenerateOptions_AreEncodedInRequest(t *testing.T) {
	lm, transport := newScriptedMockLMWithTransport(t, mock.HTTPResponseStep{
		StatusCode: http.StatusOK,
		Body:       fixtures.OpenAIChatCompletionJSON("ok", "stop", 1, 1),
	})

	opts := dsgo.DefaultGenerateOptions()
	opts.Temperature = 0.7
	opts.MaxTokens = 123
	opts.TopP = 0.9
	opts.Stop = []string{"END"}
	opts.ResponseFormat = "json"

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "hi"}}, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	reqs := transport.Requests()
	if len(reqs) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(reqs))
	}

	var payload map[string]any
	if err := json.Unmarshal(reqs[0].Body, &payload); err != nil {
		t.Fatalf("Failed to unmarshal request body: %v", err)
	}

	if payload["temperature"] != 0.7 {
		t.Fatalf("temperature = %v, want %v", payload["temperature"], 0.7)
	}
	// JSON numbers decode as float64.
	if payload["max_tokens"] != float64(123) {
		t.Fatalf("max_tokens = %v, want %v", payload["max_tokens"], 123)
	}
	if payload["top_p"] != 0.9 {
		t.Fatalf("top_p = %v, want %v", payload["top_p"], 0.9)
	}

	stop, ok := payload["stop"].([]any)
	if !ok || len(stop) != 1 || stop[0] != "END" {
		t.Fatalf("stop = %#v, want [END]", payload["stop"])
	}

	rf, ok := payload["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format missing or wrong type: %#v", payload["response_format"])
	}
	if rf["type"] != "json_object" {
		t.Fatalf("response_format.type = %v, want json_object", rf["type"])
	}
}

func TestMockLM_Generate_HTTPError(t *testing.T) {
	lm, _ := newScriptedMockLMWithTransport(t, mock.HTTPResponseStep{
		StatusCode: http.StatusInternalServerError,
		Body:       fixtures.OpenAIErrorBodyJSON("boom"),
	})

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "hi"}}, dsgo.DefaultGenerateOptions())
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestMockLM_Stream_Success(t *testing.T) {
	chunks := []string{"Hello", " ", "world"}
	lm, _ := newScriptedMockLMWithTransport(t, mock.HTTPResponseStep{
		StatusCode: http.StatusOK,
		Header:     fixtures.OpenAIStreamHeaders(),
		Body:       fixtures.OpenAIChatCompletionSSE(chunks, "stop", 10, 3),
	})

	chunkCh, errCh := lm.Stream(context.Background(), []dsgo.Message{{Role: "user", Content: "hi"}}, dsgo.DefaultGenerateOptions())

	var got string
	var finalUsage dsgo.Usage
	for c := range chunkCh {
		got += c.Content
		if c.Usage.TotalTokens != 0 {
			finalUsage = c.Usage
		}
	}

	for err := range errCh {
		if err != nil {
			t.Fatalf("Stream returned error: %v", err)
		}
	}

	if got != "Hello world" {
		t.Fatalf("Streamed content = %q, want %q", got, "Hello world")
	}
	if finalUsage.TotalTokens != 13 || finalUsage.PromptTokens != 10 || finalUsage.CompletionTokens != 3 {
		t.Fatalf("Unexpected usage: %+v", finalUsage)
	}
}

func TestMockLM_Stream_HTTPError(t *testing.T) {
	lm, _ := newScriptedMockLMWithTransport(t, mock.HTTPResponseStep{
		StatusCode: http.StatusBadRequest,
		Body:       fixtures.OpenAIErrorBodyJSON("bad"),
	})

	_, errCh := lm.Stream(context.Background(), []dsgo.Message{{Role: "user", Content: "hi"}}, dsgo.DefaultGenerateOptions())
	for err := range errCh {
		if err == nil {
			continue
		}
		return
	}
	// If channel closed without errors, fail.
	t.Fatal("Expected streaming error, got none")
}

func TestMockLM_Generate_RespectsContextTimeout(t *testing.T) {
	t.Setenv("DSGO_MOCK_API_KEY", "test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixtures.OpenAIChatCompletionJSON("ok", "stop", 1, 1)))
	}))
	defer server.Close()

	t.Setenv("DSGO_MOCK_BASE_URL", server.URL)

	lm, err := dsgo.NewLM(context.Background(), "mock/gpt-4o-mini")
	if err != nil {
		t.Fatalf("Failed to create mock LM: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, err = lm.Generate(ctx, []dsgo.Message{{Role: "user", Content: "hi"}}, dsgo.DefaultGenerateOptions())
	if err == nil {
		t.Fatal("Expected error due to context timeout")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Fatalf("Expected context-related error, got: %v", err)
	}
}

func TestMockLM_ConcurrentGenerate(t *testing.T) {
	const n = 10
	steps := make([]mock.HTTPResponseStep, 0, n)
	for range n {
		steps = append(steps, mock.HTTPResponseStep{StatusCode: http.StatusOK, Body: fixtures.OpenAIChatCompletionJSON("ok", "stop", 1, 1)})
	}

	lm, _ := newScriptedMockLMWithTransport(t, steps...)

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			res, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "hi"}}, dsgo.DefaultGenerateOptions())
			if err != nil {
				errCh <- err
				return
			}
			if res.Content != "ok" {
				errCh <- errors.New("unexpected content")
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("Concurrent Generate failed: %v", err)
		}
	}
}
