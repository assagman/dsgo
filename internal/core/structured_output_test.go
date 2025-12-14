package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGenerateStructured_ConvergesOnFirstAttempt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	calls := 0
	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		calls++
		if calls != 1 {
			return nil, errors.New("unexpected call count")
		}
		if options == nil {
			return nil, errors.New("expected non-nil options")
		}
		if options.ResponseFormat != "json" {
			return nil, errors.New("expected ResponseFormat=json")
		}
		if options.ResponseSchema == nil {
			return nil, errors.New("expected non-nil ResponseSchema")
		}
		return &GenerateResult{Content: `{"answer": "42"}`}, nil
	}

	res, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "2+2?"}, nil, GenerateStructuredOptions{
		Adapter:       NewJSONAdapter(),
		MaxAttempts:   3,
		Temperature:   0,
		UseJSONFormat: true,
	})
	if err != nil {
		t.Fatalf("GenerateStructured() error = %v", err)
	}
	if !res.Converged {
		t.Fatalf("expected Converged=true")
	}
	if res.Meta.Attempts != 1 {
		t.Fatalf("expected Attempts=1, got %d", res.Meta.Attempts)
	}
	if got := res.Outputs["answer"]; got != "42" {
		t.Fatalf("expected answer=42, got %v", got)
	}
}

func TestGenerateStructured_RetriesWithRepairPrompt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	calls := 0
	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		calls++
		switch calls {
		case 1:
			return &GenerateResult{Content: `{"foo": "bar"}`}, nil
		case 2:
			if len(messages) == 0 {
				return nil, errors.New("expected messages")
			}
			last := messages[len(messages)-1].Content
			if !strings.Contains(last, "Errors to fix") {
				return nil, errors.New("expected repair prompt to include errors")
			}
			if !strings.Contains(last, "Missing required fields") {
				return nil, errors.New("expected repair prompt to mention missing fields")
			}
			return &GenerateResult{Content: `{"answer": "ok"}`}, nil
		default:
			return nil, errors.New("unexpected extra retries")
		}
	}

	res, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "hi"}, nil, GenerateStructuredOptions{
		Adapter:       NewJSONAdapter(),
		MaxAttempts:   3,
		Temperature:   0,
		UseJSONFormat: true,
	})
	if err != nil {
		t.Fatalf("GenerateStructured() error = %v", err)
	}
	if !res.Converged {
		t.Fatalf("expected Converged=true")
	}
	if res.Meta.Attempts != 2 {
		t.Fatalf("expected Attempts=2, got %d", res.Meta.Attempts)
	}
	if got := res.Outputs["answer"]; got != "ok" {
		t.Fatalf("expected answer=ok, got %v", got)
	}
}

func TestGenerateStructured_FallsBackStrategyOnParseError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	calls := 0
	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		calls++
		switch calls {
		case 1:
			// First attempt will fail parsing.
			if options.ResponseSchema == nil {
				return nil, errors.New("expected schema on first attempt")
			}
			return &GenerateResult{Content: "not json"}, nil
		case 2:
			// After fallback to json_object, schema should be nil.
			if options.ResponseFormat != "json" {
				return nil, errors.New("expected ResponseFormat=json on second attempt")
			}
			if options.ResponseSchema != nil {
				return nil, errors.New("expected nil schema after fallback")
			}
			return &GenerateResult{Content: `{"answer": "42", "confidence": 0.9}`}, nil
		default:
			return nil, errors.New("unexpected extra retries")
		}
	}

	res, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "2+2?"}, nil, GenerateStructuredOptions{
		Adapter:       NewJSONAdapter(),
		MaxAttempts:   2,
		Temperature:   0,
		UseJSONFormat: true,
	})
	if err != nil {
		t.Fatalf("GenerateStructured() error = %v", err)
	}
	if !res.Converged {
		t.Fatalf("expected Converged=true")
	}
	if !res.Meta.StrategyFallback {
		t.Fatalf("expected StrategyFallback=true")
	}
	if res.Meta.Attempts != 2 {
		t.Fatalf("expected Attempts=2, got %d", res.Meta.Attempts)
	}
	if got := res.Outputs["answer"]; got != "42" {
		t.Fatalf("expected answer=42, got %v", got)
	}
	if got := res.Outputs["confidence"]; got != 0.9 {
		t.Fatalf("expected confidence=0.9, got %v", got)
	}
}

