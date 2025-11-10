# 05 - Resilience and Observability

**Q&A System with Fallback, Caching, Streaming, and Metrics**

## What This Demonstrates

### Modules
- ✓ **Predict** - Basic completions with resilience features

### Adapters
- ✓ **Fallback** - Automatic adapter chain (Chat → JSON)
- ✓ **Chat** - Natural dialogue format
- ✓ **JSON** - Structured output parsing

### Features
- ✓ **Streaming** - Real-time chunked output with metrics
- ✓ **Caching** - LM cache with hit/miss tracking
- ✓ **Cache TTL** - Time-to-live expiry for cache freshness
- ✓ **Provider fallback** - Primary → secondary LM
- ✓ **Adapter fallback** - Automatic format switching
- ✓ **Retry with backoff** - Transparent error recovery

### Observability
- ✓ Latency tracking (cold vs warm)
- ✓ Cache hit rate metrics
- ✓ Chunk counting (streaming)
- ✓ Adapter selection visibility
- ✓ Speedup calculations
- ✓ Comprehensive event logging

## Story Flow

1. **Turn 1**: Cold request with streaming (cache miss, full latency)
2. **Turn 2**: Repeat same question (cache hit, near-instant)
3. **Turn 3**: Different question (cache miss, full latency)
4. **Turn 4**: Complex output with TwoStep adapter demonstration
5. **Turn 5**: Cache TTL expiry demonstration (3 calls showing expiry behavior)

## Resilience Patterns

### Provider Fallback
```go
primaryLM := openai.NewOpenAI(apiKey)
fallbackLM := openrouter.NewOpenRouter(apiKey, "anthropic/claude-3-haiku")

// Use primary, fallback automatically on failure
// (simplified - full fallback requires wrapper implementation)
```

### Adapter Fallback
```go
fallbackAdapter := dsgo.NewFallbackAdapterWithChain([]dsgo.Adapter{
    dsgo.NewChatAdapter(),    // Try first
    dsgo.NewJSONAdapter(),    // Fallback
})

predict := module.NewPredict(sig, lm).WithAdapter(fallbackAdapter)
```

### Caching Strategy
```go
// Configure global cache TTL
dsgo.Configure(
    dsgo.WithCacheTTL(5*time.Minute),  // 5 minute TTL
)

// Or via environment variable
// DSGO_CACHE_TTL=5m

// Cache automatically expires entries after TTL
// TTL of 0 means infinite (no expiry)
```

### Automatic Retry
Built-in: 3 retries with exponential backoff (1s → 2s → 4s) for:
- 429 (rate limit)
- 500, 502, 503, 504 (server errors)

## Metrics Captured

### Per-Request
- Latency (milliseconds)
- Cache status (hit/miss)
- Chunk count (streaming)
- Adapter used
- Token usage (if available)

### Aggregate
- Total latency
- Average latency
- Cache hit rate
- Speedup ratio (cold vs warm)

## Run

```bash
cd examples/05-resilience-observability
go run main.go
```

### With full event logging
```bash
DSGO_LOG=pretty go run main.go
```

### With JSON event stream
```bash
DSGO_LOG=events go run main.go > events.jsonl
```

## Expected Output

