package module

import (
	"context"
	"fmt"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
)

// getMapKeys extracts keys from a map for debugging purposes
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

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
	// Ensure context has IDs
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleProgram, p.name)

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModuleProgram, time.Since(startTime), predErr)
	}()

	if len(p.modules) == 0 {
		predErr = fmt.Errorf("program has no modules")
		return nil, predErr
	}

	currentInputs := inputs
	finalOutputs := make(map[string]any)
	var lastPrediction *core.Prediction
	var totalUsage core.Usage

	// Check context cancellation before starting
	if err := ctx.Err(); err != nil {
		predErr = fmt.Errorf("program cancelled before module %d: %w", 0, err)
		return nil, predErr
	}

	for i, module := range p.modules {
		logging.GetLogger().Debug(ctx, "Program step", map[string]any{
			"step":   i + 1,
			"module": i,
		})

		prediction, err := module.Forward(ctx, currentInputs)
		if err != nil {
			predErr = fmt.Errorf("module %d failed: %w", i, err)
			return nil, predErr
		}

		// Validate outputs against module signature to catch malformed data early
		if sig := module.GetSignature(); sig != nil {
			if err := sig.ValidateOutputs(prediction.Outputs); err != nil {
				predErr = fmt.Errorf("module %d produced invalid outputs: %w", i, err)
				return nil, predErr
			}
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
		totalUsage.Cost += prediction.Usage.Cost
		totalUsage.Latency += prediction.Usage.Latency

		// Merge outputs into inputs for next module
		// This allows modules to access both original inputs and previous outputs
		merged := make(map[string]any)
		for k, v := range currentInputs {
			merged[k] = v
		}
		for k, v := range prediction.Outputs {
			merged[k] = v
		}

		// Pass Completions to next module only when the next module is a MultiChainComparison.
		// This avoids leaking internal completions into modules that don't consume them,
		// and prevents accidental overwrites of user-provided "completions" inputs.
		if len(prediction.Completions) > 0 && i+1 < len(p.modules) {
			switch p.modules[i+1].(type) {
			case *MultiChainComparison:
				merged["completions"] = prediction.Completions
			}
		}

		// Diagnostic logging for pipeline data flow debugging
		if logger := logging.GetLogger(); logger != nil {
			logger.Debug(ctx, "Program module data flow", map[string]any{
				"step":        i + 1,
				"moduleIndex": i,
				"inputKeys":   getMapKeys(currentInputs),
				"outputKeys":  getMapKeys(prediction.Outputs),
				"mergedKeys":  getMapKeys(merged),
				"module":      p.name,
			})
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

// Clone creates an independent copy of Program module
func (p *Program) Clone() core.Module {
	cloned := &Program{
		name:    p.name,
		modules: make([]core.Module, len(p.modules)),
	}

	// Clone all modules
	for i, module := range p.modules {
		cloned.modules[i] = module.Clone()
	}

	return cloned
}

// ModuleCount returns the number of modules in the program
func (p *Program) ModuleCount() int {
	return len(p.modules)
}
