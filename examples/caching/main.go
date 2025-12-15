package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

// getModelName returns the model name from environment variable or default
func getModelName() string {
	if model := os.Getenv("EXAMPLES_DEFAULT_MODEL"); model != "" {
		return model
	}
	return "openrouter/amazon/nova-2-lite-v1"
}

// SentimentInput defines input for sentiment classification
type SentimentInput struct {
	Text string `dsgo:"input,desc=Text to analyze for sentiment"`
}

// SentimentOutput defines output for sentiment classification
type SentimentOutput struct {
	Sentiment  string `dsgo:"output,desc=Detected sentiment,enum=positive|negative|neutral"`
	Confidence string `dsgo:"output,desc=Confidence level for the classification"`
}

func main() {
	ctx := context.Background()

	fmt.Println("🗄️  DSGo Caching Example")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("This example demonstrates DSGo's DSPy-parity caching system:")
	fmt.Println("• Memory cache (L1): Fast, volatile LRU cache")
	fmt.Println("• Disk cache (L2): Persistent, sharded file-based cache")
	fmt.Println("• Tiered cache: Two-tier (memory + disk) for best of both")
	fmt.Println()

	fmt.Println("📋 Example 1: Memory Cache (LRU)")
	fmt.Println(strings.Repeat("-", 50))
	memoryCacheExample(ctx)

	fmt.Println("\n📋 Example 2: Disk Cache (Sharded)")
	fmt.Println(strings.Repeat("-", 50))
	diskCacheExample(ctx)

	fmt.Println("\n📋 Example 3: Tiered Cache (Memory + Disk)")
	fmt.Println(strings.Repeat("-", 50))
	tieredCacheExample(ctx)

	fmt.Println("\n📋 Example 4: Cache Statistics & Hit Rates")
	fmt.Println(strings.Repeat("-", 50))
	cacheStatsExample(ctx)

	fmt.Println("\n📋 Example 5: Global Configuration with Caching")
	fmt.Println(strings.Repeat("-", 50))
	globalConfigExample(ctx)

	fmt.Println("\n✨ All caching examples completed!")
}

// memoryCacheExample demonstrates in-memory LRU caching
func memoryCacheExample(ctx context.Context) {
	modelName := getModelName()
	lm, err := dsgo.NewLM(ctx, modelName)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	cache := dsgo.NewLMCacheWithTTL(100, 5*time.Minute)
	fmt.Printf("✅ Created memory cache: capacity=%d, TTL=5m\n", cache.Capacity())

	classifier, err := dsgo.NewTypedPredict[SentimentInput, SentimentOutput](lm)
	if err != nil {
		log.Fatalf("Failed to create classifier: %v", err)
	}

	texts := []string{
		"I absolutely love this product!",
		"This is terrible, very disappointed.",
		"It's okay, nothing special.",
		"I absolutely love this product!", // Duplicate - should hit cache
	}

	fmt.Println("\n🔄 Processing texts (4th should hit cache)...")

	for i, text := range texts {
		input := SentimentInput{Text: text}

		cacheKey := dsgo.GenerateCacheKey(modelName, nil, &dsgo.GenerateOptions{})
		cacheKey = fmt.Sprintf("%s:%s", cacheKey, text)

		if cached, ok := cache.Get(cacheKey); ok {
			dsgo.MarkCacheHit(cached)
			fmt.Printf("   %d. CACHE HIT: %q → (cached result)\n", i+1, truncate(text, 30))
			continue
		}

		output, prediction, err := classifier.RunWithPrediction(ctx, input)
		if err != nil {
			fmt.Printf("   %d. ERROR: %v\n", i+1, err)
			continue
		}

		cache.Set(cacheKey, &dsgo.GenerateResult{
			Content: output.Sentiment,
		})

		fmt.Printf("   %d. CACHE MISS: %q → %s (tokens: %d)\n",
			i+1, truncate(text, 30), output.Sentiment, prediction.Usage.TotalTokens)
	}

	stats := cache.Stats()
	fmt.Printf("\n📊 Cache Stats: hits=%d, misses=%d, hit_rate=%.1f%%, size=%d\n",
		stats.Hits, stats.Misses, stats.HitRate(), stats.Size)
}

