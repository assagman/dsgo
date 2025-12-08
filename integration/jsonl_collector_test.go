package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/assagman/dsgo"
)

// ============================================================================
// JSONL Collector File Output Tests
// ============================================================================

// TestJSONLCollector_FileOutput tests writing 100 entries and verifying valid JSONL format.
// Validates:
// - All 100 entries are written
// - Each line is valid JSON
// - Each line can be parsed back to HistoryEntry
// - Entry count matches expected
func TestJSONLCollector_FileOutput(t *testing.T) {
	t.Parallel()
	_, cancel := ContextWithTimeout(30 * time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_output.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	const entryCount = 100

	for i := 0; i < entryCount; i++ {
		entry := createJSONLTestEntry(i)
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Failed to collect entry %d: %v", i, err)
		}
	}

	if collector.Count() != int64(entryCount) {
		t.Errorf("Expected count %d, got %d", entryCount, collector.Count())
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close collector: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry dsgo.HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Line %d is not valid JSON: %v\nLine: %s", lineCount+1, err, truncateString(line, 100))
		}

		expectedID := fmt.Sprintf("entry-%d", lineCount)
		if entry.ID != expectedID {
			t.Errorf("Line %d: expected ID %q, got %q", lineCount+1, expectedID, entry.ID)
		}

		lineCount++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	if lineCount != entryCount {
		t.Errorf("Expected %d lines, got %d", entryCount, lineCount)
	}
}

// TestJSONLCollector_FileOutputTableDriven uses table-driven tests for various entry configurations.
func TestJSONLCollector_FileOutputTableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		entryCount int
		setupEntry func(i int) *dsgo.HistoryEntry
	}{
		{
			name:       "basic entries",
			entryCount: 10,
			setupEntry: createJSONLTestEntry,
		},
		{
			name:       "entries with tool calls",
			entryCount: 10,
			setupEntry: func(i int) *dsgo.HistoryEntry {
				entry := createJSONLTestEntry(i)
				entry.Response.ToolCalls = []dsgo.ToolCall{
					{
						ID:        fmt.Sprintf("call-%d", i),
						Name:      "test_tool",
						Arguments: map[string]any{"arg": i},
					},
				}
				entry.Response.ToolCallCount = 1
				return entry
			},
		},
		{
			name:       "entries with errors",
			entryCount: 10,
			setupEntry: func(i int) *dsgo.HistoryEntry {
				entry := createJSONLTestEntry(i)
				entry.Error = &dsgo.ErrorMeta{
					Message:    fmt.Sprintf("Error %d", i),
					Code:       "TEST_ERROR",
					StatusCode: 500,
				}
				return entry
			},
		},
		{
			name:       "entries with cache hits",
			entryCount: 10,
			setupEntry: func(i int) *dsgo.HistoryEntry {
				entry := createJSONLTestEntry(i)
				entry.Cache.Hit = i%2 == 0
				entry.Cache.Source = "memory"
				entry.Cache.TTL = 3600
				return entry
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.jsonl")

			collector, err := dsgo.NewJSONLCollector(filePath)
			if err != nil {
				t.Fatalf("Failed to create collector: %v", err)
			}

			for i := 0; i < tt.entryCount; i++ {
				entry := tt.setupEntry(i)
				if err := collector.Collect(entry); err != nil {
					t.Fatalf("Failed to collect entry %d: %v", i, err)
				}
			}

			if err := collector.Close(); err != nil {
				t.Fatalf("Failed to close collector: %v", err)
			}

			lines := readJSONLFile(t, filePath)
			if len(lines) != tt.entryCount {
				t.Errorf("Expected %d entries, got %d", tt.entryCount, len(lines))
			}
		})
	}
}

// ============================================================================
// JSONL Collector Concurrent Write Tests
// ============================================================================

