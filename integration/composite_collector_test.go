package integration

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
)

// ============================================================================
// Composite Collector Dual Sink Tests
// ============================================================================

// TestCompositeCollector_DualSink tests Memory + JSONL dual collector setup.
// Validates:
// - Entry delivered to both collectors
// - Both collectors operate independently
// - Data integrity in both sinks
func TestCompositeCollector_DualSink(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	// Create temp directory for JSONL file
	tempDir := t.TempDir()
	jsonlPath := filepath.Join(tempDir, "events.jsonl")

	// Create collectors
	memoryCollector := core.NewMemoryCollector(10)
	jsonlCollector, err := core.NewJSONLCollector(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	// Create composite collector
	composite := core.NewCompositeCollector(memoryCollector, jsonlCollector)

	// Create test entry
	entry := createTestHistoryEntry("test-001", "openai", "gpt-4")

	// Collect entry
	err = composite.Collect(entry)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// Verify memory collector received entry
	memEntries := memoryCollector.GetAll()
	if len(memEntries) != 1 {
		t.Errorf("Expected 1 memory entry, got %d", len(memEntries))
	}
	if memEntries[0].ID != "test-001" {
		t.Errorf("Expected entry ID 'test-001', got %q", memEntries[0].ID)
	}

	// Close composite to flush JSONL
	if err := composite.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify JSONL file contains entry
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to read JSONL file: %v", err)
	}

	var jsonlEntry core.HistoryEntry
	if err := json.Unmarshal(data, &jsonlEntry); err != nil {
		t.Fatalf("Failed to parse JSONL entry: %v", err)
	}

	if jsonlEntry.ID != "test-001" {
		t.Errorf("JSONL entry ID = %q, want 'test-001'", jsonlEntry.ID)
	}
	if jsonlEntry.Provider != "openai" {
		t.Errorf("JSONL entry Provider = %q, want 'openai'", jsonlEntry.Provider)
	}
}

// TestCompositeCollector_IndependentOperation verifies collectors work independently.
func TestCompositeCollector_IndependentOperation(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	tempDir := t.TempDir()
	jsonlPath := filepath.Join(tempDir, "events.jsonl")

	memoryCollector := core.NewMemoryCollector(5)
	jsonlCollector, err := core.NewJSONLCollector(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	composite := core.NewCompositeCollector(memoryCollector, jsonlCollector)

	// Collect multiple entries
	entries := []struct {
		id       string
		provider string
		model    string
	}{
		{"entry-1", "openai", "gpt-4"},
		{"entry-2", "openrouter", "claude-3"},
		{"entry-3", "openai", "gpt-4o"},
	}

	for _, e := range entries {
		entry := createTestHistoryEntry(e.id, e.provider, e.model)
		if err := composite.Collect(entry); err != nil {
			t.Fatalf("Collect failed for %s: %v", e.id, err)
		}
	}

	// Verify memory collector count
	if memoryCollector.Len() != 3 {
		t.Errorf("Memory collector Len() = %d, want 3", memoryCollector.Len())
	}

	// Verify JSONL count
	if jsonlCollector.Count() != 3 {
		t.Errorf("JSONL collector Count() = %d, want 3", jsonlCollector.Count())
	}

	if err := composite.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify JSONL file has 3 lines
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to read JSONL file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Errorf("JSONL file has %d lines, want 3", len(lines))
	}
}

// ============================================================================
// Composite Collector Partial Failure Tests
// ============================================================================

