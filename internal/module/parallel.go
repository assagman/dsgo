package module

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
)

// ParallelMetrics contains execution metrics for parallel execution
type ParallelMetrics struct {
	Total     int `json:"total"`
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
	Latency   struct {
		MinMs int64 `json:"min_ms"`
		MaxMs int64 `json:"max_ms"`
		AvgMs int64 `json:"avg_ms"`
		P50Ms int64 `json:"p50_ms"`
	} `json:"latency"`
}

// parallelModuleInfo contains introspection information for logging
type parallelModuleInfo struct {
	ModuleType string `json:"module_type"`
	LMModel    string `json:"lm_model"`
}

// Parallel executes a module across multiple inputs concurrently.
//
// By default, Parallel creates isolated module instances by cloning the base module
// for each task, ensuring no shared state between parallel executions. This provides
// semantic isolation similar to DSPy's Parallel behavior.
//
// Advanced usage patterns:
//   - Default: Module is cloned per task (isolated state)
//   - NewParallelWithFactory: Custom instance creation per task
//   - NewParallelWithInstances: Pre-created instances with controlled sharing
//
// Input modes:
//   - Batch: inputs["_batch"] = []map[string]any
//   - Map-of-slices: any []any values are zipped (must have equal length)
//   - Repeat: WithRepeat(n) duplicates single input n times
type Parallel struct {
	// Module configuration
	module    core.Module
	factory   func(i int) core.Module
	instances []core.Module

	// Behavior options
	maxWorkers     int
	maxFailures    int
	failFast       bool
	returnAll      bool
	onlySuccessful bool
	batchKey       string
	repeat         int
	verbose        bool

	// Runtime state for logging/context (captured once)
	moduleInfoOnce sync.Once
	moduleInfo     parallelModuleInfo
}

// getParallelModuleInfo extracts module type and LM model information for logging
func getParallelModuleInfo(m core.Module) parallelModuleInfo {
	info := parallelModuleInfo{}
	if m == nil {
		return info
	}

	// Extract module type name (e.g., "Predict", "ChainOfThought")
	t := reflect.TypeOf(m)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	info.ModuleType = t.Name()

	// Try to find LM field and get its Name()
	v := reflect.ValueOf(m)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.IsValid() && v.Kind() == reflect.Struct {
		if f := v.FieldByName("LM"); f.IsValid() && f.CanInterface() {
			if lm, ok := f.Interface().(core.LM); ok && lm != nil {
				info.LMModel = lm.Name()
			}
		}
	}

	return info
}

// baseModule returns a representative module for introspection
// For factory-based parallels, this returns nil to avoid extra factory calls.
func (p *Parallel) baseModule() core.Module {
	if p.module != nil {
		return p.module
	}
	if len(p.instances) > 0 {
		return p.instances[0]
	}
	// factory-only: return nil to avoid calling factory outside of tasks
	return nil
}

// summarizeInputsForLog creates a log-friendly summary of inputs.
//
// Returns (summary, truncated) where truncated is true if any string value was
// shortened.
func summarizeInputsForLog(inputs map[string]any) (map[string]any, bool) {
	summary := make(map[string]any, len(inputs))
	truncated := false
	for k, v := range inputs {
		switch val := v.(type) {
		case string:
			// Truncate long strings (e.g., file_contents)
			if len(val) > 512 {
				summary[k] = val[:512] + "...[truncated]"
				truncated = true
			} else {
				summary[k] = val
			}
		default:
			// Preserve scalars, summarize complex types
			kind := reflect.TypeOf(v).Kind()
			switch kind {
			case reflect.Bool, reflect.Int, reflect.Int64, reflect.Float32, reflect.Float64:
				summary[k] = v
			default:
				summary[k] = fmt.Sprintf("<%T>", v)
			}
		}
	}
	return summary, truncated
}

func (p *Parallel) parallelMode() string {
	if p.factory != nil {
		return "factory"
	}
	if len(p.instances) > 0 {
		return "instances"
	}
	return "clone"
}

