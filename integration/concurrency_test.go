package integration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/integration/fixtures"
	"github.com/assagman/dsgo/module"
)

// ============================================================================
// Concurrent Module Execution Tests
// ============================================================================

// TestConcurrentModuleExecution_100Goroutines tests 100 concurrent module executions.
// Validates:
// - No race conditions under high concurrency
// - All goroutines complete successfully
// - Results are independently correct
func TestConcurrentModuleExecution_100Goroutines(t *testing.T) {
	ctx, cancel := ContextWithTimeout(30 * time.Second)
	defer cancel()

	const numGoroutines = 100

	// Create a thread-safe mock LM
	lm := NewConcurrentSafeMockLM(`[[ ## answer ## ]]
This is answer number %d.`)

	sig := fixtures.SimplePredictSig()

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine gets its own Predict instance to avoid shared state
			predictor := module.NewPredict(sig, lm)

			result, err := predictor.Forward(ctx, map[string]any{
				"question": fmt.Sprintf("Question from goroutine %d", id),
			})

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				errors <- fmt.Errorf("goroutine %d: %w", id, err)
				return
			}

			// Verify result is valid
			answer, ok := result.GetString("answer")
			if !ok || answer == "" {
				atomic.AddInt64(&errorCount, 1)
				errors <- fmt.Errorf("goroutine %d: empty answer", id)
				return
			}

			atomic.AddInt64(&successCount, 1)
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var allErrors []error
	for err := range errors {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		t.Errorf("Got %d errors from concurrent execution:", len(allErrors))
		for i, err := range allErrors {
			if i < 5 { // Show first 5 errors
				t.Errorf("  - %v", err)
			}
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected %d successful executions, got %d", numGoroutines, successCount)
	}
}

