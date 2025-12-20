package core

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/assagman/dsgo/logging"
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

// LazyCache defers initializing the underlying cache until first use.
// This avoids creating disk cache directories/files until an actual LM call happens.
//
// If initialization fails, it permanently falls back to a no-op cache.
type LazyCache struct {
	initOnce sync.Once
	initFn   func() Cache
	cache    Cache
}

func NewLazyCache(initFn func() Cache) *LazyCache {
	if initFn == nil {
		initFn = func() Cache { return nil }
	}
	return &LazyCache{initFn: initFn}
}

func (c *LazyCache) init() {
	if c == nil {
		return
	}
	c.initOnce.Do(func() {
		c.cache = c.initFn()
	})
}

func (c *LazyCache) Get(key string) (*GenerateResult, bool) {
	c.init()
	if c.cache == nil {
		return nil, false
	}
	return c.cache.Get(key)
}

func (c *LazyCache) Set(key string, result *GenerateResult) {
	c.init()
	if c.cache == nil {
		return
	}
	c.cache.Set(key, result)
}

func (c *LazyCache) Clear() {
	c.init()
	if c.cache == nil {
		return
	}
	c.cache.Clear()
}

func (c *LazyCache) Size() int {
	c.init()
	if c.cache == nil {
		return 0
	}
	return c.cache.Size()
}

func (c *LazyCache) Capacity() int {
	c.init()
	if c.cache == nil {
		return 0
	}
	return c.cache.Capacity()
}

func (c *LazyCache) Stats() CacheStats {
	c.init()
	if c.cache == nil {
		return CacheStats{}
	}
	return c.cache.Stats()
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

const cacheSlowLogThreshold = 100 * time.Millisecond

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

// MarkCacheHit marks a result as served from cache and clears usage stats (following DSPy pattern).
// This should be called on results returned from cache to accurately reflect that no API call was made.
func MarkCacheHit(result *GenerateResult) *GenerateResult {
	if result == nil {
		return nil
	}
	result.CacheHit = true
	result.Usage = Usage{} // Clear usage since no API call was made
	return result
}

// Get retrieves a cached result by key
// Returns a deep copy to prevent mutation of cached data
func (c *LMCache) Get(key string) (*GenerateResult, bool) {
	start := time.Now()
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
		duration := time.Since(start)
		fields := map[string]any{
			"module":      "cache.LRUCache",
			"key":         key,
			"size":        c.lru.Len(),
			"capacity":    c.capacity,
			"duration_ms": duration.Milliseconds(),
		}
		if duration > cacheSlowLogThreshold {
			logging.GetLogger().Info(context.Background(), "Cache hit", fields)
		} else {
			logging.GetLogger().Debug(context.Background(), "Cache hit", fields)
		}
		// Return a deep copy to prevent external mutation
		return deepCopyResult(entry.result), true
	}

	c.misses++
	duration := time.Since(start)
	fields := map[string]any{
		"module":      "cache.LRUCache",
		"key":         key,
		"size":        c.lru.Len(),
		"capacity":    c.capacity,
		"duration_ms": duration.Milliseconds(),
	}
	if duration > cacheSlowLogThreshold {
		logging.GetLogger().Info(context.Background(), "Cache miss", fields)
	} else {
		logging.GetLogger().Debug(context.Background(), "Cache miss", fields)
	}
	return nil, false
}

