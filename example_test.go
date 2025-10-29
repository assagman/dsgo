package dsgo

import "testing"

func TestExample_Creation(t *testing.T) {
	inputs := map[string]interface{}{"question": "What is 2+2?"}
	outputs := map[string]interface{}{"answer": "4"}
	
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
		map[string]interface{}{"in": "test"},
		map[string]interface{}{"out": "result"},
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
		map[string]interface{}{"x": 1},
		map[string]interface{}{"y": 2},
	)
	es.AddPair(
		map[string]interface{}{"x": 3},
		map[string]interface{}{"y": 4},
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
		map[string]interface{}{"in": "1"},
		map[string]interface{}{"out": "1"},
	)
	
	cloned := es.Clone()
	cloned.AddPair(
		map[string]interface{}{"in": "2"},
		map[string]interface{}{"out": "2"},
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
		map[string]interface{}{"question": "What is AI?"},
		map[string]interface{}{"answer": "Artificial Intelligence"},
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
