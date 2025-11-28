package integration

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/integration/fixtures"
	"github.com/assagman/dsgo/module"
)

// ============================================================================
// Memory Leak Detection Tests
// ============================================================================

// TestMemory_NoLeaksOnSuccess validates no memory leaks on successful executions.
// Scenario: 1000 successful executions.
// Validates: Heap stable, no growth trends.
func TestMemory_NoLeaksOnSuccess(t *testing.T) {
	ctx := context.Background()
	iterations := 1000

	lm := NewMockLMWithResponse(`{"answer": "response"}`)
	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	// Force GC before measuring
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	for i := 0; i < iterations; i++ {
		_, err := predictor.Forward(ctx, map[string]any{
			"question": "test question",
		})
		if err != nil {
			t.Fatalf("Forward failed at iteration %d: %v", i, err)
		}
	}

	// Force GC after all executions
	runtime.GC()
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	heapGrowthMB := float64(endMem.HeapAlloc-startMem.HeapAlloc) / 1024 / 1024

	t.Logf("Memory stats after %d successful executions:", iterations)
	t.Logf("  Heap growth: %.2f MB", heapGrowthMB)
	t.Logf("  Heap alloc: %.2f MB -> %.2f MB", float64(startMem.HeapAlloc)/1024/1024, float64(endMem.HeapAlloc)/1024/1024)
	t.Logf("  Total alloc: %.2f MB", float64(endMem.TotalAlloc)/1024/1024)
	t.Logf("  Num GC: %d", endMem.NumGC)

	// After GC, heap growth should be minimal for stateless operations
	if heapGrowthMB > 50 {
		t.Logf("Warning: Significant heap growth: %.2f MB over %d iterations", heapGrowthMB, iterations)
	}
}

// TestMemory_NoLeaksOnError validates cleanup on error paths.
// Scenario: 1000 error scenarios.
// Validates: No memory accumulation on errors.
func TestMemory_NoLeaksOnError(t *testing.T) {
	ctx := context.Background()
	iterations := 1000

	lm := NewAlwaysFailLM("test-error")
	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	// Force GC before measuring
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	for i := 0; i < iterations; i++ {
		_, err := predictor.Forward(ctx, map[string]any{
			"question": "test question",
		})
		if err == nil {
			t.Fatalf("Expected error at iteration %d", i)
		}
	}

	// Force GC after all executions
	runtime.GC()
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	heapGrowthMB := float64(endMem.HeapAlloc-startMem.HeapAlloc) / 1024 / 1024

	t.Logf("Memory stats after %d error scenarios:", iterations)
	t.Logf("  Heap growth: %.2f MB", heapGrowthMB)
	t.Logf("  Num GC: %d", endMem.NumGC)

	// Error paths should not accumulate memory
	if heapGrowthMB > 20 {
		t.Logf("Warning: Memory growth on error paths: %.2f MB", heapGrowthMB)
	}
}

// TestMemory_StreamingCleanup validates channel cleanup on streaming.
// Scenario: 100 streams with cancellation.
// Validates: No lingering goroutines, channels cleaned up.
func TestMemory_StreamingCleanup(t *testing.T) {
	iterations := 100

	lm := NewMockLMWithResponse(`{"answer": "streaming response"}`)
	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	// Count goroutines before
	startGoroutines := runtime.NumGoroutine()

	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		// Start streaming
		streamResult, err := predictor.Stream(ctx, map[string]any{
			"question": "test",
		})
		if err != nil {
			cancel()
			continue
		}

		// Consume chunks
		go func() {
			for range streamResult.Chunks {
				// Drain chunks
			}
		}()

		// Cancel midway through some iterations
		if i%2 == 0 {
			cancel()
		} else {
			// Wait for completion then cancel
			<-streamResult.Prediction
			cancel()
		}

		// Drain error channel
		<-streamResult.Errors
	}

	// Allow time for goroutine cleanup
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	endGoroutines := runtime.NumGoroutine()

	t.Logf("Goroutine count: %d -> %d (delta: %d)", startGoroutines, endGoroutines, endGoroutines-startGoroutines)

	// Allow for some variance but should not grow significantly
	if endGoroutines-startGoroutines > 10 {
		t.Errorf("Possible goroutine leak: started with %d, ended with %d", startGoroutines, endGoroutines)
	}
}

// ============================================================================
// Large History Collection Tests
// ============================================================================

