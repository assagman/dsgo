package dsgo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMemoryCollector tests the in-memory collector
func TestMemoryCollector(t *testing.T) {
	t.Run("basic collection", func(t *testing.T) {
		collector := NewMemoryCollector(10)

		entry := &HistoryEntry{
			ID:        "test-1",
			Timestamp: time.Now(),
			Provider:  "openai",
			Model:     "gpt-4",
		}

		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		if collector.Count() != 1 {
			t.Errorf("Count() = %d, want 1", collector.Count())
		}

		if collector.Len() != 1 {
			t.Errorf("Len() = %d, want 1", collector.Len())
		}
	})

	t.Run("ring buffer behavior", func(t *testing.T) {
		collector := NewMemoryCollector(3)

		// Add 5 entries (more than capacity)
		for i := 0; i < 5; i++ {
			entry := &HistoryEntry{
				ID:        "test-" + string(rune('0'+i)),
				Timestamp: time.Now(),
				Model:     "gpt-4",
			}
			if err := collector.Collect(entry); err != nil {
				t.Fatalf("Collect failed: %v", err)
			}
		}

		// Should only keep last 3
		if collector.Count() != 5 {
			t.Errorf("Count() = %d, want 5", collector.Count())
		}

		if collector.Len() != 3 {
			t.Errorf("Len() = %d, want 3 (ring buffer size)", collector.Len())
		}

		// GetAll should return last 3 entries
		all := collector.GetAll()
		if len(all) != 3 {
			t.Errorf("GetAll() returned %d entries, want 3", len(all))
		}
	})

	t.Run("GetLast", func(t *testing.T) {
		collector := NewMemoryCollector(10)

		// Add some entries
		for i := 0; i < 5; i++ {
			entry := &HistoryEntry{
				ID:    "test-" + string(rune('0'+i)),
				Model: "gpt-4",
			}
			_ = collector.Collect(entry)
		}

		// Get last 3
		last := collector.GetLast(3)
		if len(last) != 3 {
			t.Errorf("GetLast(3) returned %d entries, want 3", len(last))
		}

		// Get more than available
		last = collector.GetLast(10)
		if len(last) != 5 {
			t.Errorf("GetLast(10) returned %d entries, want 5", len(last))
		}

		// Get 0
		last = collector.GetLast(0)
		if len(last) != 0 {
			t.Errorf("GetLast(0) returned %d entries, want 0", len(last))
		}
	})

	t.Run("Clear", func(t *testing.T) {
		collector := NewMemoryCollector(10)

		// Add entries
		for i := 0; i < 5; i++ {
			entry := &HistoryEntry{ID: "test"}
			_ = collector.Collect(entry)
		}

		if collector.Count() != 5 {
			t.Errorf("Count before clear = %d, want 5", collector.Count())
		}

		collector.Clear()

		if collector.Count() != 0 {
			t.Errorf("Count after clear = %d, want 0", collector.Count())
		}

		if collector.Len() != 0 {
			t.Errorf("Len after clear = %d, want 0", collector.Len())
		}
	})

	t.Run("Close is no-op", func(t *testing.T) {
		collector := NewMemoryCollector(10)
		if err := collector.Close(); err != nil {
			t.Errorf("Close() returned error: %v", err)
		}
	})

	t.Run("zero capacity defaults to 100", func(t *testing.T) {
		collector := NewMemoryCollector(0)
		// Should default to 100
		for i := 0; i < 150; i++ {
			entry := &HistoryEntry{ID: "test"}
			_ = collector.Collect(entry)
		}

		// Should have last 100
		if collector.Len() != 100 {
			t.Errorf("Len() = %d, want 100 (default capacity)", collector.Len())
		}
	})

	t.Run("nil entry handling", func(t *testing.T) {
		mem := NewMemoryCollector(10)

		// Should handle nil entry gracefully
		if err := mem.Collect(nil); err != nil {
			t.Errorf("Collect(nil) returned error: %v", err)
		}
	})

	t.Run("getAllUnsafe coverage", func(t *testing.T) {
		mem := NewMemoryCollector(5)

		// Add entries
		for i := 0; i < 7; i++ {
			entry := &HistoryEntry{
				ID:    "test-" + string(rune('0'+i)),
				Model: "gpt-4",
			}
			_ = mem.Collect(entry)
		}

		// GetAll uses getAllUnsafe internally
		all := mem.GetAll()
		if len(all) != 5 {
			t.Errorf("GetAll() returned %d entries, want 5", len(all))
		}

		// GetLast also uses getAllUnsafe
		last := mem.GetLast(3)
		if len(last) != 3 {
			t.Errorf("GetLast(3) returned %d entries, want 3", len(last))
		}
	})
}