// diskCacheExample demonstrates persistent disk caching
func diskCacheExample(ctx context.Context) {
	tmpDir := filepath.Join(os.TempDir(), "dsgo_cache_example")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	diskCache, err := dsgo.NewDiskCacheWithShards(tmpDir, 100*1024*1024, 4)
	if err != nil {
		log.Fatalf("Failed to create disk cache: %v", err)
	}
	fmt.Printf("✅ Created disk cache: dir=%s, shards=4, limit=100MB\n", tmpDir)

	modelName := getModelName()
	lm, err := dsgo.NewLM(ctx, modelName)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	classifier, err := dsgo.NewTypedPredict[SentimentInput, SentimentOutput](lm)
	if err != nil {
		log.Fatalf("Failed to create classifier: %v", err)
	}

	testText := "This is a great day!"
	cacheKey := dsgo.GenerateCacheKey(modelName, nil, &dsgo.GenerateOptions{})
	cacheKey = fmt.Sprintf("%s:%s", cacheKey, testText)

	fmt.Println("\n🔄 First request (populates disk cache)...")
	input := SentimentInput{Text: testText}
	output, prediction, err := classifier.RunWithPrediction(ctx, input)
	if err != nil {
		log.Fatalf("Classification failed: %v", err)
	}

	diskCache.Set(cacheKey, &dsgo.GenerateResult{
		Content: output.Sentiment,
	})

	fmt.Printf("   Result: %s (tokens: %d)\n", output.Sentiment, prediction.Usage.TotalTokens)
	fmt.Printf("   Disk cache size: %d entries, %d bytes\n", diskCache.Size(), diskCache.SizeBytes())

	fmt.Println("\n🔄 Second request (should hit disk cache)...")
	if cached, ok := diskCache.Get(cacheKey); ok {
		dsgo.MarkCacheHit(cached)
		fmt.Printf("   CACHE HIT: Retrieved from disk!\n")
		fmt.Printf("   CacheHit flag: %v\n", cached.CacheHit)
	} else {
		fmt.Printf("   CACHE MISS (unexpected)\n")
	}

	stats := diskCache.Stats()
	fmt.Printf("\n📊 Disk Cache Stats: hits=%d, misses=%d, hit_rate=%.1f%%\n",
		stats.Hits, stats.Misses, stats.HitRate())
}

// tieredCacheExample demonstrates two-tier caching (memory + disk)
func tieredCacheExample(ctx context.Context) {
	tmpDir := filepath.Join(os.TempDir(), "dsgo_tiered_cache_example")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	opts := &dsgo.TieredCacheOptions{
		EnableMemory:   true,
		MemoryCapacity: 50,
		MemoryTTL:      5 * time.Minute,

		EnableDisk:    true,
		DiskDir:       tmpDir,
		DiskSizeLimit: 100 * 1024 * 1024,
		DiskShards:    4,
	}

	tieredCache, err := dsgo.NewTieredCache(opts)
	if err != nil {
		log.Fatalf("Failed to create tiered cache: %v", err)
	}
	fmt.Printf("✅ Created tiered cache: memory_cap=%d, disk_dir=%s\n",
		opts.MemoryCapacity, tmpDir)

	fmt.Println("\n🔄 Simulating cache flow...")

	key1 := "test-key-1"
	result1 := &dsgo.GenerateResult{Content: "positive", FinishReason: "stop"}
	tieredCache.Set(key1, result1)
	fmt.Printf("   Set %q → written to L1 (memory) and L2 (disk)\n", key1)

	if _, ok := tieredCache.Get(key1); ok {
		fmt.Printf("   Get %q → L1 HIT (memory)\n", key1)
	}

	tieredCache.ClearMemory()
	fmt.Printf("   Cleared L1 (memory cache)\n")

	if _, ok := tieredCache.Get(key1); ok {
		fmt.Printf("   Get %q → L2 HIT (disk), promoted to L1\n", key1)
	}

	if _, ok := tieredCache.Get(key1); ok {
		fmt.Printf("   Get %q → L1 HIT (memory, after promotion)\n", key1)
	}

	tieredStats := tieredCache.TieredStats()
	fmt.Printf("\n📊 Tiered Cache Stats:\n")
	fmt.Printf("   Memory: enabled=%v, size=%d, hits=%d\n",
		tieredStats.MemoryEnabled, tieredStats.MemorySize, tieredStats.MemoryHits)
	fmt.Printf("   Disk: enabled=%v, size=%d, hits=%d, bytes=%d\n",
		tieredStats.DiskEnabled, tieredStats.DiskSize, tieredStats.DiskHits, tieredStats.DiskBytes)
	fmt.Printf("   Overall: hit_rate=%.1f%%, memory_hit_rate=%.1f%%, disk_hit_rate=%.1f%%\n",
		tieredStats.HitRate(), tieredStats.MemoryHitRate(), tieredStats.DiskHitRate())
}

