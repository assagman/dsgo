package dsgo

import (
	"sync"
	"testing"
)

func TestNewLMCache(t *testing.T) {
	cache := NewLMCache(100)
	if cache.capacity != 100 {
		t.Errorf("Expected capacity 100, got %d", cache.capacity)
	}
	if cache.Size() != 0 {
		t.Errorf("Expected empty cache, got size %d", cache.Size())
	}
}

func TestNewLMCache_DefaultCapacity(t *testing.T) {
	cache := NewLMCache(0)
	if cache.capacity != 1000 {
		t.Errorf("Expected default capacity 1000, got %d", cache.capacity)
	}

	cache2 := NewLMCache(-10)
	if cache2.capacity != 1000 {
		t.Errorf("Expected default capacity 1000 for negative input, got %d", cache2.capacity)
	}
}

func TestLMCache_SetAndGet(t *testing.T) {
	cache := NewLMCache(10)

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

func TestLMCache_Miss(t *testing.T) {
	cache := NewLMCache(10)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("Expected cache miss, got hit")
	}
}

func TestLMCache_Update(t *testing.T) {
	cache := NewLMCache(10)

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

	if cache.Size() != 1 {
		t.Errorf("Expected size 1 after update, got %d", cache.Size())
	}
}

func TestLMCache_LRUEviction(t *testing.T) {
	cache := NewLMCache(3)

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Set("key2", &GenerateResult{Content: "2"})
	cache.Set("key3", &GenerateResult{Content: "3"})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	cache.Set("key4", &GenerateResult{Content: "4"})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3 after eviction, got %d", cache.Size())
	}

	_, ok := cache.Get("key1")
	if ok {
		t.Error("Expected key1 to be evicted (LRU)")
	}

	_, ok = cache.Get("key2")
	if !ok {
		t.Error("Expected key2 to still be cached")
	}
}

func TestLMCache_LRUAccess(t *testing.T) {
	cache := NewLMCache(3)

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Set("key2", &GenerateResult{Content: "2"})
	cache.Set("key3", &GenerateResult{Content: "3"})

	cache.Get("key1")

	cache.Set("key4", &GenerateResult{Content: "4"})

	_, ok := cache.Get("key2")
	if ok {
		t.Error("Expected key2 to be evicted (was LRU)")
	}

	_, ok = cache.Get("key1")
	if !ok {
		t.Error("Expected key1 to still be cached (was accessed)")
	}
}

func TestLMCache_Clear(t *testing.T) {
	cache := NewLMCache(10)

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Set("key2", &GenerateResult{Content: "2"})
	cache.Get("key1")
	cache.Get("nonexistent")

	stats := cache.Stats()
	if stats.Size != 2 {
		t.Errorf("Expected size 2 before clear, got %d", stats.Size)
	}
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit before clear, got %d", stats.Hits)
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

func TestLMCache_Stats(t *testing.T) {
	cache := NewLMCache(10)

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Get("key1")
	cache.Get("key1")
	cache.Get("key2")
	cache.Get("key3")

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

	hitRate := stats.HitRate()
	if hitRate != 50.0 {
		t.Errorf("Expected hit rate 50%%, got %.2f%%", hitRate)
	}
}

func TestCacheStats_HitRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    CacheStats
		expected float64
	}{
		{
			name:     "50% hit rate",
			stats:    CacheStats{Hits: 5, Misses: 5},
			expected: 50.0,
		},
		{
			name:     "100% hit rate",
			stats:    CacheStats{Hits: 10, Misses: 0},
			expected: 100.0,
		},
		{
			name:     "0% hit rate",
			stats:    CacheStats{Hits: 0, Misses: 10},
			expected: 0.0,
		},
		{
			name:     "no requests",
			stats:    CacheStats{Hits: 0, Misses: 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hitRate := tt.stats.HitRate()
			if hitRate != tt.expected {
				t.Errorf("Expected hit rate %.2f%%, got %.2f%%", tt.expected, hitRate)
			}
		})
	}
}

func TestLMCache_Concurrency(t *testing.T) {
	cache := NewLMCache(100)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := string(rune('a' + (j % 26)))
				result := &GenerateResult{Content: key}
				cache.Set(key, result)
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	stats := cache.Stats()
	if stats.Size > 100 {
		t.Errorf("Cache size exceeded capacity: %d", stats.Size)
	}
}

func TestGenerateCacheKey(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	options := DefaultGenerateOptions()

	key1 := GenerateCacheKey("gpt-4", messages, options)
	key2 := GenerateCacheKey("gpt-4", messages, options)

	if key1 != key2 {
		t.Error("Expected identical keys for identical inputs")
	}

	key3 := GenerateCacheKey("gpt-3.5-turbo", messages, options)
	if key1 == key3 {
		t.Error("Expected different keys for different models")
	}

	messages2 := []Message{
		{Role: "user", Content: "Goodbye"},
	}
	key4 := GenerateCacheKey("gpt-4", messages2, options)
	if key1 == key4 {
		t.Error("Expected different keys for different messages")
	}

	options2 := DefaultGenerateOptions()
	options2.Temperature = 0.5
	key5 := GenerateCacheKey("gpt-4", messages, options2)
	if key1 == key5 {
		t.Error("Expected different keys for different temperatures")
	}
}

func TestGenerateCacheKey_StopSequences(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}
	options1 := DefaultGenerateOptions()
	options1.Stop = []string{"stop1", "stop2"}

	options2 := DefaultGenerateOptions()
	options2.Stop = []string{"stop2", "stop1"}

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 != key2 {
		t.Error("Expected same key regardless of stop sequence order")
	}
}

func TestGenerateCacheKey_ResponseFormat(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	options1 := DefaultGenerateOptions()
	options1.ResponseFormat = "text"

	options2 := DefaultGenerateOptions()
	options2.ResponseFormat = "json"

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 == key2 {
		t.Error("Expected different keys for different response formats")
	}
}

func TestGenerateCacheKey_MaxTokens(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	options1 := DefaultGenerateOptions()
	options1.MaxTokens = 100

	options2 := DefaultGenerateOptions()
	options2.MaxTokens = 200

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 == key2 {
		t.Error("Expected different keys for different max tokens")
	}
}

func TestGenerateCacheKey_TopP(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	options1 := DefaultGenerateOptions()
	options1.TopP = 0.9

	options2 := DefaultGenerateOptions()
	options2.TopP = 1.0

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 == key2 {
		t.Error("Expected different keys for different TopP values")
	}
}
