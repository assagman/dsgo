package dsgo

import "testing"

func TestExample_Creation(t *testing.T) {
	inputs := map[string]any{"question": "What is 2+2?"}
	outputs := map[string]any{"answer": "4"}

	ex := NewExample(inputs, outputs)

	if ex.Inputs["question"] != "What is 2+2?" {
		t.Error("Inputs not set correctly")
	}
	if ex.Outputs["answer"] != "4" {
		t.Error("Outputs not set correctly")
	}
	if ex.Weight != 1.0 {
		t.Error("Default weight should be 1.0")
	}
}

func TestExample_WithMethods(t *testing.T) {
	ex := NewExample(
		map[string]any{"in": "test"},
		map[string]any{"out": "result"},
	).WithLabel("test-example").WithWeight(2.0).WithDescription("A test")

	if ex.Label != "test-example" {
		t.Error("Label not set")
	}
	if ex.Weight != 2.0 {
		t.Error("Weight not set")
	}
	if ex.Description != "A test" {
		t.Error("Description not set")
	}
}

func TestExampleSet_Operations(t *testing.T) {
	es := NewExampleSet("test-set")

	if !es.IsEmpty() {
		t.Error("New example set should be empty")
	}

	es.AddPair(
		map[string]any{"x": 1},
		map[string]any{"y": 2},
	)
	es.AddPair(
		map[string]any{"x": 3},
		map[string]any{"y": 4},
	)

	if es.Len() != 2 {
		t.Errorf("Expected 2 examples, got %d", es.Len())
	}

	first := es.GetN(1)
	if len(first) != 1 {
		t.Error("GetN(1) should return 1 example")
	}
}

func TestExampleSet_Clone(t *testing.T) {
	es := NewExampleSet("original")
	es.AddPair(
		map[string]any{"in": "1"},
		map[string]any{"out": "1"},
	)

	cloned := es.Clone()
	cloned.AddPair(
		map[string]any{"in": "2"},
		map[string]any{"out": "2"},
	)

	if es.Len() == cloned.Len() {
		t.Error("Clone should be independent")
	}
	if es.Len() != 1 {
		t.Error("Original should not be affected")
	}
}

func TestExampleSet_FormatExamples(t *testing.T) {
	es := NewExampleSet("test")
	es.AddPair(
		map[string]any{"question": "What is AI?"},
		map[string]any{"answer": "Artificial Intelligence"},
	)

	sig := NewSignature("Test").
		AddInput("question", FieldTypeString, "The question").
		AddOutput("answer", FieldTypeString, "The answer")

	formatted, err := es.FormatExamples(sig)
	if err != nil {
		t.Errorf("FormatExamples failed: %v", err)
	}

	if formatted == "" {
		t.Error("Formatted examples should not be empty")
	}
}

func TestExampleSet_FormatExamples_Empty(t *testing.T) {
	es := NewExampleSet("test")
	sig := NewSignature("Test")

	formatted, err := es.FormatExamples(sig)
	if err != nil {
		t.Errorf("FormatExamples failed: %v", err)
	}

	if formatted != "" {
		t.Error("Formatted examples should be empty for empty set")
	}
}

func TestExampleSet_FormatExamples_WithLabel(t *testing.T) {
	es := NewExampleSet("test")
	ex := NewExample(
		map[string]any{"in": "test"},
		map[string]any{"out": "result"},
	).WithLabel("example-1")
	es.Add(ex)

	sig := NewSignature("Test").
		AddInput("in", FieldTypeString, "Input").
		AddOutput("out", FieldTypeString, "Output")

	formatted, err := es.FormatExamples(sig)
	if err != nil {
		t.Errorf("FormatExamples failed: %v", err)
	}

	if !contains(formatted, "example-1") {
		t.Error("Formatted examples should include label")
	}
}

func TestExampleSet_GetN_EdgeCases(t *testing.T) {
	es := NewExampleSet("test")
	es.AddPair(map[string]any{"x": 1}, map[string]any{"y": 1})
	es.AddPair(map[string]any{"x": 2}, map[string]any{"y": 2})

	all := es.GetN(0)
	if len(all) != 2 {
		t.Error("GetN(0) should return all examples")
	}

	all = es.GetN(10)
	if len(all) != 2 {
		t.Error("GetN(n > length) should return all examples")
	}
}

func TestExampleSet_Clear(t *testing.T) {
	es := NewExampleSet("test")
	es.AddPair(map[string]any{"x": 1}, map[string]any{"y": 1})

	es.Clear()

	if !es.IsEmpty() {
		t.Error("ExampleSet should be empty after Clear()")
	}
}

func TestExampleSet_GetRandom(t *testing.T) {
	es := NewExampleSet("test")
	es.AddPair(map[string]any{"x": 1}, map[string]any{"y": 1})
	es.AddPair(map[string]any{"x": 2}, map[string]any{"y": 2})

	random := es.GetRandom(1)
	if len(random) != 1 {
		t.Error("GetRandom should return requested number")
	}
}

func TestExampleSet_Get(t *testing.T) {
	es := NewExampleSet("test")
	es.AddPair(map[string]any{"x": 1}, map[string]any{"y": 1})
	es.AddPair(map[string]any{"x": 2}, map[string]any{"y": 2})

	examples := es.Get()
	if len(examples) != 2 {
		t.Errorf("Expected 2 examples, got %d", len(examples))
	}
	if examples[1].Inputs["x"] != 2 {
		t.Errorf("Expected x=2 at index 1, got %v", examples[1].Inputs["x"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