func (p *Parallel) commonLogFields(parallelID string, info parallelModuleInfo, batchSize int) map[string]any {
	return map[string]any{
		"module":        logging.ModuleParallel,
		"parallel_id":   parallelID,
		"parallel_mode": p.parallelMode(),
		"inner_module":  info.ModuleType,
		"lm_model":      info.LMModel,
		"batch_size":    batchSize,
		"max_workers":   p.maxWorkers,
		"fail_fast":     p.failFast,
		"max_failures":  p.maxFailures,
		"return_all":    p.returnAll,
		"only_success":  p.onlySuccessful,
		"repeat_factor": p.repeat,
		"batch_key":     p.batchKey,
		"verbose":       p.verbose,
	}
}

func classifyParallelTaskError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "task_error"
}

func firstNStrings(in []string, n int) []string {
	if n <= 0 {
		return nil
	}
	if len(in) <= n {
		out := make([]string, len(in))
		copy(out, in)
		return out
	}
	out := make([]string, n)
	copy(out, in[:n])
	return out
}

// NewParallel creates a Parallel module with automatic module cloning.
// The base module is cloned for each task to ensure state isolation.
// This is the recommended default for all modules, including stateful ones.
func NewParallel(module core.Module) *Parallel {
	return &Parallel{
		module:         module,
		maxWorkers:     runtime.NumCPU(),
		maxFailures:    0,
		failFast:       false,
		returnAll:      true,
		onlySuccessful: true,
		batchKey:       "_batch",
		repeat:         1,
		verbose:        false,
	}
}

// NewParallelWithFactory creates a Parallel module with a factory function.
// The factory is called for each task with the task index.
// This is the recommended approach for stateful modules.
func NewParallelWithFactory(factory func(i int) core.Module) *Parallel {
	return &Parallel{
		factory:        factory,
		maxWorkers:     runtime.NumCPU(),
		maxFailures:    0,
		failFast:       false,
		returnAll:      true,
		onlySuccessful: true,
		batchKey:       "_batch",
		repeat:         1,
		verbose:        false,
	}
}

// NewParallelWithInstances creates a Parallel module with pre-created instances.
// Each task will use instances[i % len(instances)].
func NewParallelWithInstances(instances []core.Module) *Parallel {
	if len(instances) == 0 {
		panic("NewParallelWithInstances: instances slice cannot be empty")
	}
	return &Parallel{
		instances:      instances,
		maxWorkers:     len(instances),
		maxFailures:    0,
		failFast:       false,
		returnAll:      true,
		onlySuccessful: true,
		batchKey:       "_batch",
		repeat:         1,
		verbose:        false,
	}
}

// WithMaxWorkers sets the maximum number of concurrent workers
func (p *Parallel) WithMaxWorkers(n int) *Parallel {
	if n <= 0 {
		panic("WithMaxWorkers: n must be positive")
	}
	p.maxWorkers = n
	return p
}

// WithMaxFailures sets the maximum number of failures before giving up.
// Set to 0 to require all tasks to succeed.
func (p *Parallel) WithMaxFailures(n int) *Parallel {
	p.maxFailures = n
	return p
}

// WithFailFast enables cancellation on first failure
func (p *Parallel) WithFailFast(on bool) *Parallel {
	p.failFast = on
	return p
}

// WithReturnAll enables returning all results in Completions
func (p *Parallel) WithReturnAll(on bool) *Parallel {
	p.returnAll = on
	return p
}

// WithOnlySuccessful filters failures from Completions (only when ReturnAll is true)
func (p *Parallel) WithOnlySuccessful(on bool) *Parallel {
	p.onlySuccessful = on
	return p
}

// WithBatchKey sets the key to use for batch input (default: "_batch")
func (p *Parallel) WithBatchKey(key string) *Parallel {
	p.batchKey = key
	return p
}

// WithRepeat sets the number of times to repeat the same input
func (p *Parallel) WithRepeat(n int) *Parallel {
	if n <= 0 {
		panic("WithRepeat: n must be positive")
	}
	p.repeat = n
	return p
}

