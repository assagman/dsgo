package dsgo

import (
	"fmt"
	"testing"
)

// TestPrediction_WithParseDiagnostics tests adding parse diagnostics to prediction
func TestPrediction_WithParseDiagnostics(t *testing.T) {
	outputs := map[string]any{
		"answer": "Yes",
	}

	diag := &ValidationDiagnostics{
		MissingFields: []string{"confidence"},
		TypeErrors:    map[string]error{"score": fmt.Errorf("expected int, got string")},
		ClassErrors:   map[string]error{"category": fmt.Errorf("invalid class")},
	}

	pred := NewPrediction(outputs).WithParseDiagnostics(diag)

	if pred.ParseDiagnostics == nil {
		t.Fatal("Expected ParseDiagnostics to be set")
	}

	if len(pred.ParseDiagnostics.MissingFields) != 1 {
		t.Errorf("Expected 1 missing field, got %d", len(pred.ParseDiagnostics.MissingFields))
	}

	if len(pred.ParseDiagnostics.TypeErrors) != 1 {
		t.Errorf("Expected 1 type error, got %d", len(pred.ParseDiagnostics.TypeErrors))
	}

	if len(pred.ParseDiagnostics.ClassErrors) != 1 {
		t.Errorf("Expected 1 class error, got %d", len(pred.ParseDiagnostics.ClassErrors))
	}
}

// TestPrediction_WithParseDiagnostics_Nil tests adding nil diagnostics
func TestPrediction_WithParseDiagnostics_Nil(t *testing.T) {
	outputs := map[string]any{"answer": "Yes"}
	pred := NewPrediction(outputs).WithParseDiagnostics(nil)

	if pred.ParseDiagnostics != nil {
		t.Error("Expected ParseDiagnostics to be nil")
	}
}

// TestPrediction_WithParseDiagnostics_Empty tests adding empty diagnostics
func TestPrediction_WithParseDiagnostics_Empty(t *testing.T) {
	outputs := map[string]any{"answer": "Yes"}
	diag := &ValidationDiagnostics{
		MissingFields: []string{},
		TypeErrors:    map[string]error{},
		ClassErrors:   map[string]error{},
	}

	pred := NewPrediction(outputs).WithParseDiagnostics(diag)

	if pred.ParseDiagnostics == nil {
		t.Fatal("Expected ParseDiagnostics to be set")
	}

	if pred.ParseDiagnostics.HasErrors() {
		t.Error("Expected no errors in empty diagnostics")
	}
}

// TestPrediction_FullChaining tests full method chaining with diagnostics
func TestPrediction_FullChaining(t *testing.T) {
	outputs := map[string]any{"answer": "Yes"}
	inputs := map[string]any{"question": "Test?"}
	usage := Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	diag := &ValidationDiagnostics{
		MissingFields: []string{"optional_field"},
	}

	pred := NewPrediction(outputs).
		WithInputs(inputs).
		WithUsage(usage).
		WithModuleName("TestModule").
		WithRationale("Because...").
		WithScore(0.95).
		WithAdapterMetrics("ChatAdapter", 1, false).
		WithParseDiagnostics(diag)

	if pred.ModuleName != "TestModule" {
		t.Errorf("Expected ModuleName 'TestModule', got '%s'", pred.ModuleName)
	}

	if pred.Rationale != "Because..." {
		t.Errorf("Expected Rationale 'Because...', got '%s'", pred.Rationale)
	}

	if pred.Score != 0.95 {
		t.Errorf("Expected Score 0.95, got %f", pred.Score)
	}

	if pred.Usage.TotalTokens != 30 {
		t.Errorf("Expected TotalTokens 30, got %d", pred.Usage.TotalTokens)
	}

	if pred.AdapterUsed != "ChatAdapter" {
		t.Errorf("Expected AdapterUsed 'ChatAdapter', got '%s'", pred.AdapterUsed)
	}

	if pred.ParseDiagnostics == nil {
		t.Error("Expected ParseDiagnostics to be set")
	}

	if len(pred.ParseDiagnostics.MissingFields) != 1 {
		t.Errorf("Expected 1 missing field, got %d", len(pred.ParseDiagnostics.MissingFields))
	}
}
