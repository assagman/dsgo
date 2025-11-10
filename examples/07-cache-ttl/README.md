# 07 - Cache and TTL

**Cache Configuration, TTL Behavior, and Performance Metrics**

## What This Demonstrates

### Modules
- ✓ **Predict** - Basic prediction with cache integration

### Adapters
- ✓ **Chat** - Natural dialogue format

### Features
- ✓ **Caching** - LM cache with automatic hit/miss tracking
- ✓ **Cache TTL** - Time-to-live expiry for cache freshness
- ✓ **Cache statistics** - Hit rate, size, and performance metrics
- ✓ **Configuration API** - Programmatic and environment-based setup

### Observability
- ✓ Cache hit/miss tracking
- ✓ Latency measurements
- ✓ Token usage metrics
- ✓ Event logging (set `DSGO_LOG=pretty`)

## Story Flow

1. **Test 1**: First query → cache miss (full LM call)
2. **Test 2**: Same query → cache hit (instant response)
3. **Test 3**: Different query → cache miss (new query)
4. **Test 4**: Wait for TTL expiry → cache miss (fresh data)
5. **Test 5**: Cache statistics summary

## Run

**Note:** `EXAMPLES_DEFAULT_MODEL` environment variable is required.

```bash
cd examples/07-cache-ttl
EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go
```

### With different TTL (via environment)

```bash
# 10 minute TTL
DSGO_CACHE_TTL=10m EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go

# 1 hour TTL
DSGO_CACHE_TTL=1h EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go

# No expiration (cache until capacity)
DSGO_CACHE_TTL=0 EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go
```

### With verbose logging

```bash
DSGO_LOG=pretty EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go
```

### With JSON events

```bash
DSGO_LOG=events EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go > events.jsonl
```

## Expected Output

```
=== Cache Configuration ===
Model: anthropic/claude-3-haiku
Cache enabled: true
Cache capacity: 100 entries
Cache TTL: 5s

=== Test 1: Cache Miss ===
Question: What is the capital of Japan?
Answer: Tokyo is the capital of Japan and one of the world's most...
Latency: 1456ms (cache miss)
Tokens: 115 prompt, 89 completion

=== Test 2: Cache Hit (within TTL) ===
Question: What is the capital of Japan?
Answer: Tokyo is the capital of Japan and one of the world's most...
Latency: 8ms (cache hit, 182.0x faster)
Tokens: 115 prompt, 89 completion

=== Test 3: Different Question (cache miss) ===
Question: What is the largest planet in our solar system?
Answer: Jupiter is the largest planet in our solar system...
Latency: 1323ms (cache miss)
Tokens: 108 prompt, 76 completion

=== Test 4: TTL Expiry ===
Waiting 5s for TTL to expire...
TTL expired, making same request again...
Question: What is the capital of Japan?
Answer: Tokyo is the capital of Japan and one of the world's most...
Latency: 1389ms (cache expired, fresh call)
Tokens: 115 prompt, 89 completion

=== Cache Statistics ===
Cache hits: 1
Cache misses: 3
Hit rate: 25.0%
Current size: 2/100 entries

=== Summary ===
Cache behavior demonstrated:
  ✓ Test 1: First call → cache miss (full LM call)
  ✓ Test 2: Repeat call → cache hit (instant response)
  ✓ Test 3: Different question → cache miss (new query)
  ✓ Test 4: After TTL expiry → cache miss (fresh data)

Key benefits:
  • 100-1000x faster responses for cached queries
  • Reduced API costs (no tokens used on cache hits)
  • TTL ensures data freshness
  • Automatic cache management

Configuration options:
  • Programmatic: dsgo.WithCache(capacity), dsgo.WithCacheTTL(duration)
  • Environment: DSGO_CACHE_TTL=5m (e.g., 5m, 1h, 30s)
  • TTL=0: No expiration (cache until capacity limit)
```

## Key Concepts

### Cache Hit Rate

The cache hit rate indicates how often requests are served from cache:
- **High hit rate (80%+)**: Good cache efficiency, most requests are repeats
- **Low hit rate (<20%)**: Cache may not be beneficial for your use case
- **Goal**: Balance between performance and freshness

