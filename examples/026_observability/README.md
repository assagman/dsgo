# 026_observability - Comprehensive Observability

## Overview

Demonstrates DSGo's **comprehensive observability features** across all interaction modes. Shows how to extract provider metadata, track history with collectors, and enable complete observability for streaming calls.

## What it demonstrates

- **Provider metadata extraction** - Usage metrics, cache info, request tracking, rate limits
- **History tracking with MemoryCollector** - Automatic collection of HistoryEntry records
- **Streaming observability with LMWrapper** - Complete metrics for streaming calls
- **Multiple collector types** - Memory, JSONL, and composite collectors
- **Production monitoring** - Best-effort collection that never fails calls
- Use cases: cost tracking, performance monitoring, debugging, analytics

## Usage

```bash
cd examples/026_observability
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=1
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
=== Observability Example ===
Demonstrates comprehensive observability with metadata, history tracking, and streaming

--- Observability Features ---
✓ Provider metadata extraction (usage, cache, rate limits)
✓ History tracking with MemoryCollector
✓ Streaming observability with LMWrapper
✓ Complete metrics for all interaction types

────────────────────────────────────────────────────────────────────────────────

--- Demo 1: Provider Metadata Extraction ---
Extract rich metadata from provider responses

Making API call...

Response: The three primary colors are red, blue, and yellow.

💰 Usage Metrics:
  Prompt Tokens:     12
  Completion Tokens: 15
  Total Tokens:      27
  Estimated Cost:    $0.000004
  Latency:           850ms

🔍 Provider Metadata:
  Cache Status:      miss
  Generation ID:     gen_abc123
  Rate Limit:        100 requests
  Rate Remaining:    99 requests

✅ Finish Reason: stop

────────────────────────────────────────────────────────────────────────────────

--- Demo 2: History Tracking & MemoryCollector ---
Automatic collection of HistoryEntry records

Making 2 API calls to track history...
  Call 1: Call 1
  Call 2: Call 2

📋 Collected History Entries:
  Total entries: 2

  --- Entry 1 ---
  ID:            a1b2c3d4...
  Session ID:    x9y8z7w6...
  Provider:      openrouter
  Model:         google/gemini-2.0-flash-001:free
  Timestamp:     14:32:15
  Tokens:        10 prompt + 3 completion = 13 total
  Cost:          $0.000002
  Latency:       720ms
  Cache Hit:     false
  Metadata keys: 5

  --- Entry 2 ---
  ID:            e5f6g7h8...
  Session ID:    x9y8z7w6...
  Provider:      openrouter
  Model:         google/gemini-2.0-flash-001:free
  Timestamp:     14:32:16
  Tokens:        10 prompt + 3 completion = 13 total
  Cost:          $0.000002
  Latency:       680ms
  Cache Hit:     false
  Metadata keys: 5

  ✓ Saved sample entry to observability_sample.json

────────────────────────────────────────────────────────────────────────────────

--- Demo 3: Streaming with Observability ---
Complete tracking for streaming LM calls

Using model: google/gemini-2.0-flash-001:free

🔹 Standard Generate Call:
Response: Code flows swift,
Goroutines dance in concert,
Concurrency wins.
Usage: 37 tokens, $0.000005, 890ms

🔹 Streaming Call:
Response: Streaming offers real-time responsiveness and enables better user experience through progressive content delivery.
Usage: 28 tokens, $0.000004, 780ms

📊 Generate vs Stream Comparison:
Metric               Generate        Stream         
────────────────────────────────────────────────────
Tokens               37              28             
Cost                 $0.000005       $0.000004      
Latency              890ms           780ms          

✅ Both methods produce complete observability data!

────────────────────────────────────────────────────────────────────────────────

--- Observability Benefits ---
✓ Automatic metadata collection (no manual instrumentation)
✓ Complete metrics for all interaction types (Generate, Stream)
✓ Flexible collectors (memory, file, composite)
✓ Production-ready (best-effort, never fails calls)
✓ Usage tracking (tokens, costs, latency)
✓ Cache tracking (hits, misses, sources)

=== Summary ===
Observability provides:
  ✓ Rich provider metadata extraction
  ✓ Automatic history tracking with MemoryCollector
  ✓ Complete streaming observability
  ✓ Production-grade monitoring and debugging

📊 Total tokens used: 78
🔧 Total demos: 3
```

