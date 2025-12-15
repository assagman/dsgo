package core

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/assagman/dsgo/internal/logging"
)

const (
	// DefaultDiskCacheDir is the default directory for disk cache
	DefaultDiskCacheDir = ".dsgo_cache"

	// DefaultDiskCacheSizeLimit is the default size limit for disk cache (30GB like DSPy)
	DefaultDiskCacheSizeLimit = 30 * 1024 * 1024 * 1024

	// DefaultDiskCacheShards is the number of shards for concurrent access (like DSPy's FanoutCache)
	DefaultDiskCacheShards = 16

	// diskCacheFileExtension is the file extension for cache entries
	diskCacheFileExtension = ".gob"
)

// DiskCache is a file-based persistent cache with sharding for concurrent access.
// It follows DSPy's FanoutCache pattern with 16 shards by default.
type DiskCache struct {
	baseDir   string
	sizeLimit int64
	shards    int
	shardMu   []sync.RWMutex // Per-shard locks for concurrent access
	hits      int64
	misses    int64
}

// diskCacheEntry represents a cached item stored on disk
type diskCacheEntry struct {
	Result    *GenerateResult
	CreatedAt time.Time
}

func init() {
	// Register types for gob encoding
	gob.Register(&GenerateResult{})
	gob.Register(&diskCacheEntry{})
	gob.Register(map[string]any{})
	gob.Register([]any{})
}

// NewDiskCache creates a new disk-based cache.
// If baseDir is empty, uses ~/.dsgo_cache as default.
// If sizeLimit is 0, uses 30GB as default (matching DSPy).
func NewDiskCache(baseDir string, sizeLimit int64) (*DiskCache, error) {
	return NewDiskCacheWithShards(baseDir, sizeLimit, DefaultDiskCacheShards)
}

// NewDiskCacheWithShards creates a new disk-based cache with custom shard count.
func NewDiskCacheWithShards(baseDir string, sizeLimit int64, shards int) (*DiskCache, error) {
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		// Generate unique project-specific cache directory
		// Uses hash of working directory to isolate caches between projects
		projectID := generateProjectCacheID()
		baseDir = filepath.Join(homeDir, DefaultDiskCacheDir, projectID)
	}

	if sizeLimit <= 0 {
		sizeLimit = DefaultDiskCacheSizeLimit
	}

	if shards <= 0 {
		shards = DefaultDiskCacheShards
	}

	// Create base directory and shard directories
	for i := 0; i < shards; i++ {
		shardDir := filepath.Join(baseDir, fmt.Sprintf("shard_%02d", i))
		if err := os.MkdirAll(shardDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create shard directory %s: %w", shardDir, err)
		}
	}

	cache := &DiskCache{
		baseDir:   baseDir,
		sizeLimit: sizeLimit,
		shards:    shards,
		shardMu:   make([]sync.RWMutex, shards),
	}

	return cache, nil
}

// Get retrieves a cached result by key from disk
func (c *DiskCache) Get(key string) (*GenerateResult, bool) {
	start := time.Now()
	shardIdx := c.shardIndex(key)

	c.shardMu[shardIdx].RLock()
	defer c.shardMu[shardIdx].RUnlock()

	filePath := c.keyToPath(key)
	file, err := os.Open(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.GetLogger().Debug(context.Background(), "Disk cache read error", map[string]any{
				"module": "cache.DiskCache",
				"key":    key,
				"error":  err.Error(),
			})
		}
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}
	defer func() { _ = file.Close() }()

	var entry diskCacheEntry
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&entry); err != nil {
		logging.GetLogger().Debug(context.Background(), "Disk cache decode error", map[string]any{
			"module": "cache.DiskCache",
			"key":    key,
			"error":  err.Error(),
		})
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	atomic.AddInt64(&c.hits, 1)
	duration := time.Since(start)
	logging.GetLogger().Debug(context.Background(), "Disk cache hit", map[string]any{
		"module":      "cache.DiskCache",
		"key":         key,
		"duration_ms": duration.Milliseconds(),
	})

	return deepCopyResult(entry.Result), true
}