// TestJSONLCollector tests the JSONL file collector
func TestJSONLCollector(t *testing.T) {
	t.Run("basic collection", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.jsonl")

		collector, err := NewJSONLCollector(path)
		if err != nil {
			t.Fatalf("NewJSONLCollector failed: %v", err)
		}
		defer func() { _ = collector.Close() }()

		entry := &HistoryEntry{
			ID:        "test-1",
			Timestamp: time.Now(),
			Provider:  "openai",
			Model:     "gpt-4",
			Usage: Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}

		if err := collector.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		if collector.Count() != 1 {
			t.Errorf("Count() = %d, want 1", collector.Count())
		}

		if collector.Path() != path {
			t.Errorf("Path() = %s, want %s", collector.Path(), path)
		}
	})

	t.Run("multiple entries", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "multi.jsonl")

		collector, err := NewJSONLCollector(path)
		if err != nil {
			t.Fatalf("NewJSONLCollector failed: %v", err)
		}

		// Write multiple entries
		for i := 0; i < 5; i++ {
			entry := &HistoryEntry{
				ID:    "test-" + string(rune('0'+i)),
				Model: "gpt-4",
			}
			if err := collector.Collect(entry); err != nil {
				t.Fatalf("Collect failed: %v", err)
			}
		}

		_ = collector.Close()

		// Verify file exists and has content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if len(data) == 0 {
			t.Error("File is empty")
		}

		if collector.Count() != 5 {
			t.Errorf("Count() = %d, want 5", collector.Count())
		}
	})

	t.Run("Close", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "close.jsonl")

		collector, err := NewJSONLCollector(path)
		if err != nil {
			t.Fatalf("NewJSONLCollector failed: %v", err)
		}

		// Close should work
		if err := collector.Close(); err != nil {
			t.Errorf("Close() failed: %v", err)
		}

		// File should exist
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("File was not created")
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		// Try to create in non-existent directory
		_, err := NewJSONLCollector("/nonexistent/dir/file.jsonl")
		if err == nil {
			t.Error("Expected error for invalid path, got nil")
		}
	})

	t.Run("append mode", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "append.jsonl")

		// First collector
		collector1, err := NewJSONLCollector(path)
		if err != nil {
			t.Fatalf("NewJSONLCollector failed: %v", err)
		}

		entry1 := &HistoryEntry{ID: "test-1", Model: "gpt-4"}
		_ = collector1.Collect(entry1)
		_ = collector1.Close()

		// Second collector (should append)
		collector2, err := NewJSONLCollector(path)
		if err != nil {
			t.Fatalf("NewJSONLCollector failed: %v", err)
		}

		entry2 := &HistoryEntry{ID: "test-2", Model: "gpt-4"}
		_ = collector2.Collect(entry2)
		_ = collector2.Close()

		// Verify both entries are in file
		data, _ := os.ReadFile(path)
		content := string(data)

		if !contains(content, "test-1") {
			t.Error("File missing first entry")
		}
		if !contains(content, "test-2") {
			t.Error("File missing second entry")
		}
	})
}

