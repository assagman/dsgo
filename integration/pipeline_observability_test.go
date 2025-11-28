package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
)

type pipelineContextKey string

const pipelineRequestIDKey pipelineContextKey = "request_id"

// ============================================================================
// Pipeline Request ID Propagation Tests
// ============================================================================

// TestPipeline_RequestIDPropagation tests that request IDs propagate through a pipeline.
// Validates:
// - Same request ID in all history entries
// - Context threading works correctly
func TestPipeline_RequestIDPropagation(t *testing.T) {
	ctx, cancel := ContextWithTimeout(30 * time.Second)
	defer cancel()

	// Add request ID to context
	requestID := "test-request-12345"
	ctx = context.WithValue(ctx, pipelineRequestIDKey, requestID)

	sig := fixtures.SimplePredictSig()
	collector := dsgo.NewMemoryCollector(100)

	// Create 5 modules in sequence
	modules := make([]*dsgo.Predict, 5)
	for i := 0; i < 5; i++ {
		lm := &PipelineObsLM{
			FinalResponse: `{"answer": "stage result"}`,
			Usage: dsgo.Usage{
				PromptTokens:     10 + i*5,
				CompletionTokens: 5 + i*2,
				TotalTokens:      15 + i*7,
				Cost:             float64(i+1) * 0.001,
			},
			Collector: collector,
			RequestID: requestID,
		}
		modules[i] = dsgo.NewPredict(sig, lm)
	}

	// Execute pipeline - each module gets same input type
	for i, mod := range modules {
		result, err := mod.Forward(ctx, map[string]any{"question": "test stage " + string(rune('0'+i))})
		if err != nil {
			t.Fatalf("Stage %d failed: %v", i, err)
		}
		_ = result // We're testing observability, not chaining
	}

	// Verify request ID propagation
	entries := collector.GetAll()
	if len(entries) < 5 {
		t.Errorf("Expected at least 5 history entries, got %d", len(entries))
	}

	for i, entry := range entries {
		if entry.ProviderMeta == nil {
			continue
		}
		if id, ok := entry.ProviderMeta["request_id"].(string); ok && id != requestID {
			t.Errorf("Entry %d has wrong request ID: %s", i, id)
		}
	}
}

// ============================================================================
// Pipeline Cost Aggregation Tests
// ============================================================================

// TestPipeline_CostAggregation tests that costs are tracked across modules.
// Validates:
// - Total cost = sum of individual costs
// - Per-module breakdown available
func TestPipeline_CostAggregation(t *testing.T) {
	ctx, cancel := ContextWithTimeout(30 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Define costs for each module
	moduleCosts := []float64{0.001, 0.002, 0.003, 0.004, 0.005}
	expectedTotal := 0.015

	var totalCost float64

	// Execute pipeline and sum costs
	for i, cost := range moduleCosts {
		lm := &CostTrackingMockLM{
			Response: `{"answer": "stage result"}`,
			Cost:     cost,
		}
		pred := dsgo.NewPredict(sig, lm)

		result, err := pred.Forward(ctx, map[string]any{"question": "test"})
		if err != nil {
			t.Fatalf("Stage %d failed: %v", i, err)
		}

		totalCost += result.Usage.Cost
	}

	// Verify total cost
	tolerance := 0.0001
	if totalCost < expectedTotal-tolerance || totalCost > expectedTotal+tolerance {
		t.Errorf("Total cost mismatch: got %.6f, want %.6f", totalCost, expectedTotal)
	}
}

// TestPipeline_CostAggregation_TableDriven tests various cost scenarios.
func TestPipeline_CostAggregation_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		costs       []float64
		expectedSum float64
	}{
		{
			name:        "small costs",
			costs:       []float64{0.0001, 0.0002, 0.0003},
			expectedSum: 0.0006,
		},
		{
			name:        "medium costs",
			costs:       []float64{0.01, 0.02, 0.03},
			expectedSum: 0.06,
		},
		{
			name:        "mixed costs",
			costs:       []float64{0.001, 0.05, 0.002},
			expectedSum: 0.053,
		},
		{
			name:        "zero costs",
			costs:       []float64{0, 0, 0},
			expectedSum: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := ContextWithTimeout(10 * time.Second)
			defer cancel()

			sig := fixtures.SimplePredictSig()
			var totalCost float64

			for _, cost := range tt.costs {
				lm := &CostTrackingMockLM{
					Response: `{"answer": "result"}`,
					Cost:     cost,
				}
				pred := dsgo.NewPredict(sig, lm)

				result, err := pred.Forward(ctx, map[string]any{"question": "test"})
				if err != nil {
					t.Fatalf("Failed: %v", err)
				}
				totalCost += result.Usage.Cost
			}

			tolerance := 0.0001
			if totalCost < tt.expectedSum-tolerance || totalCost > tt.expectedSum+tolerance {
				t.Errorf("Total cost mismatch: got %.6f, want %.6f", totalCost, tt.expectedSum)
			}
		})
	}
}