// TestCompositeCollector_PartialFailure tests behavior when one collector fails.
// Validates:
// - Working collector still receives entry
// - Error is returned from Collect
// - Error aggregation on close
func TestCompositeCollector_PartialFailure(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	tempDir := t.TempDir()
	jsonlPath := filepath.Join(tempDir, "events.jsonl")

	memoryCollector := core.NewMemoryCollector(10)
	jsonlCollector, err := core.NewJSONLCollector(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	// Close JSONL collector to simulate failure
	if err := jsonlCollector.Close(); err != nil {
		t.Fatalf("Failed to close JSONL collector: %v", err)
	}

	composite := core.NewCompositeCollector(memoryCollector, jsonlCollector)

	entry := createTestHistoryEntry("partial-fail-001", "openai", "gpt-4")

	// Collect should return error (from closed JSONL collector)
	err = composite.Collect(entry)
	if err == nil {
		t.Error("Expected error from closed JSONL collector, got nil")
	}

	// Memory collector should still have received the entry
	memEntries := memoryCollector.GetAll()
	if len(memEntries) != 1 {
		t.Errorf("Expected 1 memory entry despite JSONL failure, got %d", len(memEntries))
	}
	if len(memEntries) > 0 && memEntries[0].ID != "partial-fail-001" {
		t.Errorf("Memory entry ID = %q, want 'partial-fail-001'", memEntries[0].ID)
	}
}

// TestCompositeCollector_PartialFailureMultipleEntries tests partial failure across multiple entries.
func TestCompositeCollector_PartialFailureMultipleEntries(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	memoryCollector := core.NewMemoryCollector(10)
	failingCollector := &FailingCollector{
		FailAfter: 2,
	}

	composite := core.NewCompositeCollector(memoryCollector, failingCollector)

	var collectErrors []error
	for i := 0; i < 5; i++ {
		entry := createTestHistoryEntry("multi-"+string(rune('A'+i)), "openai", "gpt-4")
		if err := composite.Collect(entry); err != nil {
			collectErrors = append(collectErrors, err)
		}
	}

	// Should have 3 errors (entries 3, 4, 5 fail after FailAfter=2)
	if len(collectErrors) != 3 {
		t.Errorf("Expected 3 collect errors, got %d", len(collectErrors))
	}

	// Memory collector should have all 5 entries
	memEntries := memoryCollector.GetAll()
	if len(memEntries) != 5 {
		t.Errorf("Expected 5 memory entries, got %d", len(memEntries))
	}

	// Failing collector should have only 2
	if failingCollector.SuccessCount != 2 {
		t.Errorf("Failing collector SuccessCount = %d, want 2", failingCollector.SuccessCount)
	}
}

// TestCompositeCollector_ErrorAggregation tests error aggregation on Close().
func TestCompositeCollector_ErrorAggregation(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	failingCollector1 := &FailingCollector{CloseError: errors.New("close error 1")}
	failingCollector2 := &FailingCollector{CloseError: errors.New("close error 2")}
	memoryCollector := core.NewMemoryCollector(10)

	composite := core.NewCompositeCollector(failingCollector1, memoryCollector, failingCollector2)

	err := composite.Close()
	if err == nil {
		t.Fatal("Expected error from Close, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "2 collector(s)") {
		t.Errorf("Error should mention 2 failed collectors: %v", err)
	}
}

// ============================================================================
// Composite Collector Multiple Entries Tests
// ============================================================================

// TestCompositeCollector_MultipleEntries tests handling multiple entries across collectors.
func TestCompositeCollector_MultipleEntries(t *testing.T) {
	tests := []struct {
		name          string
		entryCount    int
		memorySize    int
		expectedInMem int
	}{
		{
			name:          "all entries fit in memory",
			entryCount:    5,
			memorySize:    10,
			expectedInMem: 5,
		},
		{
			name:          "ring buffer overflow",
			entryCount:    15,
			memorySize:    10,
			expectedInMem: 10,
		},
		{
			name:          "exact fit",
			entryCount:    10,
			memorySize:    10,
			expectedInMem: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			jsonlPath := filepath.Join(tempDir, "events.jsonl")

			memoryCollector := core.NewMemoryCollector(tt.memorySize)
			jsonlCollector, err := core.NewJSONLCollector(jsonlPath)
			if err != nil {
				t.Fatalf("Failed to create JSONL collector: %v", err)
			}

			composite := core.NewCompositeCollector(memoryCollector, jsonlCollector)

			// Collect entries
			for i := 0; i < tt.entryCount; i++ {
				entry := createTestHistoryEntry(
					"entry-"+string(rune('A'+i%26)),
					"openai",
					"gpt-4",
				)
				if err := composite.Collect(entry); err != nil {
					t.Fatalf("Collect failed at %d: %v", i, err)
				}
			}

			// Verify memory collector
			memEntries := memoryCollector.GetAll()
			if len(memEntries) != tt.expectedInMem {
				t.Errorf("Memory entries = %d, want %d", len(memEntries), tt.expectedInMem)
			}

			// Verify JSONL has all entries (no ring buffer limit)
			if jsonlCollector.Count() != int64(tt.entryCount) {
				t.Errorf("JSONL count = %d, want %d", jsonlCollector.Count(), tt.entryCount)
			}

			if err := composite.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			// Verify JSONL file line count
			file, err := os.Open(jsonlPath)
			if err != nil {
				t.Fatalf("Failed to open JSONL file: %v", err)
			}
			defer func() { _ = file.Close() }()

			scanner := bufio.NewScanner(file)
			lineCount := 0
			for scanner.Scan() {
				lineCount++
			}
			if lineCount != tt.entryCount {
				t.Errorf("JSONL lines = %d, want %d", lineCount, tt.entryCount)
			}
			_ = file.Close()
		})
	}
}

// TestCompositeCollector_DataIntegrity verifies data integrity across collectors.
func TestCompositeCollector_DataIntegrity(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	tempDir := t.TempDir()
	jsonlPath := filepath.Join(tempDir, "events.jsonl")

	memoryCollector := core.NewMemoryCollector(10)
	jsonlCollector, err := core.NewJSONLCollector(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	composite := core.NewCompositeCollector(memoryCollector, jsonlCollector)

	// Create entry with detailed data
	entry := &core.HistoryEntry{
		ID:        "integrity-test-001",
		Timestamp: time.Now(),
		SessionID: "session-xyz",
		Provider:  "openai",
		Model:     "gpt-4-turbo",
		Request: core.RequestMeta{
			Messages: []core.Message{
				{Role: "user", Content: "Hello world"},
			},
			PromptLength:   11,
			MessageCount:   1,
			HasTools:       true,
			ToolCount:      2,
			ResponseFormat: "json",
		},
		Response: core.ResponseMeta{
			Content:        `{"result": "success"}`,
			FinishReason:   "stop",
			ResponseLength: 21,
		},
		Usage: core.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.005,
		},
		Cache: core.CacheMeta{
			Hit:    false,
			Source: "",
		},
		ProviderMeta: map[string]any{
			"request_id": "req-12345",
		},
	}

	if err := composite.Collect(entry); err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if err := composite.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify memory collector data integrity
	memEntries := memoryCollector.GetAll()
	if len(memEntries) != 1 {
		t.Fatalf("Expected 1 memory entry, got %d", len(memEntries))
	}
	memEntry := memEntries[0]
	assertEntryIntegrity(t, "memory", memEntry, entry)

	// Verify JSONL data integrity
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to read JSONL file: %v", err)
	}

	var jsonlEntry core.HistoryEntry
	if err := json.Unmarshal(data, &jsonlEntry); err != nil {
		t.Fatalf("Failed to parse JSONL entry: %v", err)
	}
	assertEntryIntegrity(t, "jsonl", &jsonlEntry, entry)
}