// WithVerbose enables verbose logging of parallel execution details.
//
// Verbose logging contract (schema v1)
//
// When verbose is enabled, Parallel emits the following log messages:
//   - "Parallel batch started" (always INFO)
//   - "Parallel task started"
//   - "Parallel task completed"
//   - "Parallel task failed"
//   - "Parallel batch completed"
//
// Per-task and batch-completed logs are emitted at INFO when verbose, DEBUG otherwise.
// Each log includes a stable set of structured fields to make it easy to filter
// and aggregate. The canonical module name is always "module.Parallel".
//
// Common fields (present on all Parallel verbose logs):
//   - module: "module.Parallel"
//   - parallel_id: correlation identifier for the whole batch
//   - parallel_mode: one of "clone", "factory", "instances"
//   - inner_module: inner module type name (best-effort; may be empty for factory mode at batch start)
//   - lm_model: LM model name (best-effort)
//   - batch_size: number of tasks
//   - max_workers: maximum concurrent workers
//   - fail_fast, max_failures, return_all, only_success, repeat_factor, batch_key, verbose
//
// Task fields (present on per-task logs):
//   - task_index: 0-based index within the batch
//   - task_total: total tasks in the batch (equals batch_size)
//   - inputs: summarized inputs (truncated; complex types are summarized)
//
// When enabled, per-task logs are emitted at INFO level instead of DEBUG.
func (p *Parallel) WithVerbose(verbose bool) *Parallel {
	p.verbose = verbose
	return p
}

// GetSignature returns the wrapped module's signature
func (p *Parallel) GetSignature() *core.Signature {
	if p.module != nil {
		return p.module.GetSignature()
	}
	if p.factory != nil {
		// Create a temporary instance to get signature
		tempModule := p.factory(0)
		return tempModule.GetSignature()
	}
	if len(p.instances) > 0 {
		return p.instances[0].GetSignature()
	}
	return nil
}

