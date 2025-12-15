package integration

import (
	"context"
	"reflect"
	"testing"

	"github.com/assagman/dsgo"
)

type tfIn struct {
	Text string `dsgo:"input,desc=Text"`
}

type tfOut struct {
	Answer string `dsgo:"output,desc=Answer"`
}

func TestIntegration_Func_RunWithPrediction_ReturnsPredictionOnConversionError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Ensure the underlying module succeeds (signature validation passes), but the typed
	// map->struct conversion fails.
	// We do this by using a JSON output field (always validates as map/slice/string),
	// then returning a nested value that can't unmarshal into the target struct.
	type out struct {
		Data struct {
			Count int `json:"count"`
		} `dsgo:"output,desc=Nested payload"`
	}

	lm := NewMockLMWithResponse(`{"data": {"count": "oops"}}`)
	fn, err := dsgo.NewTypedPredict[tfIn, out](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	_, pred, err := fn.RunWithPrediction(ctx, tfIn{Text: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if pred == nil {
		t.Fatal("expected prediction to be returned")
	}
	AssertPredictionError(t, err, "failed to convert output to struct")
}

func TestIntegration_Func_WithDemosTyped_LengthMismatch_ReturnsError(t *testing.T) {
	t.Parallel()
	lm := NewMockLMWithResponse("[[ ## Answer ## ]]\nok")
	fn, err := dsgo.NewTypedPredict[tfIn, tfOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	_, err = fn.WithDemosTyped([]tfIn{{Text: "a"}}, []tfOut{{Answer: "1"}, {Answer: "2"}})
	AssertPredictionError(t, err, "same length")
}

func TestIntegration_Func_NewPredictWithDescription_SetsSignatureDescription(t *testing.T) {
	t.Parallel()

	lm := NewMockLMWithResponse("[[ ## Answer ## ]]\nhello")
	fn, err := dsgo.NewTypedPredictWithDescription[tfIn, tfOut](lm, "custom description")
	if err != nil {
		t.Fatalf("NewTypedPredictWithDescription() error = %v", err)
	}

	if fn.GetSignature().Description != "custom description" {
		t.Fatalf("signature description = %q, want %q", fn.GetSignature().Description, "custom description")
	}
}

func TestIntegration_Func_Run_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse(`{"Answer": "ok"}`)
	fn, err := dsgo.NewTypedPredict[tfIn, tfOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	out, err := fn.Run(ctx, tfIn{Text: "hello"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out.Answer != "ok" {
		t.Fatalf("Run() answer = %q, want %q", out.Answer, "ok")
	}
}

func TestIntegration_Func_RunWithPrediction_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lm := NewMockLMWithResponse(`{"Answer": "ok"}`)
	fn, err := dsgo.NewTypedPredict[tfIn, tfOut](lm)
	if err != nil {
		t.Fatalf("NewTypedPredict() error = %v", err)
	}

	out, pred, err := fn.RunWithPrediction(ctx, tfIn{Text: "hello"})
	if err != nil {
		t.Fatalf("RunWithPrediction() error = %v", err)
	}
	if out.Answer != "ok" {
		t.Fatalf("RunWithPrediction() answer = %q, want %q", out.Answer, "ok")
	}
	if pred == nil {
		t.Fatal("expected prediction to be returned")
	}
	if got, ok := pred.GetString("Answer"); !ok || got != "ok" {
		t.Fatalf("prediction Answer = %q (ok=%v), want %q", got, ok, "ok")
	}
}

func TestIntegration_Typed_StructToSignature_BuildsExpectedFields(t *testing.T) {
	t.Parallel()

	type io struct {
		Question  string         `dsgo:"input,desc=Question"`
		Limit     *int           `dsgo:"input,optional,desc=Optional limit"`
		Sentiment string         `dsgo:"output,enum=positive|negative|neutral,desc=Sentiment"`
		Meta      map[string]any `dsgo:"output,desc=Metadata"`
	}

	sig, err := dsgo.StructToSignature(reflect.TypeOf(io{}), "desc")
	if err != nil {
		t.Fatalf("StructToSignature() error = %v", err)
	}

	if sig.Description != "desc" {
		t.Fatalf("Description = %q, want %q", sig.Description, "desc")
	}

	if len(sig.InputFields) != 2 {
		t.Fatalf("InputFields = %d, want %d", len(sig.InputFields), 2)
	}
	if len(sig.OutputFields) != 2 {
		t.Fatalf("OutputFields = %d, want %d", len(sig.OutputFields), 2)
	}

	// Spot-check optional and enum parsing.
	var foundLimit, foundSentiment bool
	for _, f := range sig.InputFields {
		if f.Name == "Limit" {
			foundLimit = true
			if !f.Optional {
				t.Fatalf("Limit.Optional = false, want true")
			}
		}
	}
	for _, f := range sig.OutputFields {
		if f.Name == "Sentiment" {
			foundSentiment = true
			if f.Type != dsgo.FieldTypeClass {
				t.Fatalf("Sentiment.Type = %v, want %v", f.Type, dsgo.FieldTypeClass)
			}
			if len(f.Classes) != 3 {
				t.Fatalf("Sentiment.Classes = %v, want 3 classes", f.Classes)
			}
		}
	}
	if !foundLimit {
		t.Fatal("expected to find Limit input field")
	}
	if !foundSentiment {
		t.Fatal("expected to find Sentiment output field")
	}
}

func BenchmarkIntegration_TypedPredict_Run(b *testing.B) {
	lm := NewMockLMWithResponse("[[ ## Answer ## ]]\nok")
	fn, err := dsgo.NewTypedPredict[tfIn, tfOut](lm)
	if err != nil {
		b.Fatalf("NewTypedPredict() error = %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out, err := fn.Run(ctx, tfIn{Text: "hello"})
		if err != nil {
			b.Fatalf("Run() error = %v", err)
		}
		if out.Answer != "ok" {
			b.Fatalf("Answer = %q, want %q", out.Answer, "ok")
		}
	}
}