// TestConcurrency_ModuleReuse tests the same module instance used by multiple goroutines.
// Validates:
// - Module without shared mutable state is safe for concurrent use
// - No data corruption
func TestConcurrency_ModuleReuse(t *testing.T) {
	ctx, cancel := ContextWithTimeout(20 * time.Second)
	defer cancel()

	const numGoroutines = 50

	// Create a single thread-safe mock LM
	lm := NewConcurrentSafeMockLM(`[[ ## answer ## ]]
Concurrent answer.`)

	sig := fixtures.SimplePredictSig()

	// Single predictor instance (without history - stateless)
	predictor := module.NewPredict(sig, lm)

	var wg sync.WaitGroup
	var successCount int64
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			result, err := predictor.Forward(ctx, map[string]any{
				"question": fmt.Sprintf("Reuse question %d", id),
			})

			if err != nil {
				return
			}

			answer, ok := result.GetString("answer")
			if ok && answer != "" {
				atomic.AddInt64(&successCount, 1)
				results <- answer
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// All should succeed since module is stateless
	if successCount != numGoroutines {
		t.Errorf("Expected %d successful executions, got %d", numGoroutines, successCount)
	}

	// Verify all results are valid
	count := 0
	for answer := range results {
		if answer == "" {
			t.Error("Got empty answer from concurrent execution")
		}
		count++
	}

	if count != numGoroutines {
		t.Errorf("Expected %d results, got %d", numGoroutines, count)
	}
}

// ============================================================================
// Cache Contention Tests
// ============================================================================

// TestCacheConcurrency_Contention tests 50 goroutines hitting the same cache.
// Validates:
// - No race conditions in cache operations
// - Correct hit/miss behavior
// - No cache corruption
func TestCacheConcurrency_Contention(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 20

	cache := core.NewLMCache(100)

	var wg sync.WaitGroup
	var hitCount int64
	var missCount int64
	var setCount int64

	// Mix of reads and writes to the same keys
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for op := 0; op < numOperations; op++ {
				// Use a small set of keys to maximize contention
				key := fmt.Sprintf("key_%d", op%5)

				if op%3 == 0 {
					// Write operation
					result := &core.GenerateResult{
						Content: fmt.Sprintf("Result from goroutine %d, op %d", id, op),
						Usage: core.Usage{
							PromptTokens:     10,
							CompletionTokens: 10,
							TotalTokens:      20,
						},
					}
					cache.Set(key, result)
					atomic.AddInt64(&setCount, 1)
				} else {
					// Read operation
					if _, ok := cache.Get(key); ok {
						atomic.AddInt64(&hitCount, 1)
					} else {
						atomic.AddInt64(&missCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify cache stats
	stats := cache.Stats()

	t.Logf("Cache stats: hits=%d, misses=%d, size=%d", stats.Hits, stats.Misses, stats.Size)
	t.Logf("Goroutine counts: hits=%d, misses=%d, sets=%d", hitCount, missCount, setCount)

	// Cache should have some entries
	if stats.Size == 0 {
		t.Error("Cache should have some entries after concurrent writes")
	}

	// Total tracked operations should equal stats
	if stats.Hits != hitCount {
		t.Errorf("Cache hit count mismatch: stats=%d, counted=%d", stats.Hits, hitCount)
	}
	if stats.Misses != missCount {
		t.Errorf("Cache miss count mismatch: stats=%d, counted=%d", stats.Misses, missCount)
	}
}

// TestConcurrency_CacheIntegrity tests that cached values are not corrupted.
// Validates:
// - Values retrieved from cache match what was stored
// - Deep copies prevent mutation
func TestConcurrency_CacheIntegrity(t *testing.T) {
	const numGoroutines = 30

	cache := core.NewLMCache(100)

	// Pre-populate cache with known values
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("integrity_key_%d", i)
		result := &core.GenerateResult{
			Content: fmt.Sprintf("Original content %d", i),
			Usage: core.Usage{
				PromptTokens:     i * 10,
				CompletionTokens: i * 5,
				TotalTokens:      i * 15,
			},
			Metadata: map[string]any{
				"original_id": i,
			},
		}
		cache.Set(key, result)
	}

	var wg sync.WaitGroup
	var corruptionCount int64

	// Concurrent reads and verifications
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("integrity_key_%d", j)
				result, ok := cache.Get(key)
				if !ok {
					continue
				}

				// Verify content integrity
				expectedContent := fmt.Sprintf("Original content %d", j)
				if result.Content != expectedContent {
					atomic.AddInt64(&corruptionCount, 1)
				}

				// Verify usage integrity
				if result.Usage.TotalTokens != j*15 {
					atomic.AddInt64(&corruptionCount, 1)
				}

				// Try to mutate the retrieved value (should not affect cache)
				result.Content = "mutated"
				result.Usage.TotalTokens = 9999
			}
		}(i)
	}

	wg.Wait()

	if corruptionCount > 0 {
		t.Errorf("Detected %d cache corruptions", corruptionCount)
	}

	// Verify original values are still intact
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("integrity_key_%d", i)
		result, ok := cache.Get(key)
		if !ok {
			t.Errorf("Key %s missing from cache", key)
			continue
		}

		expectedContent := fmt.Sprintf("Original content %d", i)
		if result.Content != expectedContent {
			t.Errorf("Key %s corrupted: expected %q, got %q", key, expectedContent, result.Content)
		}
	}
}

// ============================================================================
// History Collector Concurrency Tests
// ============================================================================

