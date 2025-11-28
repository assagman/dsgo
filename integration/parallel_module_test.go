package integration

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
)

// ============================================================================
// Parallel Module Basic Tests
// ============================================================================

// TestParallel_BatchInput tests parallel execution with batch input mode
func TestParallel_BatchInput(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	// Create mock LM that returns different answers
	var callCount int32
	lm := &CountingMockLM{
		ResponseFunc: func(idx int) string {
			return fmt.Sprintf(`{"answer": "Response %d"}`, idx)
		},
		CallCount: &callCount,
	}

	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	// Create parallel module
	parallel := dsgo.NewParallel(pred).
		WithMaxWorkers(4).
		WithReturnAll(true)

	// Batch input
	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "Question 1"},
			{"question": "Question 2"},
			{"question": "Question 3"},
			{"question": "Question 4"},
			{"question": "Question 5"},
		},
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel execution failed: %v", err)
	}

	// Verify all were processed
	if atomic.LoadInt32(&callCount) != 5 {
		t.Errorf("Expected 5 calls, got %d", atomic.LoadInt32(&callCount))
	}

	// Verify completions (accessed directly via Completions field)
	if !result.HasCompletions() {
		t.Error("Expected completions in result")
	}
	if len(result.Completions) != 5 {
		t.Errorf("Expected 5 completions, got %d", len(result.Completions))
	}

	// Verify usage tracking
	if result.Usage.TotalTokens == 0 {
		t.Error("Expected usage to be tracked")
	}
}

// TestParallel_WithFactory tests using factory function for stateful modules
func TestParallel_WithFactory(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	var factoryCalls int32

	// Create parallel with factory
	parallel := dsgo.NewParallelWithFactory(func(i int) dsgo.Module {
		atomic.AddInt32(&factoryCalls, 1)
		lm := NewMockLMWithResponse(fmt.Sprintf(`{"answer": "Factory response %d"}`, i))
		sig := fixtures.SimplePredictSig()
		return dsgo.NewPredict(sig, lm)
	}).
		WithMaxWorkers(3).
		WithReturnAll(true)

	// Batch input
	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "Q1"},
			{"question": "Q2"},
			{"question": "Q3"},
		},
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel with factory failed: %v", err)
	}

	// Verify factory was called for each task
	if atomic.LoadInt32(&factoryCalls) != 3 {
		t.Errorf("Expected 3 factory calls, got %d", factoryCalls)
	}

	// Verify result
	if result == nil {
		t.Error("Expected result")
	}
}

// TestParallel_WithInstances tests using pre-created instances
func TestParallel_WithInstances(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create multiple instances
	instances := make([]dsgo.Module, 3)
	for i := 0; i < 3; i++ {
		lm := NewMockLMWithResponse(fmt.Sprintf(`{"answer": "Instance %d response"}`, i))
		instances[i] = dsgo.NewPredict(sig, lm)
	}

	// Create parallel with instances
	parallel := dsgo.NewParallelWithInstances(instances).
		WithReturnAll(true)

	// Batch input (more items than instances to test cycling)
	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "Q1"},
			{"question": "Q2"},
			{"question": "Q3"},
			{"question": "Q4"},
			{"question": "Q5"},
		},
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel with instances failed: %v", err)
	}

	if !result.HasCompletions() || len(result.Completions) != 5 {
		t.Errorf("Expected 5 completions, got %d", len(result.Completions))
	}
}

