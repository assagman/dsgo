package fixtures

import (
	"errors"
	"sync"
	"time"

	"github.com/assagman/dsgo/core"
)

// ============================================================================
// Test Collector Implementations
// ============================================================================

// CountingCollector counts entries without storing them
type CountingCollector struct {
	mu    sync.Mutex
	count int64
}

// NewCountingCollector creates a new counting collector
func NewCountingCollector() *CountingCollector {
	return &CountingCollector{}
}

// Collect increments the count
func (c *CountingCollector) Collect(entry *core.HistoryEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return nil
}

// Close is a no-op
func (c *CountingCollector) Close() error {
	return nil
}

// Count returns the number of entries collected
func (c *CountingCollector) Count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// ============================================================================
// Failing Collector
// ============================================================================

// FailingCollector fails after a configured number of successful collects
type FailingCollector struct {
	mu           sync.Mutex
	SuccessCount int
	FailAfter    int
	FailError    error
	CloseError   error
}

// NewFailingCollector creates a collector that fails after n successful collects
func NewFailingCollector(failAfter int) *FailingCollector {
	return &FailingCollector{
		FailAfter: failAfter,
		FailError: errors.New("simulated collector failure"),
	}
}

// Collect fails after FailAfter successful collections
func (c *FailingCollector) Collect(entry *core.HistoryEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SuccessCount >= c.FailAfter {
		return c.FailError
	}
	c.SuccessCount++
	return nil
}

// Close returns CloseError if set
func (c *FailingCollector) Close() error {
	return c.CloseError
}

// ============================================================================
// Delayed Collector
// ============================================================================

// DelayedCollector adds artificial delay to each collect operation
type DelayedCollector struct {
	mu      sync.Mutex
	Delay   time.Duration
	Entries []*core.HistoryEntry
	MaxSize int
}

// NewDelayedCollector creates a collector with artificial delay
func NewDelayedCollector(delay time.Duration, maxSize int) *DelayedCollector {
	return &DelayedCollector{
		Delay:   delay,
		MaxSize: maxSize,
	}
}

// Collect adds delay before storing the entry
func (c *DelayedCollector) Collect(entry *core.HistoryEntry) error {
	time.Sleep(c.Delay)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.MaxSize > 0 && len(c.Entries) >= c.MaxSize {
		c.Entries = c.Entries[1:]
	}
	c.Entries = append(c.Entries, entry)
	return nil
}

// Close is a no-op
func (c *DelayedCollector) Close() error {
	return nil
}

// GetEntries returns all collected entries
func (c *DelayedCollector) GetEntries() []*core.HistoryEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]*core.HistoryEntry, len(c.Entries))
	copy(result, c.Entries)
	return result
}

// ============================================================================
// Validation Helpers
// ============================================================================

// AssertHistoryEntry validates a history entry has expected fields
func AssertHistoryEntry(entry *core.HistoryEntry, provider, model string, hasError bool) error {
	if entry == nil {
		return errors.New("entry is nil")
	}
	if entry.ID == "" {
		return errors.New("entry ID is empty")
	}
	if entry.Timestamp.IsZero() {
		return errors.New("entry timestamp is zero")
	}
	if provider != "" && entry.Provider != provider {
		return errors.New("provider mismatch")
	}
	if model != "" && entry.Model != model {
		return errors.New("model mismatch")
	}
	if hasError && entry.Error == nil {
		return errors.New("expected error but got nil")
	}
	if !hasError && entry.Error != nil {
		return errors.New("unexpected error in entry")
	}
	return nil
}

// AssertUsage validates usage fields are reasonable
func AssertUsage(usage core.Usage, minTokens, maxTokens int) error {
	if usage.TotalTokens < minTokens {
		return errors.New("total tokens too low")
	}
	if maxTokens > 0 && usage.TotalTokens > maxTokens {
		return errors.New("total tokens too high")
	}
	if usage.PromptTokens < 0 {
		return errors.New("negative prompt tokens")
	}
	if usage.CompletionTokens < 0 {
		return errors.New("negative completion tokens")
	}
	return nil
}

// AssertCost validates cost is within expected range
func AssertCost(cost, minCost, maxCost float64) error {
	if cost < minCost {
		return errors.New("cost too low")
	}
	if maxCost > 0 && cost > maxCost {
		return errors.New("cost too high")
	}
	return nil
}

// ============================================================================
// Sample History Entries
// ============================================================================

// SampleHistoryEntry creates a sample history entry for testing
func SampleHistoryEntry(id, provider, model string) *core.HistoryEntry {
	return &core.HistoryEntry{
		ID:        id,
		Timestamp: time.Now(),
		SessionID: "test-session",
		Provider:  provider,
		Model:     model,
		Request: core.RequestMeta{
			Messages: []core.Message{
				{Role: "user", Content: "Test message"},
			},
			PromptLength: 12,
			MessageCount: 1,
		},
		Response: core.ResponseMeta{
			Content:        "Test response",
			FinishReason:   "stop",
			ResponseLength: 13,
		},
		Usage: core.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             0.001,
		},
	}
}

// SampleHistoryEntryWithError creates a sample history entry with error
func SampleHistoryEntryWithError(id, provider, model, errorMsg string) *core.HistoryEntry {
	entry := SampleHistoryEntry(id, provider, model)
	entry.Error = &core.ErrorMeta{
		Message:    errorMsg,
		Code:       "TEST_ERROR",
		StatusCode: 500,
	}
	return entry
}

// SampleHistoryEntryWithToolCalls creates a sample history entry with tool calls
func SampleHistoryEntryWithToolCalls(id, provider, model string, toolCount int) *core.HistoryEntry {
	entry := SampleHistoryEntry(id, provider, model)
	entry.Request.HasTools = true
	entry.Request.ToolCount = toolCount

	toolCalls := make([]core.ToolCall, toolCount)
	for i := 0; i < toolCount; i++ {
		toolCalls[i] = core.ToolCall{
			ID:   id + "-tool-" + string(rune('0'+i)),
			Name: "test_tool",
			Arguments: map[string]any{
				"arg": i,
			},
		}
	}
	entry.Response.ToolCalls = toolCalls
	entry.Response.ToolCallCount = toolCount

	return entry
}

// SampleHistoryEntryWithCache creates a sample history entry with cache hit
func SampleHistoryEntryWithCache(id, provider, model string, cacheHit bool) *core.HistoryEntry {
	entry := SampleHistoryEntry(id, provider, model)
	entry.Cache = core.CacheMeta{
		Hit:    cacheHit,
		Source: "memory",
		TTL:    3600,
	}
	return entry
}

// ============================================================================
// Batch Entry Generators
// ============================================================================

// GenerateHistoryEntries creates n sample history entries
func GenerateHistoryEntries(n int, provider, model string) []*core.HistoryEntry {
	entries := make([]*core.HistoryEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = SampleHistoryEntry(
			"entry-"+string(rune(i)),
			provider,
			model,
		)
	}
	return entries
}

// GenerateMixedHistoryEntries creates entries with mixed success/error states
func GenerateMixedHistoryEntries(successCount, errorCount int) []*core.HistoryEntry {
	total := successCount + errorCount
	entries := make([]*core.HistoryEntry, total)

	for i := 0; i < successCount; i++ {
		entries[i] = SampleHistoryEntry("success-"+string(rune(i)), "openai", "gpt-4")
	}

	for i := 0; i < errorCount; i++ {
		entries[successCount+i] = SampleHistoryEntryWithError(
			"error-"+string(rune(i)),
			"openai",
			"gpt-4",
			"Simulated error",
		)
	}

	return entries
}
