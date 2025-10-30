package dsgo

import (
	"container/list"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Cache interface for LM result caching
type Cache interface {
	// Get retrieves a cached result by key
	Get(key string) (*GenerateResult, bool)

	// Set stores a result in the cache
	Set(key string, result *GenerateResult)

	// Clear removes all entries from the cache
	Clear()

	// Size returns the current number of cached entries
	Size() int

	// Stats returns cache hit/miss statistics
	Stats() CacheStats
}

// CacheStats holds cache performance metrics
type CacheStats struct {
	Hits   int64
	Misses int64
	Size   int
}

// HitRate returns the cache hit rate as a percentage (0-100)
func (s CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0.0
	}
	return float64(s.Hits) / float64(total) * 100.0
}

// LMCache is a thread-safe LRU cache for LM results
type LMCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	lru      *list.List
	hits     int64
	misses   int64
}

// cacheEntry represents a cached item
type cacheEntry struct {
	key    string
	result *GenerateResult
}

// NewLMCache creates a new LRU cache with the specified capacity
// Default capacity is 1000 entries
func NewLMCache(capacity int) *LMCache {
	if capacity <= 0 {
		capacity = 1000 // Default capacity
	}
	return &LMCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Get retrieves a cached result by key
func (c *LMCache) Get(key string) (*GenerateResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		// Move to front (most recently used)
		c.lru.MoveToFront(elem)
		c.hits++
		entry := elem.Value.(*cacheEntry)
		return entry.result, true
	}

	c.misses++
	return nil, false
}

// Set stores a result in the cache
func (c *LMCache) Set(key string, result *GenerateResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Update existing entry and move to front
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.result = result
		return
	}

	// Add new entry
	entry := &cacheEntry{key: key, result: result}
	elem := c.lru.PushFront(entry)
	c.items[key] = elem

	// Evict oldest entry if capacity exceeded
	if c.lru.Len() > c.capacity {
		oldest := c.lru.Back()
		if oldest != nil {
			c.lru.Remove(oldest)
			oldEntry := oldest.Value.(*cacheEntry)
			delete(c.items, oldEntry.key)
		}
	}
}

// Clear removes all entries from the cache
func (c *LMCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lru = list.New()
	c.hits = 0
	c.misses = 0
}

// Size returns the current number of cached entries
func (c *LMCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// Stats returns cache hit/miss statistics
func (c *LMCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Hits:   c.hits,
		Misses: c.misses,
		Size:   c.lru.Len(),
	}
}

// GenerateCacheKey creates a deterministic cache key from LM request parameters
// The key is based on: LM name, messages, temperature, max tokens, top_p, and response format
func GenerateCacheKey(lmName string, messages []Message, options *GenerateOptions) string {
	// Build a deterministic representation
	keyData := struct {
		LMName         string
		Messages       []Message
		Temperature    float64
		MaxTokens      int
		TopP           float64
		ResponseFormat string
		Stop           []string
	}{
		LMName:         lmName,
		Messages:       messages,
		Temperature:    options.Temperature,
		MaxTokens:      options.MaxTokens,
		TopP:           options.TopP,
		ResponseFormat: options.ResponseFormat,
		Stop:           options.Stop,
	}

	// Sort stop sequences for determinism
	if keyData.Stop != nil {
		stopCopy := make([]string, len(keyData.Stop))
		copy(stopCopy, keyData.Stop)
		sort.Strings(stopCopy)
		keyData.Stop = stopCopy
	}

	// Serialize to JSON
	data, err := json.Marshal(keyData)
	if err != nil {
		// Fallback to simple key if marshaling fails
		return fmt.Sprintf("%s:%d", lmName, len(messages))
	}

	// Hash the JSON to create a compact key
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
