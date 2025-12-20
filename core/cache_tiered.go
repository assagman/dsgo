package core

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/assagman/dsgo/logging"
)

// TieredCache implements a two-tier cache strategy following DSPy's pattern:
// - L1: In-memory LRU cache (fast, volatile)
// - L2: Disk-based cache (persistent, slower)
//
// On Get:
//  1. Check L1 (memory) - if hit, return immediately
//  2. Check L2 (disk) - if hit, promote to L1 and return
//
// On Set:
//  1. Write to L1 (memory)
//  2. Write to L2 (disk) - failures are logged but not fatal
type TieredCache struct {
	memory      *LMCache
	disk        *DiskCache
	enableMem   bool
	enableDisk  bool
	memoryHits  int64
	diskHits    int64
	totalMisses int64
}

// TieredCacheOptions configures the two-tier cache
type TieredCacheOptions struct {
	// Memory cache options
	EnableMemory   bool
	MemoryCapacity int
	MemoryTTL      time.Duration

	// Disk cache options
	EnableDisk    bool
	DiskDir       string
	DiskSizeLimit int64
	DiskShards    int
}

// DefaultTieredCacheOptions returns sensible defaults following DSPy patterns
func DefaultTieredCacheOptions() *TieredCacheOptions {
	return &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 1000000, // DSPy default: 1M entries
		MemoryTTL:      0,       // No TTL by default (LRU eviction only)

		EnableDisk:    true,
		DiskDir:       "", // Will use ~/.dsgo_cache
		DiskSizeLimit: DefaultDiskCacheSizeLimit,
		DiskShards:    DefaultDiskCacheShards,
	}
}

// NewTieredCache creates a new two-tier cache with the given options.
// If disk cache initialization fails, falls back to memory-only mode gracefully.
func NewTieredCache(opts *TieredCacheOptions) (*TieredCache, error) {
	if opts == nil {
		opts = DefaultTieredCacheOptions()
	}

	cache := &TieredCache{
		enableMem:  opts.EnableMemory,
		enableDisk: opts.EnableDisk,
	}

	// Initialize memory cache
	if opts.EnableMemory {
		cache.memory = NewLMCacheWithTTL(opts.MemoryCapacity, opts.MemoryTTL)
	}

	// Initialize disk cache with graceful fallback
	if opts.EnableDisk {
		diskCache, err := NewDiskCacheWithShards(opts.DiskDir, opts.DiskSizeLimit, opts.DiskShards)
		if err != nil {
			logging.GetLogger().Warn(context.Background(), "Failed to initialize disk cache, falling back to memory-only", map[string]any{
				"module": "cache.TieredCache",
				"error":  err.Error(),
			})
			cache.enableDisk = false
		} else {
			cache.disk = diskCache
		}
	}

	// Ensure at least one cache is enabled
	if !cache.enableMem && !cache.enableDisk {
		logging.GetLogger().Warn(context.Background(), "Both memory and disk cache disabled, enabling memory cache with defaults", map[string]any{
			"module": "cache.TieredCache",
		})
		cache.enableMem = true
		cache.memory = NewLMCacheWithTTL(DefaultTieredCacheOptions().MemoryCapacity, 0)
	}

	return cache, nil
}

// Get retrieves a cached result by key.
// Checks memory first, then disk. Promotes disk hits to memory.
func (c *TieredCache) Get(key string) (*GenerateResult, bool) {
	// L1: Memory cache lookup
	if c.enableMem && c.memory != nil {
		if result, ok := c.memory.Get(key); ok {
			atomic.AddInt64(&c.memoryHits, 1)
			return result, true
		}
	}

	// L2: Disk cache lookup
	if c.enableDisk && c.disk != nil {
		if result, ok := c.disk.Get(key); ok {
			atomic.AddInt64(&c.diskHits, 1)

			// Promote to memory cache for future fast access
			if c.enableMem && c.memory != nil {
				c.memory.Set(key, result)
			}

			return result, true
		}
	}

	atomic.AddInt64(&c.totalMisses, 1)
	return nil, false
}

// Set stores a result in both cache tiers.
// Disk write failures are logged but do not prevent memory caching.
func (c *TieredCache) Set(key string, result *GenerateResult) {
	// L1: Memory cache write
	if c.enableMem && c.memory != nil {
		c.memory.Set(key, result)
	}

	// L2: Disk cache write (non-blocking for performance)
	if c.enableDisk && c.disk != nil {
		c.disk.Set(key, result)
	}
}