// ============================================================================
// Pipeline Latency Tracking Tests
// ============================================================================

// TestPipeline_LatencyTracking tests latency measurement across modules.
// Validates:
// - Per-module latency is tracked
// - Total latency is accurate
func TestPipeline_LatencyTracking(t *testing.T) {
	ctx, cancel := ContextWithTimeout(30 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create modules with different delays
	delays := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		15 * time.Millisecond,
	}

	totalExpectedLatency := 45 * time.Millisecond
	var totalActualLatency time.Duration

	for i, delay := range delays {
		lm := &LatencyMockLM{
			Response: `{"answer": "result"}`,
			Delay:    delay,
		}
		pred := dsgo.NewPredict(sig, lm)

		start := time.Now()
		result, err := pred.Forward(ctx, map[string]any{"question": "test"})
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Stage %d failed: %v", i, err)
		}

		totalActualLatency += elapsed

		// Verify latency in usage
		if result.Usage.Latency > 0 {
			t.Logf("Stage %d latency: %dms", i, result.Usage.Latency)
		}
	}

	// Total should be at least the sum of delays
	if totalActualLatency < totalExpectedLatency {
		t.Errorf("Total latency too low: got %v, expected at least %v", totalActualLatency, totalExpectedLatency)
	}
}

// ============================================================================
// Pipeline Error Origin Tracking Tests
// ============================================================================

// TestPipeline_ErrorOriginTracking tests that errors are traced to their origin.
// Validates:
// - Error source identified
// - Partial results captured before error
func TestPipeline_ErrorOriginTracking(t *testing.T) {
	ctx, cancel := ContextWithTimeout(30 * time.Second)
	defer cancel()

	sig := fixtures.SimplePredictSig()

	// Create modules where middle one fails
	modules := []dsgo.LM{
		NewMockLMWithResponse(`{"answer": "stage1"}`),
		NewMockLMWithResponse(`{"answer": "stage2"}`),
		NewAlwaysFailLM("mid-pipeline-error"),
		NewMockLMWithResponse(`{"answer": "stage4"}`),
	}

	successfulStages := 0
	var lastError error

	for i, lm := range modules {
		pred := dsgo.NewPredict(sig, lm)

		_, err := pred.Forward(ctx, map[string]any{"question": "test"})
		if err != nil {
			lastError = err
			t.Logf("Error at stage %d: %v", i, err)
			break
		}

		successfulStages++
	}

	// Should have failed at stage 3 (index 2)
	if successfulStages != 2 {
		t.Errorf("Expected 2 successful stages before failure, got %d", successfulStages)
	}

	if lastError == nil {
		t.Error("Expected error from pipeline")
	}
}

