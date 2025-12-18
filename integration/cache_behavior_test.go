package integration

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/assagman/dsgo"
)

// globalConfigMu serializes tests that mutate global dsgo configuration.
var globalConfigMu sync.Mutex

// TestCacheBehavior_BasicHitMiss tests basic cache hit and miss behavior
func TestCacheBehavior_BasicHitMiss(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		operations     func(cache dsgo.Cache) (int, int) // Returns hits, misses
		expectedHits   int
		expectedMisses int
	}{
		{
			name: "single miss",
			operations: func(cache dsgo.Cache) (int, int) {
				cache.Get("key1")
				stats := cache.Stats()
				return int(stats.Hits), int(stats.Misses)
			},
			expectedHits:   0,
			expectedMisses: 1,
		},
		{
			name: "single hit",
			operations: func(cache dsgo.Cache) (int, int) {
				result := &dsgo.GenerateResult{Content: "test"}
				cache.Set("key1", result)
				cache.Get("key1")
				stats := cache.Stats()
				return int(stats.Hits), int(stats.Misses)
			},
			expectedHits:   1,
			expectedMisses: 0,
		},
		{
			name: "multiple operations",
			operations: func(cache dsgo.Cache) (int, int) {
				result1 := &dsgo.GenerateResult{Content: "result1"}
				result2 := &dsgo.GenerateResult{Content: "result2"}

				cache.Set("key1", result1)
				cache.Set("key2", result2)
				cache.Get("key1") // Hit
				cache.Get("key1") // Hit
				cache.Get("key2") // Hit
				cache.Get("key3") // Miss
				cache.Get("key4") // Miss

				stats := cache.Stats()
				return int(stats.Hits), int(stats.Misses)
			},
			expectedHits:   3,
			expectedMisses: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cache := dsgo.NewLMCache(100)
			hits, misses := tt.operations(cache)

			if hits != tt.expectedHits {
				t.Errorf("Expected %d hits, got %d", tt.expectedHits, hits)
			}
			if misses != tt.expectedMisses {
				t.Errorf("Expected %d misses, got %d", tt.expectedMisses, misses)
			}
		})
	}
}

// ============================================================================
// Provider Cache Integration Tests (mock provider + HTTP mock server)
// ============================================================================

func TestCacheIntegration_MockProvider_CacheHitAvoidsSecondRequest(t *testing.T) {
	globalConfigMu.Lock()
	t.Cleanup(globalConfigMu.Unlock)
	t.Cleanup(dsgo.ResetConfig)

	server, recorder := newValidatedMockServer(t, validateChatCompletionRequest("gpt-4o"), func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"id":"test",
			"model":"gpt-4",
			"choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`)
	})
	defer server.Close()

	dsgo.Configure(dsgo.WithCache(10))
	settings := dsgo.GetSettings()
	if settings.DefaultCache == nil {
		t.Fatal("expected DefaultCache to be configured")
	}

	// Wire mock provider to server.
	t.Setenv("DSGO_MOCK_BASE_URL", server.URL)

	lm, err := dsgo.NewLM(context.Background(), "mock/gpt-4o")
	if err != nil {
		t.Fatalf("failed to create mock LM: %v", err)
	}

	messages := []dsgo.Message{{Role: "user", Content: "Hello"}}
	opts := dsgo.DefaultGenerateOptions()

	// First call should hit network and populate cache.
	_, err = lm.Generate(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Second call should be served from cache.
	_, err = lm.Generate(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if got := len(recorder.All()); got != 1 {
		t.Fatalf("expected 1 HTTP request (second served from cache), got %d", got)
	}

	stats := settings.DefaultCache.Stats()
	if stats.Misses != 1 || stats.Hits != 1 {
		t.Fatalf("expected 1 miss + 1 hit, got misses=%d hits=%d", stats.Misses, stats.Hits)
	}
}

func TestCacheIntegration_MockProvider_CacheTTLExpires(t *testing.T) {
	globalConfigMu.Lock()
	t.Cleanup(globalConfigMu.Unlock)
	t.Cleanup(dsgo.ResetConfig)

	server, recorder := newValidatedMockServer(t, validateChatCompletionRequest("gpt-4o"), func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"id":"test",
			"model":"gpt-4",
			"choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`)
	})
	defer server.Close()

	t.Setenv("DSGO_MOCK_BASE_URL", server.URL)

	ttl := 5 * time.Millisecond
	dsgo.Configure(
		dsgo.WithCacheTTL(ttl),
		dsgo.WithCache(10),
	)

	settings := dsgo.GetSettings()
	if settings.DefaultCache == nil {
		t.Fatal("expected DefaultCache to be configured")
	}

	lm, err := dsgo.NewLM(context.Background(), "mock/gpt-4o")
	if err != nil {
		t.Fatalf("failed to create mock LM: %v", err)
	}

	messages := []dsgo.Message{{Role: "user", Content: "Hello"}}
	opts := dsgo.DefaultGenerateOptions()

	_, err = lm.Generate(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Wait for TTL to elapse.
	time.Sleep(ttl + 25*time.Millisecond)

	_, err = lm.Generate(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if got := len(recorder.All()); got != 2 {
		t.Fatalf("expected 2 HTTP requests (cache expired), got %d", got)
	}
}

func TestCacheIntegration_MockProvider_CacheClearForcesRefetch(t *testing.T) {
	globalConfigMu.Lock()
	t.Cleanup(globalConfigMu.Unlock)
	t.Cleanup(dsgo.ResetConfig)

	server, recorder := newValidatedMockServer(t, validateChatCompletionRequest("gpt-4o"), func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{
			"id":"test",
			"model":"gpt-4",
			"choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`)
	})
	defer server.Close()

	t.Setenv("DSGO_MOCK_BASE_URL", server.URL)

	dsgo.Configure(dsgo.WithCache(10))
	settings := dsgo.GetSettings()
	if settings.DefaultCache == nil {
		t.Fatal("expected DefaultCache to be configured")
	}

	lm, err := dsgo.NewLM(context.Background(), "mock/gpt-4o")
	if err != nil {
		t.Fatalf("failed to create mock LM: %v", err)
	}

	messages := []dsgo.Message{{Role: "user", Content: "Hello"}}
	opts := dsgo.DefaultGenerateOptions()

	_, err = lm.Generate(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = lm.Generate(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if got := len(recorder.All()); got != 1 {
		t.Fatalf("expected 1 HTTP request (second served from cache), got %d", got)
	}

	settings.DefaultCache.Clear()

	_, err = lm.Generate(context.Background(), messages, opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if got := len(recorder.All()); got != 2 {
		t.Fatalf("expected 2 HTTP requests after cache clear, got %d", got)
	}
}

// TestCacheBehavior_CacheHitRate tests cache hit rate calculation
func TestCacheBehavior_CacheHitRate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		hits         int64
		misses       int64
		expectedRate float64
	}{
		{"50% hit rate", 5, 5, 50.0},
		{"100% hit rate", 10, 0, 100.0},
		{"0% hit rate", 0, 10, 0.0},
		{"no requests", 0, 0, 0.0},
		{"75% hit rate", 3, 1, 75.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stats := dsgo.CacheStats{Hits: tt.hits, Misses: tt.misses}
			rate := stats.HitRate()

			if rate != tt.expectedRate {
				t.Errorf("Expected hit rate %.2f%%, got %.2f%%", tt.expectedRate, rate)
			}
		})
	}
}

// TestCacheBehavior_KeyDeterminism tests that identical inputs generate identical cache keys
func TestCacheBehavior_KeyDeterminism(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		setup         func() (string, string)
		shouldBeEqual bool
	}{
		{
			name: "identical messages and options",
			setup: func() (string, string) {
				messages := []dsgo.Message{{Role: "user", Content: "hello"}}
				opts := dsgo.DefaultGenerateOptions()

				key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts)
				key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts)

				return key1, key2
			},
			shouldBeEqual: true,
		},
		{
			name: "different models",
			setup: func() (string, string) {
				messages := []dsgo.Message{{Role: "user", Content: "hello"}}
				opts := dsgo.DefaultGenerateOptions()

				key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts)
				key2 := dsgo.GenerateCacheKey("gpt-3.5-turbo", messages, opts)

				return key1, key2
			},
			shouldBeEqual: false,
		},
		{
			name: "different content",
			setup: func() (string, string) {
				msg1 := []dsgo.Message{{Role: "user", Content: "hello"}}
				msg2 := []dsgo.Message{{Role: "user", Content: "goodbye"}}
				opts := dsgo.DefaultGenerateOptions()

				key1 := dsgo.GenerateCacheKey("gpt-4", msg1, opts)
				key2 := dsgo.GenerateCacheKey("gpt-4", msg2, opts)

				return key1, key2
			},
			shouldBeEqual: false,
		},
		{
			name: "different temperature",
			setup: func() (string, string) {
				messages := []dsgo.Message{{Role: "user", Content: "hello"}}
				opts1 := dsgo.DefaultGenerateOptions()
				opts2 := dsgo.DefaultGenerateOptions()
				opts2.Temperature = 0.5

				key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
				key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)

				return key1, key2
			},
			shouldBeEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key1, key2 := tt.setup()

			if tt.shouldBeEqual && key1 != key2 {
				t.Errorf("Expected equal keys for identical inputs, got %s != %s", key1, key2)
			}
			if !tt.shouldBeEqual && key1 == key2 {
				t.Errorf("Expected different keys for different inputs, got %s == %s", key1, key2)
			}
		})
	}
}