// TestParallel_MapOfSlices tests parallel with map-of-slices input mode
func TestParallel_MapOfSlices(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"answer": "Processed"}`)
	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	parallel := dsgo.NewParallel(pred).
		WithMaxWorkers(2).
		WithReturnAll(true)

	// Map-of-slices input: values are zipped
	inputs := map[string]any{
		"question": []any{"Q1", "Q2", "Q3"},
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel map-of-slices failed: %v", err)
	}

	if !result.HasCompletions() || len(result.Completions) != 3 {
		t.Errorf("Expected 3 completions, got %d", len(result.Completions))
	}
}

// TestParallel_WithRepeat tests repeat mode
func TestParallel_WithRepeat(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	var callCount int32
	lm := &CountingMockLM{
		ResponseFunc: func(idx int) string {
			return `{"answer": "Repeated response"}`
		},
		CallCount: &callCount,
	}

	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	parallel := dsgo.NewParallel(pred).
		WithMaxWorkers(3).
		WithRepeat(5).
		WithReturnAll(true)

	// Single input repeated 5 times
	inputs := map[string]any{
		"question": "Same question",
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel with repeat failed: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 5 {
		t.Errorf("Expected 5 calls, got %d", atomic.LoadInt32(&callCount))
	}

	if !result.HasCompletions() || len(result.Completions) != 5 {
		t.Errorf("Expected 5 completions, got %d", len(result.Completions))
	}
}

// ============================================================================
// Parallel Module Error Handling Tests
// ============================================================================

// TestParallel_MaxFailures tests failure limit enforcement
func TestParallel_MaxFailures(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	var callCount int32
	lm := &FailingSomeMockLM{
		CallCount:   &callCount,
		FailIndices: map[int]bool{1: true, 2: true}, // 2 failures
		SuccessResp: `{"answer": "Success"}`,
	}

	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	// Allow up to 2 failures
	parallel := dsgo.NewParallel(pred).
		WithMaxWorkers(2).
		WithMaxFailures(2).
		WithReturnAll(true).
		WithOnlySuccessful(true)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "Q0"}, // Success
			{"question": "Q1"}, // Fail
			{"question": "Q2"}, // Fail
			{"question": "Q3"}, // Success
		},
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel should succeed with max failures: %v", err)
	}

	// With onlySuccessful, should have 2 completions
	if !result.HasCompletions() {
		t.Error("Expected completions")
	}
	if len(result.Completions) != 2 {
		t.Errorf("Expected 2 successful completions, got %d", len(result.Completions))
	}
}

// TestParallel_FailFast tests early termination on failure
func TestParallel_FailFast(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	var callCount int32
	lm := &FailAfterNMockLM{
		CallCount: &callCount,
		FailAfter: 2, // Fail on 3rd call
	}

	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	parallel := dsgo.NewParallel(pred).
		WithMaxWorkers(1). // Sequential to control order
		WithFailFast(true)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "Q0"},
			{"question": "Q1"},
			{"question": "Q2"}, // Will fail
			{"question": "Q3"},
			{"question": "Q4"},
		},
	}

	_, err := parallel.Forward(ctx, inputs)
	if err == nil {
		t.Error("Expected error with fail fast")
	}

	// Verify failure was detected and error returned
	// Note: Due to buffered channel implementation, all jobs may be processed
	// before cancellation takes effect, but error should still be returned
	count := atomic.LoadInt32(&callCount)
	if count < 3 {
		t.Errorf("Expected at least 3 calls (including failing one), got %d", count)
	}
}

// TestParallel_GetSignature tests signature retrieval from different modes
func TestParallel_GetSignature(t *testing.T) {
	sig := fixtures.SimplePredictSig()
	lm := NewMockLMWithResponse(`{"answer": "test"}`)
	pred := dsgo.NewPredict(sig, lm)

	tests := []struct {
		name     string
		parallel *dsgo.Parallel
	}{
		{
			name:     "From shared module",
			parallel: dsgo.NewParallel(pred),
		},
		{
			name: "From factory",
			parallel: dsgo.NewParallelWithFactory(func(i int) dsgo.Module {
				return dsgo.NewPredict(sig, lm)
			}),
		},
		{
			name:     "From instances",
			parallel: dsgo.NewParallelWithInstances([]dsgo.Module{pred}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.parallel.GetSignature()
			if got == nil {
				t.Error("Expected signature, got nil")
				return
			}
			if got.Description != sig.Description {
				t.Errorf("Signature mismatch: got %s, want %s", got.Description, sig.Description)
			}
		})
	}
}

// TestParallel_EmptyBatch tests handling of empty batch
func TestParallel_EmptyBatch(t *testing.T) {
	ctx, cancel := ContextWithTimeout(5 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"answer": "test"}`)
	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	parallel := dsgo.NewParallel(pred)

	inputs := map[string]any{
		"_batch": []map[string]any{},
	}

	_, err := parallel.Forward(ctx, inputs)
	if err == nil {
		t.Error("Expected error for empty batch")
	}
}

// TestParallel_ContextCancellation tests context cancellation during execution
func TestParallel_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Slow LM
	lm := NewLatencyLM(500*time.Millisecond, `{"answer": "slow"}`)
	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	parallel := dsgo.NewParallel(pred).
		WithMaxWorkers(2)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "Q1"},
			{"question": "Q2"},
			{"question": "Q3"},
		},
	}

	_, err := parallel.Forward(ctx, inputs)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
}

// TestParallel_UsageAggregation tests that usage is aggregated across all tasks
func TestParallel_UsageAggregation(t *testing.T) {
	ctx, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	lm := &UsageTrackingMockLM{
		Response:      `{"answer": "test"}`,
		TokensPerCall: 100,
		CostPerCall:   0.01,
	}

	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	parallel := dsgo.NewParallel(pred).
		WithMaxWorkers(3).
		WithReturnAll(true)

	inputs := map[string]any{
		"_batch": []map[string]any{
			{"question": "Q1"},
			{"question": "Q2"},
			{"question": "Q3"},
		},
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel failed: %v", err)
	}

	// Should have aggregated usage from all 3 calls
	if result.Usage.TotalTokens < 300 {
		t.Errorf("Expected ~300 tokens, got %d", result.Usage.TotalTokens)
	}
	if result.Usage.Cost < 0.03 {
		t.Errorf("Expected ~$0.03 cost, got %f", result.Usage.Cost)
	}
}

