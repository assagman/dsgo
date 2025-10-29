package module

import (
	"context"
	"fmt"

	"github.com/assagman/dsgo"
)

// Program represents a composable pipeline of modules
type Program struct {
	modules []dsgo.Module
	name    string
}

// NewProgram creates a new program
func NewProgram(name string) *Program {
	return &Program{
		name:    name,
		modules: []dsgo.Module{},
	}
}

// AddModule adds a module to the program pipeline
func (p *Program) AddModule(module dsgo.Module) *Program {
	p.modules = append(p.modules, module)
	return p
}

// Forward executes the program by running modules in sequence
// Each module's outputs become available as inputs to subsequent modules
func (p *Program) Forward(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if len(p.modules) == 0 {
		return nil, fmt.Errorf("program has no modules")
	}

	currentInputs := inputs
	finalOutputs := make(map[string]any)

	for i, module := range p.modules {
		outputs, err := module.Forward(ctx, currentInputs)
		if err != nil {
			return nil, fmt.Errorf("module %d failed: %w", i, err)
		}

		// Accumulate outputs from all modules
		for k, v := range outputs {
			finalOutputs[k] = v
		}

		// Merge outputs into inputs for next module
		// This allows modules to access both original inputs and previous outputs
		merged := make(map[string]any)
		for k, v := range currentInputs {
			merged[k] = v
		}
		for k, v := range outputs {
			merged[k] = v
		}
		currentInputs = merged
	}

	return finalOutputs, nil
}

// GetSignature returns the signature of the last module in the pipeline
func (p *Program) GetSignature() *dsgo.Signature {
	if len(p.modules) == 0 {
		return nil
	}
	return p.modules[len(p.modules)-1].GetSignature()
}

// Name returns the program name
func (p *Program) Name() string {
	return p.name
}

// ModuleCount returns the number of modules in the program
func (p *Program) ModuleCount() int {
	return len(p.modules)
}