// TestJSONLCollector_ConcurrentWrites tests 10 goroutines writing simultaneously.
// Validates:
// - No data corruption
// - All entries are written
// - Each line is valid JSON
// - Line count matches expected
func TestJSONLCollector_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	_, cancel := ContextWithTimeout(60 * time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "concurrent_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	const goroutineCount = 10
	const entriesPerGoroutine = 100
	expectedTotal := goroutineCount * entriesPerGoroutine

	var wg sync.WaitGroup
	errors := make(chan error, expectedTotal)

	for g := 0; g < goroutineCount; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				entry := &dsgo.HistoryEntry{
					ID:        fmt.Sprintf("g%d-entry-%d", goroutineID, i),
					Timestamp: time.Now(),
					SessionID: fmt.Sprintf("session-%d", goroutineID),
					Provider:  "test-provider",
					Model:     "test-model",
					Request: dsgo.RequestMeta{
						Messages: []dsgo.Message{
							{Role: "user", Content: fmt.Sprintf("Message from goroutine %d, entry %d", goroutineID, i)},
						},
						PromptLength: 50,
						MessageCount: 1,
					},
					Response: dsgo.ResponseMeta{
						Content:        fmt.Sprintf("Response from goroutine %d, entry %d", goroutineID, i),
						FinishReason:   "stop",
						ResponseLength: 50,
					},
					Usage: dsgo.Usage{
						PromptTokens:     10,
						CompletionTokens: 10,
						TotalTokens:      20,
						Cost:             0.001,
					},
				}

				if err := collector.Collect(entry); err != nil {
					errors <- fmt.Errorf("goroutine %d, entry %d: %w", goroutineID, i, err)
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Write error: %v", err)
	}

	if collector.Count() != int64(expectedTotal) {
		t.Errorf("Expected count %d, got %d", expectedTotal, collector.Count())
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close collector: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	lineCount := 0
	seenIDs := make(map[string]bool)
	corruptedLines := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry dsgo.HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			corruptedLines++
			t.Errorf("Corrupted line %d: %v\nLine: %s", lineCount+1, err, truncateString(line, 200))
			lineCount++
			continue
		}

		if seenIDs[entry.ID] {
			t.Errorf("Duplicate ID found: %s", entry.ID)
		}
		seenIDs[entry.ID] = true

		lineCount++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	if corruptedLines > 0 {
		t.Errorf("Found %d corrupted lines out of %d", corruptedLines, lineCount)
	}

	if lineCount != expectedTotal {
		t.Errorf("Expected %d lines, got %d", expectedTotal, lineCount)
	}
}

// TestJSONLCollector_ConcurrentWritesRapidFire tests rapid concurrent writes with no delay.
func TestJSONLCollector_ConcurrentWritesRapidFire(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rapid_fire.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	const goroutineCount = 10
	const entriesPerGoroutine = 50

	var wg sync.WaitGroup
	start := make(chan struct{})

	for g := 0; g < goroutineCount; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			<-start
			for i := 0; i < entriesPerGoroutine; i++ {
				entry := createJSONLConcurrentEntry(gid, i)
				_ = collector.Collect(entry)
			}
		}(g)
	}

	close(start)
	wg.Wait()

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	lines := readJSONLFile(t, filePath)
	expected := goroutineCount * entriesPerGoroutine
	if len(lines) != expected {
		t.Errorf("Expected %d lines, got %d", expected, len(lines))
	}
}

// ============================================================================
// JSONL Collector Large Entry Tests
// ============================================================================