## Key Concepts

### 1. Provider Metadata Extraction

Extract rich metadata from provider API responses:

```go
import "github.com/assagman/dsgo"

lm := shared.GetLM(shared.GetModel())

messages := []dsgo.Message{
    {Role: "user", Content: "What are the three primary colors?"},
}

result, err := lm.Generate(ctx, messages, options)

// Usage metrics
fmt.Printf("Tokens: %d\n", result.Usage.TotalTokens)
fmt.Printf("Cost: $%.6f\n", result.Usage.Cost)
fmt.Printf("Latency: %dms\n", result.Usage.Latency)

// Provider-specific metadata
if cacheStatus, ok := result.Metadata["cache_status"].(string); ok {
    fmt.Printf("Cache Status: %s\n", cacheStatus)
}
if genID, ok := result.Metadata["generation_id"].(string); ok {
    fmt.Printf("Generation ID: %s\n", genID)
}
```

**Benefits:**
- **Cost tracking** - Monitor API usage and costs
- **Performance monitoring** - Track latency and optimization opportunities
- **Cache visibility** - See cache hits/misses
- **Rate limit awareness** - Monitor quota usage
- **Request tracing** - Correlate requests with IDs

**Metadata fields:**
- `cache_status` - "hit", "miss", or "none"
- `cache_hit` - Boolean cache hit indicator
- `request_id` - Provider's request identifier
- `generation_id` - Provider's generation identifier
- `rate_limit_requests` - Total rate limit
- `rate_limit_remaining_requests` - Remaining quota
- `rate_limit_reset` - When quota resets

**When to use:**
- Production cost monitoring
- Performance optimization
- Debugging API issues
- Rate limit management
- Cache effectiveness analysis

### 2. History Tracking with MemoryCollector

Automatic collection of all LM interactions:

```go
import "github.com/assagman/dsgo"

// Create collector with capacity for 100 entries
collector := dsgo.NewMemoryCollector(100)

// Configure global settings with collector
dsgo.Configure(
    dsgo.WithProvider("openrouter"),
    dsgo.WithModel("google/gemini-2.0-flash-001:free"),
    dsgo.WithCollector(collector),
)

// Create LM - automatically wrapped with observability
lm := shared.GetLM(shared.GetModel())

// Make calls - history automatically collected
for i := 0; i < 5; i++ {
    result, err := lm.Generate(ctx, messages, options)
    // ...
}

// Inspect collected history
entries := collector.GetAll()
for _, entry := range entries {
    fmt.Printf("ID: %s\n", entry.ID)
    fmt.Printf("Tokens: %d\n", entry.Usage.TotalTokens)
    fmt.Printf("Cost: $%.6f\n", entry.Usage.Cost)
}
```

**HistoryEntry structure:**

```go
type HistoryEntry struct {
    ID        string    // Unique call ID
    Timestamp time.Time // Call timestamp
    SessionID string    // Session identifier
    
    Provider string // "openrouter", "openai", etc.
    Model    string // Model name
    
    Request  RequestMeta  // Messages, options, metadata
    Response ResponseMeta // Content, tool calls, finish reason
    Usage    Usage        // Tokens, cost, latency
    Cache    CacheMeta    // Hit status, source, TTL
    
    ProviderMeta map[string]any // Provider-specific data
    Error        *ErrorMeta      // Error details (if failed)
}
```