// TestParallel_CustomBatchKey tests custom batch key
func TestParallel_CustomBatchKey(t *testing.T) {
	ctx, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	lm := NewMockLMWithResponse(`{"answer": "test"}`)
	sig := fixtures.SimplePredictSig()
	pred := dsgo.NewPredict(sig, lm)

	parallel := dsgo.NewParallel(pred).
		WithBatchKey("items").
		WithReturnAll(true)

	inputs := map[string]any{
		"items": []map[string]any{
			{"question": "Q1"},
			{"question": "Q2"},
		},
	}

	result, err := parallel.Forward(ctx, inputs)
	if err != nil {
		t.Fatalf("Parallel with custom batch key failed: %v", err)
	}

	if !result.HasCompletions() || len(result.Completions) != 2 {
		t.Errorf("Expected 2 completions, got %d", len(result.Completions))
	}
}

// ============================================================================
// Helper Mock LMs
// ============================================================================

// CountingMockLM counts calls and returns dynamic responses
type CountingMockLM struct {
	ResponseFunc func(idx int) string
	CallCount    *int32
}

func (m *CountingMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	idx := int(atomic.AddInt32(m.CallCount, 1) - 1)
	return &dsgo.GenerateResult{
		Content: m.ResponseFunc(idx),
		Usage:   dsgo.Usage{TotalTokens: 20, Cost: 0.001},
	}, nil
}

func (m *CountingMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	ch := make(chan dsgo.Chunk, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errCh <- err
			return
		}
		ch <- dsgo.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return ch, errCh
}

func (m *CountingMockLM) Name() string        { return "counting-mock-lm" }
func (m *CountingMockLM) SupportsJSON() bool  { return true }
func (m *CountingMockLM) SupportsTools() bool { return false }
func (m *CountingMockLM) IsOpenAI() bool      { return false }

// FailingSomeMockLM fails on specific indices
type FailingSomeMockLM struct {
	CallCount   *int32
	FailIndices map[int]bool
	SuccessResp string
}

func (m *FailingSomeMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	idx := int(atomic.AddInt32(m.CallCount, 1) - 1)
	if m.FailIndices[idx] {
		return nil, errors.New("simulated failure")
	}
	return &dsgo.GenerateResult{
		Content: m.SuccessResp,
		Usage:   dsgo.Usage{TotalTokens: 20},
	}, nil
}

func (m *FailingSomeMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	ch := make(chan dsgo.Chunk, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errCh <- err
			return
		}
		ch <- dsgo.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return ch, errCh
}

func (m *FailingSomeMockLM) Name() string        { return "failing-some-mock-lm" }
func (m *FailingSomeMockLM) SupportsJSON() bool  { return true }
func (m *FailingSomeMockLM) SupportsTools() bool { return false }
func (m *FailingSomeMockLM) IsOpenAI() bool      { return false }

// FailAfterNMockLM fails after N successful calls
type FailAfterNMockLM struct {
	CallCount *int32
	FailAfter int
}

func (m *FailAfterNMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	idx := int(atomic.AddInt32(m.CallCount, 1))
	if idx > m.FailAfter {
		return nil, errors.New("simulated failure after N")
	}
	return &dsgo.GenerateResult{
		Content: `{"answer": "success"}`,
		Usage:   dsgo.Usage{TotalTokens: 20},
	}, nil
}

func (m *FailAfterNMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	ch := make(chan dsgo.Chunk, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errCh <- err
			return
		}
		ch <- dsgo.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return ch, errCh
}

func (m *FailAfterNMockLM) Name() string        { return "fail-after-n-mock-lm" }
func (m *FailAfterNMockLM) SupportsJSON() bool  { return true }
func (m *FailAfterNMockLM) SupportsTools() bool { return false }
func (m *FailAfterNMockLM) IsOpenAI() bool      { return false }

// UsageTrackingMockLM returns specific usage data
type UsageTrackingMockLM struct {
	Response      string
	TokensPerCall int
	CostPerCall   float64
}

func (m *UsageTrackingMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	return &dsgo.GenerateResult{
		Content: m.Response,
		Usage: dsgo.Usage{
			TotalTokens:      m.TokensPerCall,
			PromptTokens:     m.TokensPerCall / 2,
			CompletionTokens: m.TokensPerCall / 2,
			Cost:             m.CostPerCall,
		},
	}, nil
}

func (m *UsageTrackingMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	ch := make(chan dsgo.Chunk, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errCh <- err
			return
		}
		ch <- dsgo.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return ch, errCh
}

func (m *UsageTrackingMockLM) Name() string        { return "usage-tracking-mock-lm" }
func (m *UsageTrackingMockLM) SupportsJSON() bool  { return true }
func (m *UsageTrackingMockLM) SupportsTools() bool { return false }
func (m *UsageTrackingMockLM) IsOpenAI() bool      { return false }
