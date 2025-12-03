package module

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assagman/dsgo/internal/core"
)

// TestHighConcurrencyParallel tests the system under high concurrency load
func TestHighConcurrencyParallel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	sig := core.NewSignature("HighConcurrencyTest").
		AddInput("task_id", core.FieldTypeInt, "Task ID").
		AddOutput("result", core.FieldTypeString, "Result").
		AddOutput("worker_id", core.FieldTypeInt, "Worker ID that processed the task")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]ok\n[[ ## worker_id ## ]]0",
				Usage:   core.Usage{TotalTokens: 5},
			}, nil
		},
	}

	history := core.NewHistory()
	predictor := NewPredict(sig, lm).WithHistory(history)
	parallel := NewParallel(predictor).WithMaxWorkers(50)

	// 1000 parallel tasks
	batch := make([]map[string]any, 1000)
	for i := 0; i < 1000; i++ {
		batch[i] = map[string]any{"task_id": i}
	}

	inputs := map[string]any{"_batch": batch}

	start := time.Now()
	result, err := parallel.Forward(context.Background(), inputs)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	if len(result.Completions) != 1000 {
		t.Errorf("Expected 1000 completions, got %d", len(result.Completions))
	}

	t.Logf("Processed 1000 tasks in %v (%.0f tasks/sec)", duration, float64(1000)/duration.Seconds())
}

// TestHistoryConcurrentStress tests History under extreme concurrent load
func TestHistoryConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	history := core.NewHistory()
	var wg sync.WaitGroup
	var operations int64

	// 100 goroutines, each doing 1000 operations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				// Mix of different operations
				switch j % 4 {
				case 0:
					history.AddUserMessage(fmt.Sprintf("user-%d-%d", id, j))
				case 1:
					history.AddAssistantMessage(fmt.Sprintf("assistant-%d-%d", id, j))
				case 2:
					_ = history.Get()
				case 3:
					_ = history.GetLast(5)
				}
				atomic.AddInt64(&operations, 1)
			}
		}(i)
	}

	wg.Wait()

	finalOps := atomic.LoadInt64(&operations)
	expectedOps := int64(100 * 1000)
	if finalOps != expectedOps {
		t.Errorf("Expected %d operations, got %d", expectedOps, finalOps)
	}

	t.Logf("Completed %d concurrent History operations successfully", finalOps)
}

// TestStreamingBufferConcurrentStress tests StreamingBuffer under concurrent load
func TestStreamingBufferConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	buffer := core.NewStreamingBuffer()
	var wg sync.WaitGroup
	var chunksWritten int64

	// 50 goroutines writing chunks concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				chunk := fmt.Sprintf("chunk-%d-%d ", id, j)
				buffer.Write(chunk)
				atomic.AddInt64(&chunksWritten, 1)
			}
		}(i)
	}

	wg.Wait()

	finalChunks := atomic.LoadInt64(&chunksWritten)
	expectedChunks := int64(50 * 100)
	if finalChunks != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, finalChunks)
	}

	// Verify final content is reasonable
	finalContent := buffer.Finalize()
	if len(finalContent) == 0 {
		t.Error("Expected non-empty final content")
	}

	t.Logf("Successfully wrote %d concurrent chunks, final content length: %d", finalChunks, len(finalContent))
}

// TestBestOfNParallelStress tests BestOfN with high parallelism
func TestBestOfNParallelStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	sig := core.NewSignature("BestOfNStressTest").
		AddInput("prompt", core.FieldTypeString, "Input prompt").
		AddOutput("response", core.FieldTypeString, "Generated response").
		AddOutput("quality", core.FieldTypeFloat, "Quality score")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Simulate variable processing time
			time.Sleep(time.Duration(5+int(time.Now().UnixNano()%20)) * time.Millisecond)
			return &core.GenerateResult{
				Content: "[[ ## response ## ]]Generated response\n[[ ## quality ## ]]0.8",
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
	}

	history := core.NewHistory()
	predictor := NewPredict(sig, lm).WithHistory(history)

	// Create BestOfN with high parallelism
	bestof := NewBestOfN(predictor, 20).
		WithScorer(func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
			if quality, ok := prediction.GetFloat("quality"); ok {
				return quality, nil
			}
			return 0.5, nil
		}).
		WithParallel(true).
		WithMaxFailures(10)

	inputs := map[string]any{"prompt": "Test prompt"}

	start := time.Now()
	result, err := bestof.Forward(context.Background(), inputs)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("BestOfN failed: %v", err)
	}

	if result.Score <= 0 {
		t.Error("Expected positive score")
	}

	t.Logf("BestOfN with 20 parallel attempts completed in %v, score: %.2f", duration, result.Score)
}

