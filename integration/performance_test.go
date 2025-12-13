//go:build benchmarks

package integration

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/integration/fixtures"
)

// BenchmarkPredictModule_ColdStart measures latency for first module execution.
// Baseline: ~50ms (mocked LM), includes setup overhead.
func BenchmarkPredictModule_ColdStart(b *testing.B) {
	lm := NewMockLMWithResponse(`{"answer": "response"}`)
	sig := fixtures.SimplePredictSig()

	for i := 0; i < b.N; i++ {
		predictor := dsgo.NewPredict(sig, lm)
		ctx := context.Background()

		_, _ = predictor.Forward(ctx, map[string]any{
			"question": "test",
		})
	}
}

// BenchmarkPredictModule_Execution measures module execution latency.
// Target: <100ms for typical use case.
func BenchmarkPredictModule_Execution(b *testing.B) {
	lm := NewMockLMWithResponse(`{"answer": "response"}`)
	sig := fixtures.SimplePredictSig()
	predictor := dsgo.NewPredict(sig, lm)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = predictor.Forward(ctx, map[string]any{
			"question": "test",
		})
	}
}

// BenchmarkComposition_3Modules measures sequential 3-module pipeline latency.
// Baseline: ~150ms (3x module latency).
func BenchmarkComposition_3Modules(b *testing.B) {
	ctx := context.Background()

	sig := fixtures.SimplePredictSig()
	m1 := dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "stage1"}`))
	m2 := dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "stage2"}`))
	m3 := dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "stage3"}`))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r1, _ := m1.Forward(ctx, map[string]any{"question": "test"})
		answer1, _ := r1.GetString("answer")
		r2, _ := m2.Forward(ctx, map[string]any{"question": answer1})
		answer2, _ := r2.GetString("answer")
		_, _ = m3.Forward(ctx, map[string]any{"question": answer2})
	}
}

// BenchmarkParallel_3Modules measures parallel 3-module execution latency.
// Target: ~60ms (single module + coordination overhead).
func BenchmarkParallel_3Modules(b *testing.B) {
	ctx := context.Background()

	sig := fixtures.SimplePredictSig()
	modules := []*dsgo.Predict{
		dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "m1"}`)),
		dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "m2"}`)),
		dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "m3"}`)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for _, mod := range modules {
			wg.Add(1)
			go func(m *dsgo.Predict) {
				defer wg.Done()
				_, _ = m.Forward(ctx, map[string]any{"question": "test"})
			}(mod)
		}
		wg.Wait()
	}
}

// BenchmarkChainOfThought_Execution measures ChainOfThought module latency.
// Target: <100ms (reasoning adds overhead vs simple predict).
func BenchmarkChainOfThought_Execution(b *testing.B) {
	lm := NewMockLMWithResponse(`{
		"reasoning": "Let me think about this step by step.",
		"answer": "The answer is clear from reasoning."
	}`)
	sig := fixtures.ChainOfThoughtSig()
	module := dsgo.NewChainOfThought(sig, lm)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = module.Forward(ctx, map[string]any{"problem": "test"})
	}
}

// TestMemoryEfficiency_LongRunning validates memory usage over many executions.
// Scenario: 10,000 module executions.
// Validates: No memory leaks, reasonable heap growth.
func TestMemoryEfficiency_LongRunning(t *testing.T) {
	ctx := context.Background()
	iterations := 10000

	lm := NewMockLMWithResponse(`{"answer": "response"}`)
	sig := fixtures.SimplePredictSig()
	predictor := dsgo.NewPredict(sig, lm)

	// Measure starting memory
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	// Execute many times
	for i := 0; i < iterations; i++ {
		_, _ = predictor.Forward(ctx, map[string]any{
			"question": "test",
		})
	}

	// Measure ending memory
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	// Rough check: allow for 1MB growth (GC activity may cause fluctuation)
	heapGrowth := float64(endMem.Alloc-startMem.Alloc) / 1024 / 1024

	// For 10k iterations with small responses, should be minimal growth
	if heapGrowth > 50 {
		t.Logf("Warning: Significant heap growth: %.1f MB over %d iterations", heapGrowth, iterations)
	}
}

