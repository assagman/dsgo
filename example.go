package dsgo

import (
	"fmt"
	"strings"
)

// Example represents an input/output pair for few-shot learning
type Example struct {
	Inputs  map[string]interface{}
	Outputs map[string]interface{}
	
	// Optional metadata
	Label       string  // Human-readable label
	Weight      float64 // Importance weight (default 1.0)
	Description string  // Optional description
}

// NewExample creates a new example
func NewExample(inputs, outputs map[string]interface{}) *Example {
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
func (es *ExampleSet) AddPair(inputs, outputs map[string]interface{}) *ExampleSet {
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
	// TODO: Implement random sampling
	return es.GetN(n)
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

// Clone creates a deep copy of the example set
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

// FormatExamples formats examples for inclusion in prompts
func (es *ExampleSet) FormatExamples(signature *Signature) (string, error) {
	if es.IsEmpty() {
		return "", nil
	}
	
	var builder strings.Builder
	builder.WriteString("Here are some examples:\n\n")
	
	for i, ex := range es.examples {
		builder.WriteString(fmt.Sprintf("Example %d:\n", i+1))
		
		if ex.Label != "" {
			builder.WriteString(fmt.Sprintf("Label: %s\n", ex.Label))
		}
		
		// Format inputs
		builder.WriteString("Inputs:\n")
		for _, field := range signature.InputFields {
			if val, ok := ex.Inputs[field.Name]; ok {
				builder.WriteString(fmt.Sprintf("  %s: %v\n", field.Name, val))
			}
		}
		
		// Format outputs
		builder.WriteString("Outputs:\n")
		for _, field := range signature.OutputFields {
			if val, ok := ex.Outputs[field.Name]; ok {
				builder.WriteString(fmt.Sprintf("  %s: %v\n", field.Name, val))
			}
		}
		
		builder.WriteString("\n")
	}
	
	return builder.String(), nil
}

// Helper function to deep copy a map
func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}