// TestConcurrency_HistoryCollector tests concurrent history collection.
// Validates:
// - Entry integrity under concurrent writes
// - Total count is accurate
// - No data loss
func TestConcurrency_HistoryCollector(t *testing.T) {
	const numGoroutines = 50
	const entriesPerGoroutine = 10

	collector := &ThreadSafeHistoryCollector{
		Entries: make([]*core.HistoryEntry, 0),
	}

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < entriesPerGoroutine; j++ {
				entry := &core.HistoryEntry{
					Request: core.RequestMeta{
						Messages: []core.Message{
							{Role: "user", Content: fmt.Sprintf("Message from goroutine %d, entry %d", id, j)},
						},
						MessageCount: 1,
					},
					Response: core.ResponseMeta{
						Content:        fmt.Sprintf("Response %d-%d", id, j),
						ResponseLength: 20,
					},
					Usage: core.Usage{
						PromptTokens:     10,
						CompletionTokens: 5,
						TotalTokens:      15,
						Cost:             0.001,
					},
				}

				if err := collector.Collect(entry); err != nil {
					t.Errorf("Failed to collect entry: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify total count
	expectedCount := numGoroutines * entriesPerGoroutine
	actualCount := len(collector.GetEntries())

	if actualCount != expectedCount {
		t.Errorf("Expected %d entries, got %d", expectedCount, actualCount)
	}

	// Verify entry integrity (spot check)
	for i, entry := range collector.GetEntries() {
		if entry.Response.Content == "" {
			t.Errorf("Entry %d has empty response content", i)
		}
		if entry.Usage.TotalTokens != 15 {
			t.Errorf("Entry %d has wrong total tokens: %d", i, entry.Usage.TotalTokens)
		}
	}

	// Verify cost accumulation
	expectedCost := float64(expectedCount) * 0.001
	actualCost := collector.GetTotalCost()
	if actualCost < expectedCost*0.99 || actualCost > expectedCost*1.01 {
		t.Errorf("Expected total cost ~%f, got %f", expectedCost, actualCost)
	}
}

// TestConcurrency_HistoryCollector_ConcurrentRead tests concurrent reads from collector.
// Validates:
// - Reads don't corrupt data
// - Consistent snapshots
func TestConcurrency_HistoryCollector_ConcurrentRead(t *testing.T) {
	const numWriters = 10
	const numReaders = 20
	const entriesPerWriter = 20

	collector := &ThreadSafeHistoryCollector{
		Entries: make([]*core.HistoryEntry, 0),
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < entriesPerWriter; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entry := &core.HistoryEntry{
					Request: core.RequestMeta{
						Messages: []core.Message{
							{Role: "user", Content: fmt.Sprintf("W%d-E%d", id, j)},
						},
						MessageCount: 1,
					},
					Response: core.ResponseMeta{
						Content:        fmt.Sprintf("Response %d-%d", id, j),
						ResponseLength: 20,
					},
					Usage: core.Usage{TotalTokens: 10},
				}
				_ = collector.Collect(entry)
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Readers
	var readCount int64
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				entries := collector.GetEntries()
				atomic.AddInt64(&readCount, 1)

				// Verify entries are valid
				for _, entry := range entries {
					if entry == nil {
						t.Error("Got nil entry from concurrent read")
					}
				}

				time.Sleep(time.Microsecond * 10)
			}
		}()
	}

	wg.Wait()

	t.Logf("Performed %d concurrent reads", readCount)

	// Final verification
	expectedCount := numWriters * entriesPerWriter
	actualCount := len(collector.GetEntries())
	if actualCount != expectedCount {
		t.Errorf("Expected %d entries, got %d", expectedCount, actualCount)
	}
}

// ============================================================================
// BestOfN Parallel Mode Tests
// ============================================================================

// TestConcurrency_BestOfN_ParallelSafety tests BestOfN parallel mode with independent instances.
// Validates:
// - Parallel execution completes without races
// - Best result is correctly selected
// - All candidates are scored
func TestConcurrency_BestOfN_ParallelSafety(t *testing.T) {
	ctx, cancel := ContextWithTimeout(20 * time.Second)
	defer cancel()

	const n = 10

	// Create thread-safe mock LM with varying scores in responses
	lm := NewConcurrentSafeMockLMWithScoring()

	sig := fixtures.BestOfNSig()

	// Create stateless predictor (safe for parallel reuse)
	predictor := module.NewPredict(sig, lm)

	scorer := func(inputs map[string]any, pred *core.Prediction) (float64, error) {
		result, ok := pred.GetString("result")
		if !ok {
			return 0, nil
		}
		// Score based on response length
		return float64(len(result)), nil
	}

	bestOfN := module.NewBestOfN(predictor, n).
		WithScorer(scorer).
		WithParallel(true).
		WithReturnAll(true)

	result, err := bestOfN.Forward(ctx, map[string]any{
		"prompt": "Generate something interesting",
	})

	if err != nil {
		t.Fatalf("BestOfN parallel failed: %v", err)
	}

	// Verify we got a result
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	bestResult, ok := result.GetString("result")
	if !ok || bestResult == "" {
		t.Error("Expected non-empty best result")
	}

	// Verify all completions were collected
	if len(result.Completions) != n {
		t.Errorf("Expected %d completions, got %d", n, len(result.Completions))
	}

	// Verify score is set
	if result.Score <= 0 {
		t.Error("Expected positive score on best result")
	}
}

// TestConcurrency_BestOfN_IndependentModules tests BestOfN with independent module instances.
// Validates:
// - Each module instance maintains independent state
// - No cross-contamination between instances
func TestConcurrency_BestOfN_IndependentModules(t *testing.T) {
	ctx, cancel := ContextWithTimeout(20 * time.Second)
	defer cancel()

	const n = 5

	// Create thread-safe mock LM
	lm := NewConcurrentSafeMockLMWithScoring()

	sig := fixtures.BestOfNSig()

	// Each iteration should see independent state
	var executionCount int64

	predictor := &CountingPredict{
		Signature:      sig,
		LM:             lm,
		executionCount: &executionCount,
	}

	scorer := func(inputs map[string]any, pred *core.Prediction) (float64, error) {
		result, _ := pred.GetString("result")
		return float64(len(result)), nil
	}

	bestOfN := module.NewBestOfN(predictor, n).
		WithScorer(scorer).
		WithParallel(true)

	_, err := bestOfN.Forward(ctx, map[string]any{
		"prompt": "Test prompt",
	})

	if err != nil {
		t.Fatalf("BestOfN failed: %v", err)
	}

	// Verify all N executions happened
	if atomic.LoadInt64(&executionCount) != int64(n) {
		t.Errorf("Expected %d executions, got %d", n, executionCount)
	}
}

// TestConcurrency_BestOfN_FailureHandling tests parallel BestOfN with some failures.
// Validates:
// - Failures are handled correctly in parallel
// - Partial success still returns best result
func TestConcurrency_BestOfN_FailureHandling(t *testing.T) {
	ctx, cancel := ContextWithTimeout(20 * time.Second)
	defer cancel()

	const n = 10

	// Create mock LM that fails on some requests
	lm := NewConcurrentSafeMockLMWithFailures(3) // 3 out of 10 will fail

	sig := fixtures.BestOfNSig()
	predictor := module.NewPredict(sig, lm)

	scorer := func(inputs map[string]any, pred *core.Prediction) (float64, error) {
		result, ok := pred.GetString("result")
		if !ok {
			return 0, nil
		}
		return float64(len(result)), nil
	}

	bestOfN := module.NewBestOfN(predictor, n).
		WithScorer(scorer).
		WithParallel(true).
		WithMaxFailures(5) // Allow up to 5 failures

	result, err := bestOfN.Forward(ctx, map[string]any{
		"prompt": "Test with failures",
	})

	if err != nil {
		t.Fatalf("BestOfN should succeed with partial failures: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	bestResult, ok := result.GetString("result")
	if !ok || bestResult == "" {
		t.Error("Expected non-empty best result despite some failures")
	}
}

// ============================================================================
// Thread-Safe Mock LM Implementations
// ============================================================================

// ConcurrentSafeMockLM is a mock LM safe for concurrent use
type ConcurrentSafeMockLM struct {
	responseTemplate string
	callCount        int64
}

// NewConcurrentSafeMockLM creates a new concurrent-safe mock LM
func NewConcurrentSafeMockLM(responseTemplate string) *ConcurrentSafeMockLM {
	return &ConcurrentSafeMockLM{
		responseTemplate: responseTemplate,
	}
}

func (m *ConcurrentSafeMockLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	count := atomic.AddInt64(&m.callCount, 1)

	return &core.GenerateResult{
		Content: fmt.Sprintf(m.responseTemplate, count),
		Usage: core.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             0.001,
		},
	}, nil
}

func (m *ConcurrentSafeMockLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- core.Chunk{
			Content: result.Content,
			Usage:   result.Usage,
		}
	}()

	return chunkChan, errChan
}

func (m *ConcurrentSafeMockLM) Name() string        { return "concurrent-safe-mock-lm" }
func (m *ConcurrentSafeMockLM) SupportsJSON() bool  { return false }
func (m *ConcurrentSafeMockLM) SupportsTools() bool { return false }
func (m *ConcurrentSafeMockLM) IsOpenAI() bool      { return false }

// ConcurrentSafeMockLMWithScoring returns varied length responses for scoring tests
type ConcurrentSafeMockLMWithScoring struct {
	callCount int64
}

func NewConcurrentSafeMockLMWithScoring() *ConcurrentSafeMockLMWithScoring {
	return &ConcurrentSafeMockLMWithScoring{}
}

func (m *ConcurrentSafeMockLMWithScoring) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	count := atomic.AddInt64(&m.callCount, 1)

	// Vary response length based on call count for scoring differentiation
	repeatCount := int(count%5) + 1
	content := ""
	for i := 0; i < repeatCount; i++ {
		content += fmt.Sprintf("Result segment %d. ", i+1)
	}

	return &core.GenerateResult{
		Content: fmt.Sprintf(`[[ ## result ## ]]
%s`, content),
		Usage: core.Usage{
			PromptTokens:     10,
			CompletionTokens: 10 * repeatCount,
			TotalTokens:      10 + 10*repeatCount,
			Cost:             0.001 * float64(repeatCount),
		},
	}, nil
}