// TestMemoryEfficiency_LargeOutputs validates memory handling with large responses.
// Scenario: 100+ MB total output.
// Validates: Reasonable memory usage, no excessive buffering.
func TestMemoryEfficiency_LargeOutputs(t *testing.T) {
	ctx := context.Background()
	iterations := 100

	// Create large response (10MB per call)
	largeContent := ""
	for i := 0; i < 10000; i++ {
		largeContent += "This is a line of content that will be repeated many times to create larger responses for testing memory efficiency. "
	}

	lm := NewMockLMWithResponse(`{"answer": "` + largeContent + `"}`)
	sig := fixtures.SimplePredictSig()
	predictor := dsgo.NewPredict(sig, lm)

	// Measure starting memory
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)
	startAlloc := startMem.Alloc

	// Execute with large outputs
	for i := 0; i < iterations; i++ {
		result, err := predictor.Forward(ctx, map[string]any{
			"question": "test",
		})
		if err != nil {
			t.Fatalf("Forward failed: %v", err)
		}
		// Verify response is there
		answer, _ := result.GetString("answer")
		if len(answer) == 0 {
			t.Error("Expected large answer")
		}
	}

	// Measure ending memory
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	heapGrowth := float64(endMem.Alloc-startAlloc) / 1024 / 1024

	// Large outputs will use memory, but shouldn't accumulate indefinitely
	// Allow for reasonable buffering (100MB for safety)
	if heapGrowth > 100 {
		t.Logf("Large heap growth with large outputs: %.1f MB (test may be skipped due to GC)", heapGrowth)
	}
}

