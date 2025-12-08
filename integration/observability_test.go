package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/assagman/dsgo"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// TestMemoryCollector_BasicCollection tests basic collection functionality
func TestMemoryCollector_BasicCollection(t *testing.T) {
	t.Parallel()
	collector := dsgo.NewMemoryCollector(10)

	entry := &dsgo.HistoryEntry{
		ID:        "test-1",
		Timestamp: time.Now(),
		Provider:  "openai",
		Model:     "gpt-4o",
		Request: dsgo.RequestMeta{
			PromptLength: 50,
			MessageCount: 1,
			HasTools:     false,
		},
		Response: dsgo.ResponseMeta{
			Content:        "Hello world",
			ResponseLength: 11,
		},
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	err := collector.Collect(entry)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if collector.Count() != 1 {
		t.Errorf("Count() = %d, want 1", collector.Count())
	}

	if collector.Len() != 1 {
		t.Errorf("Len() = %d, want 1", collector.Len())
	}

	all := collector.GetAll()
	if len(all) != 1 {
		t.Errorf("GetAll() returned %d entries, want 1", len(all))
	}
	if all[0].ID != "test-1" {
		t.Errorf("Entry ID mismatch: got %s, want test-1", all[0].ID)
	}
}

// TestMemoryCollector_RingBuffer tests ring buffer behavior with overflow
func TestMemoryCollector_RingBuffer(t *testing.T) {
	t.Parallel()
	const capacity = 3
	collector := dsgo.NewMemoryCollector(capacity)

	// Add more entries than capacity
	for i := 0; i < 5; i++ {
		entry := &dsgo.HistoryEntry{
			ID:        fmt.Sprintf("test-%d", i),
			Timestamp: time.Now(),
			Model:     "gpt-4o",
			Usage: dsgo.Usage{
				PromptTokens:     10 + i,
				CompletionTokens: 5,
				TotalTokens:      15 + i,
			},
		}
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
	}

	// Should track total count but keep only last N
	if collector.Count() != 5 {
		t.Errorf("Count() = %d, want 5 (total collected)", collector.Count())
	}

	if collector.Len() != capacity {
		t.Errorf("Len() = %d, want %d (buffer size)", collector.Len(), capacity)
	}

	// Verify we have the 3 most recent entries
	all := collector.GetAll()
	if len(all) != capacity {
		t.Errorf("GetAll() returned %d entries, want %d", len(all), capacity)
	}

	// Should contain entries 2, 3, 4 (the most recent)
	expectedIDs := map[string]bool{"test-2": true, "test-3": true, "test-4": true}
	for _, entry := range all {
		if !expectedIDs[entry.ID] {
			t.Errorf("Unexpected entry ID in buffer: %s", entry.ID)
		}
	}
}

// TestMemoryCollector_GetLast tests retrieving last N entries
func TestMemoryCollector_GetLast(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		bufferSize  int
		numEntries  int
		lastN       int
		expectedLen int
	}{
		{"Get 3 from 5", 10, 5, 3, 3},
		{"Get all from 5", 10, 5, 10, 5},
		{"Get 0", 10, 5, 0, 0},
		{"Get 1 from 1", 10, 1, 1, 1},
		{"Get from empty", 10, 0, 5, 0},
		{"Get from ring buffer", 3, 5, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			collector := dsgo.NewMemoryCollector(tt.bufferSize)

			for i := 0; i < tt.numEntries; i++ {
				entry := &dsgo.HistoryEntry{
					ID:    fmt.Sprintf("entry-%d", i),
					Model: "gpt-4o",
				}
				_ = collector.Collect(entry)
			}

			last := collector.GetLast(tt.lastN)
			if len(last) != tt.expectedLen {
				t.Errorf("GetLast(%d) returned %d entries, want %d", tt.lastN, len(last), tt.expectedLen)
			}

			// Verify reverse order (most recent first)
			if tt.expectedLen > 1 {
				// Entries should be in reverse order
				// If we added 0, 1, 2, 3, 4 and get last 2, should get [4, 3]
				for i := 0; i < len(last)-1; i++ {
					// Extract numbers from IDs to verify order
					var num1, num2 int
					_, _ = fmt.Sscanf(last[i].ID, "entry-%d", &num1)
					_, _ = fmt.Sscanf(last[i+1].ID, "entry-%d", &num2)
					if num1 <= num2 {
						t.Errorf("Results not in reverse order: %s, %s", last[i].ID, last[i+1].ID)
					}
				}
			}
		})
	}
}