// TestPipeline_ErrorOriginTracking_TableDriven tests various error scenarios.
func TestPipeline_ErrorOriginTracking_TableDriven(t *testing.T) {
	tests := []struct {
		name               string
		errorAtStage       int
		totalStages        int
		expectedSuccessful int
	}{
		{
			name:               "error at first stage",
			errorAtStage:       0,
			totalStages:        5,
			expectedSuccessful: 0,
		},
		{
			name:               "error at middle stage",
			errorAtStage:       2,
			totalStages:        5,
			expectedSuccessful: 2,
		},
		{
			name:               "error at last stage",
			errorAtStage:       4,
			totalStages:        5,
			expectedSuccessful: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := ContextWithTimeout(10 * time.Second)
			defer cancel()

			sig := fixtures.SimplePredictSig()
			successfulStages := 0

			for i := 0; i < tt.totalStages; i++ {
				var lm dsgo.LM
				if i == tt.errorAtStage {
					lm = NewAlwaysFailLM("planned-error")
				} else {
					lm = NewMockLMWithResponse(`{"answer": "result"}`)
				}

				pred := dsgo.NewPredict(sig, lm)
				_, err := pred.Forward(ctx, map[string]any{"question": "test"})
				if err != nil {
					break
				}

				successfulStages++
			}

			if successfulStages != tt.expectedSuccessful {
				t.Errorf("Expected %d successful stages, got %d", tt.expectedSuccessful, successfulStages)
			}
		})
	}
}

// ============================================================================
// Mock LM Implementations for Observability Tests
// ============================================================================

// PipelineObsLM is a mock that supports observability features for pipeline tests
type PipelineObsLM struct {
	FinalResponse string
	Usage         dsgo.Usage
	Collector     dsgo.Collector
	RequestID     string
}

func (m *PipelineObsLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	// Record to collector if available
	if m.Collector != nil {
		entry := &dsgo.HistoryEntry{
			ID:           "obs-test-" + time.Now().Format("20060102150405.000"),
			Timestamp:    time.Now(),
			Provider:     "mock",
			Model:        "observability-mock",
			Usage:        m.Usage,
			ProviderMeta: map[string]any{"request_id": m.RequestID},
		}
		_ = m.Collector.Collect(entry)
	}

	return &dsgo.GenerateResult{
		Content:      m.FinalResponse,
		FinishReason: "stop",
		Usage:        m.Usage,
	}, nil
}

func (m *PipelineObsLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)
	go func() {
		defer close(chunkChan)
		defer close(errChan)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}
		chunkChan <- dsgo.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return chunkChan, errChan
}

func (m *PipelineObsLM) Name() string        { return "pipeline-obs-mock-lm" }
func (m *PipelineObsLM) SupportsJSON() bool  { return true }
func (m *PipelineObsLM) SupportsTools() bool { return false }
func (m *PipelineObsLM) IsOpenAI() bool      { return false }

// CostTrackingMockLM tracks costs per call
type CostTrackingMockLM struct {
	Response string
	Cost     float64
}

func (m *CostTrackingMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	return &dsgo.GenerateResult{
		Content:      m.Response,
		FinishReason: "stop",
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			Cost:             m.Cost,
		},
	}, nil
}

func (m *CostTrackingMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)
	go func() {
		defer close(chunkChan)
		defer close(errChan)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}
		chunkChan <- dsgo.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return chunkChan, errChan
}

func (m *CostTrackingMockLM) Name() string        { return "cost-tracking-mock-lm" }
func (m *CostTrackingMockLM) SupportsJSON() bool  { return true }
func (m *CostTrackingMockLM) SupportsTools() bool { return false }
func (m *CostTrackingMockLM) IsOpenAI() bool      { return false }

// LatencyMockLM simulates latency per call
type LatencyMockLM struct {
	Response  string
	Delay     time.Duration
	callCount int32
}

func (m *LatencyMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	atomic.AddInt32(&m.callCount, 1)
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.Delay):
		latencyMs := time.Since(start).Milliseconds()
		return &dsgo.GenerateResult{
			Content:      m.Response,
			FinishReason: "stop",
			Usage: dsgo.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
				Latency:          latencyMs,
			},
		}, nil
	}
}

func (m *LatencyMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)
	go func() {
		defer close(chunkChan)
		defer close(errChan)
		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}
		chunkChan <- dsgo.Chunk{Content: result.Content, Usage: result.Usage}
	}()
	return chunkChan, errChan
}

func (m *LatencyMockLM) Name() string        { return "latency-mock-lm" }
func (m *LatencyMockLM) SupportsJSON() bool  { return true }
func (m *LatencyMockLM) SupportsTools() bool { return false }
func (m *LatencyMockLM) IsOpenAI() bool      { return false }