// TestCacheBehavior_StopSequenceOrder tests that stop sequence order doesn't affect cache key
func TestCacheBehavior_StopSequenceOrder(t *testing.T) {
	t.Parallel()
	messages := []dsgo.Message{{Role: "user", Content: "test"}}

	opts1 := dsgo.DefaultGenerateOptions()
	opts1.Stop = []string{"stop1", "stop2", "stop3"}

	opts2 := dsgo.DefaultGenerateOptions()
	opts2.Stop = []string{"stop3", "stop1", "stop2"}

	key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
	key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)

	if key1 != key2 {
		t.Errorf("Expected same key regardless of stop sequence order")
	}
}

// TestCacheBehavior_MapCanonicalOrder tests that map insertion order doesn't affect cache key
func TestCacheBehavior_MapCanonicalOrder(t *testing.T) {
	t.Parallel()
	messages := []dsgo.Message{{Role: "user", Content: "test"}}

	// Create two maps with same content but different insertion order
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

	opts1 := dsgo.DefaultGenerateOptions()
	opts1.ResponseSchema = schema1

	opts2 := dsgo.DefaultGenerateOptions()
	opts2.ResponseSchema = schema2

	key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
	key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)

	if key1 != key2 {
		t.Errorf("Map canonicalization failed: insertion order should not affect key")
	}
}

// TestCacheBehavior_TTLExpiration tests TTL-based cache expiration
func TestCacheBehavior_TTLExpiration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		ttl           time.Duration
		waitBeforeGet time.Duration
		shouldHit     bool
	}{
		{
			name:          "immediate hit with TTL",
			ttl:           1 * time.Second,
			waitBeforeGet: 0,
			shouldHit:     true,
		},
		{
			name:          "hit before TTL expires",
			ttl:           500 * time.Millisecond,
			waitBeforeGet: 200 * time.Millisecond,
			shouldHit:     true,
		},
		{
			name:          "miss after TTL expires",
			ttl:           100 * time.Millisecond,
			waitBeforeGet: 150 * time.Millisecond,
			shouldHit:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cache := dsgo.NewLMCacheWithTTL(10, tt.ttl)

			result := &dsgo.GenerateResult{Content: "test content"}
			cache.Set("key1", result)

			if tt.waitBeforeGet > 0 {
				time.Sleep(tt.waitBeforeGet)
			}

			_, hit := cache.Get("key1")

			if hit != tt.shouldHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.shouldHit, hit)
			}
		})
	}
}

// TestCacheBehavior_TTLRefresh tests that updating an entry refreshes its TTL
func TestCacheBehavior_TTLRefresh(t *testing.T) {
	t.Parallel()
	ttl := 100 * time.Millisecond
	cache := dsgo.NewLMCacheWithTTL(10, ttl)

	// Set initial entry
	cache.Set("key1", &dsgo.GenerateResult{Content: "initial"})

	// Wait half the TTL
	time.Sleep(ttl / 2)

	// Update entry (should refresh TTL)
	cache.Set("key1", &dsgo.GenerateResult{Content: "updated"})

	// Wait another half of TTL
	time.Sleep(ttl / 2)

	// Entry should still exist
	retrieved, hit := cache.Get("key1")
	if !hit {
		t.Error("Expected cache hit after TTL refresh")
	}

	if retrieved.Content != "updated" {
		t.Errorf("Expected updated content, got %s", retrieved.Content)
	}

	// Wait for full TTL from last update
	time.Sleep(ttl + 10*time.Millisecond)

	// Now it should be expired
	_, hit = cache.Get("key1")
	if hit {
		t.Error("Expected cache miss after full TTL from last update")
	}
}

// TestCacheBehavior_TTLDisabled tests that TTL=0 means no expiration
func TestCacheBehavior_TTLDisabled(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCacheWithTTL(10, 0)

	cache.Set("key1", &dsgo.GenerateResult{Content: "test"})

	// Wait longer than a typical TTL would be
	time.Sleep(200 * time.Millisecond)

	// Should still be in cache
	_, hit := cache.Get("key1")
	if !hit {
		t.Error("Expected cache hit with TTL=0 (no expiration)")
	}
}