// TestJSONLCollector_LargeEntries tests writing entries with 10KB+ content.
// Validates:
// - Large content is preserved completely
// - No truncation occurs
// - Data integrity maintained
func TestJSONLCollector_LargeEntries(t *testing.T) {
	t.Parallel()
	_, cancel := ContextWithTimeout(30 * time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large_entries.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	largeContentSizes := []int{
		10 * 1024,
		20 * 1024,
		50 * 1024,
		100 * 1024,
	}

	entries := make([]*dsgo.HistoryEntry, len(largeContentSizes))
	for i, size := range largeContentSizes {
		content := generateLargeContent(size, i)
		entries[i] = &dsgo.HistoryEntry{
			ID:        fmt.Sprintf("large-entry-%d", i),
			Timestamp: time.Now(),
			SessionID: "large-content-session",
			Provider:  "test-provider",
			Model:     "test-model",
			Request: dsgo.RequestMeta{
				Messages: []dsgo.Message{
					{Role: "user", Content: content},
				},
				PromptLength: len(content),
				MessageCount: 1,
			},
			Response: dsgo.ResponseMeta{
				Content:        content,
				FinishReason:   "stop",
				ResponseLength: len(content),
			},
			Usage: dsgo.Usage{
				PromptTokens:     size / 4,
				CompletionTokens: size / 4,
				TotalTokens:      size / 2,
				Cost:             float64(size) / 1000000.0,
			},
		}

		if err := collector.Collect(entries[i]); err != nil {
			t.Fatalf("Failed to collect large entry %d (%d bytes): %v", i, size, err)
		}
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close collector: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	lineIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var parsedEntry dsgo.HistoryEntry
		if err := json.Unmarshal([]byte(line), &parsedEntry); err != nil {
			t.Fatalf("Failed to parse line %d: %v", lineIndex+1, err)
		}

		originalEntry := entries[lineIndex]

		if parsedEntry.Response.Content != originalEntry.Response.Content {
			t.Errorf("Entry %d: content mismatch. Original length: %d, Parsed length: %d",
				lineIndex, len(originalEntry.Response.Content), len(parsedEntry.Response.Content))
		}

		if len(parsedEntry.Request.Messages) == 0 {
			t.Errorf("Entry %d: no messages in request", lineIndex)
		} else if parsedEntry.Request.Messages[0].Content != originalEntry.Request.Messages[0].Content {
			t.Errorf("Entry %d: request content mismatch", lineIndex)
		}

		lineIndex++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	if lineIndex != len(largeContentSizes) {
		t.Errorf("Expected %d entries, got %d", len(largeContentSizes), lineIndex)
	}
}

// TestJSONLCollector_LargeEntriesTableDriven uses table-driven tests for various large content scenarios.
func TestJSONLCollector_LargeEntriesTableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentSize int
		description string
	}{
		{"10KB content", 10 * 1024, "minimum large content"},
		{"25KB content", 25 * 1024, "medium large content"},
		{"50KB content", 50 * 1024, "large content"},
		{"100KB content", 100 * 1024, "very large content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "large.jsonl")

			collector, err := dsgo.NewJSONLCollector(filePath)
			if err != nil {
				t.Fatalf("Failed to create collector: %v", err)
			}

			content := generateLargeContent(tt.contentSize, 0)
			entry := &dsgo.HistoryEntry{
				ID:        "large-entry",
				Timestamp: time.Now(),
				SessionID: "test",
				Provider:  "test",
				Model:     "test",
				Response: dsgo.ResponseMeta{
					Content:        content,
					ResponseLength: len(content),
				},
			}

			if err := collector.Collect(entry); err != nil {
				t.Fatalf("Failed to collect: %v", err)
			}
			if err := collector.Close(); err != nil {
				t.Fatalf("Failed to close: %v", err)
			}

			lines := readJSONLFileWithLargeBuffer(t, filePath)
			if len(lines) != 1 {
				t.Fatalf("Expected 1 line, got %d", len(lines))
			}

			var parsed dsgo.HistoryEntry
			if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			if len(parsed.Response.Content) != tt.contentSize {
				t.Errorf("Content length mismatch: expected %d, got %d",
					tt.contentSize, len(parsed.Response.Content))
			}
		})
	}
}