// cacheStatsExample demonstrates cache statistics and monitoring
func cacheStatsExample(ctx context.Context) {
	cache := dsgo.NewLMCache(100)

	fmt.Println("🔄 Simulating cache operations...")

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		result := &dsgo.GenerateResult{Content: fmt.Sprintf("result-%d", i)}
		cache.Set(key, result)
	}
	fmt.Printf("   Stored 10 entries\n")

	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key-%d", i%10)
		cache.Get(key) // Simulate lookup (hit or miss)
	}
	fmt.Printf("   Performed 15 lookups (5 misses expected for keys 10-14)\n")

	stats := cache.Stats()
	fmt.Printf("\n📊 Cache Statistics:\n")
	fmt.Printf("   Hits: %d\n", stats.Hits)
	fmt.Printf("   Misses: %d\n", stats.Misses)
	fmt.Printf("   Size: %d entries\n", stats.Size)
	fmt.Printf("   Hit Rate: %.1f%%\n", stats.HitRate())
	fmt.Printf("   Capacity: %d entries\n", cache.Capacity())

	fmt.Println("\n📈 Hit Rate Analysis:")
	total := stats.Hits + stats.Misses
	fmt.Printf("   Total requests: %d\n", total)
	fmt.Printf("   Cache efficiency: %d/%d = %.1f%%\n", stats.Hits, total, stats.HitRate())
}

// globalConfigExample demonstrates global caching configuration
func globalConfigExample(ctx context.Context) {
	tmpDir := filepath.Join(os.TempDir(), "dsgo_global_cache_example")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	fmt.Println("🔧 Configuring global cache settings...")

	dsgo.Configure(
		dsgo.WithTieredCache(&dsgo.TieredCacheOptions{
			EnableMemory:   true,
			MemoryCapacity: 1000,
			MemoryTTL:      10 * time.Minute,
			EnableDisk:     true,
			DiskDir:        tmpDir,
			DiskSizeLimit:  1024 * 1024 * 1024, // 1GB
			DiskShards:     8,
		}),
	)

	fmt.Printf("   ✅ Tiered cache enabled via dsgo.Configure()\n")
	fmt.Printf("   Memory: 1000 entries, 10m TTL\n")
	fmt.Printf("   Disk: 1GB limit, 8 shards, dir=%s\n", tmpDir)

	settings := dsgo.GetSettings()
	if settings.DefaultCache != nil {
		fmt.Printf("   Cache active: %T\n", settings.DefaultCache)
	}

	fmt.Println("\n📋 Alternative Configuration Options:")
	fmt.Println("   • dsgo.WithCache(1000)              - Memory-only cache")
	fmt.Println("   • dsgo.WithCacheTTL(5m)             - Set TTL for entries")
	fmt.Println("   • dsgo.WithDiskCache(\"/path\")       - Enable disk caching")
	fmt.Println("   • dsgo.WithDiskCacheSizeLimit(30GB) - Set disk limit")
	fmt.Println("   • dsgo.WithMemoryCache(1000000)     - Set memory capacity")
	fmt.Println("   • dsgo.WithTieredCache(opts)        - Full tiered config")

	fmt.Println("\n🔑 Environment Variables:")
	fmt.Println("   • DSGO_CACHE_DISK=true/false - Enable/disable disk cache")
	fmt.Println("   • DSGO_CACHEDIR=/path        - Custom cache directory")
	fmt.Println("   • DSGO_CACHE_LIMIT=bytes     - Disk size limit")
	fmt.Println("   • DSGO_CACHE_TTL=5m          - Cache TTL")

	dsgo.ResetConfig()
}

// truncate shortens a string for display
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