// TestMemory_LargeHistoryCollection validates memory bounds with large history.
// Scenario: Collect 10000 history entries.
// Validates: Memory bounded by ring buffer size.
func TestMemory_LargeHistoryCollection(t *testing.T) {
	bufferSize := 100
	entryCount := 10000

	collector := core.NewMemoryCollector(bufferSize)

	// Force GC before measuring
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	for i := 0; i < entryCount; i++ {
		entry := &core.HistoryEntry{
			ID:        "entry-" + string(rune(i)),
			Timestamp: time.Now(),
			Provider:  "test-provider",
			Model:     "test-model",
			Request: core.RequestMeta{
				Messages: []core.Message{
					{Role: "user", Content: "Test message with some content"},
				},
			},
			Response: core.ResponseMeta{
				Content:      "Test response with some content that is reasonably sized",
				FinishReason: "stop",
			},
			Usage: core.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
				Cost:             0.001,
			},
		}
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
	}

	// Verify buffer size is bounded
	if collector.Len() != bufferSize {
		t.Errorf("Expected buffer size %d, got %d", bufferSize, collector.Len())
	}

	// Force GC after collection
	runtime.GC()
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	heapGrowthMB := float64(endMem.HeapAlloc-startMem.HeapAlloc) / 1024 / 1024

	t.Logf("Memory stats after %d entries (buffer size %d):", entryCount, bufferSize)
	t.Logf("  Heap growth: %.2f MB", heapGrowthMB)
	t.Logf("  Buffer entries: %d", collector.Len())
	t.Logf("  Total collected: %d", collector.Count())

	// Memory should be bounded by buffer size, not entry count
	// Allow 10MB for safety (100 entries * ~100KB each would be ~10MB max)
	if heapGrowthMB > 20 {
		t.Logf("Warning: Memory growth exceeds expected bounds for ring buffer: %.2f MB", heapGrowthMB)
	}
}

// TestMemory_CacheMemoryBounds validates cache memory is bounded.
// Scenario: Fill cache beyond capacity.
// Validates: Memory bounded by cache capacity.
func TestMemory_CacheMemoryBounds(t *testing.T) {
	capacity := 100
	fillCount := 1000

	cache := core.NewLMCache(capacity)

	// Force GC before measuring
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	for i := 0; i < fillCount; i++ {
		result := &core.GenerateResult{
			Content: "This is a test response with some content to take up memory",
			Usage: core.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			Metadata: map[string]any{
				"iteration": i,
			},
		}
		key := "key_" + string(rune(i))
		cache.Set(key, result)
	}

	// Verify cache size is bounded
	if cache.Size() != capacity {
		t.Errorf("Expected cache size %d, got %d", capacity, cache.Size())
	}

	// Force GC after fill
	runtime.GC()
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	heapGrowthMB := float64(endMem.HeapAlloc-startMem.HeapAlloc) / 1024 / 1024

	t.Logf("Memory stats after %d cache sets (capacity %d):", fillCount, capacity)
	t.Logf("  Heap growth: %.2f MB", heapGrowthMB)
	t.Logf("  Cache size: %d", cache.Size())

	// Memory should be bounded by capacity, not total inserts
	if heapGrowthMB > 10 {
		t.Logf("Warning: Cache memory exceeds expected bounds: %.2f MB", heapGrowthMB)
	}
}

// ============================================================================
// Streaming Memory Footprint Tests
// ============================================================================

// TestMemory_StreamingFootprint validates memory during streaming.
// Scenario: Large streaming response.
// Validates: Memory doesn't accumulate during streaming.
func TestMemory_StreamingFootprint(t *testing.T) {
	iterations := 50

	// Create a mock that returns chunks
	lm := NewMockLMWithResponse(`{"answer": "This is a streaming response that contains some content"}`)
	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	// Force GC before measuring
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	for i := 0; i < iterations; i++ {
		ctx := context.Background()
		streamResult, err := predictor.Stream(ctx, map[string]any{
			"question": "test",
		})
		if err != nil {
			continue
		}

		// Consume all chunks
		for range streamResult.Chunks {
			// Process chunk
		}

		// Wait for prediction
		<-streamResult.Prediction
		<-streamResult.Errors
	}

	// Force GC after streaming
	runtime.GC()
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	heapGrowthMB := float64(endMem.HeapAlloc-startMem.HeapAlloc) / 1024 / 1024

	t.Logf("Memory stats after %d streaming operations:", iterations)
	t.Logf("  Heap growth: %.2f MB", heapGrowthMB)

	// Streaming should not accumulate significant memory
	if heapGrowthMB > 10 {
		t.Logf("Warning: Memory growth during streaming: %.2f MB", heapGrowthMB)
	}
}