**Benefits:**
- **Zero instrumentation** - No manual tracking code
- **Complete records** - Full request/response data
- **Thread-safe** - Safe for concurrent access
- **Ring buffer** - Automatic old entry eviction
- **Development debugging** - Inspect recent calls

**MemoryCollector methods:**

```go
// Get all entries (up to capacity)
entries := collector.GetAll()

// Get last N entries
recent := collector.GetLast(10)

// Get entry count
count := collector.Count()

// Clear all entries
collector.Clear()
```

**When to use:**
- Development and testing
- Debugging recent API calls
- In-memory analytics
- Temporary monitoring
- Session-based tracking

### 3. Streaming Observability with LMWrapper

Complete observability for streaming calls:

```go
import "github.com/assagman/dsgo"

lm := shared.GetLM(shared.GetModel())

// Create collector for streaming
collector := dsgo.NewMemoryCollector(10)

// Wrap LM to enable observability
wrappedLM := dsgo.NewLMWrapper(lm, collector)

// Streaming call - metrics automatically tracked
chunkChan, errChan := wrappedLM.Stream(ctx, messages, options)

var fullContent string
for chunk := range chunkChan {
    fullContent += chunk.Content
    fmt.Print(chunk.Content)
}

if err := <-errChan; err != nil {
    log.Fatal(err)
}

// Inspect streaming metrics
entries := collector.GetAll()
if len(entries) > 0 {
    lastEntry := entries[len(entries)-1]
    fmt.Printf("Tokens: %d\n", lastEntry.Usage.TotalTokens)
    fmt.Printf("Cost: $%.6f\n", lastEntry.Usage.Cost)
    fmt.Printf("Latency: %dms\n", lastEntry.Usage.Latency)
}
```

**Benefits:**
- **Identical data** - Streaming produces same metrics as Generate
- **Automatic calculation** - Tokens/cost computed after stream completes
- **Latency tracking** - Total time from start to finish
- **Error capture** - Stream errors recorded in history
- **Production-ready** - Best-effort collection, never fails streams

**What gets tracked:**
- Full request (messages, options)
- Complete streamed content
- Token counts (computed after completion)
- Cost estimation (based on tokens)
- Latency (start to finish)
- Finish reason
- Provider metadata
- Errors (if any)

**When to use:**
- Production streaming applications
- Real-time monitoring
- Cost tracking for streams
- Performance benchmarking
- Error tracking

## Advanced Patterns

### Pattern 1: Multiple Collector Types

Combine different collectors:

```go
// Memory collector for recent history
memCollector := dsgo.NewMemoryCollector(100)

// JSONL collector for persistent logging
jsonlCollector, err := dsgo.NewJSONLCollector("observability.jsonl")
if err != nil {
    log.Fatal(err)
}
defer jsonlCollector.Close()

// Composite collector - sends to both
composite := dsgo.NewCompositeCollector(memCollector, jsonlCollector)

dsgo.Configure(
    dsgo.WithProvider("openrouter"),
    dsgo.WithModel("google/gemini-2.0-flash-001:free"),
    dsgo.WithCollector(composite),
)

// All calls logged to both memory and file
lm := shared.GetLM(shared.GetModel())
```

### Pattern 2: Production Monitoring

Production-grade observability setup:

```go
package main

import (
    "context"
    "log"
    "github.com/assagman/dsgo"
)

func setupObservability() (dsgo.Collector, error) {
    // JSONL collector for production logs
    jsonlCollector, err := dsgo.NewJSONLCollector("production_observability.jsonl")
    if err != nil {
        return nil, err
    }
    
    return jsonlCollector, nil
}

func main() {
    collector, err := setupObservability()
    if err != nil {
        log.Fatal(err)
    }
    defer collector.(*dsgo.JSONLCollector).Close()
    
    dsgo.Configure(
        dsgo.WithProvider("openrouter"),
        dsgo.WithModel("google/gemini-2.0-flash-001:free"),
        dsgo.WithCollector(collector),
    )
    
    lm, err := dsgo.NewLM(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    
    // All interactions logged to production_observability.jsonl
    // ...
}
```

