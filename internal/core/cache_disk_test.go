package core

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewDiskCache(t *testing.T) {
	tmpDir := t.TempDir()

	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	if cache.baseDir != tmpDir {
		t.Errorf("Expected baseDir %s, got %s", tmpDir, cache.baseDir)
	}

	if cache.sizeLimit != DefaultDiskCacheSizeLimit {
		t.Errorf("Expected default size limit %d, got %d", DefaultDiskCacheSizeLimit, cache.sizeLimit)
	}

	if cache.shards != DefaultDiskCacheShards {
		t.Errorf("Expected %d shards, got %d", DefaultDiskCacheShards, cache.shards)
	}
}

func TestNewDiskCache_DefaultDir(t *testing.T) {
	cache, err := NewDiskCache("", 0)
	if err != nil {
		t.Fatalf("NewDiskCache with empty dir failed: %v", err)
	}

	homeDir, _ := os.UserHomeDir()
	// Default dir now includes project-specific subdirectory: ~/.dsgo_cache/proj_<hash>
	expectedPrefix := filepath.Join(homeDir, DefaultDiskCacheDir, "proj_")
	if !strings.HasPrefix(cache.baseDir, expectedPrefix) {
		t.Errorf("Expected baseDir to start with %s, got %s", expectedPrefix, cache.baseDir)
	}

	// Cleanup the project-specific cache dir
	_ = os.RemoveAll(cache.baseDir)
}

func TestDiskCache_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	result := &GenerateResult{
		Content: "test response",
		Usage:   Usage{TotalTokens: 100},
	}

	cache.Set("key1", result)

	retrieved, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit, got miss")
	}

	if retrieved.Content != "test response" {
		t.Errorf("Expected content 'test response', got '%s'", retrieved.Content)
	}

	if retrieved.Usage.TotalTokens != 100 {
		t.Errorf("Expected 100 tokens, got %d", retrieved.Usage.TotalTokens)
	}
}

func TestDiskCache_Miss(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("Expected cache miss, got hit")
	}
}

func TestDiskCache_Update(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	result1 := &GenerateResult{Content: "first"}
	result2 := &GenerateResult{Content: "second"}

	cache.Set("key1", result1)
	cache.Set("key1", result2)

	retrieved, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit")
	}

	if retrieved.Content != "second" {
		t.Errorf("Expected updated content 'second', got '%s'", retrieved.Content)
	}
}

func TestDiskCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Set("key2", &GenerateResult{Content: "2"})
	cache.Get("key1")
	cache.Get("nonexistent")

	stats := cache.Stats()
	if stats.Size != 2 {
		t.Errorf("Expected size 2 before clear, got %d", stats.Size)
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}

	stats = cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Expected stats to be reset after clear: %+v", stats)
	}
}

func TestDiskCache_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Get("key1") // Hit
	cache.Get("key1") // Hit
	cache.Get("key2") // Miss
	cache.Get("key3") // Miss

	stats := cache.Stats()

	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}

	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses, got %d", stats.Misses)
	}

	if stats.Size != 1 {
		t.Errorf("Expected size 1, got %d", stats.Size)
	}
}

func TestDiskCache_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := string(rune('a' + (j % 26)))
				result := &GenerateResult{Content: key}
				cache.Set(key, result)
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is still functional
	cache.Set("final", &GenerateResult{Content: "test"})
	result, ok := cache.Get("final")
	if !ok {
		t.Error("Cache should be functional after concurrent operations")
	}
	if result.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Content)
	}
}

func TestDiskCache_Sharding(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCacheWithShards(tmpDir, 0, 4)
	if err != nil {
		t.Fatalf("NewDiskCacheWithShards failed: %v", err)
	}

	// Insert multiple keys and verify they're distributed across shards
	keys := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	for _, key := range keys {
		cache.Set(key, &GenerateResult{Content: key})
	}

	// Verify all entries can be retrieved
	for _, key := range keys {
		result, ok := cache.Get(key)
		if !ok {
			t.Errorf("Failed to retrieve key %s", key)
		}
		if result.Content != key {
			t.Errorf("Expected %s, got %s", key, result.Content)
		}
	}

	// Verify shard directories exist
	for i := 0; i < 4; i++ {
		shardDir := filepath.Join(tmpDir, "shard_0"+string(rune('0'+i)))
		if _, err := os.Stat(shardDir); os.IsNotExist(err) {
			t.Errorf("Shard directory %s should exist", shardDir)
		}
	}
}

func TestDiskCache_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache and store data
	cache1, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	cache1.Set("persistent_key", &GenerateResult{Content: "persistent_value"})

	// Create a new cache instance pointing to the same directory
	cache2, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	// Verify data persisted
	result, ok := cache2.Get("persistent_key")
	if !ok {
		t.Error("Data should persist across cache instances")
	}
	if result.Content != "persistent_value" {
		t.Errorf("Expected 'persistent_value', got '%s'", result.Content)
	}
}

func TestDiskCache_SizeBytes(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	// Empty cache
	if size := cache.SizeBytes(); size != 0 {
		t.Errorf("Empty cache should have 0 bytes, got %d", size)
	}

	// Add some data
	cache.Set("key1", &GenerateResult{Content: "some content"})
	cache.Set("key2", &GenerateResult{Content: "more content"})

	size := cache.SizeBytes()
	if size == 0 {
		t.Error("Cache with data should have non-zero size")
	}
}

func TestDiskCache_Dir(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	if cache.Dir() != tmpDir {
		t.Errorf("Expected dir %s, got %s", tmpDir, cache.Dir())
	}
}

func TestDiskCache_Capacity(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	// Disk cache uses size-based limits, not entry count
	if cap := cache.Capacity(); cap != -1 {
		t.Errorf("Expected capacity -1 for disk cache, got %d", cap)
	}
}

func TestDiskCache_ComplexData(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewDiskCache(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	// Store result with tool calls and metadata
	result := &GenerateResult{
		Content:      "response with tools",
		FinishReason: "tool_calls",
		Usage: Usage{
			PromptTokens:     50,
			CompletionTokens: 25,
			TotalTokens:      75,
		},
		ToolCalls: []ToolCall{
			{
				ID:   "call_123",
				Name: "search",
				Arguments: map[string]interface{}{
					"query": "test query",
					"limit": 10,
				},
			},
		},
		Metadata: map[string]any{
			"model":   "gpt-4",
			"version": "2024-01",
		},
	}

	cache.Set("complex_key", result)

	retrieved, ok := cache.Get("complex_key")
	if !ok {
		t.Fatal("Expected cache hit for complex data")
	}

	if retrieved.Content != result.Content {
		t.Errorf("Content mismatch: expected %s, got %s", result.Content, retrieved.Content)
	}
	if retrieved.FinishReason != result.FinishReason {
		t.Errorf("FinishReason mismatch: expected %s, got %s", result.FinishReason, retrieved.FinishReason)
	}
	if len(retrieved.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(retrieved.ToolCalls))
	}
	if retrieved.ToolCalls[0].Name != "search" {
		t.Errorf("Tool call name mismatch: expected search, got %s", retrieved.ToolCalls[0].Name)
	}
}