// TestCompositeCollector tests the composite collector
func TestCompositeCollector(t *testing.T) {
	t.Run("basic composition", func(t *testing.T) {
		mem := NewMemoryCollector(10)
		composite := NewCompositeCollector(mem)

		entry := &HistoryEntry{
			ID:    "test-1",
			Model: "gpt-4",
		}

		if err := composite.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		if mem.Count() != 1 {
			t.Errorf("Memory collector count = %d, want 1", mem.Count())
		}
	})

	t.Run("multiple collectors", func(t *testing.T) {
		mem1 := NewMemoryCollector(10)
		mem2 := NewMemoryCollector(10)

		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "composite.jsonl")
		jsonl, _ := NewJSONLCollector(path)
		defer func() { _ = jsonl.Close() }()

		composite := NewCompositeCollector(mem1, mem2, jsonl)

		entry := &HistoryEntry{
			ID:    "test-1",
			Model: "gpt-4",
		}

		if err := composite.Collect(entry); err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		// All collectors should have received the entry
		if mem1.Count() != 1 {
			t.Errorf("mem1.Count() = %d, want 1", mem1.Count())
		}
		if mem2.Count() != 1 {
			t.Errorf("mem2.Count() = %d, want 1", mem2.Count())
		}
		if jsonl.Count() != 1 {
			t.Errorf("jsonl.Count() = %d, want 1", jsonl.Count())
		}
	})

	t.Run("Add collectors dynamically", func(t *testing.T) {
		composite := NewCompositeCollector()

		mem1 := NewMemoryCollector(10)
		composite.Add(mem1)

		entry := &HistoryEntry{ID: "test-1", Model: "gpt-4"}
		_ = composite.Collect(entry)

		if mem1.Count() != 1 {
			t.Errorf("mem1.Count() = %d, want 1", mem1.Count())
		}

		// Add another collector
		mem2 := NewMemoryCollector(10)
		composite.Add(mem2)

		entry2 := &HistoryEntry{ID: "test-2", Model: "gpt-4"}
		_ = composite.Collect(entry2)

		// Both should have the second entry
		if mem1.Count() != 2 {
			t.Errorf("mem1.Count() = %d, want 2", mem1.Count())
		}
		if mem2.Count() != 1 {
			t.Errorf("mem2.Count() = %d, want 1", mem2.Count())
		}
	})

	t.Run("Len returns number of collectors", func(t *testing.T) {
		mem1 := NewMemoryCollector(10)
		mem2 := NewMemoryCollector(10)

		composite := NewCompositeCollector(mem1, mem2)

		if composite.Len() != 2 {
			t.Errorf("Len() = %d, want 2", composite.Len())
		}

		entry := &HistoryEntry{ID: "test", Model: "gpt-4"}
		_ = composite.Collect(entry)

		// Len still returns number of collectors, not entries
		if composite.Len() != 2 {
			t.Errorf("Len() after Collect = %d, want 2", composite.Len())
		}
	})

	t.Run("Len returns 0 for empty composite", func(t *testing.T) {
		composite := NewCompositeCollector()

		if composite.Len() != 0 {
			t.Errorf("Len() = %d, want 0", composite.Len())
		}
	})

	t.Run("Close all collectors", func(t *testing.T) {
		tmpDir := t.TempDir()
		path1 := filepath.Join(tmpDir, "file1.jsonl")
		path2 := filepath.Join(tmpDir, "file2.jsonl")

		jsonl1, _ := NewJSONLCollector(path1)
		jsonl2, _ := NewJSONLCollector(path2)

		composite := NewCompositeCollector(jsonl1, jsonl2)

		entry := &HistoryEntry{ID: "test", Model: "gpt-4"}
		_ = composite.Collect(entry)

		// Close should close all
		if err := composite.Close(); err != nil {
			t.Errorf("Close() failed: %v", err)
		}

		// Files should be created
		if _, err := os.Stat(path1); os.IsNotExist(err) {
			t.Error("File1 was not created")
		}
		if _, err := os.Stat(path2); os.IsNotExist(err) {
			t.Error("File2 was not created")
		}
	})

	t.Run("partial failure handling", func(t *testing.T) {
		mem := NewMemoryCollector(10)

		// Create a collector that will fail
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.jsonl")
		jsonl, _ := NewJSONLCollector(path)
		_ = jsonl.Close() // Close it so writes fail

		composite := NewCompositeCollector(mem, jsonl)

		entry := &HistoryEntry{ID: "test", Model: "gpt-4"}

		// Should not fail even if one collector fails
		_ = composite.Collect(entry) // May succeed or fail - both acceptable

		// Memory collector should still have received it
		if mem.Count() != 1 {
			t.Errorf("mem.Count() = %d, want 1 (should succeed despite jsonl failure)", mem.Count())
		}
	})
}

// TestCollectorSettings tests the Settings integration
func TestCollectorSettings(t *testing.T) {
	t.Run("SetCollector", func(t *testing.T) {
		s := &Settings{
			APIKey: make(map[string]string),
		}

		mem := NewMemoryCollector(10)
		s.SetCollector(mem)

		if s.Collector == nil {
			t.Error("Expected collector to be set")
		}
	})

	t.Run("GetSettings includes collector", func(t *testing.T) {
		ResetConfig()
		defer ResetConfig()

		mem := NewMemoryCollector(10)
		Configure(WithCollector(mem))

		settings := GetSettings()

		if settings.Collector == nil {
			t.Error("Expected collector in settings")
		}
	})

	t.Run("Reset clears collector", func(t *testing.T) {
		s := &Settings{
			APIKey:    make(map[string]string),
			Collector: NewMemoryCollector(10),
		}

		s.Reset()

		if s.Collector != nil {
			t.Error("Expected collector to be cleared")
		}
	})
}

// TestJSONLCollectorErrors tests error conditions
func TestJSONLCollectorErrors(t *testing.T) {
	t.Run("write after close", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "error.jsonl")

		collector, _ := NewJSONLCollector(path)
		_ = collector.Close()

		// Try to write after close - should error
		entry := &HistoryEntry{ID: "test", Model: "gpt-4"}
		err := collector.Collect(entry)
		if err == nil {
			t.Error("Expected error when writing after close, got nil")
		}
	})
}

// TestCompositeCollectorErrors tests error conditions
func TestCompositeCollectorErrors(t *testing.T) {
	t.Run("Close with errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		path1 := filepath.Join(tmpDir, "file1.jsonl")
		path2 := filepath.Join(tmpDir, "file2.jsonl")

		jsonl1, _ := NewJSONLCollector(path1)
		jsonl2, _ := NewJSONLCollector(path2)

		// Close one of them first
		_ = jsonl1.Close()

		composite := NewCompositeCollector(jsonl1, jsonl2)

		// Close should handle the already-closed collector
		_ = composite.Close() // May or may not error - both acceptable
	})
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