// Forward executes the module in parallel across expanded inputs
func (p *Parallel) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)
	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleParallel, "Parallel execution")

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModuleParallel, time.Since(startTime), predErr)
	}()

	// Expand inputs into batch
	batch, err := p.expandInputs(inputs)
	if err != nil {
		predErr = fmt.Errorf("failed to expand inputs: %w", err)
		return nil, predErr
	}

	if len(batch) == 0 {
		predErr = fmt.Errorf("no inputs to process")
		return nil, predErr
	}

	parallelID := logging.GetCorrelationID(ctx)

	// Log batch-level information
	// For factory-based parallels, we don't have module info yet.
	var info parallelModuleInfo
	if p.module != nil || len(p.instances) > 0 {
		info = getParallelModuleInfo(p.baseModule())
	}
	// For factory-based parallels, info will be captured lazily during task execution.

	logging.GetLogger().Info(ctx, "Parallel batch started", p.commonLogFields(parallelID, info, len(batch)))

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Job and result types
	type job struct {
		idx        int
		inputs     map[string]any
		enqueuedAt time.Time
	}

	type result struct {
		idx  int
		pred *core.Prediction
		err  error
		dur  time.Duration
	}

	// Create channels
	jobs := make(chan job, len(batch))
	results := make(chan result, len(batch))
	var wg sync.WaitGroup

	// Module getter
	getModule := func(i int) core.Module {
		if p.factory != nil {
			return p.factory(i)
		}
		if len(p.instances) > 0 {
			return p.instances[i%len(p.instances)]
		}
		// Default: clone the base module for each task to ensure state isolation
		return p.module.Clone()
	}

	// Start workers
	workers := min(p.maxWorkers, len(batch))
	for workerID := 0; workerID < workers; workerID++ {
		wid := workerID
		wg.Go(func() {
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				queueWaitMs := time.Since(j.enqueuedAt).Milliseconds()
				if queueWaitMs < 0 {
					queueWaitMs = 0
				}
				mod := getModule(j.idx)
				info := getParallelModuleInfo(mod)

				// Capture module info once for factory-based parallels (used in final metadata)
				p.moduleInfoOnce.Do(func() {
					p.moduleInfo = info
				})

				taskCorrelationID := fmt.Sprintf("%s/task/%d", parallelID, j.idx)
				taskCtx := logging.WithCorrelationID(ctx, taskCorrelationID)

				fields := p.commonLogFields(parallelID, info, len(batch))
				fields["task_index"] = j.idx
				fields["task_total"] = len(batch)
				fields["worker_id"] = wid
				fields["queue_wait_ms"] = queueWaitMs

				inputsSummary, inputsTruncated := summarizeInputsForLog(j.inputs)
				fields["inputs"] = inputsSummary
				fields["inputs_truncated"] = inputsTruncated

				// Use INFO level when verbose is enabled, otherwise DEBUG.
				if p.verbose {
					logging.GetLogger().Info(taskCtx, "Parallel task started", fields)
				} else {
					logging.GetLogger().Debug(taskCtx, "Parallel task started", fields)
				}

				// Verbose direct prints (bypass log level, like ReAct)
				if p.verbose {
					if path, ok := j.inputs["file_path"].(string); ok {
						fmt.Printf("Parallel[%s] task %d started: %s\n",
							info.ModuleType, j.idx, path)
					} else {
						fmt.Printf("Parallel[%s] task %d started\n", info.ModuleType, j.idx)
					}
				}

				pred, err := mod.Forward(taskCtx, j.inputs)
				duration := time.Since(start)

				// Verbose direct prints for completion (bypass log level, like ReAct)
				if p.verbose {
					if err != nil {
						fmt.Printf("Parallel[%s] task %d FAILED after %dms: %v\n",
							info.ModuleType, j.idx, duration.Milliseconds(), err)
					} else {
						fmt.Printf("Parallel[%s] task %d completed in %dms (tokens=%d cost=%.6f)\n",
							info.ModuleType, j.idx, duration.Milliseconds(),
							pred.Usage.TotalTokens, pred.Usage.Cost)
					}
				}

				// Log task completion
				completionFields := p.commonLogFields(parallelID, info, len(batch))
				completionFields["task_index"] = j.idx
				completionFields["task_total"] = len(batch)
				completionFields["worker_id"] = wid
				completionFields["queue_wait_ms"] = queueWaitMs
				completionFields["duration_ms"] = duration.Milliseconds()

				if err != nil {
					completionFields["error"] = err.Error() // legacy
					completionFields["error.message"] = err.Error()
					completionFields["error.kind"] = classifyParallelTaskError(err)
					if p.verbose {
						logging.GetLogger().Info(taskCtx, "Parallel task failed", completionFields)
					} else {
						logging.GetLogger().Debug(taskCtx, "Parallel task failed", completionFields)
					}
				} else {
					completionFields["prompt_tokens"] = pred.Usage.PromptTokens
					completionFields["completion_tokens"] = pred.Usage.CompletionTokens
					completionFields["total_tokens"] = pred.Usage.TotalTokens
					completionFields["cost"] = pred.Usage.Cost

					completionFields["adapter_used"] = pred.AdapterUsed
					completionFields["parse_attempts"] = pred.ParseAttempts
					completionFields["fallback_used"] = pred.FallbackUsed
					completionFields["parse_success"] = pred.ParseSuccess

					if pred.ParseDiagnostics != nil {
						completionFields["missing_required_fields"] = firstNStrings(pred.ParseDiagnostics.MissingFields, 5)
						completionFields["missing_required_fields_count"] = len(pred.ParseDiagnostics.MissingFields)
						completionFields["invalid_fields_count"] = len(pred.ParseDiagnostics.TypeErrors) + len(pred.ParseDiagnostics.ClassErrors)
					}

					if p.verbose {
						logging.GetLogger().Info(taskCtx, "Parallel task completed", completionFields)
					} else {
						logging.GetLogger().Debug(taskCtx, "Parallel task completed", completionFields)
					}
				}

				results <- result{
					idx:  j.idx,
					pred: pred,
					err:  err,
					dur:  duration,
				}
			}
		})
	}

	// Feed jobs
	go func() {
		defer close(jobs)
		for i, in := range batch {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{idx: i, inputs: in, enqueuedAt: time.Now()}:
			}
		}
	}()

	// Close results when all workers done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	successes := make([]*core.Prediction, 0, len(batch))
	errs := make([]error, 0)
	perIdx := make([]*core.Prediction, len(batch))
	latencies := make([]time.Duration, len(batch))
	failureCount := 0

	for r := range results {
		if r.err != nil {
			failureCount++
			errs = append(errs, fmt.Errorf("task %d: %w", r.idx, r.err))
			if p.failFast || (p.maxFailures > 0 && failureCount > p.maxFailures) {
				cancel()
			}
		} else {
			perIdx[r.idx] = r.pred
			latencies[r.idx] = r.dur
			successes = append(successes, r.pred)
		}
	}

	// With fail-fast, any failure is an error (check before all-failed case)
	if p.failFast && failureCount > 0 {
		predErr = fmt.Errorf("parallel: fail-fast triggered by %d failure(s) (successes: %d/%d)", failureCount, len(successes), len(batch))
		return nil, predErr
	}

	// Evaluate outcome
	if len(successes) == 0 {
		predErr = fmt.Errorf("parallel: all %d/%d tasks failed: %v", failureCount, len(batch), firstNErrors(errs, 3))
		return nil, predErr
	}

	if p.maxFailures >= 0 && failureCount > p.maxFailures {
		predErr = fmt.Errorf("parallel: exceeded max failures %d/%d (successes: %d)", failureCount, len(batch), len(successes))
		return nil, predErr
	}

	// Aggregate usage
	totalUsage := core.Usage{}
	for _, s := range successes {
		totalUsage.TotalTokens += s.Usage.TotalTokens
		totalUsage.PromptTokens += s.Usage.PromptTokens
		totalUsage.CompletionTokens += s.Usage.CompletionTokens
		totalUsage.Cost += s.Usage.Cost
	}

	// Calculate metrics
	metrics := ParallelMetrics{
		Total:     len(batch),
		Successes: len(successes),
		Failures:  failureCount,
	}
	metrics.Latency = summarizeLatencies(latencies)

	// Batch summary log (when verbose is on, this is always INFO)
	batchCompletedFields := p.commonLogFields(parallelID, info, len(batch))
	batchCompletedFields["successes"] = len(successes)
	batchCompletedFields["failures"] = failureCount
	batchCompletedFields["latency_min_ms"] = metrics.Latency.MinMs
	batchCompletedFields["latency_max_ms"] = metrics.Latency.MaxMs
	batchCompletedFields["latency_avg_ms"] = metrics.Latency.AvgMs
	batchCompletedFields["latency_p50_ms"] = metrics.Latency.P50Ms
	batchCompletedFields["prompt_tokens"] = totalUsage.PromptTokens
	batchCompletedFields["completion_tokens"] = totalUsage.CompletionTokens
	batchCompletedFields["total_tokens"] = totalUsage.TotalTokens
	batchCompletedFields["cost"] = totalUsage.Cost
	batchCompletedFields["error_count"] = len(errs)
	batchCompletedFields["error_sample"] = firstNErrors(errs, 3)

	if p.verbose {
		logging.GetLogger().Info(ctx, "Parallel batch completed", batchCompletedFields)
	} else {
		logging.GetLogger().Debug(ctx, "Parallel batch completed", batchCompletedFields)
	}

	// Find first successful result for primary outputs
	var primary *core.Prediction
	for _, p := range perIdx {
		if p != nil {
			primary = p
			break
		}
	}

	if primary == nil {
		predErr = fmt.Errorf("parallel: no successful predictions")
		return nil, predErr
	}

	// Build final prediction
	prediction := core.NewPrediction(primary.Outputs).
		WithUsage(totalUsage).
		WithModuleName(logging.ModuleParallel).
		WithInputs(inputs)

	// Add completions if requested
	if p.returnAll {
		var completions []map[string]any
		for i := range perIdx {
			if perIdx[i] == nil {
				// Skip failures when onlySuccessful=true
				// Could add sentinel or error info when onlySuccessful=false
				continue
			}
			completions = append(completions, perIdx[i].Outputs)
		}
		prediction.Completions = completions
	}

	// Store metrics in outputs metadata (like adapter metadata)
	prediction.Outputs["__parallel_metrics"] = metrics
	if failureCount > 0 {
		prediction.Outputs["__parallel_errors"] = summarizeErrors(errs)
	}

	// Store parallel context metadata for programmatic access
	// Use captured info when available (factory case); otherwise use current info.
	contextInfo := info
	if p.factory != nil {
		// For factory-based parallels, use the info captured during first task
		contextInfo = p.moduleInfo
	}
	prediction.Outputs["__parallel_context"] = map[string]any{
		"inner_module": contextInfo.ModuleType,
		"lm_model":     contextInfo.LMModel,
		"total_tasks":  len(batch),
	}

	return prediction, nil
}

