package module

import (
	"testing"

	"github.com/assagman/dsgo"
)

// TestChainOfThought_WithAdapter tests adapter configuration
func TestChainOfThought_WithAdapter(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	lm := &MockLM{}
	adapter := dsgo.NewChatAdapter()

	cot := NewChainOfThought(sig, lm).WithAdapter(adapter)
	if cot.Adapter != adapter {
		t.Error("WithAdapter should set custom adapter")
	}
}

// TestReAct_WithMethods tests all ReAct configuration methods
func TestReAct_WithMethods(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	lm := &MockLM{}
	tools := []dsgo.Tool{}
	history := dsgo.NewHistory()
	demos := []dsgo.Example{
		*dsgo.NewExample(
			map[string]any{"question": "test"},
			map[string]any{"answer": "test"},
		),
	}
	adapter := dsgo.NewJSONAdapter()

	react := NewReAct(sig, lm, tools).
		WithAdapter(adapter).
		WithHistory(history).
		WithDemos(demos)

	if react.Adapter != adapter {
		t.Error("WithAdapter should set adapter")
	}
	if react.History != history {
		t.Error("WithHistory should set history")
	}
	if len(react.Demos) != 1 {
		t.Error("WithDemos should set demos")
	}
}

// TestRefine_WithAdapter tests adapter configuration
func TestRefine_WithAdapter(t *testing.T) {
	sig := dsgo.NewSignature("test").
		AddInput("question", dsgo.FieldTypeString, "").
		AddOutput("answer", dsgo.FieldTypeString, "")

	lm := &MockLM{}
	adapter := dsgo.NewChatAdapter()

	refine := NewRefine(sig, lm).WithAdapter(adapter)
	if refine.Adapter != adapter {
		t.Error("WithAdapter should set custom adapter")
	}
}