```
=== Global Configuration ===
Provider: openrouter
Timeout: 30s
Max retries: 3
Cache TTL: 3s

=== Turn 1: Cold Request (Cache Miss) ===
User: How do solar panels work?
Answer: Imagine your toy car needs batteries...
Metrics: 1554ms latency (cache miss)
Usage: Prompt 123 tokens, Completion 87 tokens

=== Turn 2: Warm Request (Cache Hit Expected) ===
User: How do solar panels work?
Answer: Imagine your toy car needs batteries...
Metrics: 0ms latency (inf x faster)
Usage: Prompt 123 tokens, Completion 87 tokens

=== Turn 3: Different Question (Cache Miss) ===
User: Why is the sky blue?
Answer: The sky appears blue because of Rayleigh scattering...
Metrics: 1456ms latency
Usage: Prompt 115 tokens, Completion 92 tokens

=== Turn 4: Optional Outputs + TwoStep Adapter ===
User: Explain Photosynthesis in plants with structured output
Summary: Photosynthesis is how plants make food from sunlight...
Difficulty: 6/10
Audience: teen
Statistics: (not provided)
Adapter: TwoStep (reasoning → extraction)
Usage: Prompt 234 tokens, Completion 156 tokens

=== Turn 5: Cache TTL Expiry Demo ===
Cache TTL configured: 3s
Testing same question with time delays to demonstrate TTL expiry

User (t=0s): What is the capital of France?
Answer: Paris is the capital of France...
Latency: 1523ms (cache miss)

User (t=0.5s, within TTL): What is the capital of France?
Answer: Paris is the capital of France...
Latency: 7ms (cache hit, 217.6x faster)

Waiting for TTL expiry (3s)...

User (t=3.1s, after TTL expired): What is the capital of France?
Answer: Paris is the capital of France...
Latency: 1489ms (cache expired, fresh call)

📊 Cache TTL behavior summary:
  • Call 1 (t=0s):      1523ms - MISS (initial)
  • Call 2 (t=0.5s):       7ms - HIT (within 3s TTL)
  • Call 3 (t=3.1s):    1489ms - MISS (expired)
  • TTL ensures fresh data while balancing performance
  • Configure via dsgo.WithCacheTTL() or DSGO_CACHE_TTL env var

=== System Summary ===
Total requests: 7 (4 turns + 3 TTL demo)
Total latency: 6352ms
Avg latency: 907ms
Cache efficiency: 3 hits / 6 total cacheable = 50%

Features demonstrated:
  ✓ Global configuration (Configure + GetSettings)
  ✓ Streaming output (chunk tracking)
  ✓ LM caching (cold vs warm)
  ✓ Cache TTL (time-to-live expiry)
  ✓ Cache hit/miss observability
  ✓ TwoStep adapter (reasoning models)
  ✓ Optional outputs (graceful degradation)
  ✓ Latency and performance metrics
  ✓ Event logging (DSGO_LOG=pretty)

Resilience patterns:
  ✓ Centralized configuration
  ✓ Timeout control (30s default)
  ✓ Automatic retry (3 attempts, exponential backoff)
  ✓ Cache layer with TTL (reduce API calls, ensure freshness)
  ✓ Adapter flexibility (TwoStep for reasoning models)
```

## Event Logging

### Environment Variables
- `DSGO_LOG=off|events|pretty|both` - Logging mode (default: pretty)
- `DSGO_LOG_FILE=events.jsonl` - Output file (default: stdout)

### Event Types
- `run.start` / `run.end` - Overall conversation
- `module.start` / `module.end` - Module execution
- `cache.check` - Cache hit/miss
- `stream.chunk` - Streaming token
- `adapter.selected` - Which adapter succeeded

### Sample Event (JSON)
```json
{
  "ts": "2024-11-07T10:23:45Z",
  "level": "INFO",
  "span_id": "abc-123",
  "run_id": "xyz-789",
  "kind": "cache",
  "operation": "cache.check",
  "latency_ms": 8,
  "cache": {"status": "hit"},
  "fields": {"speedup": 229.3}
}
```

## Performance Comparison

| Scenario | Latency | Cache | Notes |
|----------|---------|-------|-------|
| Cold (streaming) | ~1800ms | miss | Full LM call + streaming |
| Warm (cached) | ~8ms | **hit** | **229x faster** |
| Cold (non-stream) | ~1450ms | miss | Full LM call |
| TTL cache hit | ~7ms | **hit** | Within TTL window |
| TTL cache expired | ~1490ms | miss | After TTL expiry |

## Key Takeaways

1. **Caching** provides 100-200x speedup for repeated queries
2. **Cache TTL** balances performance and freshness (configurable per use case)
3. **Streaming** adds minimal overhead while improving UX
4. **Fallback adapters** ensure robust parsing across LM variations
5. **Event logging** enables production debugging and optimization
6. **Built-in retry** handles transient failures transparently
