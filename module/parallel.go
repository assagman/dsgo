package module

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/logging"
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

// Parallel executes a module across multiple inputs concurrently.
//
// IMPORTANT: When using a single shared module instance with WithMaxWorkers > 1,
// ensure the module is stateless. Modules that maintain internal state
// (e.g., Predict with History) MUST use either NewParallelWithFactory or
// NewParallelWithInstances to provide isolated instances per task/worker.
//
// Safe parallel usage patterns:
//   - Use stateless modules (no internal state mutation)
//   - Create N independent instances via factory function
//   - Provide pre-created instances array
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
}

// NewParallel creates a Parallel module with a shared module instance.
// WARNING: Only use with stateless modules. For modules with state (e.g., History),
// use NewParallelWithFactory or NewParallelWithInstances instead.
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
	startTime := time.Now()
	logging.LogPredictionStart(ctx, "Parallel", "Parallel execution")

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, "Parallel", time.Since(startTime), predErr)
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

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Job and result types
	type job struct {
		idx    int
		inputs map[string]any
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
		return p.module
	}

	// Start workers
	workers := min(p.maxWorkers, len(batch))
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				mod := getModule(j.idx)
				pred, err := mod.Forward(ctx, j.inputs)
				results <- result{
					idx:  j.idx,
					pred: pred,
					err:  err,
					dur:  time.Since(start),
				}
			}
		}()
	}

	// Feed jobs
	go func() {
		defer close(jobs)
		for i, in := range batch {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{idx: i, inputs: in}:
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

	// Evaluate outcome
	if len(successes) == 0 {
		predErr = fmt.Errorf("parallel: all %d/%d tasks failed: %v", failureCount, len(batch), firstNErrors(errs, 3))
		return nil, predErr
	}

	// With fail-fast, any failure is an error
	if p.failFast && failureCount > 0 {
		predErr = fmt.Errorf("parallel: fail-fast triggered by %d failure(s) (successes: %d/%d)", failureCount, len(successes), len(batch))
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
		WithModuleName("Parallel").
		WithInputs(inputs)

	// Add completions if requested
	if p.returnAll {
		var completions []map[string]any
		for i := 0; i < len(perIdx); i++ {
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
			for k, v := range inputs {
				taskInputs[k] = v
			}
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

	sort.Slice(valid, func(i, j int) bool { return valid[i] < valid[j] })

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
	for i := 0; i < count; i++ {
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