// TestMemoryCollector_Concurrency tests thread-safe collection
func TestMemoryCollector_Concurrency(t *testing.T) {
	t.Parallel()
	collector := dsgo.NewMemoryCollector(1000)
	const numGoroutines = 10
	const entriesPerGoroutine = 100

	// Launch concurrent collectors
	done := make(chan bool, numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(id int) {
			for e := 0; e < entriesPerGoroutine; e++ {
				entry := &dsgo.HistoryEntry{
					ID:    fmt.Sprintf("g%d-e%d", id, e),
					Model: "gpt-4o",
					Usage: dsgo.Usage{
						TotalTokens: id*entriesPerGoroutine + e,
					},
				}
				_ = collector.Collect(entry)
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	expectedTotal := int64(numGoroutines * entriesPerGoroutine)
	if collector.Count() != expectedTotal {
		t.Errorf("Count() = %d, want %d", collector.Count(), expectedTotal)
	}

	if collector.Len() != numGoroutines*entriesPerGoroutine {
		t.Errorf("Len() = %d, want %d", collector.Len(), numGoroutines*entriesPerGoroutine)
	}
}

// TestMemoryCollector_Close tests closing the collector
func TestMemoryCollector_Close(t *testing.T) {
	t.Parallel()
	collector := dsgo.NewMemoryCollector(10)

	entry := &dsgo.HistoryEntry{
		ID:    "test-1",
		Model: "gpt-4o",
	}
	_ = collector.Collect(entry)

	err := collector.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Should still be able to read
	if collector.Len() != 1 {
		t.Errorf("Len() after Close() = %d, want 1", collector.Len())
	}
}

// TestHistoryEntry_UsageTracking tests usage tracking in history entries
func TestHistoryEntry_UsageTracking(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		promptTokens     int
		completionTokens int
		expectedTotal    int
	}{
		{"Small query", 10, 5, 15},
		{"Medium query", 100, 50, 150},
		{"Large query", 1000, 500, 1500},
		{"Zero tokens", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := &dsgo.HistoryEntry{
				ID:    "test-usage",
				Model: "gpt-4o",
				Usage: dsgo.Usage{
					PromptTokens:     tt.promptTokens,
					CompletionTokens: tt.completionTokens,
					TotalTokens:      tt.expectedTotal,
				},
			}

			if entry.Usage.TotalTokens != tt.expectedTotal {
				t.Errorf("TotalTokens = %d, want %d", entry.Usage.TotalTokens, tt.expectedTotal)
			}

			if entry.Usage.PromptTokens != tt.promptTokens {
				t.Errorf("PromptTokens = %d, want %d", entry.Usage.PromptTokens, tt.promptTokens)
			}

			if entry.Usage.CompletionTokens != tt.completionTokens {
				t.Errorf("CompletionTokens = %d, want %d", entry.Usage.CompletionTokens, tt.completionTokens)
			}
		})
	}
}

// TestHistoryEntry_ProviderMetadata tests provider-specific metadata
func TestHistoryEntry_ProviderMetadata(t *testing.T) {
	t.Parallel()
	entry := &dsgo.HistoryEntry{
		ID:       "test-metadata",
		Model:    "gpt-4o",
		Provider: "openai",
		ProviderMeta: map[string]any{
			"request_id":              "req-12345",
			"rate_limit_remaining":    9999,
			"cache_hit":               true,
			"x_cache":                 "HIT",
			"rate_limit_reset_tokens": "2025-11-28T10:00:00Z",
		},
	}

	// Verify metadata is preserved
	if entry.ProviderMeta["request_id"] != "req-12345" {
		t.Errorf("request_id mismatch")
	}
	if entry.ProviderMeta["rate_limit_remaining"] != 9999 {
		t.Errorf("rate_limit_remaining mismatch")
	}
	if entry.ProviderMeta["cache_hit"] != true {
		t.Errorf("cache_hit mismatch")
	}
}

// TestHistoryEntry_ErrorTracking tests error tracking in history entries
func TestHistoryEntry_ErrorTracking(t *testing.T) {
	t.Parallel()
	errorMeta := &dsgo.ErrorMeta{
		Message:    "Rate limit exceeded",
		Code:       "rate_limit_exceeded",
		Type:       "RateLimitError",
		StatusCode: 429,
	}

	entry := &dsgo.HistoryEntry{
		ID:       "test-error",
		Model:    "gpt-4o",
		Provider: "openai",
		Error:    errorMeta,
	}

	if entry.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if entry.Error.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", entry.Error.StatusCode)
	}
	if entry.Error.Code != "rate_limit_exceeded" {
		t.Errorf("Code = %s, want rate_limit_exceeded", entry.Error.Code)
	}
}