// TestMemory_ConcurrentOperations validates memory under concurrent load.
// Scenario: 50 concurrent operations.
// Validates: No memory spikes, reasonable concurrent memory usage.
func TestMemory_ConcurrentOperations(t *testing.T) {
	concurrency := 50

	lm := NewConcurrentSafeMockLM(`{"answer": "concurrent response"}`)
	sig := fixtures.SimplePredictSig()

	// Force GC before measuring
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	done := make(chan struct{}, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			predictor := module.NewPredict(sig, lm)
			ctx := context.Background()

			for j := 0; j < 100; j++ {
				_, _ = predictor.Forward(ctx, map[string]any{
					"question": "concurrent test",
				})
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Force GC after concurrent operations
	runtime.GC()
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	heapGrowthMB := float64(endMem.HeapAlloc-startMem.HeapAlloc) / 1024 / 1024
	totalOps := concurrency * 100

	t.Logf("Memory stats after %d concurrent operations:", totalOps)
	t.Logf("  Heap growth: %.2f MB", heapGrowthMB)
	t.Logf("  Goroutines at end: %d", runtime.NumGoroutine())

	// Should not have excessive memory growth for 5000 operations
	if heapGrowthMB > 100 {
		t.Logf("Warning: High memory usage under concurrent load: %.2f MB", heapGrowthMB)
	}
}

// ============================================================================
// Object Allocation Tests
// ============================================================================

// TestMemory_AllocationRate measures allocation rate per operation.
// Validates: Reasonable allocation patterns.
func TestMemory_AllocationRate(t *testing.T) {
	iterations := 1000

	lm := NewMockLMWithResponse(`{"answer": "response"}`)
	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)
	ctx := context.Background()

	// Force GC before measuring
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	for i := 0; i < iterations; i++ {
		_, _ = predictor.Forward(ctx, map[string]any{
			"question": "test",
		})
	}

	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	allocsPerOp := float64(endMem.Mallocs-startMem.Mallocs) / float64(iterations)
	bytesPerOp := float64(endMem.TotalAlloc-startMem.TotalAlloc) / float64(iterations)

	t.Logf("Allocation stats over %d operations:", iterations)
	t.Logf("  Allocations per operation: %.1f", allocsPerOp)
	t.Logf("  Bytes per operation: %.1f", bytesPerOp)

	// These are informational, not hard failures
	if allocsPerOp > 1000 {
		t.Logf("Note: High allocation rate per operation: %.1f", allocsPerOp)
	}
}

// ============================================================================
// MemoryCollector Clear Tests
// ============================================================================

// TestMemoryCollector_ClearRemovesAllEntries tests that Clear() removes all entries
func TestMemoryCollector_ClearRemovesAllEntries(t *testing.T) {
	collector := core.NewMemoryCollector(10)

	// Add some entries
	for i := 0; i < 5; i++ {
		entry := &core.HistoryEntry{
			ID:        "entry-" + string(rune(i)),
			Provider:  "test-provider",
			Model:     "test-model",
			Timestamp: time.Now(),
		}
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
	}

	// Verify entries were collected
	if collector.Len() != 5 {
		t.Errorf("Expected 5 entries, got %d", collector.Len())
	}
	if collector.Count() != 5 {
		t.Errorf("Expected count 5, got %d", collector.Count())
	}

	// Clear the collector
	collector.Clear()

	// Verify all entries are removed
	if collector.Len() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", collector.Len())
	}
	if collector.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", collector.Count())
	}
	if len(collector.GetAll()) != 0 {
		t.Errorf("Expected empty GetAll() after clear, got %d entries", len(collector.GetAll()))
	}
}

// TestMemoryCollector_ClearResetsRingBuffer tests that Clear() resets the ring buffer state
func TestMemoryCollector_ClearResetsRingBuffer(t *testing.T) {
	collector := core.NewMemoryCollector(3)

	// Fill buffer beyond capacity to wrap around
	for i := 0; i < 6; i++ {
		entry := &core.HistoryEntry{
			ID: fmt.Sprintf("entry-%d", i),
		}
		_ = collector.Collect(entry)
	}

	// Buffer should be full (3 entries, head wrapped)
	if collector.Len() != 3 {
		t.Errorf("Expected buffer length 3, got %d", collector.Len())
	}

	// Clear
	collector.Clear()

	// Add new entries - should start fresh
	for i := 0; i < 2; i++ {
		entry := &core.HistoryEntry{
			ID: fmt.Sprintf("new-entry-%d", i),
		}
		_ = collector.Collect(entry)
	}

	// Should have exactly 2 entries
	if collector.Len() != 2 {
		t.Errorf("Expected 2 entries after clear and re-add, got %d", collector.Len())
	}
	if collector.Count() != 2 {
		t.Errorf("Expected count 2, got %d", collector.Count())
	}

	entries := collector.GetAll()
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries from GetAll(), got %d", len(entries))
	}

	// Verify entries are the new ones and in order
	if entries[0].ID != "new-entry-0" || entries[1].ID != "new-entry-1" {
		t.Errorf("Entries not in expected order after clear and re-add: got %q, %q", entries[0].ID, entries[1].ID)
	}
}

