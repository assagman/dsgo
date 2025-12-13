package mock

import (
	"context"
	"math"
	"testing"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/cost"
)

func TestMockHTTP_CostTracking_Generate_WithCollector(t *testing.T) {
	// This test verifies cost integration end-to-end:
	// mock LM -> core.NewLM (with collector) -> lmWrapper computes cost.
	collector := core.NewMemoryCollector(10)
	core.Configure(core.WithCollector(collector))
	t.Cleanup(core.ResetConfig)

	const (
		promptTokens     = 100
		completionTokens = 50
		totalTokens      = 150
	)

	st := NewScriptedTransport(HTTPResponseStep{Body: `{
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "hello"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}
	}`})
	resetTransport := SetHTTPTransport(st)
	t.Cleanup(resetTransport)

	lm, err := core.NewLM(context.Background(), "mock/gpt-4o-mini")
	if err != nil {
		t.Fatalf("NewLM: %v", err)
	}

	result, err := lm.Generate(
		context.Background(),
		[]core.Message{{Role: "user", Content: "hi"}},
		core.DefaultGenerateOptions(),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if result.Usage.PromptTokens != promptTokens {
		t.Fatalf("usage.prompt=%d, want %d", result.Usage.PromptTokens, promptTokens)
	}
	if result.Usage.CompletionTokens != completionTokens {
		t.Fatalf("usage.completion=%d, want %d", result.Usage.CompletionTokens, completionTokens)
	}
	if result.Usage.TotalTokens != totalTokens {
		t.Fatalf("usage.total=%d, want %d", result.Usage.TotalTokens, totalTokens)
	}

	expectedCost := cost.Calculate("gpt-4o-mini", promptTokens, completionTokens)
	if expectedCost <= 0 {
		t.Fatalf("expected positive pricing for gpt-4o-mini; got %.12f", expectedCost)
	}

	if result.Usage.Cost <= 0 {
		t.Fatalf("result.Usage.Cost=%.12f, want > 0", result.Usage.Cost)
	}
	if math.Abs(result.Usage.Cost-expectedCost) > 1e-12 {
		t.Fatalf("cost=%.12f, want %.12f", result.Usage.Cost, expectedCost)
	}
}

func TestMockHTTP_CostTracking_Stream_WithCollector(t *testing.T) {
	collector := core.NewMemoryCollector(10)
	core.Configure(core.WithCollector(collector))
	t.Cleanup(core.ResetConfig)

	const (
		promptTokens     = 10
		completionTokens = 5
		totalTokens      = 15
	)

	st := NewScriptedTransport(HTTPResponseStep{Header: map[string][]string{"Content-Type": {"text/event-stream"}}, Body: "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":\"\"}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":\"\"}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n" +
		"data: [DONE]\n\n"})
	resetTransport := SetHTTPTransport(st)
	t.Cleanup(resetTransport)

	lm, err := core.NewLM(context.Background(), "mock/gpt-4o-mini")
	if err != nil {
		t.Fatalf("NewLM: %v", err)
	}

	chunks, errs := lm.Stream(
		context.Background(),
		[]core.Message{{Role: "user", Content: "stream"}},
		core.DefaultGenerateOptions(),
	)

	for range chunks {
		// drain
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
	}

	entries := collector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("collector entries=%d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Usage.PromptTokens != promptTokens {
		t.Fatalf("entry.usage.prompt=%d, want %d", entry.Usage.PromptTokens, promptTokens)
	}
	if entry.Usage.CompletionTokens != completionTokens {
		t.Fatalf("entry.usage.completion=%d, want %d", entry.Usage.CompletionTokens, completionTokens)
	}
	if entry.Usage.TotalTokens != totalTokens {
		t.Fatalf("entry.usage.total=%d, want %d", entry.Usage.TotalTokens, totalTokens)
	}

	expectedCost := cost.Calculate("gpt-4o-mini", promptTokens, completionTokens)
	if expectedCost <= 0 {
		t.Fatalf("expected positive pricing for gpt-4o-mini; got %.12f", expectedCost)
	}
	if entry.Usage.Cost <= 0 {
		t.Fatalf("entry.Usage.Cost=%.12f, want > 0", entry.Usage.Cost)
	}
	if math.Abs(entry.Usage.Cost-expectedCost) > 1e-12 {
		t.Fatalf("entry cost=%.12f, want %.12f", entry.Usage.Cost, expectedCost)
	}
}