// TestJSONLCollector_LargeEntriesWithSpecialCharacters tests large content with special JSON characters.
func TestJSONLCollector_LargeEntriesWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "special_chars.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	specialContent := generateContentWithSpecialChars(15 * 1024)

	entry := &dsgo.HistoryEntry{
		ID:        "special-chars-entry",
		Timestamp: time.Now(),
		SessionID: "test",
		Provider:  "test",
		Model:     "test",
		Request: dsgo.RequestMeta{
			Messages: []dsgo.Message{
				{Role: "user", Content: specialContent},
			},
		},
		Response: dsgo.ResponseMeta{
			Content: specialContent,
		},
	}

	if err := collector.Collect(entry); err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}
	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	lines := readJSONLFileWithLargeBuffer(t, filePath)
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lines))
	}

	var parsed dsgo.HistoryEntry
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("Failed to parse entry with special chars: %v", err)
	}

	if parsed.Response.Content != specialContent {
		t.Error("Special character content was not preserved correctly")
	}
}

// ============================================================================
// JSONL Collector Cleanup Tests
// ============================================================================

// TestJSONLCollector_Cleanup tests proper file handle closure and cleanup.
// Validates:
// - File handle is closed after Close()
// - Data is flushed before close
// - No data loss on close
func TestJSONLCollector_Cleanup(t *testing.T) {
	t.Parallel()
	_, cancel := ContextWithTimeout(15 * time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cleanup_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create JSONL collector: %v", err)
	}

	const entryCount = 50
	for i := 0; i < entryCount; i++ {
		entry := createJSONLTestEntry(i)
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Failed to collect entry %d: %v", i, err)
		}
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close collector: %v", err)
	}

	if err := collector.Close(); err != nil {
		t.Logf("Second close returned: %v (expected nil or specific error)", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file after close: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		if scanner.Text() != "" {
			lineCount++
		}
	}

	if lineCount != entryCount {
		t.Errorf("Data not flushed properly: expected %d entries, got %d", entryCount, lineCount)
	}
}

// TestJSONLCollector_CleanupTableDriven uses table-driven tests for cleanup scenarios.
func TestJSONLCollector_CleanupTableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		entryCount  int
		closeCount  int
		expectPanic bool
		description string
	}{
		{
			name:        "single close",
			entryCount:  10,
			closeCount:  1,
			expectPanic: false,
			description: "normal single close",
		},
		{
			name:        "double close",
			entryCount:  10,
			closeCount:  2,
			expectPanic: false,
			description: "double close should not panic",
		},
		{
			name:        "triple close",
			entryCount:  10,
			closeCount:  3,
			expectPanic: false,
			description: "multiple closes should not panic",
		},
		{
			name:        "close with no writes",
			entryCount:  0,
			closeCount:  1,
			expectPanic: false,
			description: "close empty collector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "cleanup.jsonl")

			collector, err := dsgo.NewJSONLCollector(filePath)
			if err != nil {
				t.Fatalf("Failed to create collector: %v", err)
			}

			for i := 0; i < tt.entryCount; i++ {
				entry := createJSONLTestEntry(i)
				if err := collector.Collect(entry); err != nil {
					t.Fatalf("Failed to collect: %v", err)
				}
			}

			for i := 0; i < tt.closeCount; i++ {
				err := collector.Close()
				if i == 0 && err != nil {
					t.Errorf("First close failed: %v", err)
				}
			}

			if tt.entryCount > 0 {
				lines := readJSONLFile(t, filePath)
				if len(lines) != tt.entryCount {
					t.Errorf("Expected %d entries, got %d", tt.entryCount, len(lines))
				}
			}
		})
	}
}

// TestJSONLCollector_FlushOnClose verifies data is flushed before close.
func TestJSONLCollector_FlushOnClose(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "flush_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	for i := 0; i < 100; i++ {
		entry := createJSONLTestEntry(i)
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Failed to collect: %v", err)
		}
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("File is empty after close - data was not flushed")
	}

	lines := readJSONLFile(t, filePath)
	if len(lines) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(lines))
	}
}

// TestJSONLCollector_PathAccessor verifies the Path() method returns correct path.
func TestJSONLCollector_PathAccessor(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "path_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}
	defer func() { _ = collector.Close() }()

	if collector.Path() != filePath {
		t.Errorf("Path() returned %q, expected %q", collector.Path(), filePath)
	}
}