// ============================================================================
// Composite Collector Close Tests
// ============================================================================

// TestCompositeCollector_CloseAll verifies Close() closes all collectors.
func TestCompositeCollector_CloseAll(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	tempDir := t.TempDir()
	jsonlPath1 := filepath.Join(tempDir, "events1.jsonl")
	jsonlPath2 := filepath.Join(tempDir, "events2.jsonl")

	memoryCollector := core.NewMemoryCollector(10)
	jsonlCollector1, err := core.NewJSONLCollector(jsonlPath1)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector 1: %v", err)
	}
	jsonlCollector2, err := core.NewJSONLCollector(jsonlPath2)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector 2: %v", err)
	}

	composite := core.NewCompositeCollector(memoryCollector, jsonlCollector1, jsonlCollector2)

	// Verify collector count
	if composite.Len() != 3 {
		t.Errorf("Composite Len() = %d, want 3", composite.Len())
	}

	// Collect an entry
	entry := createTestHistoryEntry("close-test-001", "openai", "gpt-4")
	if err := composite.Collect(entry); err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// Close composite
	if err := composite.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify JSONL files were flushed and closed by attempting to collect
	// After close, JSONL collectors should fail
	err = jsonlCollector1.Collect(entry)
	if err == nil {
		t.Error("Expected error when collecting to closed JSONL collector 1")
	}

	err = jsonlCollector2.Collect(entry)
	if err == nil {
		t.Error("Expected error when collecting to closed JSONL collector 2")
	}

	// Verify files exist and have content
	for _, path := range []string{jsonlPath1, jsonlPath2} {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("JSONL file %s should exist: %v", path, err)
		} else if info.Size() == 0 {
			t.Errorf("JSONL file %s should not be empty", path)
		}
	}
}