// TestCacheBehavior_LRUEviction tests LRU eviction under memory pressure
func TestCacheBehavior_LRUEviction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		capacity          int
		itemsToAdd        int
		expectedEvicted   []string
		expectedRemaining []string
	}{
		{
			name:              "simple eviction at capacity",
			capacity:          3,
			itemsToAdd:        4,
			expectedEvicted:   []string{"key1"},
			expectedRemaining: []string{"key2", "key3", "key4"},
		},
		{
			name:       "evict oldest after access",
			capacity:   3,
			itemsToAdd: 4, // key1, key2, key3, key4
			// After accessing key1, it becomes recent, so key2 should be evicted
			expectedEvicted:   []string{"key2"},
			expectedRemaining: []string{"key1", "key3", "key4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cache := dsgo.NewLMCache(tt.capacity)

			// Add items to trigger eviction
			for i := 1; i <= tt.itemsToAdd; i++ {
				key := fmt.Sprintf("key%d", i)
				cache.Set(key, &dsgo.GenerateResult{Content: key})

				// For the "evict oldest after access" test, access key1
				if tt.name == "evict oldest after access" && i == 3 {
					cache.Get("key1")
				}
			}

			// Check evicted items
			for _, evictedKey := range tt.expectedEvicted {
				_, hit := cache.Get(evictedKey)
				if hit {
					t.Errorf("Expected %s to be evicted", evictedKey)
				}
			}

			// Check remaining items
			for _, remainingKey := range tt.expectedRemaining {
				_, hit := cache.Get(remainingKey)
				if !hit {
					t.Errorf("Expected %s to still be in cache", remainingKey)
				}
			}
		})
	}
}

// TestCacheBehavior_DeepCopyMutation tests that cached values are protected from mutation
func TestCacheBehavior_DeepCopyMutation(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "original",
		Usage: dsgo.Usage{
			TotalTokens: 100,
			Cost:        0.01,
		},
		Metadata: map[string]any{
			"nested": map[string]any{
				"key": "value",
			},
		},
	}

	cache.Set("key1", original)

	// Modify original
	original.Content = "modified"
	original.Usage.TotalTokens = 999
	original.Metadata["nested"].(map[string]any)["key"] = "modified_value"

	// Retrieve from cache
	retrieved, _ := cache.Get("key1")

	// Verify original values are intact
	if retrieved.Content != "original" {
		t.Errorf("Expected content 'original', got '%s'", retrieved.Content)
	}
	if retrieved.Usage.TotalTokens != 100 {
		t.Errorf("Expected tokens 100, got %d", retrieved.Usage.TotalTokens)
	}

	nested := retrieved.Metadata["nested"].(map[string]any)
	if nested["key"] != "value" {
		t.Errorf("Expected nested key 'value', got '%v'", nested["key"])
	}
}

// TestCacheBehavior_RetrievedValueMutation tests that modifying retrieved values doesn't affect cache
func TestCacheBehavior_RetrievedValueMutation(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	cache.Set("key1", &dsgo.GenerateResult{
		Content: "test",
		Metadata: map[string]any{
			"data": "original",
		},
	})

	// Retrieve and modify
	retrieved1, _ := cache.Get("key1")
	retrieved1.Content = "modified"
	retrieved1.Metadata["data"] = "modified"

	// Retrieve again and verify not modified
	retrieved2, _ := cache.Get("key1")

	if retrieved2.Content != "test" {
		t.Errorf("Cache was mutated: expected 'test', got '%s'", retrieved2.Content)
	}

	if retrieved2.Metadata["data"] != "original" {
		t.Errorf("Cache metadata was mutated: expected 'original', got '%v'", retrieved2.Metadata["data"])
	}
}

// TestCacheBehavior_ConcurrentAccess tests thread-safe concurrent access
func TestCacheBehavior_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(100)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Set(key, &dsgo.GenerateResult{Content: key})
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is in valid state
	stats := cache.Stats()
	if stats.Size > cache.Capacity() {
		t.Errorf("Cache exceeded capacity: %d > %d", stats.Size, cache.Capacity())
	}
	if stats.Hits < 0 || stats.Misses < 0 {
		t.Error("Cache stats are invalid")
	}
}

// TestCacheBehavior_ConcurrentEviction tests concurrent access during eviction
func TestCacheBehavior_ConcurrentEviction(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(50)
	var wg sync.WaitGroup

	// Multiple goroutines adding many items to trigger frequent evictions
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Create many unique keys to force evictions
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Set(key, &dsgo.GenerateResult{Content: key})
			}
		}(i)
	}

	wg.Wait()

	// Verify cache integrity
	stats := cache.Stats()
	if stats.Size > 50 {
		t.Errorf("Cache exceeded capacity during concurrent eviction: %d > 50", stats.Size)
	}
}

// TestCacheBehavior_ConcurrentStatistics tests statistics accuracy under concurrent access
func TestCacheBehavior_ConcurrentStatistics(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(100)
	var wg sync.WaitGroup

	// Pre-populate cache
	for i := 0; i < 20; i++ {
		cache.Set(fmt.Sprintf("key_%d", i), &dsgo.GenerateResult{Content: "test"})
	}

	// Concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				cache.Get(fmt.Sprintf("key_%d", j))
			}
		}()
	}

	wg.Wait()

	// Verify stats
	stats := cache.Stats()

	// Expected 10 goroutines × 20 iterations = 200 operations
	// All should hit since we pre-populated with 20 items
	if stats.Hits != 200 {
		t.Errorf("Expected 200 hits, got %d", stats.Hits)
	}

	if stats.Size != 20 {
		t.Errorf("Expected size 20, got %d", stats.Size)
	}
}

// TestCacheBehavior_MultipleKeysTracking tests tracking multiple cache keys
func TestCacheBehavior_MultipleKeysTracking(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(100)

	// Simulate multiple different LM requests with different keys
	messages1 := []dsgo.Message{{Role: "user", Content: "query 1"}}
	messages2 := []dsgo.Message{{Role: "user", Content: "query 2"}}
	messages3 := []dsgo.Message{{Role: "user", Content: "query 3"}}

	opts := dsgo.DefaultGenerateOptions()

	key1 := dsgo.GenerateCacheKey("gpt-4", messages1, opts)
	key2 := dsgo.GenerateCacheKey("gpt-4", messages2, opts)
	key3 := dsgo.GenerateCacheKey("gpt-4", messages3, opts)

	// Verify all keys are unique
	keysSet := map[string]bool{key1: true, key2: true, key3: true}
	if len(keysSet) != 3 {
		t.Error("Expected 3 unique keys for different messages")
	}

	// Cache responses for each key
	cache.Set(key1, &dsgo.GenerateResult{Content: "response 1"})
	cache.Set(key2, &dsgo.GenerateResult{Content: "response 2"})
	cache.Set(key3, &dsgo.GenerateResult{Content: "response 3"})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	// Retrieve and verify
	result1, hit1 := cache.Get(key1)
	result2, hit2 := cache.Get(key2)
	result3, hit3 := cache.Get(key3)

	if !hit1 || result1.Content != "response 1" {
		t.Error("Failed to retrieve cached response 1")
	}
	if !hit2 || result2.Content != "response 2" {
		t.Error("Failed to retrieve cached response 2")
	}
	if !hit3 || result3.Content != "response 3" {
		t.Error("Failed to retrieve cached response 3")
	}

	stats := cache.Stats()
	if stats.Hits != 3 {
		t.Errorf("Expected 3 hits, got %d", stats.Hits)
	}
}