// expandInputs converts inputs into a slice of input maps
func (p *Parallel) expandInputs(inputs map[string]any) ([]map[string]any, error) {
	// Check for explicit batch
	if batchVal, ok := inputs[p.batchKey]; ok {
		batch, ok := batchVal.([]map[string]any)
		if !ok {
			return nil, fmt.Errorf("batch key %q must be []map[string]any, got %T", p.batchKey, batchVal)
		}
		return batch, nil
	}

	// Detect map-of-slices
	var sliceFields []string
	var sliceLength int
	for k, v := range inputs {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice {
			length := rv.Len()
			if sliceLength == 0 {
				sliceLength = length
			} else if length != sliceLength {
				return nil, fmt.Errorf("all slice fields must have equal length (found %d and %d for field %q)", sliceLength, length, k)
			}
			sliceFields = append(sliceFields, k)
		}
	}

	// If we found slices, zip them
	if len(sliceFields) > 0 {
		batch := make([]map[string]any, sliceLength)
		for i := 0; i < sliceLength; i++ {
			taskInputs := make(map[string]any)
			// Copy scalar fields
			for k, v := range inputs {
				rv := reflect.ValueOf(v)
				if rv.Kind() != reflect.Slice {
					taskInputs[k] = v
				}
			}
			// Extract slice elements
			for _, k := range sliceFields {
				rv := reflect.ValueOf(inputs[k])
				taskInputs[k] = rv.Index(i).Interface()
			}
			batch[i] = taskInputs
		}
		return batch, nil
	}

	// No batch, no slices - repeat if configured
	if p.repeat > 1 {
		batch := make([]map[string]any, p.repeat)
		for i := 0; i < p.repeat; i++ {
			// Deep copy inputs to prevent sharing
			taskInputs := make(map[string]any)
			maps.Copy(taskInputs, inputs)
			batch[i] = taskInputs
		}
		return batch, nil
	}

	// Single input
	return []map[string]any{inputs}, nil
}

