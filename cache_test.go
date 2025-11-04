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

// TestLMCache_Concurrency_EdgeCases tests concurrent access patterns that could cause race conditions
func TestLMCache_Concurrency_EdgeCases(t *testing.T) {
	cache := NewLMCache(10)
	var wg sync.WaitGroup

	// Test concurrent eviction
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Fill cache and trigger eviction
			for j := 0; j < 20; j++ {
				key := string(rune('a' + (j % 26)))
				cache.Set(key, &GenerateResult{Content: key})
			}
		}(i)
	}

	// Test concurrent stats access
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cache.Stats()
			}
		}()
	}

	wg.Wait()

	// Cache should remain in valid state
	if cache.Size() > 10 {
		t.Errorf("Cache exceeded capacity after concurrent operations: %d", cache.Size())
	}

	stats := cache.Stats()
	if stats.Hits < 0 || stats.Misses < 0 || stats.Size < 0 {
		t.Errorf("Invalid stats after concurrent operations: %+v", stats)
	}
}

// TestLMCache_CacheKeyVariations tests edge cases in cache key generation
func TestLMCache_CacheKeyVariations(t *testing.T) {

	// Test with empty messages
	key1 := GenerateCacheKey("model", []Message{}, DefaultGenerateOptions())
	key2 := GenerateCacheKey("model", []Message{}, DefaultGenerateOptions())
	if key1 != key2 {
		t.Error("Empty message lists should generate same key")
	}

	// Test with default options
	opts := DefaultGenerateOptions()
	key3 := GenerateCacheKey("model", []Message{{Role: "user", Content: "test"}}, opts)
	key4 := GenerateCacheKey("model", []Message{{Role: "user", Content: "test"}}, opts)
	if key3 != key4 {
		t.Error("Default options should generate same key")
	}

	// Test with options that have default values
	opts1 := DefaultGenerateOptions()
	opts2 := DefaultGenerateOptions()
	opts2.TopP = 1.0 // Default value
	key5 := GenerateCacheKey("model", []Message{{Role: "user", Content: "test"}}, opts1)
	key6 := GenerateCacheKey("model", []Message{{Role: "user", Content: "test"}}, opts2)
	if key5 != key6 {
		t.Error("Options with default values should generate same key")
	}
}

// TestLMCache_Eviction_UnderLoad tests cache behavior under high load with eviction
func TestLMCache_Eviction_UnderLoad(t *testing.T) {
	cache := NewLMCache(3) // Small capacity to force eviction

	// Fill cache to capacity
	cache.Set("a", &GenerateResult{Content: "a"})
	cache.Set("b", &GenerateResult{Content: "b"})
	cache.Set("c", &GenerateResult{Content: "c"})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	// Add one more, should evict oldest (a)
	cache.Set("d", &GenerateResult{Content: "d"})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3 after eviction, got %d", cache.Size())
	}

	// a should be evicted
	if _, ok := cache.Get("a"); ok {
		t.Error("Item 'a' should have been evicted")
	}

	// Access b to make it recently used
	cache.Get("b")

	// Add another, should evict oldest (c, since b was accessed)
	cache.Set("e", &GenerateResult{Content: "e"})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3 after second eviction, got %d", cache.Size())
	}

	// c should be evicted, b should still be there
	if _, ok := cache.Get("c"); ok {
		t.Error("Item 'c' should have been evicted")
	}

	if _, ok := cache.Get("b"); !ok {
		t.Error("Recently accessed item 'b' should still be in cache")
	}
}

// TestGenerateCacheKey_Tools tests cache key generation with tools
func TestGenerateCacheKey_Tools(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	tool1 := Tool{
		Name:        "get_weather",
		Description: "Get weather info",
		Parameters: []ToolParameter{
			{Name: "location", Type: "string", Description: "Location", Required: true},
		},
	}

	tool2 := Tool{
		Name:        "calculator",
		Description: "Perform calculations",
		Parameters: []ToolParameter{
			{Name: "expression", Type: "string", Description: "Math expression", Required: true},
		},
	}

	options1 := DefaultGenerateOptions()
	options1.Tools = []Tool{tool1}

	options2 := DefaultGenerateOptions()
	options2.Tools = []Tool{tool2}

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 == key2 {
		t.Error("Expected different keys for different tools")
	}

	// Same tools should produce same key
	options3 := DefaultGenerateOptions()
	options3.Tools = []Tool{tool1}
	key3 := GenerateCacheKey("gpt-4", messages, options3)

	if key1 != key3 {
		t.Error("Expected same key for identical tools")
	}
}

