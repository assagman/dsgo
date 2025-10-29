package dsgo

import "testing"

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