func TestGenerateStructured_ExhaustsAttemptsAddsMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "").
		AddOutput("confidence", FieldTypeFloat, "")

	calls := 0
	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		calls++
		switch calls {
		case 1:
			// Missing confidence
			return &GenerateResult{Content: `{"answer": "ok"}`}, nil
		case 2:
			// Missing answer (different error so we don't early-stop)
			return &GenerateResult{Content: `{"confidence": 0.5}`}, nil
		default:
			return nil, errors.New("unexpected extra retries")
		}
	}

	res, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "hi"}, nil, GenerateStructuredOptions{
		Adapter:       NewJSONAdapter(),
		MaxAttempts:   2,
		Temperature:   0,
		UseJSONFormat: true,
	})
	if err != nil {
		t.Fatalf("GenerateStructured() error = %v", err)
	}
	if res.Converged {
		t.Fatalf("expected Converged=false")
	}
	if res.Meta.Attempts != 2 {
		t.Fatalf("expected Attempts=2, got %d", res.Meta.Attempts)
	}

	metaAny, ok := res.Outputs["__structured_meta"]
	if !ok {
		t.Fatalf("expected __structured_meta in outputs")
	}
	meta, ok := metaAny.(map[string]any)
	if !ok {
		t.Fatalf("expected __structured_meta to be map, got %T", metaAny)
	}
	if meta["attempts"] != 2 {
		t.Fatalf("expected meta.attempts=2, got %v", meta["attempts"])
	}
	if meta["converged"] != false {
		t.Fatalf("expected meta.converged=false, got %v", meta["converged"])
	}
	if meta["strategy"] == "" {
		t.Fatalf("expected meta.strategy to be non-empty")
	}
}

func TestGenerateStructured_NilAdapterReturnsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	lm := NewMockLM()

	_, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "hi"}, nil, GenerateStructuredOptions{
		Adapter:       nil, // nil adapter
		MaxAttempts:   3,
		Temperature:   0,
		UseJSONFormat: true,
	})
	if err == nil {
		t.Fatal("expected error for nil adapter")
	}
	if !strings.Contains(err.Error(), "Adapter is required") {
		t.Fatalf("expected 'Adapter is required' error, got: %v", err)
	}
}

func TestGenerateStructured_NoParseSucceededReturnsError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	calls := 0
	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		calls++
		// Return content that will definitely fail parsing (no braces at all)
		return &GenerateResult{Content: "This is plain text with no JSON whatsoever and no braces"}, nil
	}

	// Use a strict adapter that won't try fallback parsing
	strictAdapter := &strictParseAdapter{}

	_, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "hi"}, nil, GenerateStructuredOptions{
		Adapter:       strictAdapter,
		MaxAttempts:   2,
		Temperature:   0,
		UseJSONFormat: true,
	})
	if err == nil {
		t.Fatal("expected error when no parse succeeded")
	}
	if !strings.Contains(err.Error(), "no valid JSON produced") {
		t.Fatalf("expected 'no valid JSON produced' error, got: %v", err)
	}
}

// strictParseAdapter is a test adapter that fails to parse non-JSON content
type strictParseAdapter struct{}

func (a *strictParseAdapter) Format(sig *Signature, inputs map[string]any, demos []Example) ([]Message, error) {
	return NewJSONAdapter().Format(sig, inputs, demos)
}

func (a *strictParseAdapter) Parse(sig *Signature, content string) (map[string]any, error) {
	// Only accept valid JSON
	start := strings.Index(content, "{")
	if start == -1 {
		return nil, errors.New("no JSON object found")
	}
	return NewJSONAdapter().Parse(sig, content)
}

func (a *strictParseAdapter) FormatHistory(history *History) []Message {
	return NewJSONAdapter().FormatHistory(history)
}

