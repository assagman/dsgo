package dsgo

import "testing"

func TestPrediction_Creation(t *testing.T) {
	outputs := map[string]interface{}{
		"answer": "42",
		"confidence": 0.95,
	}
	
	p := NewPrediction(outputs)
	
	if p.Outputs["answer"] != "42" {
		t.Error("Outputs not set correctly")
	}
}

func TestPrediction_WithMethods(t *testing.T) {
	p := NewPrediction(map[string]interface{}{"result": "test"}).
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
	p := NewPrediction(map[string]interface{}{
		"text": "hello",
		"number": 42.0,
		"flag": true,
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
}

func TestPrediction_WithCompletions(t *testing.T) {
	completions := []map[string]interface{}{
		{"answer": "option1"},
		{"answer": "option2"},
	}
	
	p := NewPrediction(map[string]interface{}{"answer": "best"}).
		WithCompletions(completions)
	
	if !p.HasCompletions() {
		t.Error("Prediction should have completions")
	}
	
	if len(p.Completions) != 2 {
		t.Errorf("Expected 2 completions, got %d", len(p.Completions))
	}
}