// Set stores a result in the disk cache
func (c *DiskCache) Set(key string, result *GenerateResult) {
	start := time.Now()
	shardIdx := c.shardIndex(key)

	c.shardMu[shardIdx].Lock()
	defer c.shardMu[shardIdx].Unlock()

	filePath := c.keyToPath(key)
	entry := diskCacheEntry{
		Result:    deepCopyResult(result),
		CreatedAt: time.Now(),
	}

	// Write to temporary file first, then rename for atomicity
	tmpPath := filePath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		logging.GetLogger().Debug(context.Background(), "Disk cache write error", map[string]any{
			"module": "cache.DiskCache",
			"key":    key,
			"error":  err.Error(),
		})
		return
	}

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(&entry); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		logging.GetLogger().Debug(context.Background(), "Disk cache encode error", map[string]any{
			"module": "cache.DiskCache",
			"key":    key,
			"error":  err.Error(),
		})
		return
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		logging.GetLogger().Debug(context.Background(), "Disk cache close error", map[string]any{
			"module": "cache.DiskCache",
			"key":    key,
			"error":  err.Error(),
		})
		return
	}

	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		logging.GetLogger().Debug(context.Background(), "Disk cache rename error", map[string]any{
			"module": "cache.DiskCache",
			"key":    key,
			"error":  err.Error(),
		})
		return
	}

	duration := time.Since(start)
	logging.GetLogger().Debug(context.Background(), "Disk cache set", map[string]any{
		"module":      "cache.DiskCache",
		"key":         key,
		"duration_ms": duration.Milliseconds(),
	})
}

// Clear removes all entries from the disk cache
func (c *DiskCache) Clear() {
	for i := 0; i < c.shards; i++ {
		c.shardMu[i].Lock()
	}
	defer func() {
		for i := c.shards - 1; i >= 0; i-- {
			c.shardMu[i].Unlock()
		}
	}()

	for i := 0; i < c.shards; i++ {
		shardDir := filepath.Join(c.baseDir, fmt.Sprintf("shard_%02d", i))
		entries, err := os.ReadDir(shardDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == diskCacheFileExtension {
				_ = os.Remove(filepath.Join(shardDir, entry.Name()))
			}
		}
	}

	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}

// Size returns the current number of cached entries on disk
func (c *DiskCache) Size() int {
	total := 0
	for i := 0; i < c.shards; i++ {
		c.shardMu[i].RLock()
		shardDir := filepath.Join(c.baseDir, fmt.Sprintf("shard_%02d", i))
		entries, err := os.ReadDir(shardDir)
		c.shardMu[i].RUnlock()
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == diskCacheFileExtension {
				total++
			}
		}
	}
	return total
}

// Capacity returns the size limit (not applicable for disk cache in same way as memory)
// Returns -1 to indicate disk cache uses size-based limits, not entry count
func (c *DiskCache) Capacity() int {
	return -1
}

// Stats returns cache hit/miss statistics
func (c *DiskCache) Stats() CacheStats {
	return CacheStats{
		Hits:   atomic.LoadInt64(&c.hits),
		Misses: atomic.LoadInt64(&c.misses),
		Size:   c.Size(),
	}
}

// SizeBytes returns the total size of the cache in bytes
func (c *DiskCache) SizeBytes() int64 {
	var total int64
	for i := 0; i < c.shards; i++ {
		c.shardMu[i].RLock()
		shardDir := filepath.Join(c.baseDir, fmt.Sprintf("shard_%02d", i))
		entries, err := os.ReadDir(shardDir)
		c.shardMu[i].RUnlock()
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == diskCacheFileExtension {
				info, err := entry.Info()
				if err == nil {
					total += info.Size()
				}
			}
		}
	}
	return total
}

// Dir returns the base directory of the disk cache
func (c *DiskCache) Dir() string {
	return c.baseDir
}

// shardIndex returns the shard index for a given key
func (c *DiskCache) shardIndex(key string) int {
	// Use first 8 chars of key hash to determine shard
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:4])
	var sum int
	for _, b := range hashStr {
		sum += int(b)
	}
	return sum % c.shards
}

// keyToPath converts a cache key to a file path
func (c *DiskCache) keyToPath(key string) string {
	shardIdx := c.shardIndex(key)
	shardDir := filepath.Join(c.baseDir, fmt.Sprintf("shard_%02d", shardIdx))
	return filepath.Join(shardDir, key+diskCacheFileExtension)
}

// generateProjectCacheID creates a unique identifier for the current project.
// It uses the working directory path to generate a short hash, ensuring
// different projects get isolated cache directories.
// Format: "proj_<8-char-hash>" (e.g., "proj_a1b2c3d4")
func generateProjectCacheID() string {
	wd, err := os.Getwd()
	if err != nil {
		// Fallback to a default if we can't get working directory
		return "proj_default"
	}

	// Hash the working directory path
	hash := sha256.Sum256([]byte(wd))
	hashStr := hex.EncodeToString(hash[:4]) // First 4 bytes = 8 hex chars

	return "proj_" + hashStr
}
