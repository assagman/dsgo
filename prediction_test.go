package dsgo

import (
	"fmt"
	"testing"
)

func TestPrediction_Creation(t *testing.T) {
	outputs := map[string]any{
		"answer":     "42",
		"confidence": 0.95,
	}

	p := NewPrediction(outputs)

	if p.Outputs["answer"] != "42" {
		t.Error("Outputs not set correctly")
	}
}

func TestPrediction_WithMethods(t *testing.T) {
	p := NewPrediction(map[string]any{"result": "test"}).
		WithRationale("thinking step by step").
		WithScore(0.85).
		WithModuleName("TestModule")

	if !p.HasRationale() {
		t.Error("Prediction should have rationale")
	}

	if p.Score != 0.85 {
		t.Errorf("Expected score 0.85, got %f", p.Score)
	}

	if p.ModuleName != "TestModule" {
		t.Error("Module name not set")
	}
}

func TestPrediction_GetMethods(t *testing.T) {
	p := NewPrediction(map[string]any{
		"text":   "hello",
		"number": 42.0,
		"flag":   true,
	})

	if str, ok := p.GetString("text"); !ok || str != "hello" {
		t.Error("GetString failed")
	}

	if num, ok := p.GetFloat("number"); !ok || num != 42.0 {
		t.Error("GetFloat failed")
	}

	if flag, ok := p.GetBool("flag"); !ok || !flag {
		t.Error("GetBool failed")
	}

	if _, ok := p.GetString("nonexistent"); ok {
		t.Error("GetString should return false for missing key")
	}

	// Test GetFloat with wrong type
	if _, ok := p.GetFloat("text"); ok {
		t.Error("GetFloat should return false for non-float value")
	}

	// Test GetBool with wrong type
	if _, ok := p.GetBool("text"); ok {
		t.Error("GetBool should return false for non-bool value")
	}
}

func TestPrediction_WithCompletions(t *testing.T) {
	completions := []map[string]any{
		{"answer": "option1"},
		{"answer": "option2"},
	}

	p := NewPrediction(map[string]any{"answer": "best"}).
		WithCompletions(completions)

	if !p.HasCompletions() {
		t.Error("Prediction should have completions")
	}

	if len(p.Completions) != 2 {
		t.Errorf("Expected 2 completions, got %d", len(p.Completions))
	}
}