// TestJSONLCollector_CountAccuracy verifies Count() is accurate during and after writes.
func TestJSONLCollector_CountAccuracy(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "count_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	if collector.Count() != 0 {
		t.Errorf("Initial count should be 0, got %d", collector.Count())
	}

	for i := 0; i < 50; i++ {
		entry := createJSONLTestEntry(i)
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Failed to collect: %v", err)
		}

		expectedCount := int64(i + 1)
		if collector.Count() != expectedCount {
			t.Errorf("After %d writes, count should be %d, got %d", i+1, expectedCount, collector.Count())
		}
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	if collector.Count() != 50 {
		t.Errorf("Final count should be 50, got %d", collector.Count())
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func createJSONLTestEntry(index int) *dsgo.HistoryEntry {
	return &dsgo.HistoryEntry{
		ID:        fmt.Sprintf("entry-%d", index),
		Timestamp: time.Now(),
		SessionID: fmt.Sprintf("session-%d", index%10),
		Provider:  "test-provider",
		Model:     "test-model",
		Request: dsgo.RequestMeta{
			Messages: []dsgo.Message{
				{Role: "user", Content: fmt.Sprintf("Test message %d", index)},
			},
			PromptLength: 20,
			MessageCount: 1,
			HasTools:     false,
			ToolCount:    0,
		},
		Response: dsgo.ResponseMeta{
			Content:        fmt.Sprintf("Test response %d", index),
			FinishReason:   "stop",
			ResponseLength: 20,
		},
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             0.001,
		},
		Cache: dsgo.CacheMeta{
			Hit: false,
		},
	}
}

func createJSONLConcurrentEntry(goroutineID, entryIndex int) *dsgo.HistoryEntry {
	return &dsgo.HistoryEntry{
		ID:        fmt.Sprintf("g%d-e%d", goroutineID, entryIndex),
		Timestamp: time.Now(),
		SessionID: fmt.Sprintf("session-%d", goroutineID),
		Provider:  "concurrent-provider",
		Model:     "concurrent-model",
		Request: dsgo.RequestMeta{
			Messages: []dsgo.Message{
				{Role: "user", Content: fmt.Sprintf("Concurrent message g%d e%d", goroutineID, entryIndex)},
			},
		},
		Response: dsgo.ResponseMeta{
			Content:      fmt.Sprintf("Response g%d e%d", goroutineID, entryIndex),
			FinishReason: "stop",
		},
		Usage: dsgo.Usage{
			TotalTokens: 20,
			Cost:        0.001,
		},
	}
}

func generateLargeContent(size int, seed int) string {
	var sb strings.Builder
	sb.Grow(size)

	pattern := fmt.Sprintf("Content block %d: This is test content that will be repeated to create large entries for testing purposes. ", seed)
	patternLen := len(pattern)

	for sb.Len() < size {
		remaining := size - sb.Len()
		if remaining >= patternLen {
			sb.WriteString(pattern)
		} else {
			sb.WriteString(pattern[:remaining])
		}
	}

	return sb.String()
}