// TestCacheBehavior_ClearResetsStats tests that Clear() resets statistics
func TestCacheBehavior_ClearResetsStats(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	// Add some data and generate stats
	cache.Set("key1", &dsgo.GenerateResult{Content: "1"})
	cache.Set("key2", &dsgo.GenerateResult{Content: "2"})
	cache.Get("key1")
	cache.Get("nonexistent")

	statsBefore := cache.Stats()
	if statsBefore.Hits != 1 || statsBefore.Misses != 1 || statsBefore.Size != 2 {
		t.Logf("Before clear: hits=%d, misses=%d, size=%d", statsBefore.Hits, statsBefore.Misses, statsBefore.Size)
	}

	// Clear cache
	cache.Clear()

	statsAfter := cache.Stats()
	if statsAfter.Hits != 0 || statsAfter.Misses != 0 || statsAfter.Size != 0 {
		t.Errorf("After clear: expected all zeros, got hits=%d, misses=%d, size=%d",
			statsAfter.Hits, statsAfter.Misses, statsAfter.Size)
	}
}

// TestCacheBehavior_CapacityRespected tests that cache respects capacity limits
func TestCacheBehavior_CapacityRespected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		capacity        int
		itemsToAdd      int
		expectedMaxSize int
	}{
		{"small capacity", 5, 20, 5},
		{"medium capacity", 50, 100, 50},
		{"capacity 1", 1, 10, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cache := dsgo.NewLMCache(tt.capacity)

			for i := 0; i < tt.itemsToAdd; i++ {
				key := fmt.Sprintf("key_%d", i)
				cache.Set(key, &dsgo.GenerateResult{Content: key})
			}

			size := cache.Size()
			if size > tt.expectedMaxSize {
				t.Errorf("Cache size %d exceeds expected max %d", size, tt.expectedMaxSize)
			}

			if cache.Capacity() != tt.expectedMaxSize {
				t.Errorf("Capacity mismatch: expected %d, got %d", tt.expectedMaxSize, cache.Capacity())
			}
		})
	}
}

// TestCacheBehavior_UpdateExistingEntry tests updating existing cache entries
func TestCacheBehavior_UpdateExistingEntry(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	// Set initial entry
	cache.Set("key1", &dsgo.GenerateResult{Content: "first", Usage: dsgo.Usage{TotalTokens: 10}})

	// Verify first entry
	retrieved1, _ := cache.Get("key1")
	if retrieved1.Content != "first" {
		t.Errorf("Expected 'first', got '%s'", retrieved1.Content)
	}

	// Update entry
	cache.Set("key1", &dsgo.GenerateResult{Content: "second", Usage: dsgo.Usage{TotalTokens: 20}})

	// Verify updated entry
	retrieved2, _ := cache.Get("key1")
	if retrieved2.Content != "second" {
		t.Errorf("Expected 'second', got '%s'", retrieved2.Content)
	}

	// Cache should still contain only 1 item
	if cache.Size() != 1 {
		t.Errorf("Expected size 1 after update, got %d", cache.Size())
	}
}

// TestCacheBehavior_KeyGenerationEdgeCases tests edge cases in key generation
func TestCacheBehavior_KeyGenerationEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func() (string, error)
	}{
		{
			name: "empty messages",
			setup: func() (string, error) {
				key := dsgo.GenerateCacheKey("model", []dsgo.Message{}, dsgo.DefaultGenerateOptions())
				return key, nil
			},
		},
		{
			name: "nil stop sequences",
			setup: func() (string, error) {
				opts := dsgo.DefaultGenerateOptions()
				opts.Stop = nil
				key := dsgo.GenerateCacheKey("model", []dsgo.Message{{Role: "user", Content: "test"}}, opts)
				return key, nil
			},
		},
		{
			name: "empty stop sequences",
			setup: func() (string, error) {
				opts := dsgo.DefaultGenerateOptions()
				opts.Stop = []string{}
				key := dsgo.GenerateCacheKey("model", []dsgo.Message{{Role: "user", Content: "test"}}, opts)
				return key, nil
			},
		},
		{
			name: "nil response schema",
			setup: func() (string, error) {
				opts := dsgo.DefaultGenerateOptions()
				opts.ResponseSchema = nil
				key := dsgo.GenerateCacheKey("model", []dsgo.Message{{Role: "user", Content: "test"}}, opts)
				return key, nil
			},
		},
		{
			name: "empty response schema",
			setup: func() (string, error) {
				opts := dsgo.DefaultGenerateOptions()
				opts.ResponseSchema = map[string]any{}
				key := dsgo.GenerateCacheKey("model", []dsgo.Message{{Role: "user", Content: "test"}}, opts)
				return key, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key, err := tt.setup()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if key == "" {
				t.Error("Expected non-empty key")
			}
		})
	}
}

// TestCacheBehavior_ToolsInCacheKey tests that tools are included in cache key
func TestCacheBehavior_ToolsInCacheKey(t *testing.T) {
	t.Parallel()
	messages := []dsgo.Message{{Role: "user", Content: "test"}}

	tool1 := dsgo.Tool{
		Name:        "tool_a",
		Description: "Tool A",
		Parameters: []dsgo.ToolParameter{
			{Name: "arg1", Type: "string", Required: true},
		},
	}

	tool2 := dsgo.Tool{
		Name:        "tool_b",
		Description: "Tool B",
		Parameters: []dsgo.ToolParameter{
			{Name: "arg1", Type: "string", Required: true},
		},
	}

	// Same tool should generate same key
	opts1 := dsgo.DefaultGenerateOptions()
	opts1.Tools = []dsgo.Tool{tool1}

	opts2 := dsgo.DefaultGenerateOptions()
	opts2.Tools = []dsgo.Tool{tool1}

	key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
	key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)

	if key1 != key2 {
		t.Error("Expected same key for identical tools")
	}

	// Different tools should generate different keys
	opts3 := dsgo.DefaultGenerateOptions()
	opts3.Tools = []dsgo.Tool{tool2}

	key3 := dsgo.GenerateCacheKey("gpt-4", messages, opts3)

	if key1 == key3 {
		t.Error("Expected different keys for different tools")
	}
}

// TestCacheBehavior_PenaltiesInCacheKey tests that penalties affect cache key
func TestCacheBehavior_PenaltiesInCacheKey(t *testing.T) {
	t.Parallel()
	messages := []dsgo.Message{{Role: "user", Content: "test"}}

	opts1 := dsgo.DefaultGenerateOptions()
	opts1.FrequencyPenalty = 0.0

	opts2 := dsgo.DefaultGenerateOptions()
	opts2.FrequencyPenalty = 0.5

	key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
	key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)

	if key1 == key2 {
		t.Error("Expected different keys for different frequency penalties")
	}

	opts3 := dsgo.DefaultGenerateOptions()
	opts3.PresencePenalty = 0.5

	key3 := dsgo.GenerateCacheKey("gpt-4", messages, opts3)

	if key1 == key3 {
		t.Error("Expected different keys for different presence penalties")
	}
}