// Set stores a result in the cache
// Stores a deep copy to prevent external mutation of cached data
func (c *LMCache) Set(key string, result *GenerateResult) {
	start := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	// Deep copy to prevent external mutation
	resultCopy := deepCopyResult(result)

	// Calculate expiration time
	var expires time.Time
	if c.ttl > 0 {
		expires = time.Now().Add(c.ttl)
	}

	if elem, ok := c.items[key]; ok {
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.result = resultCopy
		entry.expires = expires
		return
	}

	entry := &cacheEntry{
		key:     key,
		result:  resultCopy,
		expires: expires,
	}
	elem := c.lru.PushFront(entry)
	c.items[key] = elem
	duration := time.Since(start)
	fields := map[string]any{
		"module":      "cache.LRUCache",
		"key":         key,
		"size":        c.lru.Len(),
		"capacity":    c.capacity,
		"duration_ms": duration.Milliseconds(),
	}
	if duration > cacheSlowLogThreshold {
		logging.GetLogger().Info(context.Background(), "Cache set", fields)
	} else {
		logging.GetLogger().Debug(context.Background(), "Cache set", fields)
	}

	// Evict oldest entry if capacity exceeded
	if c.lru.Len() > c.capacity {
		oldest := c.lru.Back()
		if oldest != nil {
			c.lru.Remove(oldest)
			oldEntry := oldest.Value.(*cacheEntry)
			delete(c.items, oldEntry.key)
			evictDuration := time.Since(start)
			evictFields := map[string]any{
				"module":      "cache.LRUCache",
				"key":         oldEntry.key,
				"size":        c.lru.Len(),
				"capacity":    c.capacity,
				"duration_ms": evictDuration.Milliseconds(),
			}
			if evictDuration > cacheSlowLogThreshold {
				logging.GetLogger().Info(context.Background(), "Cache eviction", evictFields)
			} else {
				logging.GetLogger().Debug(context.Background(), "Cache eviction", evictFields)
			}
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

// DefaultIgnoredCacheKeyArgs are fields that are ignored when generating cache keys.
// These match DSPy's default ignored fields for cache key generation.
var DefaultIgnoredCacheKeyArgs = []string{
	"api_key",
	"api_base",
	"base_url",
	"apiKey",
	"apiBase",
	"baseUrl",
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
//   - ProviderParams (provider-specific parameters, excluding ignored fields)
//
// Maps (ResponseSchema, Tool.Parameters, ProviderParams) are canonicalized to ensure
// deterministic key generation regardless of insertion order.
//
// By default, sensitive fields like api_key, api_base, base_url are ignored
// (following DSPy's pattern) to ensure cache hits across different API configurations.
func GenerateCacheKey(lmName string, messages []Message, options *GenerateOptions) string {
	return GenerateCacheKeyWithIgnored(lmName, messages, options, DefaultIgnoredCacheKeyArgs)
}

// GenerateCacheKeyWithIgnored creates a cache key while ignoring specified fields.
// This allows customization of which fields are excluded from the cache key.
func GenerateCacheKeyWithIgnored(lmName string, messages []Message, options *GenerateOptions, ignoredArgs []string) string {
	if options == nil {
		options = DefaultGenerateOptions()
	}

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
		ProviderParams   string // Canonicalized JSON
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

	// Canonicalize ProviderParams map (excluding ignored args)
	if options.ProviderParams != nil {
		filteredParams := filterIgnoredArgs(options.ProviderParams, ignoredArgs)
		if len(filteredParams) > 0 {
			canonical, err := canonicalizeMap(filteredParams)
			if err == nil {
				keyData.ProviderParams = canonical
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

// filterIgnoredArgs removes ignored keys from a map
func filterIgnoredArgs(m map[string]any, ignoredArgs []string) map[string]any {
	if m == nil || len(ignoredArgs) == 0 {
		return m
	}

	ignored := make(map[string]bool, len(ignoredArgs))
	for _, arg := range ignoredArgs {
		ignored[arg] = true
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		if !ignored[k] {
			result[k] = v
		}
	}
	return result
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
		CacheHit:     r.CacheHit,
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
				result.ToolCalls[i].Arguments = DeepCopyMap(tc.Arguments)
			}
		}
	}

	// Deep copy Metadata map
	if r.Metadata != nil {
		result.Metadata = DeepCopyMap(r.Metadata)
	}

	return result
}

// DeepCopyMap creates a deep copy of a map[string]any
func DeepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			result[k] = DeepCopyMap(val)
		case []any:
			result[k] = DeepCopySlice(val)
		default:
			result[k] = val
		}
	}
	return result
}

// DeepCopySlice creates a deep copy of a []any slice
func DeepCopySlice(s []any) []any {
	if s == nil {
		return nil
	}

	result := make([]any, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]any:
			result[i] = DeepCopyMap(val)
		case []any:
			result[i] = DeepCopySlice(val)
		default:
			result[i] = val
		}
	}
	return result
}
