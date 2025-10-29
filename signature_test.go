package dsgo

import (
	"testing"
)

func TestSignature_AddFields(t *testing.T) {
	sig := NewSignature("Test signature")

	sig.AddInput("text", FieldTypeString, "Input text").
		AddOutput("result", FieldTypeString, "Output result").
		AddClassOutput("category", []string{"A", "B", "C"}, "Category")

	if len(sig.InputFields) != 1 {
		t.Errorf("Expected 1 input field, got %d", len(sig.InputFields))
	}

	if len(sig.OutputFields) != 2 {
		t.Errorf("Expected 2 output fields, got %d", len(sig.OutputFields))
	}

	if sig.OutputFields[1].Type != FieldTypeClass {
		t.Errorf("Expected class type, got %v", sig.OutputFields[1].Type)
	}
}

func TestSignature_ValidateInputs(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("required", FieldTypeString, "Required field")

	tests := []struct {
		name    string
		inputs  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid input",
			inputs:  map[string]interface{}{"required": "value"},
			wantErr: false,
		},
		{
			name:    "missing required field",
			inputs:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sig.ValidateInputs(tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignature_ValidateOutputs(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("required", FieldTypeString, "Required field").
		AddOptionalOutput("optional", FieldTypeString, "Optional field").
		AddClassOutput("category", []string{"A", "B", "C"}, "Category")

	tests := []struct {
		name    string
		outputs map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid outputs",
			outputs: map[string]interface{}{
				"required": "value",
				"category": "A",
			},
			wantErr: false,
		},
		{
			name: "valid with optional",
			outputs: map[string]interface{}{
				"required": "value",
				"optional": "opt",
				"category": "B",
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			outputs: map[string]interface{}{
				"category": "A",
			},
			wantErr: true,
		},
		{
			name: "invalid class value",
			outputs: map[string]interface{}{
				"required": "value",
				"category": "D",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sig.ValidateOutputs(tt.outputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignature_BuildPrompt(t *testing.T) {
	sig := NewSignature("Analyze sentiment").
		AddInput("text", FieldTypeString, "Input text").
		AddClassOutput("sentiment", []string{"positive", "negative"}, "Sentiment")

	inputs := map[string]interface{}{
		"text": "I love this!",
	}

	prompt, err := sig.BuildPrompt(inputs)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	// Check that prompt contains key elements
	if !contains(prompt, "Analyze sentiment") {
		t.Error("Prompt should contain description")
	}
	if !contains(prompt, "I love this!") {
		t.Error("Prompt should contain input value")
	}
	if !contains(prompt, "sentiment") {
		t.Error("Prompt should contain output field name")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