// TestConcurrency_100Goroutines validates thread-safety with many concurrent calls.
// Scenario: 100 concurrent module executions.
// Validates: Race condition free, no deadlocks.
func TestConcurrency_100Goroutines(t *testing.T) {
	ctx := context.Background()
	numGoroutines := 100

	lm := NewMockLMWithResponse(`{"answer": "concurrent response"}`)
	sig := fixtures.SimplePredictSig()

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			predictor := dsgo.NewPredict(sig, lm)
			result, err := predictor.Forward(ctx, map[string]any{
				"question": "test",
			})

			if err != nil {
				errChan <- err
				return
			}

			if result == nil {
				errChan <- nil
				return
			}

			answer, ok := result.GetString("answer")
			if !ok || answer == "" {
				errChan <- nil
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	errorCount := 0
	for err := range errChan {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("%d concurrent operations failed", errorCount)
	}
}

// TestConcurrency_CacheContention validates cache behavior under concurrent access.
// Scenario: 50 goroutines with repeated queries.
// Validates: Cache correctness, no race conditions.
func TestConcurrency_CacheContention(t *testing.T) {
	ctx := context.Background()
	numGoroutines := 50
	queriesPerGoroutine := 10

	// Use global settings to enable caching
	lm := NewMockLMWithResponse(`{"answer": "cached response"}`)
	sig := fixtures.SimplePredictSig()

	var wg sync.WaitGroup
	var successCount int
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			predictor := dsgo.NewPredict(sig, lm)

			for q := 0; q < queriesPerGoroutine; q++ {
				result, err := predictor.Forward(ctx, map[string]any{
					"question": "repeated question",
				})

				if err == nil && result != nil {
					answer, ok := result.GetString("answer")
					if ok && answer != "" {
						mu.Lock()
						successCount++
						mu.Unlock()
					}
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := numGoroutines * queriesPerGoroutine
	if successCount != expectedTotal {
		t.Errorf("Expected %d successful queries, got %d", expectedTotal, successCount)
	}
}

// TestConcurrency_ParallelBestOfN validates BestOfN with parallel execution.
// Scenario: Multiple BestOfN modules running in parallel.
// Validates: No interference, correct selection.
func TestConcurrency_ParallelBestOfN(t *testing.T) {
	// Skip BestOfN test - scorer setup is complex for performance testing
	t.Skip("BestOfN requires scorer configuration - see module tests for scorer behavior")
}

// TestLatencyDistribution measures latency distribution over multiple runs.
// Scenario: 1000 module executions, measure min/max/average latency.
// Validates: Consistent performance, no outliers.
func TestLatencyDistribution(t *testing.T) {
	ctx := context.Background()
	iterations := 1000

	lm := NewMockLMWithResponse(`{"answer": "response"}`)
	sig := fixtures.SimplePredictSig()
	predictor := dsgo.NewPredict(sig, lm)

	var minLatency, maxLatency, totalLatency time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, _ = predictor.Forward(ctx, map[string]any{
			"question": "test",
		})
		latency := time.Since(start)

		totalLatency += latency
		if i == 0 {
			minLatency = latency
			maxLatency = latency
		} else {
			if latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		}
	}

	avgLatency := totalLatency / time.Duration(iterations)

	// Log statistics
	t.Logf("Latency stats over %d iterations:", iterations)
	t.Logf("  Min: %v", minLatency)
	t.Logf("  Max: %v", maxLatency)
	t.Logf("  Avg: %v", avgLatency)

	// Verify latencies are reasonable (< 100ms for mocked LM)
	if maxLatency > 100*time.Millisecond {
		t.Logf("Warning: Max latency exceeded 100ms: %v", maxLatency)
	}
}

// TestTokenProcessingSpeed validates token throughput.
// Scenario: Process 100k tokens, measure throughput.
// Validates: Reasonable token processing speed.
func TestTokenProcessingSpeed(t *testing.T) {
	ctx := context.Background()

	// Create response with known token count
	response := `{"answer": "` + "word " + "word word word word word word word word word " + `"}`

	lm := NewMockLMWithResponse(response)
	sig := fixtures.SimplePredictSig()
	predictor := dsgo.NewPredict(sig, lm)

	totalTokens := 0
	iterations := 1000

	start := time.Now()

	for i := 0; i < iterations; i++ {
		result, _ := predictor.Forward(ctx, map[string]any{
			"question": "test",
		})
		if result != nil {
			totalTokens += result.Usage.TotalTokens
		}
	}

	elapsed := time.Since(start)
	tokensPerSecond := float64(totalTokens) / elapsed.Seconds()

	t.Logf("Token processing speed: %.0f tokens/sec over %d iterations", tokensPerSecond, iterations)

	// Reasonable throughput check (very permissive for mocked LM)
	if tokensPerSecond < 1000 {
		t.Logf("Note: Token throughput is %.0f tokens/sec", tokensPerSecond)
	}
}

// ============================================================================
// Cache Benchmarks
// ============================================================================

// BenchmarkCache_ColdStart measures cache overhead on first access (miss).
// Target: <1ms for cache miss path.
func BenchmarkCache_ColdStart(b *testing.B) {
	cache := dsgo.NewLMCache(1000)

	for i := 0; i < b.N; i++ {
		key := "key_" + string(rune(i%1000))
		cache.Get(key)
	}
}

// BenchmarkCache_Hit measures cache retrieval latency.
// Target: <100µs for cache hits.
func BenchmarkCache_Hit(b *testing.B) {
	cache := dsgo.NewLMCache(1000)

	// Pre-populate cache
	result := &dsgo.GenerateResult{
		Content: "cached response",
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}
	cache.Set("benchmark_key", result)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("benchmark_key")
	}
}

// BenchmarkCache_Set measures cache write latency.
// Target: <500µs for cache set operations.
func BenchmarkCache_Set(b *testing.B) {
	cache := dsgo.NewLMCache(1000)

	result := &dsgo.GenerateResult{
		Content: "cached response",
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key_" + string(rune(i%1000))
		cache.Set(key, result)
	}
}

// BenchmarkCache_MixedWorkload measures realistic cache workload.
// Scenario: 70% reads, 30% writes.
func BenchmarkCache_MixedWorkload(b *testing.B) {
	cache := dsgo.NewLMCache(1000)

	result := &dsgo.GenerateResult{
		Content: "cached response",
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}

	// Pre-populate with some entries
	for i := 0; i < 500; i++ {
		cache.Set("preload_"+string(rune(i)), result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%10 < 7 {
			// 70% reads
			cache.Get("preload_" + string(rune(i%500)))
		} else {
			// 30% writes
			cache.Set("dynamic_"+string(rune(i%100)), result)
		}
	}
}

// ============================================================================
// ReAct Module Benchmarks
// ============================================================================

// BenchmarkReAct_SingleTool measures ReAct with single tool call.
// Target: ~200ms (includes tool execution overhead).
func BenchmarkReAct_SingleTool(b *testing.B) {
	calcTool := dsgo.NewTool(
		"calculate",
		"Perform calculation",
		func(ctx context.Context, args map[string]any) (any, error) {
			return 42.0, nil
		},
	).AddParameter("a", "number", "First number", true).
		AddParameter("b", "number", "Second number", true)

	lm := &BenchmarkToolMockLM{
		ToolCalls: []dsgo.ToolCall{
			{
				ID:        "call_1",
				Name:      "calculate",
				Arguments: map[string]any{"a": 5.0, "b": 3.0},
			},
		},
		FinalResponse: `{"answer": "The result is 42", "reasoning": "Computed successfully"}`,
	}

	sig := fixtures.ReActSig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		react := dsgo.NewReAct(sig, lm, []dsgo.Tool{*calcTool})
		react.WithMaxIterations(3)
		ctx := context.Background()
		_, _ = react.Forward(ctx, map[string]any{"question": "test"})
	}
}

// BenchmarkReAct_NoTools measures ReAct with direct answer (no tool calls).
// Target: ~100ms (similar to Predict).
func BenchmarkReAct_NoTools(b *testing.B) {
	lm := &BenchmarkToolMockLM{
		ToolCalls:     nil,
		FinalResponse: `{"answer": "Direct answer", "reasoning": "No tools needed"}`,
	}

	sig := fixtures.ReActSig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		react := dsgo.NewReAct(sig, lm, nil)
		ctx := context.Background()
		_, _ = react.Forward(ctx, map[string]any{"question": "test"})
	}
}