// TestGenerateCacheKey_ToolChoice tests cache key generation with tool choice
func TestGenerateCacheKey_ToolChoice(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}
	tool := Tool{Name: "test_tool", Description: "Test"}

	options1 := DefaultGenerateOptions()
	options1.Tools = []Tool{tool}
	options1.ToolChoice = "auto"

	options2 := DefaultGenerateOptions()
	options2.Tools = []Tool{tool}
	options2.ToolChoice = "none"

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 == key2 {
		t.Error("Expected different keys for different tool choices")
	}
}

// TestGenerateCacheKey_Penalties tests cache key generation with penalties
func TestGenerateCacheKey_Penalties(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	options1 := DefaultGenerateOptions()
	options1.FrequencyPenalty = 0.5

	options2 := DefaultGenerateOptions()
	options2.FrequencyPenalty = 0.0

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 == key2 {
		t.Error("Expected different keys for different frequency penalties")
	}

	options3 := DefaultGenerateOptions()
	options3.PresencePenalty = 0.5

	key3 := GenerateCacheKey("gpt-4", messages, options3)

	if key2 == key3 {
		t.Error("Expected different keys for different presence penalties")
	}
}

// TestGenerateCacheKey_ResponseSchema tests cache key generation with response schema
func TestGenerateCacheKey_ResponseSchema(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	schema1 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "number"},
		},
	}

	schema2 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
		},
	}

	options1 := DefaultGenerateOptions()
	options1.ResponseSchema = schema1

	options2 := DefaultGenerateOptions()
	options2.ResponseSchema = schema2

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 == key2 {
		t.Error("Expected different keys for different response schemas")
	}

	// Test that map order doesn't matter (canonicalization)
	schema3 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"age":  map[string]any{"type": "number"},
			"name": map[string]any{"type": "string"},
		},
	}

	options3 := DefaultGenerateOptions()
	options3.ResponseSchema = schema3

	key3 := GenerateCacheKey("gpt-4", messages, options3)

	if key1 != key3 {
		t.Error("Expected same key regardless of map insertion order (canonicalization)")
	}
}

// TestGenerateCacheKey_MapCanonicalization tests map canonicalization
func TestGenerateCacheKey_MapCanonicalization(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	// Create schemas with same data but different insertion order
	schema1 := map[string]any{
		"z_field": "value",
		"a_field": "value",
		"m_field": "value",
	}

	schema2 := map[string]any{
		"a_field": "value",
		"m_field": "value",
		"z_field": "value",
	}

	options1 := DefaultGenerateOptions()
	options1.ResponseSchema = schema1

	options2 := DefaultGenerateOptions()
	options2.ResponseSchema = schema2

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	if key1 != key2 {
		t.Error("Map canonicalization should produce identical keys regardless of insertion order")
	}
}

// TestLMCache_DeepCopy tests that cached results are deep copied
func TestLMCache_DeepCopy(t *testing.T) {
	cache := NewLMCache(10)

	original := &GenerateResult{
		Content: "original content",
		ToolCalls: []ToolCall{
			{
				ID:   "call1",
				Name: "tool1",
				Arguments: map[string]interface{}{
					"arg1": "value1",
				},
			},
		},
		Metadata: map[string]any{
			"key1": "value1",
		},
	}

	cache.Set("key1", original)

	// Modify original
	original.Content = "modified"
	original.ToolCalls[0].Name = "modified_tool"
	original.ToolCalls[0].Arguments["arg1"] = "modified_value"
	original.Metadata["key1"] = "modified_value"

	// Get from cache
	retrieved, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit")
	}

	// Verify cached value was not modified
	if retrieved.Content != "original content" {
		t.Errorf("Expected content 'original content', got '%s'", retrieved.Content)
	}

	if retrieved.ToolCalls[0].Name != "tool1" {
		t.Errorf("Expected tool name 'tool1', got '%s'", retrieved.ToolCalls[0].Name)
	}

	if retrieved.ToolCalls[0].Arguments["arg1"] != "value1" {
		t.Errorf("Expected arg1 'value1', got '%v'", retrieved.ToolCalls[0].Arguments["arg1"])
	}

	if retrieved.Metadata["key1"] != "value1" {
		t.Errorf("Expected metadata 'value1', got '%v'", retrieved.Metadata["key1"])
	}
}