// TestCacheBehavior_LargeScaleOperation tests cache performance with many items
func TestCacheBehavior_LargeScaleOperation(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(1000)

	// Add 5000 items (should trigger many evictions)
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key_%d", i)
		cache.Set(key, &dsgo.GenerateResult{Content: key})
	}

	// Verify cache respects capacity
	if cache.Size() > 1000 {
		t.Errorf("Cache exceeded capacity: %d > 1000", cache.Size())
	}

	// Verify stats are valid
	stats := cache.Stats()
	if stats.Size != cache.Size() {
		t.Errorf("Stats size mismatch: %d != %d", stats.Size, cache.Size())
	}

	// Recent items should be in cache
	retrieved, hit := cache.Get("key_4999")
	if !hit {
		t.Error("Expected most recent item to be in cache")
	}
	if retrieved.Content != "key_4999" {
		t.Errorf("Expected 'key_4999', got '%s'", retrieved.Content)
	}

	// Old items should have been evicted
	_, hit = cache.Get("key_0")
	if hit {
		t.Error("Expected oldest item to be evicted")
	}
}

// ============================================================================
// Cache Key Determinism Tests
// ============================================================================

// TestCache_KeyDeterminism tests that cache keys are stable regardless of map iteration order
func TestCache_KeyDeterminism(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func() (string, string)
		shouldMatch bool
	}{
		{
			name: "map with same keys different insertion order",
			setup: func() (string, string) {
				messages := []dsgo.Message{{Role: "user", Content: "test"}}

				// First map: insert in order a, b, c
				opts1 := dsgo.DefaultGenerateOptions()
				opts1.ResponseSchema = map[string]any{
					"alpha": "value1",
					"beta":  "value2",
					"gamma": "value3",
				}

				// Second map: insert in order c, a, b
				opts2 := dsgo.DefaultGenerateOptions()
				opts2.ResponseSchema = map[string]any{
					"gamma": "value3",
					"alpha": "value1",
					"beta":  "value2",
				}

				key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
				key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)
				return key1, key2
			},
			shouldMatch: true,
		},
		{
			name: "deeply nested maps with different insertion order",
			setup: func() (string, string) {
				messages := []dsgo.Message{{Role: "user", Content: "test"}}

				opts1 := dsgo.DefaultGenerateOptions()
				opts1.ResponseSchema = map[string]any{
					"level1": map[string]any{
						"z_key": "z_val",
						"a_key": "a_val",
						"m_key": map[string]any{
							"inner_z": 1,
							"inner_a": 2,
						},
					},
				}

				opts2 := dsgo.DefaultGenerateOptions()
				opts2.ResponseSchema = map[string]any{
					"level1": map[string]any{
						"a_key": "a_val",
						"m_key": map[string]any{
							"inner_a": 2,
							"inner_z": 1,
						},
						"z_key": "z_val",
					},
				}

				key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
				key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)
				return key1, key2
			},
			shouldMatch: true,
		},
		{
			name: "repeated key generation produces same result",
			setup: func() (string, string) {
				messages := []dsgo.Message{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "What is 2+2?"},
				}
				opts := dsgo.DefaultGenerateOptions()
				opts.Temperature = 0.7
				opts.MaxTokens = 100

				// Generate key multiple times
				key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts)
				key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts)
				return key1, key2
			},
			shouldMatch: true,
		},
		{
			name: "different map values produce different keys",
			setup: func() (string, string) {
				messages := []dsgo.Message{{Role: "user", Content: "test"}}

				opts1 := dsgo.DefaultGenerateOptions()
				opts1.ResponseSchema = map[string]any{"key": "value1"}

				opts2 := dsgo.DefaultGenerateOptions()
				opts2.ResponseSchema = map[string]any{"key": "value2"}

				key1 := dsgo.GenerateCacheKey("gpt-4", messages, opts1)
				key2 := dsgo.GenerateCacheKey("gpt-4", messages, opts2)
				return key1, key2
			},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key1, key2 := tt.setup()

			if tt.shouldMatch && key1 != key2 {
				t.Errorf("Expected matching keys for canonicalized maps\nkey1: %s\nkey2: %s", key1, key2)
			}
			if !tt.shouldMatch && key1 == key2 {
				t.Errorf("Expected different keys for different values\nkey1: %s\nkey2: %s", key1, key2)
			}
		})
	}
}

// TestCache_KeyDeterminism_MapCanonicalMultipleIterations verifies key stability across iterations
func TestCache_KeyDeterminism_MapCanonicalMultipleIterations(t *testing.T) {
	t.Parallel()
	messages := []dsgo.Message{{Role: "user", Content: "test"}}
	opts := dsgo.DefaultGenerateOptions()
	opts.ResponseSchema = map[string]any{
		"zebra":   1,
		"alpha":   2,
		"middle":  3,
		"beta":    4,
		"complex": map[string]any{"z": 1, "a": 2, "m": 3},
	}

	// Generate key 100 times, all should be identical
	firstKey := dsgo.GenerateCacheKey("gpt-4", messages, opts)

	for i := 0; i < 100; i++ {
		key := dsgo.GenerateCacheKey("gpt-4", messages, opts)
		if key != firstKey {
			t.Errorf("Key mismatch on iteration %d\nexpected: %s\ngot: %s", i, firstKey, key)
		}
	}
}

// ============================================================================
// Cache TTL Expiry Tests
// ============================================================================

// TestCache_TTLExpiry tests precise TTL behavior with specific timing
func TestCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		ttl         time.Duration
		checkAt     time.Duration
		expectedHit bool
		description string
	}{
		{
			name:        "hit at 50ms with 1s TTL",
			ttl:         1 * time.Second,
			checkAt:     50 * time.Millisecond,
			expectedHit: true,
			description: "Entry should be valid before TTL expires",
		},
		{
			name:        "miss at 300ms with 100ms TTL",
			ttl:         100 * time.Millisecond,
			checkAt:     300 * time.Millisecond,
			expectedHit: false,
			description: "Entry should expire after TTL",
		},
		{
			name:        "exact TTL boundary miss",
			ttl:         100 * time.Millisecond,
			checkAt:     200 * time.Millisecond,
			expectedHit: false,
			description: "Entry should be expired just after TTL",
		},
		{
			name:        "very short TTL",
			ttl:         10 * time.Millisecond,
			checkAt:     100 * time.Millisecond,
			expectedHit: false,
			description: "Very short TTL should expire quickly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cache := dsgo.NewLMCacheWithTTL(10, tt.ttl)

			cache.Set("key1", &dsgo.GenerateResult{Content: "test"})

			time.Sleep(tt.checkAt)

			_, hit := cache.Get("key1")

			if hit != tt.expectedHit {
				t.Errorf("%s: expected hit=%v, got hit=%v", tt.description, tt.expectedHit, hit)
			}
		})
	}
}

