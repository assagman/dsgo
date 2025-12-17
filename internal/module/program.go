package module

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
)

// StepStatus represents the execution status of a program step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// ExecutionStatus represents the overall program execution status
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
)

// StepExecution contains complete execution data for a single step
type StepExecution struct {
	Index      int              `json:"index"`
	ModuleName string           `json:"module_name"`
	Status     StepStatus       `json:"status"`
	Prediction *core.Prediction `json:"prediction,omitempty"`
	Error      error            `json:"error,omitempty"`
	StartTime  time.Time        `json:"start_time"`
	Duration   time.Duration    `json:"duration"`
	Inputs     map[string]any   `json:"inputs"`
}

// ProgramExecution contains complete execution trace
type ProgramExecution struct {
	ProgramName string          `json:"program_name"`
	Steps       []StepExecution `json:"steps"`
	Status      ExecutionStatus `json:"status"`
	TotalUsage  core.Usage      `json:"total_usage"`
	StartTime   time.Time       `json:"start_time"`
	Duration    time.Duration   `json:"duration"`
	Error       error           `json:"error,omitempty"`
}

// Metrics returns aggregated metrics from execution
func (e *ProgramExecution) Metrics() ProgramMetrics {
	m := ProgramMetrics{
		TotalSteps:    len(e.Steps),
		TotalDuration: e.Duration,
		TotalUsage:    e.TotalUsage,
	}
	for _, step := range e.Steps {
		switch step.Status {
		case StepStatusCompleted:
			m.CompletedSteps++
		case StepStatusFailed:
			m.FailedSteps++
		case StepStatusSkipped:
			m.SkippedSteps++
		}
		if step.Duration > m.SlowestStep {
			m.SlowestStep = step.Duration
			m.SlowestStepIndex = step.Index
		}
	}
	return m
}

// ProgramMetrics provides execution statistics
type ProgramMetrics struct {
	TotalSteps       int           `json:"total_steps"`
	CompletedSteps   int           `json:"completed_steps"`
	FailedSteps      int           `json:"failed_steps"`
	SkippedSteps     int           `json:"skipped_steps"`
	TotalDuration    time.Duration `json:"total_duration"`
	SlowestStep      time.Duration `json:"slowest_step"`
	SlowestStepIndex int           `json:"slowest_step_index"`
	TotalUsage       core.Usage    `json:"total_usage"`
}

// CompletionsConsumer is implemented by modules that accept completions input
type CompletionsConsumer interface {
	RequiresCompletions() bool
}

// SignatureMismatch describes a signature compatibility error
type SignatureMismatch struct {
	ModuleIndex     int      `json:"module_index"`
	ModuleName      string   `json:"module_name"`
	MissingInputs   []string `json:"missing_inputs"`
	AvailableFields []string `json:"available_fields"`
}

func (e *SignatureMismatch) Error() string {
	return fmt.Sprintf(
		"signature mismatch at module %d (%s): missing required inputs %v; available: %v",
		e.ModuleIndex, e.ModuleName, e.MissingInputs, e.AvailableFields,
	)
}

// Program represents a composable pipeline of modules
type Program struct {
	modules []core.Module
	name    string

	// Execution state (protected by mutex for concurrent access)
	mu            sync.RWMutex
	lastExecution *ProgramExecution
}

// NewProgram creates a new program
func NewProgram(name string) *Program {
	return &Program{
		name:    name,
		modules: []core.Module{},
	}
}

// ValidateSignatures checks that all module signatures are compatible.
// Returns nil if valid, or SignatureMismatch error with details.
func (p *Program) ValidateSignatures(programInputs []string) error {
	available := make(map[string]bool)
	for _, name := range programInputs {
		available[name] = true
	}

	for i, module := range p.modules {
		sig := module.GetSignature()
		if sig == nil {
			continue
		}

		// Check required inputs
		var missing []string
		for _, field := range sig.InputFields {
			if !field.Optional && !available[field.Name] {
				missing = append(missing, field.Name)
			}
		}

		if len(missing) > 0 {
			availableList := make([]string, 0, len(available))
			for k := range available {
				availableList = append(availableList, k)
			}
			return &SignatureMismatch{
				ModuleIndex:     i,
				ModuleName:      sig.Description,
				MissingInputs:   missing,
				AvailableFields: availableList,
			}
		}

		// Add this module's outputs to available
		for _, field := range sig.OutputFields {
			available[field.Name] = true
		}
	}

	return nil
}

// AddModule adds a module with optional eager validation
func (p *Program) AddModule(module core.Module) *Program {
	p.modules = append(p.modules, module)
	return p
}

// AddModuleValidated adds a module and validates signature compatibility
func (p *Program) AddModuleValidated(module core.Module, programInputs []string) error {
	p.modules = append(p.modules, module)
	if err := p.ValidateSignatures(programInputs); err != nil {
		// Rollback
		p.modules = p.modules[:len(p.modules)-1]
		return err
	}
	return nil
}