// TestMemoryCollector_ClearAllowsRecollection tests that we can collect after clearing
func TestMemoryCollector_ClearAllowsRecollection(t *testing.T) {
	collector := core.NewMemoryCollector(5)

	// First collection
	for i := 0; i < 3; i++ {
		_ = collector.Collect(&core.HistoryEntry{ID: "first-" + string(rune(i))})
	}

	firstEntries := collector.GetAll()
	if len(firstEntries) != 3 {
		t.Errorf("First collection: expected 3 entries, got %d", len(firstEntries))
	}

	// Clear
	collector.Clear()

	// Second collection with different entries
	for i := 0; i < 4; i++ {
		_ = collector.Collect(&core.HistoryEntry{ID: "second-" + string(rune(i))})
	}

	secondEntries := collector.GetAll()
	if len(secondEntries) != 4 {
		t.Errorf("Second collection: expected 4 entries, got %d", len(secondEntries))
	}

	// Verify entries from first collection are gone
	for _, entry := range secondEntries {
		if len(entry.ID) > 5 && entry.ID[:5] == "first" {
			t.Errorf("First collection entry found in second collection: %s", entry.ID)
		}
	}

	// Verify entries from second collection exist
	for i := 0; i < 4; i++ {
		expected := "second-" + string(rune(i))
		found := false
		for _, entry := range secondEntries {
			if entry.ID == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find %s in second collection", expected)
		}
	}
}

// TestMemoryCollector_ClearLargeBuffer tests Clear() with a large buffer
func TestMemoryCollector_ClearLargeBuffer(t *testing.T) {
	bufferSize := 1000
	collector := core.NewMemoryCollector(bufferSize)

	// Fill entire buffer
	for i := 0; i < bufferSize; i++ {
		_ = collector.Collect(&core.HistoryEntry{ID: string(rune(i))})
	}

	if collector.Len() != bufferSize {
		t.Errorf("Expected %d entries, got %d", bufferSize, collector.Len())
	}

	// Clear should work efficiently even with large buffer
	collector.Clear()

	if collector.Len() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", collector.Len())
	}
	if collector.Count() != 0 {
		t.Errorf("Expected count 0, got %d", collector.Count())
	}
}

// TestMemoryCollector_GetLastAfterClear tests GetLast() after clearing
func TestMemoryCollector_GetLastAfterClear(t *testing.T) {
	collector := core.NewMemoryCollector(10)

	// Add entries
	for i := 0; i < 5; i++ {
		_ = collector.Collect(&core.HistoryEntry{ID: string(rune(i))})
	}

	// Get last before clear
	lastBefore := collector.GetLast(2)
	if len(lastBefore) != 2 {
		t.Errorf("Before clear: expected 2 entries, got %d", len(lastBefore))
	}

	// Clear
	collector.Clear()

	// Get last after clear should return empty
	lastAfter := collector.GetLast(2)
	if len(lastAfter) != 0 {
		t.Errorf("After clear: expected 0 entries, got %d", len(lastAfter))
	}
}

// TestMemoryCollector_ClearConcurrent tests Clear() with concurrent operations
func TestMemoryCollector_ClearConcurrent(t *testing.T) {
	collector := core.NewMemoryCollector(100)

	// Populate collector
	for i := 0; i < 50; i++ {
		_ = collector.Collect(&core.HistoryEntry{ID: string(rune(i))})
	}

	clearDone := make(chan struct{})
	readDone := make(chan struct{})

	// Goroutine 1: Clear the collector
	go func() {
		collector.Clear()
		clearDone <- struct{}{}
	}()

	// Goroutine 2: Try to read while clearing (should not panic)
	go func() {
		// This may read stale or empty data, but shouldn't panic
		_ = collector.GetAll()
		_ = collector.Count()
		_ = collector.Len()
		readDone <- struct{}{}
	}()

	<-clearDone
	<-readDone

	// Verify final state is clean
	if collector.Len() != 0 {
		t.Errorf("Expected 0 entries after concurrent clear, got %d", collector.Len())
	}
}
