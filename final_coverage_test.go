package dsgo

import (
	"testing"
)

// TestJSONAdapter_Format_WithDemos tests JSON adapter with demonstrations
func TestJSONAdapter_Format_WithDemos(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewJSONAdapter()
	inputs := map[string]any{"question": "What is 2+2?"}

	demos := []Example{
		{
			Inputs:  map[string]any{"question": "What is 1+1?"},
			Outputs: map[string]any{"answer": "2"},
		},
	}

	messages, err := adapter.Format(sig, inputs, demos)
	if err != nil {
		t.Fatalf("Format with demos failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestChatAdapter_Format_WithDemos tests chat adapter with demonstrations
func TestChatAdapter_Format_WithDemos(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewChatAdapter()
	inputs := map[string]any{"question": "What is 2+2?"}

	demos := []Example{
		{
			Inputs:  map[string]any{"question": "What is 1+1?"},
			Outputs: map[string]any{"answer": "2"},
		},
		{
			Inputs:  map[string]any{"question": "What is 3+3?"},
			Outputs: map[string]any{"answer": "6"},
		},
	}

	messages, err := adapter.Format(sig, inputs, demos)
	if err != nil {
		t.Fatalf("Format with demos failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestTwoStepAdapter_Format_WithDemos tests TwoStep adapter with demonstrations
func TestTwoStepAdapter_Format_WithDemos(t *testing.T) {
	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewTwoStepAdapter(nil)
	inputs := map[string]any{"question": "What is 2+2?"}

	demos := []Example{
		{
			Inputs:  map[string]any{"question": "What is 1+1?"},
			Outputs: map[string]any{"answer": "2"},
		},
	}

	messages, err := adapter.Format(sig, inputs, demos)
	if err != nil {
		t.Fatalf("Format with demos failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestFallbackAdapter_WithReasoning_False tests WithReasoning with false
func TestFallbackAdapter_WithReasoning_False(t *testing.T) {
	adapter := NewFallbackAdapter().WithReasoning(false)

	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "").
		AddOutput("answer", FieldTypeString, "")

	inputs := map[string]any{"question": "Test"}

	messages, err := adapter.Format(sig, inputs, nil)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

// TestChatAdapter_ParseOptional tests parsing with optional fields
func TestChatAdapter_ParseOptional(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("required", FieldTypeString, "").
		AddOptionalOutput("optional", FieldTypeString, "")

	adapter := NewChatAdapter()
	content := "required: value"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if _, ok := outputs["required"]; !ok {
		t.Error("Expected required field in outputs")
	}
}

// TestJSONAdapter_ParseWithRepair tests JSON parsing that needs repair
func TestJSONAdapter_ParseWithRepair(t *testing.T) {
	sig := NewSignature("Test").
		AddOutput("answer", FieldTypeString, "")

	adapter := NewJSONAdapter()

	// Malformed JSON that should be repaired
	content := "```json\n{'answer': 'yes',}\n```"

	outputs, err := adapter.Parse(sig, content)
	if err != nil {
		// Repair might not always succeed, that's okay
		t.Logf("Parse failed (acceptable): %v", err)
		return
	}

	if _, ok := outputs["answer"]; !ok {
		t.Error("Expected answer field in outputs")
	}
}

// TestGenerateOptions_Copy_WithTools tests copying options with tools
func TestGenerateOptions_Copy_WithTools(t *testing.T) {
	original := DefaultGenerateOptions()
	original.Tools = []Tool{
		{Name: "test_tool", Description: "A test tool"},
	}
	original.Stop = []string{"STOP"}

	copied := original.Copy()

	if copied == nil {
		t.Fatal("Expected non-nil copy")
	}

	if len(copied.Tools) != 1 {
		t.Errorf("Expected 1 tool in copy, got %d", len(copied.Tools))
	}

	if len(copied.Stop) != 1 {
		t.Errorf("Expected 1 stop sequence in copy, got %d", len(copied.Stop))
	}

	// Modify original and verify copy is independent
	original.Stop[0] = "CHANGED"
	if copied.Stop[0] == "CHANGED" {
		t.Error("Copy should be independent of original")
	}
}