// Clear removes all entries from both cache tiers
func (c *TieredCache) Clear() {
	if c.enableMem && c.memory != nil {
		c.memory.Clear()
	}
	if c.enableDisk && c.disk != nil {
		c.disk.Clear()
	}
	atomic.StoreInt64(&c.memoryHits, 0)
	atomic.StoreInt64(&c.diskHits, 0)
	atomic.StoreInt64(&c.totalMisses, 0)
}

// Size returns the total number of entries across both tiers
func (c *TieredCache) Size() int {
	var total int
	if c.enableMem && c.memory != nil {
		total += c.memory.Size()
	}
	// Note: Disk entries may overlap with memory, so this is an approximation
	if c.enableDisk && c.disk != nil {
		total += c.disk.Size()
	}
	return total
}

// Capacity returns the memory cache capacity (disk has size-based limit)
func (c *TieredCache) Capacity() int {
	if c.enableMem && c.memory != nil {
		return c.memory.Capacity()
	}
	return -1
}

// Stats returns cache hit/miss statistics
func (c *TieredCache) Stats() CacheStats {
	memHits := atomic.LoadInt64(&c.memoryHits)
	diskHits := atomic.LoadInt64(&c.diskHits)
	misses := atomic.LoadInt64(&c.totalMisses)

	return CacheStats{
		Hits:   memHits + diskHits,
		Misses: misses,
		Size:   c.Size(),
	}
}

// TieredStats returns detailed statistics for each tier
func (c *TieredCache) TieredStats() TieredCacheStats {
	stats := TieredCacheStats{
		MemoryEnabled: c.enableMem,
		DiskEnabled:   c.enableDisk,
		MemoryHits:    atomic.LoadInt64(&c.memoryHits),
		DiskHits:      atomic.LoadInt64(&c.diskHits),
		TotalMisses:   atomic.LoadInt64(&c.totalMisses),
	}

	if c.enableMem && c.memory != nil {
		stats.MemorySize = c.memory.Size()
		stats.MemoryCapacity = c.memory.Capacity()
	}

	if c.enableDisk && c.disk != nil {
		stats.DiskSize = c.disk.Size()
		stats.DiskBytes = c.disk.SizeBytes()
		stats.DiskDir = c.disk.Dir()
	}

	return stats
}

// TieredCacheStats provides detailed statistics for each cache tier
type TieredCacheStats struct {
	// Tier enablement
	MemoryEnabled bool
	DiskEnabled   bool

	// Hit counters
	MemoryHits  int64
	DiskHits    int64
	TotalMisses int64

	// Memory stats
	MemorySize     int
	MemoryCapacity int

	// Disk stats
	DiskSize  int
	DiskBytes int64
	DiskDir   string
}

// HitRate returns the overall cache hit rate as a percentage (0-100)
func (s TieredCacheStats) HitRate() float64 {
	total := s.MemoryHits + s.DiskHits + s.TotalMisses
	if total == 0 {
		return 0.0
	}
	return float64(s.MemoryHits+s.DiskHits) / float64(total) * 100.0
}

// MemoryHitRate returns the memory cache hit rate as a percentage (0-100)
func (s TieredCacheStats) MemoryHitRate() float64 {
	total := s.MemoryHits + s.DiskHits + s.TotalMisses
	if total == 0 {
		return 0.0
	}
	return float64(s.MemoryHits) / float64(total) * 100.0
}

// DiskHitRate returns the disk cache hit rate as a percentage (0-100)
func (s TieredCacheStats) DiskHitRate() float64 {
	total := s.MemoryHits + s.DiskHits + s.TotalMisses
	if total == 0 {
		return 0.0
	}
	return float64(s.DiskHits) / float64(total) * 100.0
}

// MemoryCache returns the underlying memory cache (for advanced use)
func (c *TieredCache) MemoryCache() *LMCache {
	return c.memory
}

// DiskCache returns the underlying disk cache (for advanced use)
func (c *TieredCache) DiskCache() *DiskCache {
	return c.disk
}

// ClearMemory clears only the memory tier
func (c *TieredCache) ClearMemory() {
	if c.enableMem && c.memory != nil {
		c.memory.Clear()
	}
}

// ClearDisk clears only the disk tier
func (c *TieredCache) ClearDisk() {
	if c.enableDisk && c.disk != nil {
		c.disk.Clear()
	}
}