// Forward executes the program by running modules in sequence
func (p *Program) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	// Initialize inputs if nil
	if inputs == nil {
		inputs = make(map[string]any)
	}

	// Initialize execution trace
	execution := &ProgramExecution{
		ProgramName: p.name,
		Steps:       make([]StepExecution, len(p.modules)),
		Status:      ExecutionStatusRunning,
		StartTime:   time.Now(),
	}

	// Store execution reference
	p.mu.Lock()
	p.lastExecution = execution
	p.mu.Unlock()

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleProgram, p.name)

	var predErr error
	defer func() {
		execution.Duration = time.Since(startTime)
		if predErr != nil {
			execution.Status = ExecutionStatusFailed
			execution.Error = predErr
		} else {
			execution.Status = ExecutionStatusCompleted
		}
		logging.LogPredictionEnd(ctx, logging.ModuleProgram, time.Since(startTime), predErr)
	}()

	if len(p.modules) == 0 {
		predErr = fmt.Errorf("program has no modules")
		return nil, predErr
	}

	// Check context cancellation before starting
	if err := ctx.Err(); err != nil {
		predErr = fmt.Errorf("program cancelled before execution: %w", err)
		return nil, predErr
	}

	currentInputs := inputs
	var lastPrediction *core.Prediction

	for i, module := range p.modules {
		// Initialize step
		execution.Steps[i] = StepExecution{
			Index:      i,
			ModuleName: p.getModuleName(module, i),
			Status:     StepStatusRunning,
			StartTime:  time.Now(),
			Inputs:     copyMap(currentInputs),
		}

		logging.GetLogger().Debug(ctx, "Program step", map[string]any{
			"step":   i + 1,
			"module": execution.Steps[i].ModuleName,
		})

		prediction, err := module.Forward(ctx, currentInputs)
		execution.Steps[i].Duration = time.Since(execution.Steps[i].StartTime)

		if err != nil {
			execution.Steps[i].Status = StepStatusFailed
			execution.Steps[i].Error = err
			// Mark remaining steps as skipped
			for j := i + 1; j < len(p.modules); j++ {
				execution.Steps[j].Status = StepStatusSkipped
			}
			predErr = fmt.Errorf("module %d (%s) failed: %w", i, execution.Steps[i].ModuleName, err)
			return nil, predErr
		}

		// Store complete prediction (unmodified)
		execution.Steps[i].Status = StepStatusCompleted
		execution.Steps[i].Prediction = prediction

		// Accumulate usage
		execution.TotalUsage.PromptTokens += prediction.Usage.PromptTokens
		execution.TotalUsage.CompletionTokens += prediction.Usage.CompletionTokens
		execution.TotalUsage.TotalTokens += prediction.Usage.TotalTokens
		execution.TotalUsage.Cost += prediction.Usage.Cost
		execution.TotalUsage.Latency += prediction.Usage.Latency

		// Build inputs for next module
		currentInputs = p.buildNextInputs(currentInputs, prediction, i)
		lastPrediction = prediction
	}

	// Return LAST prediction directly (not synthetic merge)
	// Only override usage with accumulated total
	lastPrediction.Usage = execution.TotalUsage
	lastPrediction.ModuleName = p.name
	lastPrediction.Inputs = inputs

	return lastPrediction, nil
}

// buildNextInputs constructs inputs for the next module
func (p *Program) buildNextInputs(current map[string]any, pred *core.Prediction, currentIndex int) map[string]any {
	merged := make(map[string]any, len(current)+len(pred.Outputs))

	// Copy current inputs
	for k, v := range current {
		merged[k] = v
	}

	// Add prediction outputs
	for k, v := range pred.Outputs {
		merged[k] = v
	}

	// Pass completions if next module requires them (interface-based)
	if len(pred.Completions) > 0 && currentIndex+1 < len(p.modules) {
		if consumer, ok := p.modules[currentIndex+1].(CompletionsConsumer); ok && consumer.RequiresCompletions() {
			merged["completions"] = pred.Completions
		}
	}

	return merged
}

// getModuleName extracts a name for logging
func (p *Program) getModuleName(module core.Module, index int) string {
	if sig := module.GetSignature(); sig != nil && sig.Description != "" {
		return sig.Description
	}
	return fmt.Sprintf("module_%d", index)
}

// GetExecution returns the last execution trace (thread-safe)
func (p *Program) GetExecution() *ProgramExecution {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastExecution
}

// GetMetrics returns metrics from the last execution
func (p *Program) GetMetrics() *ProgramMetrics {
	exec := p.GetExecution()
	if exec == nil {
		return nil
	}
	metrics := exec.Metrics()
	return &metrics
}

// copyMap creates a shallow copy of a map
func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
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
