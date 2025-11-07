package module

import (
	"context"
	"fmt"

	"github.com/assagman/dsgo/core"
)

// Program represents a composable pipeline of modules
type Program struct {
	modules []core.Module
	name    string
}

// NewProgram creates a new program
func NewProgram(name string) *Program {
	return &Program{
		name:    name,
		modules: []core.Module{},
	}
}

// AddModule adds a module to the program pipeline
func (p *Program) AddModule(module core.Module) *Program {
	p.modules = append(p.modules, module)
	return p
}

// Forward executes the program by running modules in sequence
// Each module's outputs become available as inputs to subsequent modules
func (p *Program) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	if len(p.modules) == 0 {
		return nil, fmt.Errorf("program has no modules")
	}

	currentInputs := inputs
	finalOutputs := make(map[string]any)
	var lastPrediction *core.Prediction
	var totalUsage core.Usage

	for i, module := range p.modules {
		prediction, err := module.Forward(ctx, currentInputs)
		if err != nil {
			return nil, fmt.Errorf("module %d failed: %w", i, err)
		}

		// Accumulate outputs from all modules
		for k, v := range prediction.Outputs {
			finalOutputs[k] = v
		}

		// Track last prediction
		lastPrediction = prediction

		// Accumulate usage stats
		totalUsage.PromptTokens += prediction.Usage.PromptTokens
		totalUsage.CompletionTokens += prediction.Usage.CompletionTokens
		totalUsage.TotalTokens += prediction.Usage.TotalTokens

		// Merge outputs into inputs for next module
		// This allows modules to access both original inputs and previous outputs
		merged := make(map[string]any)
		for k, v := range currentInputs {
			merged[k] = v
		}
		for k, v := range prediction.Outputs {
			merged[k] = v
		}
		currentInputs = merged
	}

	// Build final prediction from accumulated results
	finalPrediction := core.NewPrediction(finalOutputs).
		WithUsage(totalUsage).
		WithModuleName(p.name).
		WithInputs(inputs)

	// Carry over rationale from last prediction if available
	if lastPrediction != nil && lastPrediction.Rationale != "" {
		finalPrediction.Rationale = lastPrediction.Rationale
	}

	return finalPrediction, nil
}

// GetSignature returns the signature of the last module in the pipeline
func (p *Program) GetSignature() *core.Signature {
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