### TTL (Time-To-Live)

TTL determines how long cached entries remain valid:
- **Short TTL (30s-5m)**: Fresh data, good for rapidly changing information
- **Medium TTL (5m-1h)**: Balanced performance and freshness
- **Long TTL (1h+)**: Maximum performance, stable information
- **No TTL (0)**: Cache until capacity limit, manual invalidation only

### Cache Capacity

Maximum number of entries the cache can hold:
- When capacity is reached, least recently used (LRU) entries are evicted
- Choose capacity based on your query pattern and memory constraints
- Typical range: 100-10000 entries

## Configuration Options

### Programmatic Configuration

```go
// Enable cache with capacity
dsgo.Configure(
    dsgo.WithCache(1000),              // 1000 entry capacity
    dsgo.WithCacheTTL(5*time.Minute),  // 5 minute TTL
)

// Get current settings
settings := dsgo.GetSettings()
fmt.Printf("Cache capacity: %d\n", settings.DefaultCache.Capacity())
fmt.Printf("Cache TTL: %v\n", settings.CacheTTL)
```

### Environment Configuration

```bash
# Set via environment variable
export DSGO_CACHE_TTL=5m

# Supported formats:
# - 30s  (30 seconds)
# - 5m   (5 minutes)
# - 1h   (1 hour)
# - 2h30m (2 hours 30 minutes)
# - 0    (no expiration)
```

### Cache Statistics

```go
settings := dsgo.GetSettings()
stats := settings.DefaultCache.Stats()

fmt.Printf("Hits: %d\n", stats.Hits)
fmt.Printf("Misses: %d\n", stats.Misses)
fmt.Printf("Hit rate: %.1f%%\n", stats.HitRate()*100)
fmt.Printf("Size: %d/%d\n", settings.DefaultCache.Size(), settings.DefaultCache.Capacity())
```

## Use Cases

### Customer Support Chatbot
- **TTL**: 1 hour
- **Capacity**: 1000 entries
- **Benefit**: Instant responses to frequently asked questions

### Code Generation API
- **TTL**: 5 minutes
- **Capacity**: 500 entries
- **Benefit**: Fast repeated code suggestions

### Data Analysis Queries
- **TTL**: 0 (no expiration)
- **Capacity**: 10000 entries
- **Benefit**: Cache expensive analytical queries indefinitely

### Real-time Information
- **TTL**: 30 seconds
- **Capacity**: 100 entries
- **Benefit**: Balance freshness with performance

## Performance Characteristics

| Scenario | Latency | Tokens Used | Use Case |
|----------|---------|-------------|----------|
| Cache miss | 1000-2000ms | Full query | First request, expired TTL |
| Cache hit | 1-10ms | None | Repeated query within TTL |
| Speedup | 100-1000x | 100% savings | High repeat rate scenarios |

## Integration with Other Features

Cache works seamlessly with:
- **Streaming**: Cached responses return instantly (no streaming needed)
- **History**: Each unique history state is cached separately
- **Adapters**: Cache keys include adapter configuration
- **Tools**: Tool definitions are included in cache keys
- **Retry**: Cache is checked before retry logic

## Troubleshooting

### Low cache hit rate
- Increase TTL if data doesn't change frequently
- Increase capacity if you have many unique queries
- Check that queries are identical (including whitespace)

### Cache not working
- Verify `WithCache()` was called before `NewLM()`
- Check that `DSGO_CACHE_TTL` is a valid duration format
- Ensure cache capacity > 0

### Stale data in cache
- Reduce TTL for more frequent refresh
- Use `TTL=0` and implement manual cache invalidation
- Consider cache key versioning for data updates

## Related Examples

- **01-hello-chat**: Basic caching with history
- **05-resilience-observability**: Cache with TTL demonstration
- **06-parallel**: Cache behavior in parallel execution

## Further Reading

- [Core Cache Implementation](../../core/cache.go)
- [Configuration API](../../core/configure.go)
- [Settings Management](../../core/settings.go)