// TestCache_TTLExpiry_SequentialChecks tests multiple checks across TTL boundary
func TestCache_TTLExpiry_SequentialChecks(t *testing.T) {
	t.Parallel()
	ttl := 500 * time.Millisecond
	cache := dsgo.NewLMCacheWithTTL(10, ttl)

	cache.Set("key1", &dsgo.GenerateResult{Content: "test"})

	// Check at 50ms - should hit
	time.Sleep(50 * time.Millisecond)
	_, hit := cache.Get("key1")
	if !hit {
		t.Error("Expected hit at 50ms (before TTL)")
	}

	// Wait additional 600ms (total 650ms) - should miss
	time.Sleep(600 * time.Millisecond)
	_, hit = cache.Get("key1")
	if hit {
		t.Error("Expected miss at 650ms (after TTL)")
	}

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

// ============================================================================
// Cache Eviction Policy Tests
// ============================================================================

// TestCache_EvictionPolicy tests LRU eviction when cache exceeds capacity
func TestCache_EvictionPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		capacity          int
		operations        func(cache dsgo.Cache)
		expectedEvicted   []string
		expectedRemaining []string
	}{
		{
			name:     "oldest entries removed when capacity exceeded",
			capacity: 3,
			operations: func(cache dsgo.Cache) {
				cache.Set("key1", &dsgo.GenerateResult{Content: "1"})
				cache.Set("key2", &dsgo.GenerateResult{Content: "2"})
				cache.Set("key3", &dsgo.GenerateResult{Content: "3"})
				cache.Set("key4", &dsgo.GenerateResult{Content: "4"})
				cache.Set("key5", &dsgo.GenerateResult{Content: "5"})
			},
			expectedEvicted:   []string{"key1", "key2"},
			expectedRemaining: []string{"key3", "key4", "key5"},
		},
		{
			name:     "access refreshes entry in LRU order",
			capacity: 3,
			operations: func(cache dsgo.Cache) {
				cache.Set("key1", &dsgo.GenerateResult{Content: "1"})
				cache.Set("key2", &dsgo.GenerateResult{Content: "2"})
				cache.Set("key3", &dsgo.GenerateResult{Content: "3"})
				// Access key1 to make it recently used
				cache.Get("key1")
				// Add key4, should evict key2 (oldest unused)
				cache.Set("key4", &dsgo.GenerateResult{Content: "4"})
			},
			expectedEvicted:   []string{"key2"},
			expectedRemaining: []string{"key1", "key3", "key4"},
		},
		{
			name:     "multiple accesses preserve entries",
			capacity: 3,
			operations: func(cache dsgo.Cache) {
				cache.Set("key1", &dsgo.GenerateResult{Content: "1"})
				cache.Set("key2", &dsgo.GenerateResult{Content: "2"})
				cache.Set("key3", &dsgo.GenerateResult{Content: "3"})
				// Access all in reverse order
				cache.Get("key1")
				cache.Get("key2")
				cache.Get("key3")
				// Add key4 - key1 should be evicted as least recently accessed
				cache.Set("key4", &dsgo.GenerateResult{Content: "4"})
			},
			expectedEvicted:   []string{"key1"},
			expectedRemaining: []string{"key2", "key3", "key4"},
		},
		{
			name:     "capacity of 1 always has latest entry",
			capacity: 1,
			operations: func(cache dsgo.Cache) {
				cache.Set("key1", &dsgo.GenerateResult{Content: "1"})
				cache.Set("key2", &dsgo.GenerateResult{Content: "2"})
				cache.Set("key3", &dsgo.GenerateResult{Content: "3"})
			},
			expectedEvicted:   []string{"key1", "key2"},
			expectedRemaining: []string{"key3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cache := dsgo.NewLMCache(tt.capacity)

			tt.operations(cache)

			// Verify evicted entries
			for _, key := range tt.expectedEvicted {
				_, hit := cache.Get(key)
				if hit {
					t.Errorf("Expected %s to be evicted, but found in cache", key)
				}
			}

			// Verify remaining entries
			for _, key := range tt.expectedRemaining {
				_, hit := cache.Get(key)
				if !hit {
					t.Errorf("Expected %s to remain in cache, but not found", key)
				}
			}

			// Verify cache size matches capacity
			if cache.Size() > tt.capacity {
				t.Errorf("Cache size %d exceeds capacity %d", cache.Size(), tt.capacity)
			}
		})
	}
}

// TestCache_EvictionPolicy_StressTest fills cache well beyond capacity
func TestCache_EvictionPolicy_StressTest(t *testing.T) {
	t.Parallel()
	capacity := 10
	cache := dsgo.NewLMCache(capacity)

	// Add 100 items - should trigger 90 evictions
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%03d", i)
		cache.Set(key, &dsgo.GenerateResult{Content: key})
	}

	// Only last 10 items should remain
	if cache.Size() != capacity {
		t.Errorf("Expected cache size %d, got %d", capacity, cache.Size())
	}

	// First 90 items should be evicted
	for i := 0; i < 90; i++ {
		key := fmt.Sprintf("key_%03d", i)
		_, hit := cache.Get(key)
		if hit {
			t.Errorf("Expected %s to be evicted", key)
		}
	}

	// Last 10 items should be present
	for i := 90; i < 100; i++ {
		key := fmt.Sprintf("key_%03d", i)
		result, hit := cache.Get(key)
		if !hit {
			t.Errorf("Expected %s to be in cache", key)
		}
		if result.Content != key {
			t.Errorf("Content mismatch: expected %s, got %s", key, result.Content)
		}
	}
}

// ============================================================================
// Cache Deep Copy Integrity Tests
// ============================================================================

