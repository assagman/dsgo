package core

import (
	"sync"
	"testing"
)

func TestNewTieredCache(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
		DiskSizeLimit:  1024 * 1024, // 1MB
		DiskShards:     4,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	if cache.memory == nil {
		t.Error("Memory cache should be initialized")
	}
	if cache.disk == nil {
		t.Error("Disk cache should be initialized")
	}
}

func TestNewTieredCache_DefaultOptions(t *testing.T) {
	cache, err := NewTieredCache(nil)
	if err != nil {
		t.Fatalf("NewTieredCache with nil opts failed: %v", err)
	}

	if cache.memory == nil {
		t.Error("Memory cache should be initialized with defaults")
	}
}

func TestTieredCache_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
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
}

func TestTieredCache_MemoryHitFirst(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "test"})
	cache.Get("key1")
	cache.Get("key1")

	stats := cache.TieredStats()
	if stats.MemoryHits != 2 {
		t.Errorf("Expected 2 memory hits, got %d", stats.MemoryHits)
	}
	if stats.DiskHits != 0 {
		t.Errorf("Expected 0 disk hits, got %d", stats.DiskHits)
	}
}

func TestTieredCache_DiskPromotion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache, add entry, then clear memory to force disk hit
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "test"})

	// Clear only memory cache
	cache.ClearMemory()

	// Should hit disk and promote to memory
	result, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit from disk")
	}
	if result.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Content)
	}

	stats := cache.TieredStats()
	if stats.DiskHits != 1 {
		t.Errorf("Expected 1 disk hit, got %d", stats.DiskHits)
	}

	// Next hit should be from memory (promoted)
	cache.Get("key1")
	stats = cache.TieredStats()
	if stats.MemoryHits != 1 {
		t.Errorf("Expected 1 memory hit after promotion, got %d", stats.MemoryHits)
	}
}

func TestTieredCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Set("key2", &GenerateResult{Content: "2"})

	cache.Clear()

	_, ok := cache.Get("key1")
	if ok {
		t.Error("Cache should be empty after Clear")
	}

	stats := cache.TieredStats()
	if stats.MemoryHits != 0 || stats.DiskHits != 0 || stats.TotalMisses != 1 {
		t.Errorf("Stats should be reset after clear: %+v", stats)
	}
}

func TestTieredCache_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Get("key1") // Memory hit
	cache.Get("key2") // Miss

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestTieredCache_TieredStats(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	stats := cache.TieredStats()
	if !stats.MemoryEnabled {
		t.Error("Memory should be enabled")
	}
	if !stats.DiskEnabled {
		t.Error("Disk should be enabled")
	}
	if stats.MemoryCapacity != 100 {
		t.Errorf("Expected memory capacity 100, got %d", stats.MemoryCapacity)
	}
	if stats.DiskDir != tmpDir {
		t.Errorf("Expected disk dir %s, got %s", tmpDir, stats.DiskDir)
	}
}

func TestTieredCache_HitRates(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "1"})
	cache.Get("key1") // Memory hit
	cache.Get("key1") // Memory hit
	cache.ClearMemory()
	cache.Get("key1") // Disk hit
	cache.Get("key2") // Miss

	stats := cache.TieredStats()
	// 2 memory hits + 1 disk hit + 1 miss = 4 total
	hitRate := stats.HitRate()
	if hitRate != 75.0 {
		t.Errorf("Expected 75%% hit rate, got %.2f%%", hitRate)
	}

	memHitRate := stats.MemoryHitRate()
	if memHitRate != 50.0 {
		t.Errorf("Expected 50%% memory hit rate, got %.2f%%", memHitRate)
	}

	diskHitRate := stats.DiskHitRate()
	if diskHitRate != 25.0 {
		t.Errorf("Expected 25%% disk hit rate, got %.2f%%", diskHitRate)
	}
}

func TestTieredCache_MemoryOnly(t *testing.T) {
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     false,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "test"})
	result, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit")
	}
	if result.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Content)
	}

	stats := cache.TieredStats()
	if !stats.MemoryEnabled {
		t.Error("Memory should be enabled")
	}
	if stats.DiskEnabled {
		t.Error("Disk should be disabled")
	}
}

func TestTieredCache_DiskOnly(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory: false,
		EnableDisk:   true,
		DiskDir:      tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "test"})
	result, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit")
	}
	if result.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Content)
	}

	stats := cache.TieredStats()
	if stats.MemoryEnabled {
		t.Error("Memory should be disabled")
	}
	if !stats.DiskEnabled {
		t.Error("Disk should be enabled")
	}
}

func TestTieredCache_GracefulFallback(t *testing.T) {
	opts := &TieredCacheOptions{
		EnableMemory: false, // Disabled
		EnableDisk:   false, // Disabled - should fallback to memory
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	// Should have fallback to memory cache
	if cache.memory == nil {
		t.Error("Should have fallback memory cache")
	}

	cache.Set("key1", &GenerateResult{Content: "test"})
	result, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit with fallback")
	}
	if result.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Content)
	}
}

func TestTieredCache_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
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

func TestTieredCache_ClearMemoryOnly(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "test"})
	cache.ClearMemory()

	// Should still be available from disk
	result, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit from disk after memory clear")
	}
	if result.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Content)
	}
}

func TestTieredCache_ClearDiskOnly(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	cache.Set("key1", &GenerateResult{Content: "test"})
	cache.ClearDisk()

	// Should still be available from memory
	result, ok := cache.Get("key1")
	if !ok {
		t.Fatal("Expected cache hit from memory after disk clear")
	}
	if result.Content != "test" {
		t.Errorf("Expected 'test', got '%s'", result.Content)
	}

	// Now clear memory and it should be gone
	cache.ClearMemory()
	_, ok = cache.Get("key1")
	if ok {
		t.Error("Should miss after both tiers cleared")
	}
}

func TestTieredCache_AccessUnderlyingCaches(t *testing.T) {
	tmpDir := t.TempDir()
	opts := &TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 100,
		EnableDisk:     true,
		DiskDir:        tmpDir,
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}

	memCache := cache.MemoryCache()
	if memCache == nil {
		t.Error("MemoryCache() should return non-nil")
	}

	diskCache := cache.DiskCache()
	if diskCache == nil {
		t.Error("DiskCache() should return non-nil")
	}
}

func TestDefaultTieredCacheOptions(t *testing.T) {
	opts := DefaultTieredCacheOptions()

	if !opts.EnableMemory {
		t.Error("Memory should be enabled by default")
	}
	if opts.MemoryCapacity != 1000000 {
		t.Errorf("Expected default memory capacity 1000000, got %d", opts.MemoryCapacity)
	}
	if !opts.EnableDisk {
		t.Error("Disk should be enabled by default")
	}
	if opts.DiskSizeLimit != DefaultDiskCacheSizeLimit {
		t.Errorf("Expected default disk size limit %d, got %d", DefaultDiskCacheSizeLimit, opts.DiskSizeLimit)
	}
	if opts.DiskShards != DefaultDiskCacheShards {
		t.Errorf("Expected default disk shards %d, got %d", DefaultDiskCacheShards, opts.DiskShards)
	}
}