// TestCompositeCollector_CloseIdempotent tests that Close() can be called multiple times.
func TestCompositeCollector_CloseIdempotent(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	memoryCollector := core.NewMemoryCollector(10)
	composite := core.NewCompositeCollector(memoryCollector)

	// First close should succeed
	if err := composite.Close(); err != nil {
		t.Errorf("First Close() failed: %v", err)
	}

	// Second close should also succeed (memory collector Close is no-op)
	if err := composite.Close(); err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}

// TestCompositeCollector_AddCollector tests dynamic addition of collectors.
func TestCompositeCollector_AddCollector(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	memoryCollector1 := core.NewMemoryCollector(10)
	composite := core.NewCompositeCollector(memoryCollector1)

	if composite.Len() != 1 {
		t.Errorf("Initial Len() = %d, want 1", composite.Len())
	}

	// Add another collector
	memoryCollector2 := core.NewMemoryCollector(10)
	composite.Add(memoryCollector2)

	if composite.Len() != 2 {
		t.Errorf("After Add() Len() = %d, want 2", composite.Len())
	}

	// Collect entry - should go to both
	entry := createTestHistoryEntry("add-test-001", "openai", "gpt-4")
	if err := composite.Collect(entry); err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if memoryCollector1.Len() != 1 {
		t.Errorf("Collector 1 Len() = %d, want 1", memoryCollector1.Len())
	}
	if memoryCollector2.Len() != 1 {
		t.Errorf("Collector 2 Len() = %d, want 1", memoryCollector2.Len())
	}
}

// TestCompositeCollector_EmptyComposite tests behavior with no collectors.
func TestCompositeCollector_EmptyComposite(t *testing.T) {
	_, cancel := ContextWithTimeout(10 * time.Second)
	defer cancel()

	composite := core.NewCompositeCollector()

	if composite.Len() != 0 {
		t.Errorf("Empty composite Len() = %d, want 0", composite.Len())
	}

	// Collect should succeed (no-op)
	entry := createTestHistoryEntry("empty-test-001", "openai", "gpt-4")
	if err := composite.Collect(entry); err != nil {
		t.Errorf("Collect on empty composite failed: %v", err)
	}

	// Close should succeed
	if err := composite.Close(); err != nil {
		t.Errorf("Close on empty composite failed: %v", err)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// createTestHistoryEntry creates a test history entry with basic fields.
func createTestHistoryEntry(id, provider, model string) *core.HistoryEntry {
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
			PromptLength:   12,
			MessageCount:   1,
			ResponseFormat: "text",
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

// assertEntryIntegrity verifies that an entry matches expected values.
func assertEntryIntegrity(t *testing.T, source string, got, want *core.HistoryEntry) {
	t.Helper()

	if got.ID != want.ID {
		t.Errorf("%s: ID = %q, want %q", source, got.ID, want.ID)
	}
	if got.Provider != want.Provider {
		t.Errorf("%s: Provider = %q, want %q", source, got.Provider, want.Provider)
	}
	if got.Model != want.Model {
		t.Errorf("%s: Model = %q, want %q", source, got.Model, want.Model)
	}
	if got.Usage.TotalTokens != want.Usage.TotalTokens {
		t.Errorf("%s: TotalTokens = %d, want %d", source, got.Usage.TotalTokens, want.Usage.TotalTokens)
	}
	if got.Usage.Cost != want.Usage.Cost {
		t.Errorf("%s: Cost = %f, want %f", source, got.Usage.Cost, want.Usage.Cost)
	}
	if got.Request.PromptLength != want.Request.PromptLength {
		t.Errorf("%s: PromptLength = %d, want %d", source, got.Request.PromptLength, want.Request.PromptLength)
	}
	if got.Response.FinishReason != want.Response.FinishReason {
		t.Errorf("%s: FinishReason = %q, want %q", source, got.Response.FinishReason, want.Response.FinishReason)
	}
}

// ============================================================================
// Mock Collectors
// ============================================================================

// FailingCollector is a test collector that fails after a configurable number of calls.
type FailingCollector struct {
	FailAfter    int
	SuccessCount int
	CloseError   error
}

// Collect implements core.Collector.
func (fc *FailingCollector) Collect(entry *core.HistoryEntry) error {
	if fc.SuccessCount >= fc.FailAfter {
		return errors.New("simulated collector failure")
	}
	fc.SuccessCount++
	return nil
}

// Close implements core.Collector.
func (fc *FailingCollector) Close() error {
	return fc.CloseError
}