// summarizeLatencies calculates min/max/avg/p50 from latencies
func summarizeLatencies(latencies []time.Duration) struct {
	MinMs int64 `json:"min_ms"`
	MaxMs int64 `json:"max_ms"`
	AvgMs int64 `json:"avg_ms"`
	P50Ms int64 `json:"p50_ms"`
} {
	summary := struct {
		MinMs int64 `json:"min_ms"`
		MaxMs int64 `json:"max_ms"`
		AvgMs int64 `json:"avg_ms"`
		P50Ms int64 `json:"p50_ms"`
	}{}

	if len(latencies) == 0 {
		return summary
	}

	// Filter out zero values (failed tasks)
	var valid []int64
	for _, d := range latencies {
		if d > 0 {
			valid = append(valid, d.Milliseconds())
		}
	}

	if len(valid) == 0 {
		return summary
	}

	slices.Sort(valid)

	summary.MinMs = valid[0]
	summary.MaxMs = valid[len(valid)-1]

	var sum int64
	for _, v := range valid {
		sum += v
	}
	summary.AvgMs = sum / int64(len(valid))

	// P50 (median)
	p50Idx := len(valid) / 2
	summary.P50Ms = valid[p50Idx]

	return summary
}

// firstNErrors returns the first n error messages
func firstNErrors(errs []error, n int) []string {
	count := min(n, len(errs))
	msgs := make([]string, count)
	for i := range count {
		msgs[i] = errs[i].Error()
	}
	return msgs
}

// summarizeErrors returns a summary string of errors
func summarizeErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	msgs := firstNErrors(errs, 5)
	summary := fmt.Sprintf("%d errors (showing first %d): %v", len(errs), len(msgs), msgs)
	return summary
}

// Clone creates an independent copy of Parallel module
func (p *Parallel) Clone() core.Module {
	cloned := &Parallel{
		factory:        p.factory,
		instances:      make([]core.Module, len(p.instances)),
		maxWorkers:     p.maxWorkers,
		maxFailures:    p.maxFailures,
		failFast:       p.failFast,
		returnAll:      p.returnAll,
		onlySuccessful: p.onlySuccessful,
		batchKey:       p.batchKey,
		repeat:         p.repeat,
		verbose:        p.verbose,
	}

	// Only clone module if it exists
	if p.module != nil {
		cloned.module = p.module.Clone()
	}

	// Clone instances if they exist
	for i, instance := range p.instances {
		if instance != nil {
			cloned.instances[i] = instance.Clone()
		}
	}

	return cloned
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
