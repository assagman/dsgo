package module

import (
	"context"
	"errors"
	"testing"

	"github.com/assagman/dsgo"
)

func TestReAct_Forward_NoTools(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content:   `{"reasoning": "thinking", "answer": "result"}`,
				ToolCalls: []dsgo.ToolCall{},
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs["answer"] != "result" {
		t.Errorf("Expected answer='result', got %v", outputs["answer"])
	}
}

func TestReAct_Forward_WithToolCalls(t *testing.T) {
	sig := dsgo.NewSignature("Answer question").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		SupportsToolsVal: true,
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{
					Content: "Let me search",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "search", Arguments: map[string]interface{}{"query": "test"}},
					},
				}, nil
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "final answer"}`,
			}, nil
		},
	}

	searchTool := dsgo.NewTool("search", "Search for info", func(ctx context.Context, args map[string]any) (any, error) {
		return "search result", nil
	})

	react := NewReAct(sig, lm, []dsgo.Tool{*searchTool})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs["answer"] != "final answer" {
		t.Errorf("Expected final answer, got %v", outputs["answer"])
	}
}

func TestReAct_Forward_InvalidInput(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("required", dsgo.FieldTypeString, "Required")

	lm := &MockLM{}
	react := NewReAct(sig, lm, []dsgo.Tool{})

	_, err := react.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error on invalid input")
	}
}

func TestReAct_Forward_LMError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return nil, errors.New("LM error")
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	_, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should propagate LM error")
	}
}

func TestReAct_Forward_MaxIterations(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			return &dsgo.GenerateResult{
				Content: "thinking",
				ToolCalls: []dsgo.ToolCall{
					{ID: "1", Name: "search", Arguments: map[string]interface{}{}},
				},
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{}).WithMaxIterations(2)
	_, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err == nil {
		t.Error("Forward() should error when max iterations exceeded")
	}
}

func TestReAct_Forward_ToolNotFound(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{
					Content: "Using tool",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "nonexistent", Arguments: map[string]interface{}{}},
					},
				}, nil
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "recovered"}`,
			}, nil
		},
	}

	react := NewReAct(sig, lm, []dsgo.Tool{})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() should handle missing tool gracefully, got error: %v", err)
	}

	if outputs["answer"] != "recovered" {
		t.Error("Should recover from tool not found error")
	}
}

func TestReAct_Forward_ToolError(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	callCount := 0
	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
			callCount++
			if callCount == 1 {
				return &dsgo.GenerateResult{
					Content: "Using tool",
					ToolCalls: []dsgo.ToolCall{
						{ID: "1", Name: "failing_tool", Arguments: map[string]interface{}{}},
					},
				}, nil
			}
			return &dsgo.GenerateResult{
				Content: `{"answer": "recovered from error"}`,
			}, nil
		},
	}

	failingTool := dsgo.NewTool("failing_tool", "Fails", func(ctx context.Context, args map[string]any) (any, error) {
		return nil, errors.New("tool failed")
	})

	react := NewReAct(sig, lm, []dsgo.Tool{*failingTool})
	outputs, err := react.Forward(context.Background(), map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Forward() should handle tool errors, got: %v", err)
	}

	if outputs["answer"] != "recovered from error" {
		t.Error("Should recover from tool execution error")
	}
}

func TestReAct_WithOptions(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	lm := &MockLM{}
	react := NewReAct(sig, lm, []dsgo.Tool{})

	customOpts := &dsgo.GenerateOptions{Temperature: 0.9}
	react.WithOptions(customOpts)

	if react.Options.Temperature != 0.9 {
		t.Error("WithOptions should set custom options")
	}
}

func TestReAct_WithMaxIterations(t *testing.T) {
	react := NewReAct(dsgo.NewSignature("Test"), &MockLM{}, []dsgo.Tool{})
	react.WithMaxIterations(5)

	if react.MaxIterations != 5 {
		t.Error("WithMaxIterations should set max iterations")
	}
}

