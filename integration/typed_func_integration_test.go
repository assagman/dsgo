package integration

import (
	"context"
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
