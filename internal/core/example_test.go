package core

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

	negativeGetN := es.GetN(-1)
	if len(negativeGetN) != 2 {
		t.Error("GetN(negative) should return all examples")
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

	for i := 0; i < 10; i++ {
		es.AddPair(
			map[string]any{"input": i},
			map[string]any{"output": i * 2},
		)
	}

	tests := []struct {
		name     string
		n        int
		wantSize int
	}{
		{"get 1 random", 1, 1},
		{"get 5 random", 5, 5},
		{"get more than available", 20, 10},
		{"get 0", 0, 0},
		{"get negative", -1, 0},
		{"get all", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			random := es.GetRandom(tt.n)
			if len(random) != tt.wantSize {
				t.Errorf("GetRandom(%d) returned %d examples, want %d", tt.n, len(random), tt.wantSize)
			}
		})
	}

	results := make(map[int]bool)
	for i := 0; i < 5; i++ {
		random := es.GetRandom(3)
		if len(random) > 0 {
			firstInput := random[0].Inputs["input"].(int)
			results[firstInput] = true
		}
	}
	if len(results) == 1 && es.Len() > 3 {
		t.Log("GetRandom might not be random (or got unlucky in test)")
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