func (m *ConcurrentSafeMockLMWithScoring) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- core.Chunk{
			Content: result.Content,
			Usage:   result.Usage,
		}
	}()

	return chunkChan, errChan
}

func (m *ConcurrentSafeMockLMWithScoring) Name() string        { return "concurrent-safe-mock-lm-scoring" }
func (m *ConcurrentSafeMockLMWithScoring) SupportsJSON() bool  { return false }
func (m *ConcurrentSafeMockLMWithScoring) SupportsTools() bool { return false }
func (m *ConcurrentSafeMockLMWithScoring) IsOpenAI() bool      { return false }

// ConcurrentSafeMockLMWithFailures fails on some requests
type ConcurrentSafeMockLMWithFailures struct {
	callCount    int64
	failureCount int
}

func NewConcurrentSafeMockLMWithFailures(failureCount int) *ConcurrentSafeMockLMWithFailures {
	return &ConcurrentSafeMockLMWithFailures{
		failureCount: failureCount,
	}
}

func (m *ConcurrentSafeMockLMWithFailures) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	count := atomic.AddInt64(&m.callCount, 1)

	// Fail on first N requests
	if int(count) <= m.failureCount {
		return nil, fmt.Errorf("simulated failure %d", count)
	}

	return &core.GenerateResult{
		Content: fmt.Sprintf(`[[ ## result ## ]]
Success response %d`, count),
		Usage: core.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             0.001,
		},
	}, nil
}