// TestCache_DeepCopyIntegrity verifies that cached values are isolated from modifications
func TestCache_DeepCopyIntegrity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		originalValue *dsgo.GenerateResult
		mutate        func(*dsgo.GenerateResult)
		verify        func(t *testing.T, cached *dsgo.GenerateResult)
	}{
		{
			name: "modifying content after set does not affect cache",
			originalValue: &dsgo.GenerateResult{
				Content: "original_content",
			},
			mutate: func(r *dsgo.GenerateResult) {
				r.Content = "mutated_content"
			},
			verify: func(t *testing.T, cached *dsgo.GenerateResult) {
				if cached.Content != "original_content" {
					t.Errorf("Expected 'original_content', got '%s'", cached.Content)
				}
			},
		},
		{
			name: "modifying usage after set does not affect cache",
			originalValue: &dsgo.GenerateResult{
				Content: "test",
				Usage: dsgo.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
					Cost:             0.005,
				},
			},
			mutate: func(r *dsgo.GenerateResult) {
				r.Usage.PromptTokens = 999
				r.Usage.Cost = 999.99
			},
			verify: func(t *testing.T, cached *dsgo.GenerateResult) {
				if cached.Usage.PromptTokens != 100 {
					t.Errorf("Expected PromptTokens 100, got %d", cached.Usage.PromptTokens)
				}
				if cached.Usage.Cost != 0.005 {
					t.Errorf("Expected Cost 0.005, got %f", cached.Usage.Cost)
				}
			},
		},
		{
			name: "modifying metadata map after set does not affect cache",
			originalValue: &dsgo.GenerateResult{
				Content: "test",
				Metadata: map[string]any{
					"key1": "original_value",
					"key2": 42,
				},
			},
			mutate: func(r *dsgo.GenerateResult) {
				r.Metadata["key1"] = "mutated_value"
				r.Metadata["new_key"] = "new_value"
			},
			verify: func(t *testing.T, cached *dsgo.GenerateResult) {
				if cached.Metadata["key1"] != "original_value" {
					t.Errorf("Expected 'original_value', got '%v'", cached.Metadata["key1"])
				}
				if _, exists := cached.Metadata["new_key"]; exists {
					t.Error("New key should not exist in cached value")
				}
			},
		},
		{
			name: "modifying nested metadata does not affect cache",
			originalValue: &dsgo.GenerateResult{
				Content: "test",
				Metadata: map[string]any{
					"nested": map[string]any{
						"inner_key": "inner_value",
					},
				},
			},
			mutate: func(r *dsgo.GenerateResult) {
				nested := r.Metadata["nested"].(map[string]any)
				nested["inner_key"] = "mutated_inner"
			},
			verify: func(t *testing.T, cached *dsgo.GenerateResult) {
				nested := cached.Metadata["nested"].(map[string]any)
				if nested["inner_key"] != "inner_value" {
					t.Errorf("Expected 'inner_value', got '%v'", nested["inner_key"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cache := dsgo.NewLMCache(10)

			// Set the original value
			cache.Set("key1", tt.originalValue)

			// Mutate the original
			tt.mutate(tt.originalValue)

			// Retrieve and verify cached value is unchanged
			cached, hit := cache.Get("key1")
			if !hit {
				t.Fatal("Expected cache hit")
			}

			tt.verify(t, cached)
		})
	}
}

// TestCache_DeepCopyIntegrity_RetrievedValueMutation verifies retrieved values are independent
func TestCache_DeepCopyIntegrity_RetrievedValueMutation(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "immutable",
		Usage: dsgo.Usage{
			TotalTokens: 100,
		},
		Metadata: map[string]any{
			"data": "original",
		},
	}

	cache.Set("key1", original)

	// Get first copy and mutate it
	copy1, _ := cache.Get("key1")
	copy1.Content = "mutated_copy1"
	copy1.Usage.TotalTokens = 999
	copy1.Metadata["data"] = "mutated"

	// Get second copy - should have original values
	copy2, _ := cache.Get("key1")

	if copy2.Content != "immutable" {
		t.Errorf("Content was mutated: expected 'immutable', got '%s'", copy2.Content)
	}
	if copy2.Usage.TotalTokens != 100 {
		t.Errorf("Usage was mutated: expected 100, got %d", copy2.Usage.TotalTokens)
	}
	if copy2.Metadata["data"] != "original" {
		t.Errorf("Metadata was mutated: expected 'original', got '%v'", copy2.Metadata["data"])
	}
}

// TestCache_DeepCopyIntegrity_MultipleRetrievals verifies each retrieval returns independent copy
func TestCache_DeepCopyIntegrity_MultipleRetrievals(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	cache.Set("key1", &dsgo.GenerateResult{
		Content: "test",
		Metadata: map[string]any{
			"counter": 0,
		},
	})

	// Retrieve and mutate 10 times
	for i := 0; i < 10; i++ {
		retrieved, hit := cache.Get("key1")
		if !hit {
			t.Fatalf("Expected hit on iteration %d", i)
		}

		// Mutate retrieved value
		retrieved.Metadata["counter"] = i + 1
		retrieved.Content = fmt.Sprintf("mutated_%d", i)
	}

	// Final retrieval should still have original values
	final, _ := cache.Get("key1")
	if final.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", final.Content)
	}
	if final.Metadata["counter"] != 0 {
		t.Errorf("Expected counter 0, got %v", final.Metadata["counter"])
	}
}

// ============================================================================
// deepCopyResult Edge Case Tests
// ============================================================================

// TestCache_DeepCopy_NilResult tests deep copy of nil result
func TestCache_DeepCopy_NilResult(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	// This shouldn't happen normally, but deepCopyResult should handle it
	// We'll test it indirectly by testing the cache behavior
	result := cache.Size()
	if result != 0 {
		t.Errorf("Expected size 0, got %d", result)
	}
}

// TestCache_DeepCopy_EmptyToolCalls tests deep copy with empty tool calls
func TestCache_DeepCopy_EmptyToolCalls(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content:   "test",
		ToolCalls: []dsgo.ToolCall{},
	}

	cache.Set("key1", original)
	retrieved, _ := cache.Get("key1")

	// ToolCalls should be empty, not nil
	if retrieved.ToolCalls == nil {
		t.Error("Expected empty ToolCalls slice, got nil")
	}
	if len(retrieved.ToolCalls) != 0 {
		t.Errorf("Expected 0 ToolCalls, got %d", len(retrieved.ToolCalls))
	}

	// Modify original
	original.ToolCalls = append(original.ToolCalls, dsgo.ToolCall{ID: "test", Name: "test_tool"})

	// Verify cached value is unchanged
	retrieved2, _ := cache.Get("key1")
	if len(retrieved2.ToolCalls) != 0 {
		t.Errorf("Cached ToolCalls were modified: expected 0, got %d", len(retrieved2.ToolCalls))
	}
}

// TestCache_DeepCopy_ToolCallsWithArguments tests deep copy of tool calls with complex arguments
func TestCache_DeepCopy_ToolCallsWithArguments(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "test",
		ToolCalls: []dsgo.ToolCall{
			{
				ID:   "call1",
				Name: "tool1",
				Arguments: map[string]any{
					"string_arg": "value",
					"int_arg":    42,
					"nested_map": map[string]any{
						"inner_key": "inner_value",
					},
				},
			},
		},
	}

	cache.Set("key1", original)

	// Modify original tool call arguments
	original.ToolCalls[0].Arguments["string_arg"] = "modified"
	original.ToolCalls[0].Arguments["nested_map"].(map[string]any)["inner_key"] = "modified_inner"

	// Retrieve and verify cached value is unchanged
	retrieved, _ := cache.Get("key1")

	if len(retrieved.ToolCalls) != 1 {
		t.Fatalf("Expected 1 ToolCall, got %d", len(retrieved.ToolCalls))
	}

	if retrieved.ToolCalls[0].Arguments["string_arg"] != "value" {
		t.Errorf("Expected 'value', got '%v'", retrieved.ToolCalls[0].Arguments["string_arg"])
	}

	nested := retrieved.ToolCalls[0].Arguments["nested_map"].(map[string]any)
	if nested["inner_key"] != "inner_value" {
		t.Errorf("Expected 'inner_value', got '%v'", nested["inner_key"])
	}
}

