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
4. **Turn 4**: Complex output with fallback adapter demonstration

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
lm = dsgo.NewLMCache(lm, 50)  // Cache last 50 requests
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
=== Setup: Provider Fallback Strategy ===
Primary: OpenAI (gpt-4)
Fallback: OpenRouter (claude-3-haiku)
Cache: 50 entries per provider

=== Turn 1: Cold Request (Streaming) ===
▶ turn1_cold.start streaming=true cache=cold

Answer (streaming): Solar panels work by converting sunlight into 
electricity through photovoltaic cells. When sunlight hits the cells, 
it knocks electrons loose, creating an electrical current...

• streaming.complete chunks=47 latency_ms=1834
✓ turn1_cold.end 1834ms

Metrics: 47 chunks, 1834ms latency

=== Turn 2: Warm Request (Cache Hit Expected) ===
▶ turn2_warm.start streaming=false cache=expected_hit
• cache.check status=hit latency_ms=8 speedup=229.3
✓ turn2_warm.end 8ms

Answer (cached): Solar panels work by converting sunlight into 
electricity through photovoltaic cells...
Metrics: 8ms latency (229.3x faster)

=== Turn 3: Different Question (Cache Miss) ===
▶ turn3_miss.start cache=expected_miss
• cache.check status=miss latency_ms=1456
✓ turn3_miss.end 1456ms

Answer: The sky appears blue because of Rayleigh scattering. When 
sunlight enters Earth's atmosphere...
Metrics: 1456ms latency

=== Turn 4: Adapter Fallback Demo ===
▶ turn4_fallback.start adapter=fallback_chain

Summary: Photosynthesis is how plants make food from sunlight...
Difficulty: 6/10
Audience: teen
Adapter used: chat

• adapter.selected adapter=chat
✓ turn4_fallback.end 1102ms

=== System Summary ===
Total requests: 4
Total latency: 3298ms
Avg latency: 824ms
Cache efficiency: 1 hit / 3 total = 33%

Features demonstrated:
  ✓ Streaming output (chunk tracking)
  ✓ LM caching (cold vs warm)
  ✓ Cache hit/miss observability
  ✓ Fallback adapter chain
  ✓ Adapter selection tracking
  ✓ Latency and performance metrics
  ✓ Event logging (DSGO_LOG=pretty)

Resilience patterns:
  ✓ Provider fallback (primary → secondary)
  ✓ Adapter fallback (Chat → JSON)
  ✓ Automatic retry (transparent, exponential backoff)
  ✓ Cache layer (reduce API calls)
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
| Fallback adapter | ~1100ms | - | Auto-selects working format |

## Key Takeaways

1. **Caching** provides 100-200x speedup for repeated queries
2. **Streaming** adds minimal overhead while improving UX
3. **Fallback adapters** ensure robust parsing across LM variations
4. **Event logging** enables production debugging and optimization
5. **Built-in retry** handles transient failures transparently