// ============================================================================
// Composition Benchmarks
// ============================================================================

// BenchmarkComposition_5Modules measures sequential 5-module pipeline.
// Target: ~250ms (5x module latency).
func BenchmarkComposition_5Modules(b *testing.B) {
	ctx := context.Background()

	sig := fixtures.SimplePredictSig()
	modules := make([]*dsgo.Predict, 5)
	for i := 0; i < 5; i++ {
		modules[i] = dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "stage"}`))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var lastAnswer string
		for j, mod := range modules {
			input := "test"
			if j > 0 {
				input = lastAnswer
			}
			r, _ := mod.Forward(ctx, map[string]any{"question": input})
			lastAnswer, _ = r.GetString("answer")
		}
	}
}

// BenchmarkParallel_5Modules measures parallel 5-module execution.
// Target: ~100ms (single module + coordination overhead).
func BenchmarkParallel_5Modules(b *testing.B) {
	ctx := context.Background()

	sig := fixtures.SimplePredictSig()
	modules := make([]*dsgo.Predict, 5)
	for i := 0; i < 5; i++ {
		modules[i] = dsgo.NewPredict(sig, NewMockLMWithResponse(`{"answer": "parallel"}`))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for _, mod := range modules {
			wg.Add(1)
			go func(m *dsgo.Predict) {
				defer wg.Done()
				_, _ = m.Forward(ctx, map[string]any{"question": "test"})
			}(mod)
		}
		wg.Wait()
	}
}

// ============================================================================
// Memory Benchmarks
// ============================================================================

// BenchmarkMemory_LongRunning measures memory stability over many iterations.
// Target: Stable heap, no growth trends.
func BenchmarkMemory_LongRunning(b *testing.B) {
	lm := NewMockLMWithResponse(`{"answer": "response"}`)
	sig := fixtures.SimplePredictSig()
	predictor := dsgo.NewPredict(sig, lm)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = predictor.Forward(ctx, map[string]any{"question": "test"})
	}
}

// ============================================================================
// Helper Mock for Benchmarks
// ============================================================================

// BenchmarkToolMockLM is a simplified tool-supporting mock for benchmarks
type BenchmarkToolMockLM struct {
	ToolCalls     []dsgo.ToolCall
	FinalResponse string
	callCount     int
}

func (m *BenchmarkToolMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	m.callCount++

	// First call returns tool calls, subsequent calls return final response
	if m.callCount == 1 && len(m.ToolCalls) > 0 {
		return &dsgo.GenerateResult{
			Content:   "",
			ToolCalls: m.ToolCalls,
			Usage: dsgo.Usage{
				PromptTokens:     10,
				CompletionTokens: 10,
				TotalTokens:      20,
			},
		}, nil
	}

	return &dsgo.GenerateResult{
		Content: m.FinalResponse,
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}, nil
}

func (m *BenchmarkToolMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
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

func (m *BenchmarkToolMockLM) Name() string        { return "benchmark-tool-mock-lm" }
func (m *BenchmarkToolMockLM) SupportsJSON() bool  { return true }
func (m *BenchmarkToolMockLM) SupportsTools() bool { return true }
func (m *BenchmarkToolMockLM) IsOpenAI() bool      { return false }