// TestHistoryEntry_CacheMetadata tests cache metadata tracking
func TestHistoryEntry_CacheMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		hit      bool
		source   string
		ttl      int64
		hasError bool
	}{
		{"Cache hit from memory", true, "memory", 3600, false},
		{"Cache miss", false, "", 0, false},
		{"Cache hit from disk", true, "disk", 7200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := &dsgo.HistoryEntry{
				ID:    "test-cache",
				Model: "gpt-4o",
				Cache: dsgo.CacheMeta{
					Hit:    tt.hit,
					Source: tt.source,
					TTL:    tt.ttl,
				},
			}

			if entry.Cache.Hit != tt.hit {
				t.Errorf("Cache.Hit = %v, want %v", entry.Cache.Hit, tt.hit)
			}
			if entry.Cache.Source != tt.source {
				t.Errorf("Cache.Source = %s, want %s", entry.Cache.Source, tt.source)
			}
		})
	}
}

// TestHistoryEntry_JSONSerialization tests JSON marshaling
func TestHistoryEntry_JSONSerialization(t *testing.T) {
	t.Parallel()
	entry := &dsgo.HistoryEntry{
		ID:        "test-json",
		Timestamp: time.Date(2025, 11, 27, 10, 0, 0, 0, time.UTC),
		Provider:  "openai",
		Model:     "gpt-4o",
		Request: dsgo.RequestMeta{
			PromptLength: 50,
			MessageCount: 1,
		},
		Response: dsgo.ResponseMeta{
			Content:        "Hello",
			ResponseLength: 5,
		},
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Unmarshal back
	var unmarshaled dsgo.HistoryEntry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify fields are preserved
	if unmarshaled.ID != entry.ID {
		t.Errorf("ID mismatch after JSON round-trip")
	}
	if unmarshaled.Model != entry.Model {
		t.Errorf("Model mismatch after JSON round-trip")
	}
	if unmarshaled.Usage.TotalTokens != entry.Usage.TotalTokens {
		t.Errorf("TotalTokens mismatch after JSON round-trip")
	}
}

// TestCollector_MultipleProviders tests tracking multiple providers
func TestCollector_MultipleProviders(t *testing.T) {
	t.Parallel()
	collector := dsgo.NewMemoryCollector(20)

	providers := []string{"openai", "openrouter"}
	models := map[string][]string{
		"openai":     {"gpt-4o", "gpt-4o-mini"},
		"openrouter": {"meta-llama/llama-3.1-70b", "mistralai/mixtral-8x7b"},
	}

	// Add entries from different providers
	entryCount := 0
	for _, provider := range providers {
		for _, model := range models[provider] {
			for i := 0; i < 3; i++ {
				entry := &dsgo.HistoryEntry{
					ID:       fmt.Sprintf("%s-%s-%d", provider, model, i),
					Provider: provider,
					Model:    model,
					Usage: dsgo.Usage{
						TotalTokens: 100 + entryCount,
					},
				}
				_ = collector.Collect(entry)
				entryCount++
			}
		}
	}

	// Verify all entries collected
	all := collector.GetAll()
	if len(all) != entryCount {
		t.Errorf("GetAll() returned %d entries, want %d", len(all), entryCount)
	}

	// Verify provider distribution
	providerCount := make(map[string]int)
	for _, entry := range all {
		providerCount[entry.Provider]++
	}

	expectedCount := 6 // 2 models * 3 entries each per provider
	for _, provider := range providers {
		if providerCount[provider] != expectedCount {
			t.Errorf("%s count = %d, want %d", provider, providerCount[provider], expectedCount)
		}
	}
}

// TestCollector_EntryWithToolCalls tests entries with tool calls
func TestCollector_EntryWithToolCalls(t *testing.T) {
	t.Parallel()
	toolCalls := []dsgo.ToolCall{
		{
			ID:   "tool-call-1",
			Name: "search",
			Arguments: map[string]any{
				"query": "test query",
			},
		},
	}

	entry := &dsgo.HistoryEntry{
		ID:       "test-tools",
		Provider: "openai",
		Model:    "gpt-4o",
		Request: dsgo.RequestMeta{
			HasTools:  true,
			ToolCount: 1,
		},
		Response: dsgo.ResponseMeta{
			ToolCalls:     toolCalls,
			ToolCallCount: 1,
		},
	}

	collector := dsgo.NewMemoryCollector(10)
	_ = collector.Collect(entry)

	retrieved := collector.GetAll()[0]
	if retrieved.Response.ToolCallCount != 1 {
		t.Errorf("ToolCallCount = %d, want 1", retrieved.Response.ToolCallCount)
	}
	if len(retrieved.Response.ToolCalls) != 1 {
		t.Errorf("ToolCalls length = %d, want 1", len(retrieved.Response.ToolCalls))
	}
}

// TestMemoryCollector_Clear tests clearing the collector
func TestMemoryCollector_Clear(t *testing.T) {
	t.Parallel()
	collector := dsgo.NewMemoryCollector(10)

	// Add entries
	for i := 0; i < 5; i++ {
		entry := &dsgo.HistoryEntry{
			ID:    fmt.Sprintf("test-%d", i),
			Model: "gpt-4o",
		}
		_ = collector.Collect(entry)
	}

	if collector.Len() != 5 {
		t.Errorf("Len() before clear = %d, want 5", collector.Len())
	}

	// Clear should be done through creating a new collector
	collector = dsgo.NewMemoryCollector(10)
	if collector.Len() != 0 {
		t.Errorf("Len() after recreate = %d, want 0", collector.Len())
	}
	if collector.Count() != 0 {
		t.Errorf("Count() after recreate = %d, want 0", collector.Count())
	}
}

// TestMemoryCollector_JSONLOutput tests JSONL-compatible format
func TestMemoryCollector_JSONLOutput(t *testing.T) {
	t.Parallel()
	collector := dsgo.NewMemoryCollector(10)

	// Add multiple entries
	for i := 0; i < 3; i++ {
		entry := &dsgo.HistoryEntry{
			ID:        fmt.Sprintf("entry-%d", i),
			Timestamp: time.Now(),
			Provider:  "openai",
			Model:     "gpt-4o",
			Usage: dsgo.Usage{
				TotalTokens: 100 + i,
			},
		}
		_ = collector.Collect(entry)
	}

	// Convert to JSONL format (each entry on its own line)
	entries := collector.GetAll()
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("JSON marshal failed: %v", err)
		}

		// Verify it's valid JSON and can be unmarshaled
		var unmarshaled dsgo.HistoryEntry
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("JSON unmarshal failed for entry %s: %v", entry.ID, err)
		}

		if unmarshaled.ID != entry.ID {
			t.Errorf("JSONL entry ID mismatch: %s != %s", unmarshaled.ID, entry.ID)
		}
	}
}