func generateContentWithSpecialChars(size int) string {
	var sb strings.Builder
	sb.Grow(size)

	specialPatterns := []string{
		`"quoted text"`,
		`\backslash\path`,
		"\n\nnewlines\n",
		"\ttabs\there\t",
		`{"json": "object"}`,
		`unicode: 你好世界 🌍`,
		`special: <>&'"`,
		"\r\nwindows\r\n",
	}

	for sb.Len() < size {
		for _, pattern := range specialPatterns {
			if sb.Len() >= size {
				break
			}
			remaining := size - sb.Len()
			if remaining >= len(pattern) {
				sb.WriteString(pattern)
			} else {
				sb.WriteString(pattern[:remaining])
			}
			sb.WriteString(" ")
		}
	}

	return sb.String()[:size]
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func readJSONLFile(t *testing.T, filePath string) []string {
	t.Helper()

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	return lines
}

func readJSONLFileWithLargeBuffer(t *testing.T, filePath string) []string {
	t.Helper()

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	return lines
}

// ============================================================================
// JSONL Collector Error Handling Tests
// ============================================================================

// TestJSONLCollector_InvalidPath tests collector creation with invalid path
func TestJSONLCollector_InvalidPath(t *testing.T) {
	t.Parallel()
	// Try to create collector in a non-existent directory
	invalidPath := "/nonexistent/directory/structure/that/should/not/exist/test.jsonl"

	_, err := dsgo.NewJSONLCollector(invalidPath)
	if err == nil {
		t.Error("Expected error creating collector with invalid path, got nil")
	}
}

// TestJSONLCollector_WriteAfterClose tests that writing after close returns error
func TestJSONLCollector_WriteAfterClose(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "write_after_close_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// Write a normal entry
	entry := createJSONLTestEntry(0)
	if err := collector.Collect(entry); err != nil {
		t.Fatalf("Failed to collect entry: %v", err)
	}

	// Close the collector
	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close collector: %v", err)
	}

	// Try to write after close - this should fail
	entry2 := createJSONLTestEntry(1)
	err = collector.Collect(entry2)
	if err == nil {
		t.Error("Expected error writing after close, got nil")
	}
}

// TestJSONLCollector_ConcurrentCollectAndClose tests concurrent collect and close operations
func TestJSONLCollector_ConcurrentCollectAndClose(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "concurrent_close_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Goroutine 1: Close the collector
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		if err := collector.Close(); err != nil {
			errors <- fmt.Errorf("close error: %w", err)
		}
	}()

	// Goroutine 2-20: Try to collect entries while possibly closing
	for i := 0; i < 19; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			entry := createJSONLTestEntry(id)
			err := collector.Collect(entry)
			if err != nil {
				// This is expected if we hit the collector after it's closed
				errors <- fmt.Errorf("collect error (expected): %w", err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Log errors for debugging (they may be expected)
	errorCount := 0
	for err := range errors {
		t.Logf("Operation error (may be expected): %v", err)
		errorCount++
	}

	// We may have some errors due to race, but that's OK - file should still be readable
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("File stat failed: %v", err)
	}
}

// TestJSONLCollector_PermissionDenied tests collector behavior with permission issues
// Note: This test is platform-specific and may behave differently on Windows
func TestJSONLCollector_PermissionDenied(t *testing.T) {
	t.Parallel()
	if os.Getenv("SKIP_PERMISSION_TEST") != "" {
		t.Skip("Permission test skipped")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "permission_test.jsonl")

	// Create the file with restricted permissions
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	_ = file.Close()

	// Try to remove write permissions
	err = os.Chmod(filePath, 0444) // read-only
	if err != nil {
		t.Skip("Cannot change file permissions on this system")
	}
	defer func() { _ = os.Chmod(filePath, 0644) }() // restore permissions

	// Try to open collector on read-only file - this should fail on write
	collector, err := dsgo.NewJSONLCollector(filePath)
	if err == nil {
		defer func() { _ = collector.Close() }()
		// Try to collect - this should fail
		entry := createJSONLTestEntry(0)
		err = collector.Collect(entry)
		if err == nil {
			t.Error("Expected error writing to read-only file")
		}
	} else {
		// It's also OK if open fails due to permissions
		t.Logf("Open failed as expected: %v", err)
	}
}

