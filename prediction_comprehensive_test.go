package dsgo

import "testing"

// TestPrediction_GetFloat tests float retrieval with all scenarios
func TestPrediction_GetFloat(t *testing.T) {
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

// TestPrediction_GetBool tests bool retrieval with all scenarios
func TestPrediction_GetBool(t *testing.T) {
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

// TestPrediction_AllMethods tests all prediction methods
func TestPrediction_AllMethods(t *testing.T) {
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

	// Test HasRationale
	if !p.HasRationale() {
		t.Error("HasRationale() should return true")
	}

	// Test HasCompletions
	if !p.HasCompletions() {
		t.Error("HasCompletions() should return true")
	}

	// Test all metadata
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

	// Test prediction without rationale/completions
	p2 := NewPrediction(outputs)
	if p2.HasRationale() {
		t.Error("Empty prediction should not have rationale")
	}
	if p2.HasCompletions() {
		t.Error("Empty prediction should not have completions")
	}
}