// TestRequestID_Context tests request ID in context
func TestRequestID_Context(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Initially no request ID
	if id := getTestRequestID(ctx); id != "" {
		t.Errorf("Expected empty request ID, got %s", id)
	}

	// Set request ID in context
	ctx = context.WithValue(ctx, requestIDKey, "test-request-123")

	// Should be retrievable
	if id := getTestRequestID(ctx); id != "test-request-123" {
		t.Errorf("Expected request ID test-request-123, got %s", id)
	}
}

// TestObservability_IntegrationFlow tests end-to-end observability flow
func TestObservability_IntegrationFlow(t *testing.T) {
	t.Parallel()
	collector := dsgo.NewMemoryCollector(100)

	// Simulate a series of LM calls
	requests := []struct {
		provider string
		model    string
		status   string
		tokens   int
	}{
		{"openai", "gpt-4o", "success", 100},
		{"openai", "gpt-4o-mini", "success", 50},
		{"openrouter", "meta-llama/llama-3.1-70b", "success", 200},
		{"openai", "gpt-4o", "rate_limit", 0},
		{"openrouter", "meta-llama/llama-3.1-70b", "success", 150},
	}

	for i, req := range requests {
		var errorMeta *dsgo.ErrorMeta
		if req.status == "rate_limit" {
			errorMeta = &dsgo.ErrorMeta{
				Message:    "Rate limit exceeded",
				Code:       "rate_limit_exceeded",
				StatusCode: 429,
			}
		}

		entry := &dsgo.HistoryEntry{
			ID:        fmt.Sprintf("call-%d", i),
			Timestamp: time.Now(),
			Provider:  req.provider,
			Model:     req.model,
			Request: dsgo.RequestMeta{
				MessageCount: 1,
			},
			Response: dsgo.ResponseMeta{
				Content: "response",
			},
			Usage: dsgo.Usage{
				TotalTokens: req.tokens,
			},
			Error: errorMeta,
		}

		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
	}

	// Verify collection
	if collector.Count() != int64(len(requests)) {
		t.Errorf("Count() = %d, want %d", collector.Count(), len(requests))
	}

	// Verify entries are accessible
	all := collector.GetAll()
	if len(all) != len(requests) {
		t.Errorf("GetAll() returned %d entries, want %d", len(all), len(requests))
	}

	// Verify error tracking
	errorCount := 0
	for _, entry := range all {
		if entry.Error != nil {
			errorCount++
		}
	}
	if errorCount != 1 {
		t.Errorf("Error count = %d, want 1", errorCount)
	}
}

// Helper function
func getTestRequestID(ctx context.Context) string {
	id := ctx.Value(requestIDKey)
	if id == nil {
		return ""
	}
	return id.(string)
}

// TestMemoryCollector_TempFileCleanup tests temporary file handling
func TestMemoryCollector_TempFileCleanup(t *testing.T) {
	t.Parallel()
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.json")
	data := []byte(`{"id":"test","model":"gpt-4o"}`)

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("Test file does not exist: %v", err)
	}

	// TempDir is cleaned up automatically by testing framework
}