func (m *ConcurrentSafeMockLMWithFailures) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- core.Chunk{
			Content: result.Content,
			Usage:   result.Usage,
		}
	}()

	return chunkChan, errChan
}

func (m *ConcurrentSafeMockLMWithFailures) Name() string        { return "concurrent-safe-mock-lm-failures" }
func (m *ConcurrentSafeMockLMWithFailures) SupportsJSON() bool  { return false }
func (m *ConcurrentSafeMockLMWithFailures) SupportsTools() bool { return false }
func (m *ConcurrentSafeMockLMWithFailures) IsOpenAI() bool      { return false }

// ============================================================================
// Thread-Safe History Collector
// ============================================================================

// ThreadSafeHistoryCollector is a history collector safe for concurrent use
type ThreadSafeHistoryCollector struct {
	mu      sync.RWMutex
	Entries []*core.HistoryEntry
}

func (c *ThreadSafeHistoryCollector) Collect(entry *core.HistoryEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Entries = append(c.Entries, entry)
	return nil
}

func (c *ThreadSafeHistoryCollector) Close() error {
	return nil
}

func (c *ThreadSafeHistoryCollector) GetEntries() []*core.HistoryEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return a copy to prevent race conditions
	entries := make([]*core.HistoryEntry, len(c.Entries))
	copy(entries, c.Entries)
	return entries
}

func (c *ThreadSafeHistoryCollector) GetTotalCost() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := 0.0
	for _, entry := range c.Entries {
		total += entry.Usage.Cost
	}
	return total
}

// ============================================================================
// Counting Module for Execution Tracking
// ============================================================================

// CountingPredict is a module that counts executions
type CountingPredict struct {
	Signature      *core.Signature
	LM             core.LM
	executionCount *int64
}

func (p *CountingPredict) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	atomic.AddInt64(p.executionCount, 1)

	result, err := p.LM.Generate(ctx, nil, nil)
	if err != nil {
		return nil, err
	}

	outputs := map[string]any{
		"result": result.Content,
	}

	return core.NewPrediction(outputs).WithUsage(result.Usage), nil
}

func (p *CountingPredict) GetSignature() *core.Signature {
	return p.Signature
}