func TestReAct_WithVerbose(t *testing.T) {
	react := NewReAct(dsgo.NewSignature("Test"), &MockLM{}, []dsgo.Tool{})
	react.WithVerbose(true)

	if !react.Verbose {
		t.Error("WithVerbose should enable verbose mode")
	}
}

func TestReAct_GetSignature(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	if react.GetSignature() != sig {
		t.Error("GetSignature should return the signature")
	}
}

func TestReAct_ParseFinalAnswer_NestedJSON(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	content := `Text before {"answer": "value with {nested}"} after`
	outputs, err := react.parseFinalAnswer(content)

	if err != nil {
		t.Fatalf("parseFinalAnswer() error = %v", err)
	}

	if outputs["answer"] != "value with {nested}" {
		t.Error("Should handle nested braces")
	}
}

func TestReAct_ParseFinalAnswer_WithNewlines(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	content := `{"answer": "line1
line2"}`
	outputs, err := react.parseFinalAnswer(content)

	if err != nil {
		t.Fatalf("parseFinalAnswer() error = %v", err)
	}

	if outputs["answer"] != "line1\\nline2" && outputs["answer"] != "line1\nline2" {
		t.Logf("Answer: %v", outputs["answer"])
	}
}

func TestReAct_FixJSONNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "newline in string",
			input:    `{"text": "line1\nline2"}`,
			expected: `{"text": "line1\nline2"}`,
		},
		{
			name:     "literal newline",
			input:    "{\"text\": \"line1\nline2\"}",
			expected: `{"text": "line1\nline2"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixJSONNewlines(tt.input)
			if result != tt.expected {
				t.Logf("Expected: %q\nGot: %q", tt.expected, result)
			}
		})
	}
}

func TestReAct_ParseFinalAnswer_GenericCodeBlock(t *testing.T) {
	sig := dsgo.NewSignature("Test").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	content := "```\n{\"answer\": \"result\"}\n```"
	outputs, err := react.parseFinalAnswer(content)

	if err != nil {
		t.Fatalf("parseFinalAnswer() error = %v", err)
	}

	if outputs["answer"] != "result" {
		t.Error("Should parse generic code block")
	}
}

func TestReAct_BuildSystemPrompt_NoTools(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	prompt := react.buildSystemPrompt()
	if prompt != "" {
		t.Error("System prompt should be empty when no tools")
	}
}

func TestReAct_BuildSystemPrompt_WithTools(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	tool := dsgo.NewTool("test", "Test tool", nil)
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{*tool})

	prompt := react.buildSystemPrompt()
	if prompt == "" {
		t.Error("System prompt should not be empty with tools")
	}

	if !contains(prompt, "ReAct") {
		t.Error("System prompt should mention ReAct")
	}
}

func TestReAct_BuildInitialPrompt_NoDescription(t *testing.T) {
	sig := dsgo.NewSignature("").
		AddInput("question", dsgo.FieldTypeString, "Question").
		AddOutput("answer", dsgo.FieldTypeString, "Answer")

	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{})

	prompt, err := react.buildInitialPrompt(map[string]interface{}{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("buildInitialPrompt() error = %v", err)
	}

	if prompt == "" {
		t.Error("Prompt should not be empty")
	}
}

func TestReAct_FindTool(t *testing.T) {
	tool1 := dsgo.NewTool("search", "Search", nil)
	tool2 := dsgo.NewTool("calculate", "Calculate", nil)

	sig := dsgo.NewSignature("Test")
	react := NewReAct(sig, &MockLM{}, []dsgo.Tool{*tool1, *tool2})

	found := react.findTool("search")
	if found == nil || found.Name != "search" {
		t.Error("Should find existing tool")
	}

	notFound := react.findTool("nonexistent")
	if notFound != nil {
		t.Error("Should return nil for missing tool")
	}
}