### Pattern 3: Session-based Tracking

Track all calls within a user session:

```go
func handleUserSession(ctx context.Context, sessionID string) {
    // Create session-specific collector
    collector := dsgo.NewMemoryCollector(1000)
    
    // Configure for this session
    dsgo.Configure(
        dsgo.WithProvider("openrouter"),
        dsgo.WithModel("google/gemini-2.0-flash-001:free"),
        dsgo.WithCollector(collector),
    )
    
    lm := shared.GetLM(shared.GetModel())
    
    // Make multiple calls in session
    // ...
    
    // Analyze session metrics
    entries := collector.GetAll()
    var totalTokens, totalCost int
    for _, entry := range entries {
        totalTokens += entry.Usage.TotalTokens
        totalCost += entry.Usage.Cost
    }
    
    log.Printf("Session %s: %d calls, %d tokens, $%.6f", 
        sessionID, len(entries), totalTokens, totalCost)
}
```

### Pattern 4: Custom Analytics

Build custom analytics from history:

```go
func analyzePerformance(collector *dsgo.MemoryCollector) {
    entries := collector.GetAll()
    
    var totalTokens int
    var totalCost float64
    var totalLatency int
    var cacheHits int
    
    for _, entry := range entries {
        totalTokens += entry.Usage.TotalTokens
        totalCost += entry.Usage.Cost
        totalLatency += entry.Usage.Latency
        if entry.Cache.Hit {
            cacheHits++
        }
    }
    
    fmt.Println("=== Performance Analytics ===")
    fmt.Printf("Total calls: %d\n", len(entries))
    fmt.Printf("Total tokens: %d\n", totalTokens)
    fmt.Printf("Total cost: $%.6f\n", totalCost)
    fmt.Printf("Avg latency: %dms\n", totalLatency/len(entries))
    fmt.Printf("Cache hit rate: %.1f%%\n", 
        float64(cacheHits)/float64(len(entries))*100)
}
```

## Troubleshooting

### No History Collected

**Symptom:** MemoryCollector returns empty entries

**Diagnosis:**
```go
// Check if collector is configured
// Check if LM is created after Configure()
```

**Solution:**
```go
// Configure with collector BEFORE creating LM
dsgo.Configure(
    dsgo.WithCollector(collector),
)

// Create LM after configuration
lm := shared.GetLM(shared.GetModel())
```

### Missing Metadata Fields

**Symptom:** Expected metadata fields are nil

**Diagnosis:**
```go
// Different providers return different metadata
// Not all providers support all fields
```

**Solution:**
```go
// Always check if field exists before using
if reqID, ok := result.Metadata["request_id"].(string); ok {
    fmt.Printf("Request ID: %s\n", reqID)
} else {
    fmt.Println("Request ID not available")
}
```

### Stream Metrics Not Updating

**Symptom:** Stream entry shows 0 tokens

**Diagnosis:**
```go
// Metrics calculated after stream completes
// Check timing of inspection
```

**Solution:**
```go
// Wait for stream to complete before checking metrics
for chunk := range chunkChan {
    // Process chunks
}
err := <-errChan  // Wait for completion

// Now metrics are available
entries := collector.GetAll()
lastEntry := entries[len(entries)-1]
fmt.Printf("Tokens: %d\n", lastEntry.Usage.TotalTokens)
```

### JSONL File Not Created

**Symptom:** JSONLCollector doesn't create file

**Diagnosis:**
```go
// Check file path permissions
// Check if Close() is called
```

**Solution:**
```go
jsonlCollector, err := dsgo.NewJSONLCollector("logs/observability.jsonl")
if err != nil {
    log.Fatal(err)  // Will show permission error
}

// MUST call Close() to flush entries
defer jsonlCollector.Close()
```

## Performance Considerations

### Collector Overhead

