package core

import (
	"math/rand"
)

// Example represents an input/output pair for few-shot learning
type Example struct {
	Inputs  map[string]any
	Outputs map[string]any

	// Optional metadata
	Label       string  // Human-readable label
	Weight      float64 // Importance weight (default 1.0)
	Description string  // Optional description
}

// NewExample creates a new example
func NewExample(inputs, outputs map[string]any) *Example {
	return &Example{
		Inputs:  inputs,
		Outputs: outputs,
		Weight:  1.0,
	}
}

// WithLabel adds a label to the example
func (e *Example) WithLabel(label string) *Example {
	e.Label = label
	return e
}

// WithWeight sets the importance weight
func (e *Example) WithWeight(weight float64) *Example {
	e.Weight = weight
	return e
}

// WithDescription adds a description
func (e *Example) WithDescription(desc string) *Example {
	e.Description = desc
	return e
}

// ExampleSet manages a collection of examples
type ExampleSet struct {
	examples []*Example
	name     string
}

// NewExampleSet creates a new example set
func NewExampleSet(name string) *ExampleSet {
	return &ExampleSet{
		examples: []*Example{},
		name:     name,
	}
}

// Add adds an example to the set
func (es *ExampleSet) Add(example *Example) *ExampleSet {
	es.examples = append(es.examples, example)
	return es
}

// AddPair adds an input/output pair as an example
func (es *ExampleSet) AddPair(inputs, outputs map[string]any) *ExampleSet {
	es.Add(NewExample(inputs, outputs))
	return es
}

// Get returns all examples
func (es *ExampleSet) Get() []*Example {
	return es.examples
}

// GetN returns the first n examples
func (es *ExampleSet) GetN(n int) []*Example {
	if n <= 0 || n >= len(es.examples) {
		return es.examples
	}
	return es.examples[:n]
}

// GetRandom returns n random examples
func (es *ExampleSet) GetRandom(n int) []*Example {
	if n <= 0 {
		return []*Example{}
	}

	if n >= len(es.examples) {
		return es.examples
	}

	// Create a copy of indices and shuffle them
	indices := make([]int, len(es.examples))
	for i := range indices {
		indices[i] = i
	}

	rand.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	// Select the first n shuffled examples
	result := make([]*Example, n)
	for i := 0; i < n; i++ {
		result[i] = es.examples[indices[i]]
	}

	return result
}

// Len returns the number of examples
func (es *ExampleSet) Len() int {
	return len(es.examples)
}

// IsEmpty returns true if the set has no examples
func (es *ExampleSet) IsEmpty() bool {
	return len(es.examples) == 0
}

// Clear removes all examples
func (es *ExampleSet) Clear() {
	es.examples = []*Example{}
}

// Clone creates a copy of the example set (shallow copy of map values)
func (es *ExampleSet) Clone() *ExampleSet {
	cloned := NewExampleSet(es.name)
	for _, ex := range es.examples {
		cloned.Add(&Example{
			Inputs:      copyMap(ex.Inputs),
			Outputs:     copyMap(ex.Outputs),
			Label:       ex.Label,
			Weight:      ex.Weight,
			Description: ex.Description,
		})
	}
	return cloned
}

// Helper function to deep copy a map
func copyMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}
