package integration

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/assagman/dsgo"
)

// Canonical typed structs (kept local to avoid brittleness / extra fixtures).

type tpStringIn struct {
	Question string `dsgo:"input,desc=Question"`
}

type tpStringOut struct {
	Answer string `dsgo:"output,desc=Answer"`
}

type tpOptionalOut struct {
	Answer  string  `dsgo:"output,desc=Answer"`
	Summary *string `dsgo:"output,optional,desc=Optional summary"`
}

type tpClassIn struct {
	Text string `dsgo:"input,desc=Text to classify"`
}

type tpClassOut struct {
	Sentiment string `dsgo:"output,enum=positive|negative|neutral,desc=Sentiment"`
}

type tpJSONIn struct {
	Query string `dsgo:"input,desc=Query"`
}

type tpNested struct {
	User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	} `json:"user"`
	Tags []string `json:"tags"`
}

type tpJSONOut struct {
	Data tpNested `dsgo:"output,desc=Nested JSON payload"`
}

type tpNilRequiredIn struct {
	Question *string `dsgo:"input,desc=Required question pointer"`
}

type tpBadNested struct {
	Count int `json:"count"`
}

type tpBadJSONOut struct {
	Data tpBadNested `dsgo:"output,desc=Nested JSON payload"`
}

func TestIntegration_TypedPredict_E2E_StringInStringOut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse(`{"Answer": "hello"}`)
	fn, err := dsgo.NewTypedPredict[tpStringIn, tpStringOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	out, pred, err := fn.RunWithPrediction(ctx, tpStringIn{Question: "hi"})
	if err != nil {
		t.Fatalf("RunWithPrediction() error = %v", err)
	}
	if out.Answer != "hello" {
		t.Fatalf("Answer = %q, want %q", out.Answer, "hello")
	}

	AssertPredictionValid(t, pred, []string{"Answer"})
	AssertAdapterUsed(t, pred, "adapter.JSONAdapter")
	AssertFallbackUsed(t, pred, false)
}

func TestIntegration_TypedPredict_E2E_OptionalOutput_OmittedInResponse_NilPointer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse("[[ ## Answer ## ]]\nprimary")
	fn, err := dsgo.NewTypedPredict[tpStringIn, tpOptionalOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	out, err := fn.Run(ctx, tpStringIn{Question: "hi"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out.Answer != "primary" {
		t.Fatalf("Answer = %q, want %q", out.Answer, "primary")
	}
	if out.Summary != nil {
		t.Fatalf("Summary != nil, want nil")
	}
}

func TestIntegration_TypedPredict_E2E_ClassOutput_ValidEnumValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse(`{"Sentiment": "positive"}`)
	fn, err := dsgo.NewTypedPredict[tpClassIn, tpClassOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	out, pred, err := fn.RunWithPrediction(ctx, tpClassIn{Text: "I love it"})
	if err != nil {
		t.Fatalf("RunWithPrediction() error = %v", err)
	}
	if out.Sentiment != "positive" {
		t.Fatalf("Sentiment = %q, want %q", out.Sentiment, "positive")
	}

	AssertAdapterUsed(t, pred, "adapter.JSONAdapter")
	AssertFallbackUsed(t, pred, false)
}

func TestIntegration_TypedPredict_E2E_JSONOutput_ParsesIntoNestedStruct(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Use JSON response - JSONAdapter is now first in chain, so it succeeds without fallback
	lm := NewMockLMWithResponse(`{"data": {"user": {"name": "Alice", "age": 30}, "tags": ["x", "y"]}}`)
	fn, err := dsgo.NewTypedPredict[tpJSONIn, tpJSONOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	out, pred, err := fn.RunWithPrediction(ctx, tpJSONIn{Query: "q"})
	if err != nil {
		t.Fatalf("RunWithPrediction() error = %v", err)
	}
	if out.Data.User.Name != "Alice" || out.Data.User.Age != 30 {
		t.Fatalf("Data.User = %#v, want name=Alice age=30", out.Data.User)
	}
	if !reflect.DeepEqual(out.Data.Tags, []string{"x", "y"}) {
		t.Fatalf("Data.Tags = %#v, want %#v", out.Data.Tags, []string{"x", "y"})
	}

	AssertAdapterUsed(t, pred, "adapter.JSONAdapter")
	AssertFallbackUsed(t, pred, false) // JSON is now first, no fallback needed
}

func TestIntegration_TypedPredict_Error_MissingRequiredInput_NilPointer_OmittedFromMap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// LM shouldn't matter; validation should fail first.
	lm := NewMockLMWithResponse("[[ ## Answer ## ]]\nignored")
	fn, err := dsgo.NewTypedPredict[tpNilRequiredIn, tpStringOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	_, err = fn.Run(ctx, tpNilRequiredIn{Question: nil})
	// Nil pointers are omitted from the input map, so this should show up as a
	// "missing required input" validation failure.
	AssertPredictionError(t, err, "missing required input")
}

func TestIntegration_TypedPredict_Error_MissingRequiredOutput_ParseFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse(`{"not_answer": "nope"}`)
	fn, err := dsgo.NewTypedPredict[tpStringIn, tpStringOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	_, err = fn.Run(ctx, tpStringIn{Question: "hi"})
	AssertPredictionError(t, err, "missing required output field")
}

func TestIntegration_TypedPredict_Error_WrongOutputType_MapToStructFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Signature validates JSON (map), but map->struct conversion should fail (count is string).
	lm := NewMockLMWithResponse(`{"data": {"count": "oops"}}`)
	fn, err := dsgo.NewTypedPredict[tpJSONIn, tpBadJSONOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	_, pred, err := fn.RunWithPrediction(ctx, tpJSONIn{Query: "q"})
	if err == nil {
		t.Fatal("expected error")
	}
	if pred == nil {
		t.Fatal("expected prediction to be returned")
	}
	AssertPredictionError(t, err, "failed to convert output to struct")
	AssertAdapterUsed(t, pred, "adapter.JSONAdapter")
}

func TestIntegration_TypedPredict_E2E_ClassOutput_InvalidEnumValue_ReturnsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse("[[ ## Sentiment ## ]]\nvery_happy")
	fn, err := dsgo.NewTypedPredict[tpClassIn, tpClassOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	_, err = fn.Run(ctx, tpClassIn{Text: "yay"})
	AssertPredictionError(t, err, "invalid class value")
}

func TestIntegration_TypedPredict_ConcurrencySmoke(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse(`{"Answer": "hello"}`)
	fn, err := dsgo.NewTypedPredict[tpStringIn, tpStringOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	const goroutines = 25
	const perG = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	errCh := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				out, err := fn.Run(ctx, tpStringIn{Question: "hi"})
				if err != nil {
					errCh <- err
					return
				}
				if out.Answer != "hello" {
					errCh <- fmt.Errorf("Answer = %q, want %q", out.Answer, "hello")
					return
				}

			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrency smoke failed: %v", err)
		}
	}
}