func TestPrediction_GetInt(t *testing.T) {
	tests := []struct {
		name   string
		value  interface{}
		want   int
		wantOk bool
	}{
		{"int value", 42, 42, true},
		{"float64 value", 42.0, 42, true},
		{"string value", "not an int", 0, false},
		{"missing key", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPrediction(map[string]any{})
			if tt.value != nil {
				p.Outputs["test"] = tt.value
			}

			got, ok := p.GetInt("test")
			if ok != tt.wantOk {
				t.Errorf("GetInt() ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("GetInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrediction_Get(t *testing.T) {
	p := NewPrediction(map[string]any{"key": "value"})

	val, ok := p.Get("key")
	if !ok || val != "value" {
		t.Error("Get should return existing value")
	}

	_, ok = p.Get("nonexistent")
	if ok {
		t.Error("Get should return false for missing key")
	}
}

func TestPrediction_WithUsage(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	p := NewPrediction(map[string]any{}).WithUsage(usage)

	if p.Usage.TotalTokens != 150 {
		t.Errorf("Expected total tokens 150, got %d", p.Usage.TotalTokens)
	}
}

func TestPrediction_WithInputs(t *testing.T) {
	inputs := map[string]any{"question": "What is AI?"}
	p := NewPrediction(map[string]any{}).WithInputs(inputs)

	if p.Inputs["question"] != "What is AI?" {
		t.Error("WithInputs should store inputs")
	}
}

func TestPrediction_HasRationale(t *testing.T) {
	p1 := NewPrediction(map[string]any{})
	if p1.HasRationale() {
		t.Error("New prediction should not have rationale")
	}

	p2 := NewPrediction(map[string]any{}).WithRationale("thinking...")
	if !p2.HasRationale() {
		t.Error("Prediction with rationale should return true")
	}
}

func TestPrediction_GetFloat_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		outputs   map[string]any
		key       string
		wantValue float64
		wantOk    bool
	}{
		{
			name:      "valid float64",
			outputs:   map[string]any{"score": float64(3.14)},
			key:       "score",
			wantValue: 3.14,
			wantOk:    true,
		},
		{
			name:      "missing key",
			outputs:   map[string]any{},
			key:       "score",
			wantValue: 0,
			wantOk:    false,
		},
		{
			name:      "wrong type",
			outputs:   map[string]any{"score": "not a float"},
			key:       "score",
			wantValue: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPrediction(tt.outputs)
			value, ok := p.GetFloat(tt.key)
			if ok != tt.wantOk {
				t.Errorf("GetFloat() ok = %v, want %v", ok, tt.wantOk)
			}
			if value != tt.wantValue {
				t.Errorf("GetFloat() value = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestPrediction_GetBool_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		outputs   map[string]any
		key       string
		wantValue bool
		wantOk    bool
	}{
		{
			name:      "valid true",
			outputs:   map[string]any{"flag": true},
			key:       "flag",
			wantValue: true,
			wantOk:    true,
		},
		{
			name:      "valid false",
			outputs:   map[string]any{"flag": false},
			key:       "flag",
			wantValue: false,
			wantOk:    true,
		},
		{
			name:      "missing key",
			outputs:   map[string]any{},
			key:       "flag",
			wantValue: false,
			wantOk:    false,
		},
		{
			name:      "wrong type",
			outputs:   map[string]any{"flag": "true"},
			key:       "flag",
			wantValue: false,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPrediction(tt.outputs)
			value, ok := p.GetBool(tt.key)
			if ok != tt.wantOk {
				t.Errorf("GetBool() ok = %v, want %v", ok, tt.wantOk)
			}
			if value != tt.wantValue {
				t.Errorf("GetBool() value = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestPrediction_AllMethodsWithMetadata(t *testing.T) {
	outputs := map[string]any{
		"answer":  "test",
		"score":   0.95,
		"count":   42,
		"enabled": true,
	}

	p := NewPrediction(outputs).
		WithRationale("reasoning here").
		WithScore(0.95).
		WithCompletions([]map[string]any{{"alt": "alternative"}}).
		WithUsage(Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}).
		WithModuleName("TestModule").
		WithInputs(map[string]any{"question": "test"})

	if !p.HasRationale() {
		t.Error("HasRationale() should return true")
	}

	if !p.HasCompletions() {
		t.Error("HasCompletions() should return true")
	}

	if p.Rationale != "reasoning here" {
		t.Error("Rationale not set correctly")
	}
	if p.Score != 0.95 {
		t.Error("Score not set correctly")
	}
	if len(p.Completions) != 1 {
		t.Error("Completions not set correctly")
	}
	if p.Usage.TotalTokens != 30 {
		t.Error("Usage not set correctly")
	}
	if p.ModuleName != "TestModule" {
		t.Error("ModuleName not set correctly")
	}
	if p.Inputs["question"] != "test" {
		t.Error("Inputs not set correctly")
	}

	p2 := NewPrediction(outputs)
	if p2.HasRationale() {
		t.Error("Empty prediction should not have rationale")
	}
	if p2.HasCompletions() {
		t.Error("Empty prediction should not have completions")
	}
}

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

func TestPrediction_WithParseDiagnostics_Nil(t *testing.T) {
	outputs := map[string]any{"answer": "Yes"}
	pred := NewPrediction(outputs).WithParseDiagnostics(nil)

	if pred.ParseDiagnostics != nil {
		t.Error("Expected ParseDiagnostics to be nil")
	}
}

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

// TestExtractAdapterMetadata tests the ExtractAdapterMetadata function
func TestExtractAdapterMetadata(t *testing.T) {
	tests := []struct {
		name         string
		outputs      map[string]any
		wantAdapter  string
		wantAttempts int
		wantFallback bool
	}{
		{
			name: "all metadata present",
			outputs: map[string]any{
				"__adapter_used":   "JSONAdapter",
				"__parse_attempts": 2,
				"__fallback_used":  true,
				"answer":           "test",
			},
			wantAdapter:  "JSONAdapter",
			wantAttempts: 2,
			wantFallback: true,
		},
		{
			name:         "no metadata",
			outputs:      map[string]any{"answer": "test"},
			wantAdapter:  "",
			wantAttempts: 0,
			wantFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputsCopy := make(map[string]any)
			for k, v := range tt.outputs {
				outputsCopy[k] = v
			}

			adapter, attempts, fallback := ExtractAdapterMetadata(outputsCopy)

			if adapter != tt.wantAdapter {
				t.Errorf("adapter = %q, want %q", adapter, tt.wantAdapter)
			}
			if attempts != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", attempts, tt.wantAttempts)
			}
			if fallback != tt.wantFallback {
				t.Errorf("fallback = %v, want %v", fallback, tt.wantFallback)
			}

			// Verify metadata was removed
			for _, key := range []string{"__adapter_used", "__parse_attempts", "__fallback_used"} {
				if _, exists := outputsCopy[key]; exists {
					t.Errorf("metadata key %q should have been removed", key)
				}
			}
		})
	}
}
