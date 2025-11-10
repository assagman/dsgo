package core

import (
	"container/list"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
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

	// Capacity returns the maximum number of entries the cache can hold
	Capacity() int

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
	ttl      time.Duration
	items    map[string]*list.Element
	lru      *list.List
	hits     int64
	misses   int64
}

// cacheEntry represents a cached item
type cacheEntry struct {
	key     string
	result  *GenerateResult
	expires time.Time
}

// NewLMCache creates a new LRU cache with the specified capacity
// Default capacity is 1000 entries
// Default TTL is 0 (no expiration)
func NewLMCache(capacity int) *LMCache {
	return NewLMCacheWithTTL(capacity, 0)
}

// NewLMCacheWithTTL creates a new LRU cache with capacity and TTL
// TTL of 0 means no expiration
func NewLMCacheWithTTL(capacity int, ttl time.Duration) *LMCache {
	if capacity <= 0 {
		capacity = 1000 // Default capacity
	}
	return &LMCache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Get retrieves a cached result by key
// Returns a deep copy to prevent mutation of cached data
func (c *LMCache) Get(key string) (*GenerateResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)

		// Check TTL expiration
		if c.ttl > 0 && time.Now().After(entry.expires) {
			// Entry expired, remove it
			c.lru.Remove(elem)
			delete(c.items, key)
			c.misses++
			return nil, false
		}

		// Move to front (most recently used)
		c.lru.MoveToFront(elem)
		c.hits++
		// Return a deep copy to prevent external mutation
		return deepCopyResult(entry.result), true
	}

	c.misses++
	return nil, false
}

// Set stores a result in the cache
// Stores a deep copy to prevent external mutation of cached data
func (c *LMCache) Set(key string, result *GenerateResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Deep copy to prevent external mutation
	resultCopy := deepCopyResult(result)

	// Calculate expiration time
	var expires time.Time
	if c.ttl > 0 {
		expires = time.Now().Add(c.ttl)
	}

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Update existing entry and move to front
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.result = resultCopy
		entry.expires = expires
		return
	}

	// Add new entry
	entry := &cacheEntry{
		key:     key,
		result:  resultCopy,
		expires: expires,
	}
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

// Capacity returns the maximum number of entries the cache can hold
func (c *LMCache) Capacity() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capacity
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
//
// Cache key components (all affect cache key generation):
//   - LM name (model identifier)
//   - Messages (conversation history)
//   - Temperature, MaxTokens, TopP (generation parameters)
//   - ResponseFormat, ResponseSchema (output format)
//   - Stop sequences (canonicalized/sorted)
//   - Tools and ToolChoice (function calling)
//   - FrequencyPenalty, PresencePenalty (repetition controls)
//
// Maps (ResponseSchema, Tool.Parameters) are canonicalized to ensure
// deterministic key generation regardless of insertion order.
func GenerateCacheKey(lmName string, messages []Message, options *GenerateOptions) string {
	// Build a deterministic representation
	keyData := struct {
		LMName           string
		Messages         []Message
		Temperature      float64
		MaxTokens        int
		TopP             float64
		ResponseFormat   string
		ResponseSchema   string // Canonicalized JSON
		Stop             []string
		Tools            []canonicalTool
		ToolChoice       string
		FrequencyPenalty float64
		PresencePenalty  float64
	}{
		LMName:           lmName,
		Messages:         messages,
		Temperature:      options.Temperature,
		MaxTokens:        options.MaxTokens,
		TopP:             options.TopP,
		ResponseFormat:   options.ResponseFormat,
		ToolChoice:       options.ToolChoice,
		FrequencyPenalty: options.FrequencyPenalty,
		PresencePenalty:  options.PresencePenalty,
	}

	// Sort stop sequences for determinism
	if options.Stop != nil {
		stopCopy := make([]string, len(options.Stop))
		copy(stopCopy, options.Stop)
		sort.Strings(stopCopy)
		keyData.Stop = stopCopy
	}

	// Canonicalize ResponseSchema map
	if options.ResponseSchema != nil {
		canonical, err := canonicalizeMap(options.ResponseSchema)
		if err == nil {
			keyData.ResponseSchema = canonical
		}
	}

	// Canonicalize Tools
	if len(options.Tools) > 0 {
		keyData.Tools = make([]canonicalTool, len(options.Tools))
		for i, tool := range options.Tools {
			keyData.Tools[i] = canonicalTool{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters, // ToolParameter is deterministic already
			}
		}
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

// canonicalTool is a deterministic representation of Tool for cache keys
type canonicalTool struct {
	Name        string
	Description string
	Parameters  []ToolParameter // Tool parameters (already deterministic)
}

// canonicalizeMap converts a map to a deterministic JSON string
// by sorting keys and recursively canonicalizing nested maps
func canonicalizeMap(m map[string]any) (string, error) {
	if m == nil {
		return "", nil
	}

	// Sort keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build canonical map with sorted keys
	canonical := make(map[string]any, len(m))
	for _, k := range keys {
		v := m[k]
		// Recursively canonicalize nested maps
		if nestedMap, ok := v.(map[string]any); ok {
			nestedCanonical, err := canonicalizeMap(nestedMap)
			if err != nil {
				return "", err
			}
			canonical[k] = nestedCanonical
		} else {
			canonical[k] = v
		}
	}

	// Marshal to JSON with sorted keys
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// deepCopyResult creates a deep copy of GenerateResult to prevent mutation
func deepCopyResult(r *GenerateResult) *GenerateResult {
	if r == nil {
		return nil
	}

	result := &GenerateResult{
		Content:      r.Content,
		FinishReason: r.FinishReason,
		Usage:        r.Usage, // Usage is a value type, automatically copied
	}

	// Deep copy ToolCalls slice
	if r.ToolCalls != nil {
		result.ToolCalls = make([]ToolCall, len(r.ToolCalls))
		for i, tc := range r.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Name: tc.Name,
			}
			// Deep copy Arguments map
			if tc.Arguments != nil {
				result.ToolCalls[i].Arguments = deepCopyMap(tc.Arguments)
			}
		}
	}

	// Deep copy Metadata map
	if r.Metadata != nil {
		result.Metadata = deepCopyMap(r.Metadata)
	}

	return result
}

// deepCopyMap creates a deep copy of a map[string]any
func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			result[k] = deepCopyMap(val)
		case []any:
			result[k] = deepCopySlice(val)
		default:
			result[k] = val
		}
	}
	return result
}

// deepCopySlice creates a deep copy of a []any slice
func deepCopySlice(s []any) []any {
	if s == nil {
		return nil
	}

	result := make([]any, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]any:
			result[i] = deepCopyMap(val)
		case []any:
			result[i] = deepCopySlice(val)
		default:
			result[i] = val
		}
	}
	return result
}