// TestMixedConcurrencyStress tests multiple components concurrently
func TestMixedConcurrencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	var wg sync.WaitGroup
	var completedOps int64

	// Test History operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		history := core.NewHistory()
		for i := 0; i < 500; i++ {
			history.AddUserMessage(fmt.Sprintf("message %d", i))
			_ = history.Get()
			atomic.AddInt64(&completedOps, 1)
		}
	}()

	// Test StreamingBuffer operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := core.NewStreamingBuffer()
		for i := 0; i < 500; i++ {
			buffer.Write(fmt.Sprintf("chunk %d ", i))
			_ = buffer.String()
			atomic.AddInt64(&completedOps, 1)
		}
	}()

	// Test Parallel module operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		sig := core.NewSignature("MixedTest").
			AddInput("value", core.FieldTypeInt, "Value").
			AddOutput("result", core.FieldTypeString, "Result")

		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## result ## ]]ok",
					Usage:   core.Usage{TotalTokens: 3},
				}, nil
			},
		}

		predictor := NewPredict(sig, lm)
		parallel := NewParallel(predictor).WithMaxWorkers(10)

		for i := 0; i < 50; i++ {
			batch := make([]map[string]any, 10)
			for j := 0; j < 10; j++ {
				batch[j] = map[string]any{"value": i*10 + j}
			}
			inputs := map[string]any{"_batch": batch}
			_, err := parallel.Forward(context.Background(), inputs)
			if err == nil {
				atomic.AddInt64(&completedOps, 1)
			}
		}
	}()

	// Test BestOfN operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		sig := core.NewSignature("BestOfNMixed").
			AddInput("prompt", core.FieldTypeString, "Prompt").
			AddOutput("response", core.FieldTypeString, "Response")

		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## response ## ]]response",
					Usage:   core.Usage{TotalTokens: 5},
				}, nil
			},
		}

		predictor := NewPredict(sig, lm)
		bestof := NewBestOfN(predictor, 5).
			WithScorer(func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
				return 0.7, nil
			}).
			WithParallel(true)

		for i := 0; i < 20; i++ {
			inputs := map[string]any{"prompt": fmt.Sprintf("prompt %d", i)}
			_, err := bestof.Forward(context.Background(), inputs)
			if err == nil {
				atomic.AddInt64(&completedOps, 1)
			}
		}
	}()

	wg.Wait()

	finalOps := atomic.LoadInt64(&completedOps)
	if finalOps < 1000 { // Should have completed many operations
		t.Errorf("Expected at least 1000 completed operations, got %d", finalOps)
	}

	t.Logf("Mixed concurrency stress test completed with %d successful operations", finalOps)
}

// TestMemoryUsageUnderConcurrency checks for memory leaks under high concurrency
func TestMemoryUsageUnderConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	// Force GC to get baseline
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	sig := core.NewSignature("MemoryTest").
		AddInput("data", core.FieldTypeString, "Input data").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]processed",
				Usage:   core.Usage{TotalTokens: 8},
			}, nil
		},
	}

	// Run many operations with different components
	for i := 0; i < 10; i++ {
		history := core.NewHistory()
		predictor := NewPredict(sig, lm).WithHistory(history)
		parallel := NewParallel(predictor).WithMaxWorkers(20)

		batch := make([]map[string]any, 100)
		for j := 0; j < 100; j++ {
			batch[j] = map[string]any{"data": fmt.Sprintf("data-%d-%d", i, j)}
		}

		inputs := map[string]any{"_batch": batch}
		_, err := parallel.Forward(context.Background(), inputs)
		if err != nil {
			t.Fatalf("Parallel execution failed: %v", err)
		}
	}

	// Force GC and check memory usage
	runtime.GC()
	runtime.GC() // Call twice to ensure finalizers run
	runtime.GC() // Third time for good measure
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calculate memory increase safely
	var memIncrease uint64
	if m2.Alloc > m1.Alloc {
		memIncrease = m2.Alloc - m1.Alloc
	} else {
		memIncrease = 0 // GC can free memory, so handle negative values
	}
	memIncreaseMB := float64(memIncrease) / 1024 / 1024

	t.Logf("Memory usage increased by %.2f MB after 1000 concurrent operations", memIncreaseMB)
	t.Logf("  Before: %d MB, After: %d MB", m1.Alloc/1024/1024, m2.Alloc/1024/1024)

	// Allow some memory increase but should be reasonable (< 50MB for this test)
	if memIncreaseMB > 50 {
		t.Errorf("Memory increase seems too high: %.2f MB", memIncreaseMB)
	}
}