// TestJSONLCollector_EmptyEntry tests collecting entries with minimal data
func TestJSONLCollector_EmptyEntry(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty_entry_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// Create minimal entry
	entry := &dsgo.HistoryEntry{
		ID: "minimal-entry",
	}

	if err := collector.Collect(entry); err != nil {
		t.Fatalf("Failed to collect minimal entry: %v", err)
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify entry was written
	lines := readJSONLFile(t, filePath)
	if len(lines) != 1 {
		t.Errorf("Expected 1 line, got %d", len(lines))
	}

	var parsed dsgo.HistoryEntry
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if parsed.ID != "minimal-entry" {
		t.Errorf("Expected ID 'minimal-entry', got %q", parsed.ID)
	}
}

// TestJSONLCollector_LargeContentCount tests collecting with extremely large count/usage values
func TestJSONLCollector_LargeContentCount(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "large_counts_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// Create entry with large values
	entry := &dsgo.HistoryEntry{
		ID:        "large-count-entry",
		Timestamp: time.Now(),
		Provider:  "test",
		Model:     "test",
		Usage: dsgo.Usage{
			PromptTokens:     1000000,
			CompletionTokens: 1000000,
			TotalTokens:      2000000,
			Cost:             999999.99,
		},
		Response: dsgo.ResponseMeta{
			Content:        strings.Repeat("x", 10000), // 10KB content
			ResponseLength: 10000,
		},
	}

	if err := collector.Collect(entry); err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify entry was written correctly
	lines := readJSONLFile(t, filePath)
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lines))
	}

	var parsed dsgo.HistoryEntry
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if parsed.Usage.TotalTokens != 2000000 {
		t.Errorf("Expected 2000000 tokens, got %d", parsed.Usage.TotalTokens)
	}

	if parsed.Usage.Cost != 999999.99 {
		t.Errorf("Expected cost 999999.99, got %f", parsed.Usage.Cost)
	}
}

// TestJSONLCollector_NilFieldsHandling tests handling of nil/empty fields
func TestJSONLCollector_NilFieldsHandling(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nil_fields_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// Create entry with nil/empty fields
	entry := &dsgo.HistoryEntry{
		ID: "nil-entry",
		Request: dsgo.RequestMeta{
			Messages: nil,
		},
		Error: nil,
		Cache: dsgo.CacheMeta{},
	}

	if err := collector.Collect(entry); err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify entry was written
	lines := readJSONLFile(t, filePath)
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lines))
	}

	var parsed dsgo.HistoryEntry
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if parsed.ID != "nil-entry" {
		t.Errorf("Expected ID 'nil-entry', got %q", parsed.ID)
	}
}

// TestJSONLCollector_CounterAccuracyAfterError tests that count is accurate even after errors
func TestJSONLCollector_CounterAccuracyAfterError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "count_accuracy_test.jsonl")

	collector, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}

	// Collect 5 entries
	for i := 0; i < 5; i++ {
		entry := createJSONLTestEntry(i)
		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Failed to collect entry %d: %v", i, err)
		}
	}

	if collector.Count() != 5 {
		t.Errorf("Expected count 5, got %d", collector.Count())
	}

	if err := collector.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Count should remain 5 after close
	if collector.Count() != 5 {
		t.Errorf("Count changed after close: expected 5, got %d", collector.Count())
	}
}

// TestJSONLCollector_RecreateAfterDelete tests creating collector for same path after deleting file
func TestJSONLCollector_RecreateAfterDelete(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "recreate_test.jsonl")

	// First collector
	collector1, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create first collector: %v", err)
	}

	if err := collector1.Collect(createJSONLTestEntry(0)); err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	if err := collector1.Close(); err != nil {
		t.Fatalf("Failed to close first collector: %v", err)
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Create second collector for same path
	collector2, err := dsgo.NewJSONLCollector(filePath)
	if err != nil {
		t.Fatalf("Failed to create second collector: %v", err)
	}

	if err := collector2.Collect(createJSONLTestEntry(1)); err != nil {
		t.Fatalf("Failed to collect in second collector: %v", err)
	}

	if err := collector2.Close(); err != nil {
		t.Fatalf("Failed to close second collector: %v", err)
	}

	// Verify file has only the second entry
	lines := readJSONLFile(t, filePath)
	if len(lines) != 1 {
		t.Errorf("Expected 1 line, got %d", len(lines))
	}

	var parsed dsgo.HistoryEntry
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if parsed.ID != "entry-1" {
		t.Errorf("Expected 'entry-1', got %q", parsed.ID)
	}
}
