# Observability Example

Comprehensive demonstration of DSGo's observability features across all interaction modes.

## What This Example Shows

This example demonstrates three key observability capabilities:

1. **Metadata Extraction** - Provider-specific metadata from API responses
2. **History Tracking** - Automatic collection of `HistoryEntry` records with `MemoryCollector`
3. **Streaming Observability** - Complete tracking for streaming LM calls

## Features Demonstrated

### 1. Provider Metadata Extraction

Extract rich metadata from provider responses:
- **Usage metrics**: Token counts, costs, latency
- **Cache information**: Cache hits, cache status
- **Request tracking**: Request IDs, generation IDs
- **Rate limits**: Limit headers, remaining quota, reset times
- **Finish reasons**: Completion status

Metadata is automatically extracted by providers and available in `GenerateResult.Metadata`.

### 2. History Tracking with MemoryCollector

Automatic tracking of all LM interactions:
- **HistoryEntry structure**: ID, timestamp, session ID, provider, model
- **Request metadata**: Messages, options, prompt length, tool info
- **Response metadata**: Content, tool calls, finish reason
- **Usage tracking**: Tokens, cost, latency (automatically calculated)
- **Cache tracking**: Hit/miss status, source, TTL
- **Provider metadata**: Request IDs, rate limits, custom headers

The `MemoryCollector` stores entries in a ring buffer for inspection and debugging.

### 3. Streaming Observability

Complete observability for streaming calls:
- **LMWrapper**: Transparently wraps streaming to collect metrics
- **Usage tracking**: Tokens and cost calculated after stream completes
- **Latency tracking**: Total time from start to finish
- **Error handling**: Stream errors properly captured in history
- **Parity**: Streaming produces identical observability data to Generate

## How It Works

```go
// 1. Create collector
collector := dsgo.NewMemoryCollector(100)

// 2. Configure with collector
dsgo.Configure(
    dsgo.WithProvider("openrouter"),
    dsgo.WithModel("google/gemini-2.0-flash-001:free"),
    dsgo.WithCollector(collector),
)

// 3. Create LM - automatically wrapped with observability
lm, err := dsgo.NewLM(ctx)

// 4. Make calls - metadata automatically collected
result, err := lm.Generate(ctx, messages, options)

// 5. Inspect collected history
entries := collector.(*dsgo.MemoryCollector).GetAll()
```

## Running the Example

```bash
# Set your API key
export OPENROUTER_API_KEY=your-key-here

# Run the example
cd examples/observability
go run main.go
```

## Demo Structure

### Demo 1: Metadata Extraction
- Makes a standard API call
- Displays response content
- Shows usage metrics (tokens, cost, latency)
- Extracts and displays provider-specific metadata
- Demonstrates cache status, request IDs, rate limits

### Demo 2: History Tracking
- Creates `MemoryCollector` for history storage
- Configures global settings with collector
- Makes multiple API calls
- Inspects collected `HistoryEntry` records
- Displays detailed metadata for each entry
- Saves sample entry to JSON file

### Demo 3: Streaming Observability
- Creates `LMWrapper` for automatic tracking
- Compares Generate vs Stream calls
- Shows that both produce complete observability data
- Demonstrates token/cost/latency tracking for streams
- Side-by-side comparison of metrics

## Expected Output

```
=== DSGo Observability Example ===

📊 Demo 1: Provider Metadata Extraction
──────────────────────────────────────────────────
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

📊 Demo 2: History Tracking & MemoryCollector
──────────────────────────────────────────────────
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

  ✓ Saved sample entry to observability_sample.json

📊 Demo 3: Streaming with Observability
──────────────────────────────────────────────────
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

=== All Demos Complete ===
```

## HistoryEntry Structure

Each collected entry contains:

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

## Collectors

### MemoryCollector
- Ring buffer with configurable capacity
- Thread-safe for concurrent access
- Useful for development and testing
- Methods: `GetAll()`, `GetLast(n)`, `Count()`, `Clear()`

### JSONLCollector
- Writes entries to JSON Lines file
- Suitable for production logging
- One entry per line for easy parsing

### CompositeCollector
- Sends entries to multiple collectors
- Combine memory + file logging
- Error handling for each collector

## Production Usage

```go
// Production example
jsonlCollector, err := dsgo.NewJSONLCollector("observability.jsonl")
if err != nil {
    log.Fatal(err)
}
defer jsonlCollector.Close()

dsgo.Configure(
    dsgo.WithProvider("openrouter"),
    dsgo.WithModel("google/gemini-2.0-flash-001:free"),
    dsgo.WithCollector(jsonlCollector),
)

lm, err := dsgo.NewLM(ctx)
// All interactions are now logged to observability.jsonl
```

## Key Takeaways

- ✅ **Automatic**: No manual instrumentation needed
- ✅ **Complete**: Full metadata for all interaction types
- ✅ **Flexible**: Multiple collector types (memory, file, composite)
- ✅ **Unified**: Generate and Stream produce identical data
- ✅ **Production-ready**: Best-effort collection (never fails calls)

## Related Examples

- `streaming/` - Basic streaming without observability
- `logging_tracing/` - Structured logging with request IDs
- `global_config/` - Global settings and configuration
- `caching/` - LRU caching with automatic tracking

## See Also

- [ROADMAP.md](../../ROADMAP.md) - Phase 4: Observability Parity
- [lm_wrapper.go](../../lm_wrapper.go) - LMWrapper implementation
- [history_entry.go](../../history_entry.go) - HistoryEntry structure
- [collector.go](../../collector.go) - Collector implementations