func TestGenerateStructured_ConvergedIncludesMeta(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		return &GenerateResult{Content: `{"answer": "42"}`}, nil
	}

	res, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "2+2?"}, nil, GenerateStructuredOptions{
		Adapter:       NewJSONAdapter(),
		MaxAttempts:   3,
		Temperature:   0,
		UseJSONFormat: true,
	})
	if err != nil {
		t.Fatalf("GenerateStructured() error = %v", err)
	}
	if !res.Converged {
		t.Fatalf("expected Converged=true")
	}

	// __structured_meta should be present even on successful convergence
	metaAny, ok := res.Outputs["__structured_meta"]
	if !ok {
		t.Fatalf("expected __structured_meta in outputs even on successful convergence")
	}
	meta, ok := metaAny.(map[string]any)
	if !ok {
		t.Fatalf("expected __structured_meta to be map, got %T", metaAny)
	}
	if meta["converged"] != true {
		t.Fatalf("expected meta.converged=true, got %v", meta["converged"])
	}
	if meta["attempts"] != 1 {
		t.Fatalf("expected meta.attempts=1, got %v", meta["attempts"])
	}
}

func TestGenerateStructured_BaseOptionsPropagated(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	var capturedOpts *GenerateOptions
	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		capturedOpts = options
		return &GenerateResult{Content: `{"answer": "42"}`}, nil
	}

	baseOpts := &GenerateOptions{
		MaxTokens:        1000,
		TopP:             0.9,
		Stop:             []string{"END"},
		FrequencyPenalty: 0.5,
		PresencePenalty:  0.3,
	}

	res, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "hi"}, nil, GenerateStructuredOptions{
		Adapter:       NewJSONAdapter(),
		BaseOptions:   baseOpts,
		MaxAttempts:   1,
		Temperature:   0.2,
		UseJSONFormat: true,
	})
	if err != nil {
		t.Fatalf("GenerateStructured() error = %v", err)
	}
	if !res.Converged {
		t.Fatalf("expected Converged=true")
	}

	// Verify base options were propagated
	if capturedOpts.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens=1000, got %d", capturedOpts.MaxTokens)
	}
	if capturedOpts.TopP != 0.9 {
		t.Errorf("expected TopP=0.9, got %v", capturedOpts.TopP)
	}
	if len(capturedOpts.Stop) != 1 || capturedOpts.Stop[0] != "END" {
		t.Errorf("expected Stop=[END], got %v", capturedOpts.Stop)
	}
	if capturedOpts.FrequencyPenalty != 0.5 {
		t.Errorf("expected FrequencyPenalty=0.5, got %v", capturedOpts.FrequencyPenalty)
	}
	if capturedOpts.PresencePenalty != 0.3 {
		t.Errorf("expected PresencePenalty=0.3, got %v", capturedOpts.PresencePenalty)
	}
	// Temperature should be overridden by structured options (use approximate comparison for float32->float64)
	if capturedOpts.Temperature < 0.19 || capturedOpts.Temperature > 0.21 {
		t.Errorf("expected Temperature≈0.2 (overridden), got %v", capturedOpts.Temperature)
	}
}

func TestGenerateStructured_UseJSONFormatFalseSkipsJSONStrategies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sig := NewSignature("Answer").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	var capturedOpts *GenerateOptions
	lm := NewMockLM()
	lm.GenerateFunc = func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
		capturedOpts = options
		return &GenerateResult{Content: `{"answer": "42"}`}, nil
	}

	res, err := GenerateStructured(ctx, lm, sig, map[string]any{"question": "hi"}, nil, GenerateStructuredOptions{
		Adapter:       NewJSONAdapter(),
		MaxAttempts:   1,
		Temperature:   0,
		UseJSONFormat: false, // Explicitly disable JSON format
	})
	if err != nil {
		t.Fatalf("GenerateStructured() error = %v", err)
	}
	if !res.Converged {
		t.Fatalf("expected Converged=true")
	}

	// With UseJSONFormat=false, should use plain strategy (no ResponseFormat)
	if capturedOpts.ResponseFormat != "" {
		t.Errorf("expected empty ResponseFormat with UseJSONFormat=false, got %q", capturedOpts.ResponseFormat)
	}
	if capturedOpts.ResponseSchema != nil {
		t.Errorf("expected nil ResponseSchema with UseJSONFormat=false")
	}
}
