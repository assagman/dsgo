package module

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/logging"
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

// ExecutionID uniquely identifies a program execution
type ExecutionID string

// ProgramExecution contains complete execution trace
type ProgramExecution struct {
	ID          ExecutionID     `json:"id"`
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

// ProgramResult contains both prediction and execution trace
type ProgramResult struct {
	Prediction  *core.Prediction  `json:"prediction"`
	Execution   *ProgramExecution `json:"execution"`
	ExecutionID ExecutionID       `json:"execution_id"`
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
	verbose bool // Enable debug logging

	// Baseline inputs for validation
	baselineInputs []string

	// Execution state (protected by mutex for concurrent access)
	mu              sync.RWMutex
	lastExecution   *ProgramExecution
	executionStore  map[ExecutionID]*ProgramExecution // Store by ID
	executionOrder  []ExecutionID                     // Track insertion order for O(1) trim
	retentionSize   int                               // Maximum number of executions to retain
	nextExecutionID uint64                            // For generating unique IDs
}

// NewProgram creates a new program
func NewProgram(name string) *Program {
	return &Program{
		name:           name,
		modules:        []core.Module{},
		executionStore: make(map[ExecutionID]*ProgramExecution),
		executionOrder: []ExecutionID{},
		retentionSize:  10, // Keep last 10 executions by default
	}
}

// WithExecutionRetention sets the maximum number of executions to retain
func (p *Program) WithExecutionRetention(size int) *Program {
	if size < 0 {
		size = 0
	}
	p.retentionSize = size
	return p
}

// generateExecutionID creates a unique execution ID
func (p *Program) generateExecutionID() ExecutionID {
	id := atomic.AddUint64(&p.nextExecutionID, 1)
	return ExecutionID(fmt.Sprintf("%s-%d-%d", p.name, time.Now().UnixNano(), id))
}

// trimExecutionStore removes oldest executions to maintain retention size
func (p *Program) trimExecutionStore() {
	if len(p.executionStore) <= p.retentionSize {
		return
	}

	// Optimize to O(1) using executionOrder
	oldestID := p.executionOrder[0]
	delete(p.executionStore, oldestID)
	p.executionOrder = p.executionOrder[1:]
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

// WithVerbose enables debug logging for program execution
func (p *Program) WithVerbose(verbose bool) *Program {
	p.verbose = verbose
	return p
}

// WithInputs sets baseline inputs for signature validation
func (p *Program) WithInputs(inputs []string) *Program {
	p.baselineInputs = inputs
	return p
}

// AddModuleValidated adds a module and validates signature compatibility.
// Uses baseline inputs if programInputs is nil.
// Returns an error if both programInputs and baseline inputs are nil.
func (p *Program) AddModuleValidated(module core.Module, programInputs []string) error {
	inputs := programInputs
	if inputs == nil {
		inputs = p.baselineInputs
	}
	if inputs == nil {
		return fmt.Errorf("programInputs required: pass explicitly or set via WithInputs()")
	}

	p.modules = append(p.modules, module)
	if err := p.ValidateSignatures(inputs); err != nil {
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

	// Generate execution ID and initialize trace
	executionID := p.generateExecutionID()
	execution := &ProgramExecution{
		ID:          executionID,
		ProgramName: p.name,
		Steps:       make([]StepExecution, len(p.modules)),
		Status:      ExecutionStatusRunning,
		StartTime:   time.Now(),
	}

	// Initialize all steps as pending
	for i := range execution.Steps {
		execution.Steps[i] = StepExecution{
			Index:  i,
			Status: StepStatusPending,
		}
	}

	// Store execution reference with retention policy
	p.mu.Lock()
	p.lastExecution = execution
	p.executionStore[executionID] = execution
	p.executionOrder = append(p.executionOrder, executionID)

	// Apply retention policy
	if len(p.executionStore) > p.retentionSize {
		p.trimExecutionStore()
	}
	p.mu.Unlock()

	if p.verbose {
		logging.GetLogger().Info(ctx, "Program execution started", map[string]any{
			"program_name": p.name,
			"module_count": len(p.modules),
			"input_keys":   getMapKeys(inputs),
		})
	}

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleProgram, p.name)

	var predErr error
	defer func() {
		p.mu.Lock()
		execution.Duration = time.Since(startTime)
		if predErr != nil {
			execution.Status = ExecutionStatusFailed
			execution.Error = predErr
		} else {
			execution.Status = ExecutionStatusCompleted
		}
		p.mu.Unlock()

		if p.verbose {
			if predErr != nil {
				logging.GetLogger().Error(ctx, "Program execution failed", map[string]any{
					"program_name": p.name,
					"duration":     execution.Duration,
					"error":        predErr.Error(),
					"completed_steps": func() int {
						count := 0
						p.mu.RLock()
						for _, step := range execution.Steps {
							if step.Status == StepStatusCompleted {
								count++
							}
						}
						p.mu.RUnlock()
						return count
					}(),
					"total_steps": len(p.modules),
				})
			} else {
				p.mu.RLock()
				totalTokens := execution.TotalUsage.TotalTokens
				totalCost := execution.TotalUsage.Cost
				p.mu.RUnlock()
				logging.GetLogger().Info(ctx, "Program execution completed", map[string]any{
					"program_name": p.name,
					"duration":     execution.Duration,
					"total_steps":  len(p.modules),
					"total_tokens": totalTokens,
					"total_cost":   totalCost,
				})
			}
		}
		logging.LogPredictionEnd(ctx, logging.ModuleProgram, time.Since(startTime), predErr)
	}()

	if len(p.modules) == 0 {
		if p.verbose {
			logging.GetLogger().Error(ctx, "Program has no modules", nil)
		}
		predErr = fmt.Errorf("program has no modules")
		return nil, predErr
	}

	// Check context cancellation before starting
	if err := ctx.Err(); err != nil {
		if p.verbose {
			logging.GetLogger().Warn(ctx, "Program cancelled before execution", map[string]any{
				"error": err.Error(),
			})
		}
		predErr = fmt.Errorf("program cancelled before execution: %w", err)
		return nil, predErr
	}

	currentInputs := inputs
	var lastPrediction *core.Prediction

	for i, module := range p.modules {
		// Initialize step (protected)
		moduleName := p.getModuleName(module, i)
		stepStartTime := time.Now()
		inputsCopy := copyMap(currentInputs)

		p.mu.Lock()
		execution.Steps[i] = StepExecution{
			Index:      i,
			ModuleName: moduleName,
			Status:     StepStatusRunning,
			StartTime:  stepStartTime,
			Inputs:     inputsCopy,
		}
		p.mu.Unlock()

		if p.verbose {
			logging.GetLogger().Info(ctx, "Program step started", map[string]any{
				"step":       i + 1,
				"module":     moduleName,
				"input_keys": getMapKeys(currentInputs),
			})
		} else {
			logging.GetLogger().Debug(ctx, "Program step", map[string]any{
				"step":   i + 1,
				"module": moduleName,
			})
		}

		prediction, err := module.Forward(ctx, currentInputs)
		stepDuration := time.Since(stepStartTime)

		if err != nil {
			p.mu.Lock()
			execution.Steps[i].Duration = stepDuration
			execution.Steps[i].Status = StepStatusFailed
			execution.Steps[i].Error = err
			// Mark remaining steps as skipped
			for j := i + 1; j < len(p.modules); j++ {
				execution.Steps[j].Status = StepStatusSkipped
			}
			p.mu.Unlock()

			if p.verbose {
				logging.GetLogger().Error(ctx, "Program step failed", map[string]any{
					"step":       i + 1,
					"module":     moduleName,
					"duration":   stepDuration,
					"error":      err.Error(),
					"input_keys": getMapKeys(inputsCopy),
				})
			}

			predErr = fmt.Errorf("module %d (%s) failed: %w", i, moduleName, err)
			return nil, predErr
		}

		// Validate outputs against signature
		if sig := module.GetSignature(); sig != nil {
			if err := sig.ValidateOutputs(prediction.Outputs); err != nil {
				stepErr := fmt.Errorf("module %d (%s) output validation failed: %w", i, moduleName, err)

				p.mu.Lock()
				execution.Steps[i].Duration = stepDuration
				execution.Steps[i].Status = StepStatusFailed
				execution.Steps[i].Error = stepErr
				// Mark remaining steps as skipped
				for j := i + 1; j < len(p.modules); j++ {
					execution.Steps[j].Status = StepStatusSkipped
				}
				p.mu.Unlock()

				if p.verbose {
					logging.GetLogger().Error(ctx, "Program step output validation failed", map[string]any{
						"step":        i + 1,
						"module":      moduleName,
						"duration":    stepDuration,
						"error":       stepErr.Error(),
						"output_keys": getMapKeys(prediction.Outputs),
					})
				}

				predErr = stepErr
				return nil, predErr
			}
		}

		// Store complete prediction (protected)
		p.mu.Lock()
		execution.Steps[i].Duration = stepDuration
		execution.Steps[i].Status = StepStatusCompleted
		execution.Steps[i].Prediction = prediction

		// Accumulate usage
		execution.TotalUsage.PromptTokens += prediction.Usage.PromptTokens
		execution.TotalUsage.CompletionTokens += prediction.Usage.CompletionTokens
		execution.TotalUsage.TotalTokens += prediction.Usage.TotalTokens
		execution.TotalUsage.Cost += prediction.Usage.Cost
		execution.TotalUsage.Latency += prediction.Usage.Latency
		p.mu.Unlock()

		if p.verbose {
			logging.GetLogger().Info(ctx, "Program step completed", map[string]any{
				"step":              i + 1,
				"module":            moduleName,
				"duration":          stepDuration,
				"input_keys":        getMapKeys(inputsCopy),
				"output_keys":       getMapKeys(prediction.Outputs),
				"prompt_tokens":     prediction.Usage.PromptTokens,
				"completion_tokens": prediction.Usage.CompletionTokens,
				"cost":              prediction.Usage.Cost,
			})
		}

		// Build inputs for next module
		currentInputs = p.buildNextInputs(currentInputs, prediction, i)
		lastPrediction = prediction
	}

	// Return a COPY of the last prediction to avoid mutation
	// Override usage with accumulated total and set program metadata
	resultPrediction := copyPrediction(lastPrediction)
	p.mu.RLock()
	resultPrediction.Usage = execution.TotalUsage
	p.mu.RUnlock()
	resultPrediction.ModuleName = p.name
	resultPrediction.Inputs = copyMap(inputs)

	return resultPrediction, nil
}

// ForwardWithTrace executes the program and returns both prediction and execution trace
func (p *Program) ForwardWithTrace(ctx context.Context, inputs map[string]any) (*ProgramResult, error) {
	prediction, err := p.Forward(ctx, inputs)
	if err != nil {
		return nil, err
	}

	execution := p.GetExecution()
	if execution == nil {
		return nil, fmt.Errorf("execution trace not available")
	}

	result := &ProgramResult{
		Prediction:  prediction,
		Execution:   execution,
		ExecutionID: execution.ID,
	}

	return result, nil
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

// getMapKeys returns keys from a map for logging
func getMapKeys(m map[string]any) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getModuleName extracts a name for logging
func (p *Program) getModuleName(module core.Module, index int) string {
	if sig := module.GetSignature(); sig != nil && sig.Description != "" {
		return sig.Description
	}
	return fmt.Sprintf("module_%d", index)
}

// GetExecution returns a copy of the last execution trace (thread-safe)
func (p *Program) GetExecution() *ProgramExecution {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return copyExecution(p.lastExecution)
}

// GetExecutionByID returns execution trace by ID (thread-safe)
func (p *Program) GetExecutionByID(id ExecutionID) *ProgramExecution {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return copyExecution(p.executionStore[id])
}

// GetLastExecutionID returns the ID of the last execution
func (p *Program) GetLastExecutionID() ExecutionID {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.lastExecution == nil {
		return ""
	}
	return p.lastExecution.ID
}

// GetAllExecutionIDs returns all stored execution IDs (thread-safe)
func (p *Program) GetAllExecutionIDs() []ExecutionID {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]ExecutionID, 0, len(p.executionStore))
	for id := range p.executionStore {
		ids = append(ids, id)
	}
	return ids
}

// GetStepPrediction returns the prediction for a specific step (thread-safe)
func (p *Program) GetStepPrediction(executionID ExecutionID, stepIndex int) (*core.Prediction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	execution := p.executionStore[executionID]
	if execution == nil {
		return nil, fmt.Errorf("execution %s not found", executionID)
	}

	if stepIndex < 0 || stepIndex >= len(execution.Steps) {
		return nil, fmt.Errorf("step index %d out of bounds (0-%d)", stepIndex, len(execution.Steps)-1)
	}

	step := execution.Steps[stepIndex]
	if step.Status != StepStatusCompleted {
		return nil, fmt.Errorf("step %d not completed (status: %s)", stepIndex, step.Status)
	}

	return copyPrediction(step.Prediction), nil
}

// GetStepOutput returns a specific output value from a step (thread-safe)
func (p *Program) GetStepOutput(executionID ExecutionID, stepIndex int, key string) (any, error) {
	prediction, err := p.GetStepPrediction(executionID, stepIndex)
	if err != nil {
		return nil, err
	}

	value, exists := prediction.Outputs[key]
	if !exists {
		return nil, fmt.Errorf("output key '%s' not found in step %d", key, stepIndex)
	}

	return value, nil
}

// GetLastStepPrediction returns prediction from last execution (convenience method)
func (p *Program) GetLastStepPrediction(stepIndex int) (*core.Prediction, error) {
	executionID := p.GetLastExecutionID()
	if executionID == "" {
		return nil, fmt.Errorf("no executions available")
	}

	return p.GetStepPrediction(executionID, stepIndex)
}

// GetLastStepOutput returns output from last execution (convenience method)
func (p *Program) GetLastStepOutput(stepIndex int, key string) (any, error) {
	prediction, err := p.GetLastStepPrediction(stepIndex)
	if err != nil {
		return nil, err
	}

	value, exists := prediction.Outputs[key]
	if !exists {
		return nil, fmt.Errorf("output key '%s' not found in step %d", key, stepIndex)
	}

	return value, nil
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

// copyExecution creates a deep copy of a program execution
func copyExecution(exec *ProgramExecution) *ProgramExecution {
	if exec == nil {
		return nil
	}

	// Copy steps slice
	stepsCopy := make([]StepExecution, len(exec.Steps))
	for i, step := range exec.Steps {
		stepsCopy[i] = StepExecution{
			Index:      step.Index,
			ModuleName: step.ModuleName,
			Status:     step.Status,
			Prediction: copyPrediction(step.Prediction),
			Error:      step.Error,
			StartTime:  step.StartTime,
			Duration:   step.Duration,
			Inputs:     copyMap(step.Inputs),
		}
	}

	// Create copy
	return &ProgramExecution{
		ID:          exec.ID,
		ProgramName: exec.ProgramName,
		Steps:       stepsCopy,
		Status:      exec.Status,
		TotalUsage:  exec.TotalUsage, // Struct copies by value
		StartTime:   exec.StartTime,
		Duration:    exec.Duration,
		Error:       exec.Error,
	}
}

// copyPrediction creates a deep copy of a prediction
func copyPrediction(p *core.Prediction) *core.Prediction {
	if p == nil {
		return nil
	}

	// Copy completions slice
	completionsCopy := make([]map[string]any, len(p.Completions))
	for i, completion := range p.Completions {
		completionsCopy[i] = core.DeepCopyMap(completion)
	}

	// Create new prediction
	var diagCopy *core.ValidationDiagnostics
	if p.ParseDiagnostics != nil {
		diagCopy = p.ParseDiagnostics.Clone()
	}

	resultCopy := &core.Prediction{
		Outputs:          core.DeepCopyMap(p.Outputs),
		Usage:            p.Usage, // Usage is a struct, copies by value
		ModuleName:       p.ModuleName,
		Inputs:           core.DeepCopyMap(p.Inputs),
		Rationale:        p.Rationale,
		Score:            p.Score,
		Completions:      completionsCopy,
		AdapterUsed:      p.AdapterUsed,
		ParseSuccess:     p.ParseSuccess,
		ParseAttempts:    p.ParseAttempts,
		FallbackUsed:     p.FallbackUsed,
		ParseDiagnostics: diagCopy,
		Metadata:         core.DeepCopyMap(p.Metadata),
	}

	return resultCopy
}

// copyMap creates a deep copy of a map using core.DeepCopyMap
func copyMap(m map[string]any) map[string]any {
	return core.DeepCopyMap(m)
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
	p.mu.RLock()
	defer p.mu.RUnlock()

	cloned := &Program{
		name:            p.name,
		modules:         make([]core.Module, len(p.modules)),
		verbose:         p.verbose,
		retentionSize:   p.retentionSize,
		executionStore:  make(map[ExecutionID]*ProgramExecution),
		executionOrder:  []ExecutionID{},
		nextExecutionID: 0,
	}

	if p.baselineInputs != nil {
		cloned.baselineInputs = append([]string(nil), p.baselineInputs...)
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