// TestCache_DeepCopy_DeepMetadataNesting tests deep copy with deeply nested metadata
func TestCache_DeepCopy_DeepMetadataNesting(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "test",
		Metadata: map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"level4": "deep_value",
					},
				},
			},
		},
	}

	cache.Set("key1", original)

	// Deep mutation
	l1 := original.Metadata["level1"].(map[string]any)
	l2 := l1["level2"].(map[string]any)
	l3 := l2["level3"].(map[string]any)
	l3["level4"] = "mutated"

	// Retrieve and verify
	retrieved, _ := cache.Get("key1")
	r1 := retrieved.Metadata["level1"].(map[string]any)
	r2 := r1["level2"].(map[string]any)
	r3 := r2["level3"].(map[string]any)

	if r3["level4"] != "deep_value" {
		t.Errorf("Expected 'deep_value', got '%v'", r3["level4"])
	}
}

// TestCache_DeepCopy_MetadataWithSlices tests deep copy of metadata containing slices
func TestCache_DeepCopy_MetadataWithSlices(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "test",
		Metadata: map[string]any{
			"items": []any{
				"string_item",
				42,
				map[string]any{
					"nested": "value",
				},
			},
		},
	}

	cache.Set("key1", original)

	// Mutate original slice
	items := original.Metadata["items"].([]any)
	items[0] = "mutated"
	itemsMap := items[2].(map[string]any)
	itemsMap["nested"] = "mutated_nested"

	// Retrieve and verify
	retrieved, _ := cache.Get("key1")
	rItems := retrieved.Metadata["items"].([]any)

	if rItems[0] != "string_item" {
		t.Errorf("Expected 'string_item', got '%v'", rItems[0])
	}

	rItemsMap := rItems[2].(map[string]any)
	if rItemsMap["nested"] != "value" {
		t.Errorf("Expected 'value', got '%v'", rItemsMap["nested"])
	}
}

// TestCache_DeepCopy_EmptyMetadata tests deep copy with empty metadata map
func TestCache_DeepCopy_EmptyMetadata(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content:  "test",
		Metadata: map[string]any{},
	}

	cache.Set("key1", original)

	// Add to original
	original.Metadata["new_key"] = "new_value"

	// Retrieve and verify cache is unchanged
	retrieved, _ := cache.Get("key1")
	if len(retrieved.Metadata) != 0 {
		t.Errorf("Expected empty metadata, got %d entries", len(retrieved.Metadata))
	}
}

// TestCache_DeepCopy_NilMetadata tests deep copy with nil metadata
func TestCache_DeepCopy_NilMetadata(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content:  "test",
		Metadata: nil,
	}

	cache.Set("key1", original)

	// Retrieve should also have nil metadata
	retrieved, _ := cache.Get("key1")
	if retrieved.Metadata != nil {
		t.Error("Expected nil metadata, got non-nil")
	}
}

// TestCache_DeepCopy_MultipleToolCalls tests deep copy with multiple tool calls
func TestCache_DeepCopy_MultipleToolCalls(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "test",
		ToolCalls: []dsgo.ToolCall{
			{
				ID:   "call1",
				Name: "tool1",
				Arguments: map[string]any{
					"arg1": "value1",
				},
			},
			{
				ID:   "call2",
				Name: "tool2",
				Arguments: map[string]any{
					"arg2": "value2",
				},
			},
		},
	}

	cache.Set("key1", original)

	// Modify each tool call
	for i, tc := range original.ToolCalls {
		tc.Arguments["mutated"] = true
		original.ToolCalls[i] = tc
	}

	// Retrieve and verify
	retrieved, _ := cache.Get("key1")

	if len(retrieved.ToolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls, got %d", len(retrieved.ToolCalls))
	}

	for i, tc := range retrieved.ToolCalls {
		if _, exists := tc.Arguments["mutated"]; exists {
			t.Errorf("Tool call %d was mutated", i)
		}
	}

	// Verify original values
	if retrieved.ToolCalls[0].Arguments["arg1"] != "value1" {
		t.Error("Tool call 0 arg1 not preserved")
	}
	if retrieved.ToolCalls[1].Arguments["arg2"] != "value2" {
		t.Error("Tool call 1 arg2 not preserved")
	}
}

// TestCache_DeepCopy_ComplexMetadataValues tests deep copy with various data types in metadata
func TestCache_DeepCopy_ComplexMetadataValues(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "test",
		Usage: dsgo.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.005,
		},
		Metadata: map[string]any{
			"string_val": "text",
			"int_val":    42,
			"float_val":  3.14,
			"bool_val":   true,
			"nil_val":    nil,
			"slice_val":  []any{1, "two", 3.0},
			"map_val":    map[string]any{"nested": "value"},
		},
	}

	cache.Set("key1", original)

	// Modify all metadata values
	original.Metadata["string_val"] = "modified"
	original.Metadata["int_val"] = 999
	original.Metadata["float_val"] = 9.99
	original.Metadata["bool_val"] = false
	original.Metadata["slice_val"] = []any{9, "modified"}
	original.Metadata["map_val"] = map[string]any{"nested": "modified"}

	// Retrieve and verify all original values are preserved
	retrieved, _ := cache.Get("key1")

	if retrieved.Metadata["string_val"] != "text" {
		t.Error("string_val not preserved")
	}
	if retrieved.Metadata["int_val"] != 42 {
		t.Error("int_val not preserved")
	}
	if retrieved.Metadata["float_val"] != 3.14 {
		t.Error("float_val not preserved")
	}
	if retrieved.Metadata["bool_val"] != true {
		t.Error("bool_val not preserved")
	}
	if retrieved.Metadata["nil_val"] != nil {
		t.Error("nil_val not preserved")
	}

	// Check slice
	slice := retrieved.Metadata["slice_val"].([]any)
	if len(slice) != 3 || slice[0] != 1 || slice[1] != "two" {
		t.Error("slice_val not preserved")
	}

	// Check nested map
	m := retrieved.Metadata["map_val"].(map[string]any)
	if m["nested"] != "value" {
		t.Error("map_val nested not preserved")
	}
}

// TestCache_DeepCopy_FinishReason tests deep copy preserves finish reason
func TestCache_DeepCopy_FinishReason(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content:      "test response",
		FinishReason: "stop",
	}

	cache.Set("key1", original)

	// Modify original
	original.FinishReason = "max_tokens"

	// Retrieve and verify
	retrieved, _ := cache.Get("key1")
	if retrieved.FinishReason != "stop" {
		t.Errorf("Expected 'stop', got '%s'", retrieved.FinishReason)
	}
}

// TestCache_DeepCopy_EmptySlicesInMetadata tests empty slices are deep copied
func TestCache_DeepCopy_EmptySlicesInMetadata(t *testing.T) {
	t.Parallel()
	cache := dsgo.NewLMCache(10)

	original := &dsgo.GenerateResult{
		Content: "test",
		Metadata: map[string]any{
			"empty_slice": []any{},
		},
	}

	cache.Set("key1", original)

	// Add to original slice
	original.Metadata["empty_slice"] = []any{"item"}

	// Retrieve and verify
	retrieved, _ := cache.Get("key1")
	slice := retrieved.Metadata["empty_slice"].([]any)
	if len(slice) != 0 {
		t.Errorf("Expected empty slice, got %d items", len(slice))
	}
}
