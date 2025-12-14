package module

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/assagman/dsgo/internal/core"
)

// withStructuredOutputs enables structured outputs for a single test and restores after.
func withStructuredOutputs(t *testing.T, enabled bool) func() {
	t.Helper()
	originalSettings := core.GetSettings()
	core.Configure(core.WithStructuredOutputEnabled(enabled))
	return func() {
		core.Configure(core.WithStructuredOutputEnabled(originalSettings.StructuredOutput.Enabled))
	}
}

func TestPredict_StructuredMode_ConvergesOnFirstAttempt(t *testing.T) {
	cleanup := withStructuredOutputs(t, true)
	defer cleanup()

	sig := core.NewSignature("Answer").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	calls := 0
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			calls++
			if calls != 1 {
				return nil, errors.New("unexpected call count")
			}
			if options.ResponseFormat != "json" {
				return nil, errors.New("expected ResponseFormat=json")
			}
			return &core.GenerateResult{Content: `{"answer": "42"}`}, nil
		},
	}

	p := NewPredict(sig, lm)
	result, err := p.Forward(context.Background(), map[string]any{"question": "2+2?"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if result.Outputs["answer"] != "42" {
		t.Errorf("expected answer=42, got %v", result.Outputs["answer"])
	}

	meta, ok := result.Outputs["__structured_meta"].(map[string]any)
	if !ok {
		t.Fatal("expected __structured_meta in outputs")
	}
	if meta["converged"] != true {
		t.Errorf("expected converged=true, got %v", meta["converged"])
	}
}

func TestPredict_StructuredMode_RetriesOnValidationFailure(t *testing.T) {
	cleanup := withStructuredOutputs(t, true)
	defer cleanup()

	sig := core.NewSignature("Answer").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	calls := 0
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			calls++
			switch calls {
			case 1:
				return &core.GenerateResult{Content: `{"wrong": "field"}`}, nil
			case 2:
				lastMsg := messages[len(messages)-1].Content
				if !strings.Contains(lastMsg, "Missing required fields") {
					return nil, errors.New("expected repair prompt with missing fields")
				}
				return &core.GenerateResult{Content: `{"answer": "fixed"}`}, nil
			default:
				return nil, errors.New("unexpected extra retries")
			}
		},
	}

	p := NewPredict(sig, lm)
	result, err := p.Forward(context.Background(), map[string]any{"question": "hi"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if result.Outputs["answer"] != "fixed" {
		t.Errorf("expected answer=fixed, got %v", result.Outputs["answer"])
	}

	meta, ok := result.Outputs["__structured_meta"].(map[string]any)
	if !ok {
		t.Fatal("expected __structured_meta in outputs")
	}
	if meta["attempts"].(int) < 2 {
		t.Errorf("expected at least 2 attempts, got %v", meta["attempts"])
	}
}

func TestChainOfThought_StructuredMode_ExtractsReasoning(t *testing.T) {
	cleanup := withStructuredOutputs(t, true)
	defer cleanup()

	sig := core.NewSignature("Reason").
		AddInput("problem", core.FieldTypeString, "").
		AddOutput("solution", core.FieldTypeString, "")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: `{"reasoning": "Step 1: think. Step 2: conclude.", "solution": "42"}`,
			}, nil
		},
	}

	cot := NewChainOfThought(sig, lm)
	result, err := cot.Forward(context.Background(), map[string]any{"problem": "solve"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if result.Outputs["solution"] != "42" {
		t.Errorf("expected solution=42, got %v", result.Outputs["solution"])
	}

	if result.Rationale == "" {
		t.Error("expected rationale to be extracted")
	}

	meta, ok := result.Outputs["__structured_meta"].(map[string]any)
	if !ok {
		t.Fatal("expected __structured_meta in outputs")
	}
	if meta["converged"] != true {
		t.Errorf("expected converged=true, got %v", meta["converged"])
	}
}

func TestPredict_StructuredMode_PropagatesBaseOptions(t *testing.T) {
	cleanup := withStructuredOutputs(t, true)
	defer cleanup()

	sig := core.NewSignature("Answer").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	var capturedOpts *core.GenerateOptions
	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			capturedOpts = options
			return &core.GenerateResult{Content: `{"answer": "42"}`}, nil
		},
	}

	p := NewPredict(sig, lm).
		WithOptions(&core.GenerateOptions{
			MaxTokens: 1000,
			TopP:      0.9,
			Stop:      []string{"END"},
		})

	_, err := p.Forward(context.Background(), map[string]any{"question": "hi"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if capturedOpts.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens=1000, got %d", capturedOpts.MaxTokens)
	}
	if capturedOpts.TopP != 0.9 {
		t.Errorf("expected TopP=0.9, got %v", capturedOpts.TopP)
	}
	if len(capturedOpts.Stop) != 1 || capturedOpts.Stop[0] != "END" {
		t.Errorf("expected Stop=[END], got %v", capturedOpts.Stop)
	}
}

func TestReAct_StructuredMode_PropagatesToolsForBedrock(t *testing.T) {
	cleanup := withStructuredOutputs(t, true)
	defer cleanup()

	sig := core.NewSignature("Search").
		AddInput("query", core.FieldTypeString, "").
		AddOutput("result", core.FieldTypeString, "")

	tool := core.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "found it", nil
	}).AddParameter("q", "string", "Query string", true)

	callNum := 0
	var extractOpts *core.GenerateOptions
	lm := &MockLM{
		SupportsJSONVal:  true,
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			callNum++
			if callNum == 1 {
				return &core.GenerateResult{
					ToolCalls: []core.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]any{"q": "test"}},
					},
				}, nil
			}
			if callNum == 2 {
				return &core.GenerateResult{
					Content: `{"result": "done"}`,
				}, nil
			}
			extractOpts = options
			return &core.GenerateResult{Content: `{"result": "final"}`}, nil
		},
	}

	react := NewReAct(sig, lm, []core.Tool{*tool}).WithMaxIterations(3)
	_, err := react.Forward(context.Background(), map[string]any{"query": "test"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if extractOpts != nil && extractOpts.Tools == nil {
		t.Error("expected Tools to be propagated for Bedrock compatibility")
	}
	if extractOpts != nil && extractOpts.ToolChoice != "none" {
		t.Errorf("expected ToolChoice=none, got %v", extractOpts.ToolChoice)
	}
}

func TestPredict_StructuredMode_LMErrorPropagates(t *testing.T) {
	cleanup := withStructuredOutputs(t, true)
	defer cleanup()

	sig := core.NewSignature("Answer").
		AddInput("question", core.FieldTypeString, "").
		AddOutput("answer", core.FieldTypeString, "")

	lm := &MockLM{
		SupportsJSONVal: true,
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return nil, errors.New("API error")
		},
	}

	p := NewPredict(sig, lm)
	_, err := p.Forward(context.Background(), map[string]any{"question": "hi"})
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("expected error to contain 'API error', got: %v", err)
	}
}