**Impact:**
- MemoryCollector: ~1-2% overhead (JSON marshaling, struct copying)
- JSONLCollector: ~2-5% overhead (file I/O, JSON encoding)
- CompositeCollector: Combined overhead of all collectors

**Best practices:**
- Use MemoryCollector in development (minimal overhead)
- Use JSONLCollector in production (persistent logs)
- Set appropriate capacity (avoid excessive memory)
- Don't collect in performance-critical paths (optional)

### Memory Usage

**MemoryCollector:**
- Each entry: ~1-5 KB (depends on message size)
- Capacity 100: ~100-500 KB max
- Capacity 1000: ~1-5 MB max
- Ring buffer: Old entries automatically evicted

**Best practices:**
- Set capacity based on use case
- Development: 100-1000 entries
- Session tracking: 100-10000 entries
- Don't use unlimited capacity

### Best-Effort Collection

Observability is **best-effort** - it never fails calls:

```go
// If collection fails, call still succeeds
result, err := lm.Generate(ctx, messages, options)
// err is from Generate, not from collection
```

**Benefits:**
- Production safety (never breaks calls)
- Graceful degradation
- Silent failures for collectors
- Reliability over completeness

## Comparison with Alternatives

**vs. Manual tracking:**
- **Observability**: Automatic, complete, zero code
- **Manual**: More control, more code, error-prone

**vs. APM tools:**
- **Observability**: Built-in, LM-specific, free
- **APM**: More features, external service, cost

**vs. Logging:**
- **Observability**: Structured, queryable, complete
- **Logging**: Unstructured, text-based, incomplete

**vs. No observability:**
- **Observability**: Monitoring, debugging, analytics
- **No observability**: Faster, no insights, blind

## See Also

- [025_logging_tracing](../025_logging_tracing/) - Logging & tracing with Request ID
- [022_caching](../022_caching/) - LM cache for performance
- [023_global_config](../023_global_config/) - Global configuration system
- [024_lm_factory](../024_lm_factory/) - LM factory pattern
- [020_streaming](../020_streaming/) - Real-time streaming output
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide

## Production Tips

1. **Choose the Right Collector**: Memory for dev, JSONL for prod, Composite for both
2. **Set Appropriate Capacity**: Balance memory vs history retention
3. **Monitor Costs**: Use metadata to track API spending
4. **Track Performance**: Use latency metrics for optimization
5. **Analyze Cache**: Monitor cache hit rates for effectiveness
6. **Session Tracking**: Use session-specific collectors for user analytics
7. **Error Analysis**: Inspect Error metadata for debugging
8. **Rate Limit Monitoring**: Track rate limit headers to avoid throttling
9. **Periodic Analysis**: Build dashboards from JSONL files
10. **Clean Up Resources**: Always Close() file-based collectors

## Architecture Notes

Observability flow:

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Code                         │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  LMWrapper     │ ◄─── Optional wrapper
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │  LM.Generate() │
                    │  or Stream()   │
                    └────────┬───────┘
                             │
                ┌────────────┴────────────┐
                │                         │
                ▼                         ▼
       ┌────────────────┐      ┌──────────────────┐
       │ Provider API   │      │  HistoryEntry    │
       │ (OpenRouter,   │      │  Creation        │
       │  OpenAI, etc.) │      └────────┬─────────┘
       └────────┬───────┘               │
                │                       │
                ▼                       ▼
       ┌────────────────┐      ┌──────────────────┐
       │ GenerateResult │      │   Collector      │
       │  + Metadata    │      │  (Memory, JSONL, │
       └────────────────┘      │   Composite)     │
                               └──────────────────┘
```

**Design Principles:**
- **Non-invasive** - Observability doesn't change business logic
- **Automatic** - No manual instrumentation required
- **Complete** - All interaction types supported
- **Best-effort** - Never fails API calls
- **Flexible** - Multiple collector types for different needs
- **Production-ready** - Thread-safe, efficient, reliable
