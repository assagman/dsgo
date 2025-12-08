package integration

import (
	"testing"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
)

func TestBuilder_DefaultConfiguration(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().Build(t)
	defer tc.Cleanup()

	result := tc.MustForward(map[string]any{"question": "test"})
	answer, ok := result.GetString("answer")
	if !ok {
		t.Fatal("expected answer field")
	}
	if answer != "test response" {
		t.Errorf("expected 'test response', got %q", answer)
	}
}

func TestBuilder_CustomSignature(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().
		WithSignature(fixtures.ClassificationSig()).
		WithMockResponse(`{"sentiment": "positive"}`).
		Build(t)
	defer tc.Cleanup()

	result := tc.MustForward(map[string]any{"text": "I love this!"})
	sentiment, ok := result.GetString("sentiment")
	if !ok {
		t.Fatal("expected sentiment field")
	}
	if sentiment != "positive" {
		t.Errorf("expected 'positive', got %q", sentiment)
	}
}

func TestBuilder_MultipleResponses(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().
		WithMockResponses([]string{
			`{"answer": "first"}`,
			`{"answer": "second"}`,
		}).
		Build(t)
	defer tc.Cleanup()

	// First call
	r1 := tc.MustForward(map[string]any{"question": "q1"})
	a1, _ := r1.GetString("answer")
	if a1 != "first" {
		t.Errorf("expected 'first', got %q", a1)
	}

	// Second call
	r2 := tc.MustForward(map[string]any{"question": "q2"})
	a2, _ := r2.GetString("answer")
	if a2 != "second" {
		t.Errorf("expected 'second', got %q", a2)
	}
}

func TestBuilder_WithError(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().
		WithError("test error").
		Build(t)
	defer tc.Cleanup()

	tc.AssertError(map[string]any{"question": "q"}, "test error")
}

func TestBuilder_WithTimeout(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().
		WithTimeout(100 * time.Millisecond).
		Build(t)
	defer tc.Cleanup()

	// Should work within timeout
	_ = tc.MustForward(map[string]any{"question": "q"})
}

func TestBuilder_WithAdapter(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().
		WithAdapter(dsgo.NewJSONAdapter()).
		WithMockResponse(`{"answer": "json response"}`).
		Build(t)
	defer tc.Cleanup()

	result := tc.MustForward(map[string]any{"question": "q"})
	answer, _ := result.GetString("answer")
	if answer != "json response" {
		t.Errorf("expected 'json response', got %q", answer)
	}
}

func TestBuilder_ChainOfThought(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().
		WithSignature(fixtures.ChainOfThoughtSig()).
		WithModuleType("cot").
		WithMockResponse(`{"reasoning": "step by step", "answer": "42"}`).
		Build(t)
	defer tc.Cleanup()

	result := tc.MustForward(map[string]any{"problem": "What is 6*7?"})
	answer, ok := result.GetString("answer")
	if !ok || answer != "42" {
		t.Errorf("expected answer '42', got %q", answer)
	}
}

func TestBuilder_Refine(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().
		WithSignature(fixtures.RefineSig()).
		WithModuleType("refine").
		WithMockResponse(`{"output": "generated content"}`).
		Build(t)
	defer tc.Cleanup()

	// Without feedback, Refine returns initial generation
	result := tc.MustForward(map[string]any{"topic": "test"})
	output, ok := result.GetString("output")
	if !ok {
		t.Fatal("expected output field")
	}
	if output != "generated content" {
		t.Errorf("expected 'generated content', got %q", output)
	}
}

func TestBuilder_AssertSuccess(t *testing.T) {
	t.Parallel()
	tc := NewTestBuilder().Build(t)
	defer tc.Cleanup()

	result := tc.AssertSuccess(
		map[string]any{"question": "q"},
		[]string{"answer"},
	)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