// TestLMCache_DeepCopy_Mutation tests that modifying retrieved results doesn't affect cache
func TestLMCache_DeepCopy_Mutation(t *testing.T) {
	cache := NewLMCache(10)

	original := &GenerateResult{
		Content: "test",
		Metadata: map[string]any{
			"nested": map[string]any{
				"level2": map[string]any{
					"level3": "deep value",
				},
			},
		},
	}

	cache.Set("key1", original)

	// Get and modify
	retrieved1, _ := cache.Get("key1")
	if nested, ok := retrieved1.Metadata["nested"].(map[string]any); ok {
		if level2, ok := nested["level2"].(map[string]any); ok {
			level2["level3"] = "modified"
		}
	}

	// Get again and verify not modified
	retrieved2, _ := cache.Get("key1")
	nested := retrieved2.Metadata["nested"].(map[string]any)
	level2 := nested["level2"].(map[string]any)

	if level2["level3"] != "deep value" {
		t.Errorf("Expected 'deep value', got '%v' - cache was mutated!", level2["level3"])
	}
}

// TestDeepCopyResult tests the deepCopyResult function
func TestDeepCopyResult(t *testing.T) {
	tests := []struct {
		name   string
		result *GenerateResult
	}{
		{
			name:   "nil result",
			result: nil,
		},
		{
			name: "simple result",
			result: &GenerateResult{
				Content:      "test",
				FinishReason: "stop",
			},
		},
		{
			name: "result with tool calls",
			result: &GenerateResult{
				Content: "test",
				ToolCalls: []ToolCall{
					{ID: "1", Name: "tool1", Arguments: map[string]interface{}{"a": 1}},
					{ID: "2", Name: "tool2", Arguments: map[string]interface{}{"b": 2}},
				},
			},
		},
		{
			name: "result with nested metadata",
			result: &GenerateResult{
				Content: "test",
				Metadata: map[string]any{
					"simple": "value",
					"nested": map[string]any{
						"deep": map[string]any{
							"deeper": "value",
						},
					},
					"array": []any{1, "two", map[string]any{"three": 3}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := deepCopyResult(tt.result)

			if tt.result == nil {
				if copy != nil {
					t.Error("Expected nil copy for nil input")
				}
				return
			}

			// Verify copy is not the same pointer
			if copy == tt.result {
				t.Error("Deep copy should create a new pointer")
			}

			// Verify content is equal
			if copy.Content != tt.result.Content {
				t.Errorf("Content mismatch: expected %s, got %s", tt.result.Content, copy.Content)
			}

			// Verify modifying copy doesn't affect original
			copy.Content = "modified"
			if tt.result.Content == "modified" {
				t.Error("Modifying copy affected original")
			}
		})
	}
}

// TestCanonicalizeMap_NilInput tests nil handling in canonicalizeMap
func TestCanonicalizeMap_NilInput(t *testing.T) {
	result, err := canonicalizeMap(nil)
	if err != nil {
		t.Errorf("Expected no error for nil input, got %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty string for nil input, got %s", result)
	}
}

// TestCanonicalizeMap_NestedMaps tests nested map canonicalization
func TestCanonicalizeMap_NestedMaps(t *testing.T) {
	nestedMap := map[string]any{
		"outer": map[string]any{
			"inner": map[string]any{
				"deep": "value",
			},
		},
	}

	result, err := canonicalizeMap(nestedMap)
	if err != nil {
		t.Errorf("Expected no error for nested maps, got %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result for nested maps")
	}
}

// TestDeepCopyMap_NilInput tests nil handling in deepCopyMap
func TestDeepCopyMap_NilInput(t *testing.T) {
	result := deepCopyMap(nil)
	if result != nil {
		t.Error("Expected nil result for nil input")
	}
}

// TestDeepCopySlice_NilInput tests nil handling in deepCopySlice
func TestDeepCopySlice_NilInput(t *testing.T) {
	result := deepCopySlice(nil)
	if result != nil {
		t.Error("Expected nil result for nil input")
	}
}

// TestDeepCopySlice_ComplexTypes tests slice copying with various types
func TestDeepCopySlice_ComplexTypes(t *testing.T) {
	slice := []any{
		"string",
		123,
		true,
		map[string]any{"key": "value"},
		[]any{"nested", "slice"},
	}

	copied := deepCopySlice(slice)
	if copied == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(copied) != len(slice) {
		t.Errorf("Expected length %d, got %d", len(slice), len(copied))
	}

	// Modify original and verify copy is independent
	slice[0] = "modified"
	if copied[0] == "modified" {
		t.Error("Modifying original affected copy")
	}
}

// TestGenerateCacheKey_NilStop tests cache key with nil stop sequences
func TestGenerateCacheKey_NilStop(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	options1 := DefaultGenerateOptions()
	options1.Stop = nil

	options2 := DefaultGenerateOptions()
	options2.Stop = []string{}

	key1 := GenerateCacheKey("gpt-4", messages, options1)
	key2 := GenerateCacheKey("gpt-4", messages, options2)

	// Both should generate valid keys
	if key1 == "" || key2 == "" {
		t.Error("Expected non-empty keys")
	}
}

// TestGenerateCacheKey_NilResponseSchema tests with nil response schema
func TestGenerateCacheKey_NilResponseSchema(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	options := DefaultGenerateOptions()
	options.ResponseSchema = nil

	key := GenerateCacheKey("gpt-4", messages, options)
	if key == "" {
		t.Error("Expected non-empty key for nil response schema")
	}
}

// TestDeepCopyResult_NilToolCalls tests deep copy with nil tool calls
func TestDeepCopyResult_NilToolCalls(t *testing.T) {
	original := &GenerateResult{
		Content:   "test",
		ToolCalls: nil,
		Metadata:  nil,
	}

	copied := deepCopyResult(original)
	if copied == nil {
		t.Fatal("Expected non-nil copy")
	}

	if copied.ToolCalls != nil {
		t.Error("Expected nil tool calls in copy")
	}

	if copied.Metadata != nil {
		t.Error("Expected nil metadata in copy")
	}
}

// TestDeepCopyResult_EmptyToolCalls tests deep copy with empty tool calls
func TestDeepCopyResult_EmptyToolCalls(t *testing.T) {
	original := &GenerateResult{
		Content:   "test",
		ToolCalls: []ToolCall{},
	}

	copied := deepCopyResult(original)
	if copied == nil {
		t.Fatal("Expected non-nil copy")
	}

	if copied.ToolCalls == nil {
		t.Error("Expected non-nil but empty tool calls slice")
	}

	if len(copied.ToolCalls) != 0 {
		t.Errorf("Expected empty tool calls, got %d", len(copied.ToolCalls))
	}
}

// TestDeepCopyResult_ToolCallsWithNilArguments tests tool calls with nil arguments
func TestDeepCopyResult_ToolCallsWithNilArguments(t *testing.T) {
	original := &GenerateResult{
		Content: "test",
		ToolCalls: []ToolCall{
			{ID: "1", Name: "tool1", Arguments: nil},
		},
	}

	copied := deepCopyResult(original)
	if copied == nil {
		t.Fatal("Expected non-nil copy")
	}

	if len(copied.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(copied.ToolCalls))
	}

	if copied.ToolCalls[0].Arguments != nil {
		t.Error("Expected nil arguments in copied tool call")
	}
}

// TestCanonicalizeMap_UnsupportedType tests error handling for unsupported types
func TestCanonicalizeMap_UnsupportedType(t *testing.T) {
	// Maps containing channels or functions cannot be marshaled to JSON
	unsupportedMap := map[string]any{
		"channel": make(chan int),
	}

	_, err := canonicalizeMap(unsupportedMap)
	if err == nil {
		t.Error("Expected error for map with unsupported type (channel)")
	}
}

// TestCanonicalizeMap_NestedUnsupportedType tests error handling in nested maps
func TestCanonicalizeMap_NestedUnsupportedType(t *testing.T) {
	// Nested map with unsupported type should return error
	nestedUnsupportedMap := map[string]any{
		"outer": map[string]any{
			"inner": make(chan int),
		},
	}

	_, err := canonicalizeMap(nestedUnsupportedMap)
	if err == nil {
		t.Error("Expected error for nested map with unsupported type")
	}
}

// TestGenerateCacheKey_EmptyTools tests with empty tools list
func TestGenerateCacheKey_EmptyTools(t *testing.T) {
	messages := []Message{{Role: "user", Content: "test"}}

	options := DefaultGenerateOptions()
	options.Tools = []Tool{}

	key := GenerateCacheKey("gpt-4", messages, options)
	if key == "" {
		t.Error("Expected non-empty key for empty tools list")
	}
}

// TestGenerateCacheKey_MarshalError tests fallback when JSON marshaling fails
func TestGenerateCacheKey_MarshalError(t *testing.T) {
	// Create a message with a tool call that contains an unmarshalable type (channel)
	messages := []Message{
		{
			Role:    "assistant",
			Content: "test",
			ToolCalls: []ToolCall{
				{
					ID:   "call1",
					Name: "test_tool",
					Arguments: map[string]interface{}{
						"channel": make(chan int), // This cannot be marshaled to JSON
					},
				},
			},
		},
	}

	options := DefaultGenerateOptions()

	// This should trigger the error path in GenerateCacheKey and use the fallback
	key := GenerateCacheKey("gpt-4", messages, options)

	// The fallback key format is "lmName:messageCount"
	expectedFallback := "gpt-4:1"
	if key != expectedFallback {
		t.Errorf("Expected fallback key '%s', got '%s'", expectedFallback, key)
	}
}
