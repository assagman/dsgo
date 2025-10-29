package dsgo

import "testing"

// TestExampleSet_GetRandomComprehensive tests random example selection
func TestExampleSet_GetRandomComprehensive(t *testing.T) {
	es := NewExampleSet("test")

	// Add examples
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

	// Test randomness - running GetRandom multiple times should potentially give different results
	// (though not guaranteed due to randomness)
	results := make(map[int]bool)
	for i := 0; i < 5; i++ {
		random := es.GetRandom(3)
		if len(random) > 0 {
			firstInput := random[0].Inputs["input"].(int)
			results[firstInput] = true
		}
	}
	// We should see some variation (not a strong test, but reasonable)
	if len(results) == 1 && es.Len() > 3 {
		t.Log("GetRandom might not be random (or got unlucky in test)")
	}
}

// TestExample_WithMethodsComprehensive tests all Example builder methods
func TestExample_WithMethodsComprehensive(t *testing.T) {
	ex := NewExample(
		map[string]any{"input": "test"},
		map[string]any{"output": "result"},
	).
		WithLabel("example1").
		WithWeight(2.0).
		WithDescription("test example")

	if ex.Label != "example1" {
		t.Error("Label not set")
	}
	if ex.Weight != 2.0 {
		t.Error("Weight not set")
	}
	if ex.Description != "test example" {
		t.Error("Description not set")
	}

	// Default weight
	ex2 := NewExample(map[string]any{}, map[string]any{})
	if ex2.Weight != 1.0 {
		t.Error("Default weight should be 1.0")
	}
}

// TestExampleSet_AllMethods tests comprehensive ExampleSet functionality
func TestExampleSet_AllMethods(t *testing.T) {
	es := NewExampleSet("test-set")

	// Test empty set
	if !es.IsEmpty() {
		t.Error("New set should be empty")
	}
	if es.Len() != 0 {
		t.Error("Empty set should have length 0")
	}

	// Add examples
	es.Add(NewExample(
		map[string]any{"q": "1"},
		map[string]any{"a": "1"},
	))
	es.AddPair(
		map[string]any{"q": "2"},
		map[string]any{"a": "2"},
	)

	if es.IsEmpty() {
		t.Error("Set with examples should not be empty")
	}
	if es.Len() != 2 {
		t.Errorf("Expected 2 examples, got %d", es.Len())
	}

	// Test Get
	all := es.Get()
	if len(all) != 2 {
		t.Error("Get() should return all examples")
	}

	// Test GetN
	first := es.GetN(1)
	if len(first) != 1 {
		t.Error("GetN(1) should return 1 example")
	}

	allViaGetN := es.GetN(10)
	if len(allViaGetN) != 2 {
		t.Error("GetN(n > len) should return all examples")
	}

	zeroGetN := es.GetN(0)
	if len(zeroGetN) != 2 {
		t.Error("GetN(0) should return all examples")
	}

	negativeGetN := es.GetN(-1)
	if len(negativeGetN) != 2 {
		t.Error("GetN(negative) should return all examples")
	}

	// Test Clone
	cloned := es.Clone()
	if cloned.Len() != es.Len() {
		t.Error("Clone should have same length")
	}

	// Modify clone
	cloned.AddPair(map[string]any{"q": "3"}, map[string]any{"a": "3"})
	if cloned.Len() == es.Len() {
		t.Error("Clone should be independent")
	}

	// Test Clear
	es.Clear()
	if !es.IsEmpty() {
		t.Error("Clear() should empty the set")
	}
}
