# Telemetry Demo: Metadata Persistence (Phase 4.1)

This example demonstrates the **Phase 4.1: Metadata Persistence** feature, which ensures that critical metadata extracted by providers is correctly persisted in history entries.

## What This Demo Shows

### 1. Provider Metadata Collection
The example shows how DSGo now captures provider-specific metadata including:
- **Request IDs**: Unique identifiers for API calls
- **Rate Limits**: Remaining quota and reset times
- **Cache Headers**: Cache hit/miss status from provider
- **Custom Headers**: Any other provider-specific metadata

### 2. Cache Hit Tracking
- Automatically detects cache hits from provider metadata
- Supports both `cache_status: "hit"` (string) and `cache_hit: true` (boolean) formats
- Sets `Cache.Hit` and `Cache.Source` in HistoryEntry

### 3. Provider Name Resolution
- Uses `settings.DefaultProvider` when configured globally
- Falls back to model name heuristics (gpt-4 → openai, claude → anthropic, etc.)
- Provides consistent provider identification for telemetry

## How It Works

```go
import (
    "github.com/assagman/dsgo"
    _ "github.com/assagman/dsgo/providers/openrouter" // Register provider
)

// 1. Create collector to capture telemetry
collector := dsgo.NewMemoryCollector(100)

// 2. Configure global settings with collector
dsgo.Configure(
    dsgo.WithProvider("openrouter"),
    dsgo.WithModel("google/gemini-2.0-flash-exp:free"),
    dsgo.WithCollector(collector),
)

// 3. Create LM - automatically wrapped with telemetry
lm, err := dsgo.NewLM(ctx)

// 4. Make API calls - metadata is automatically collected
result, err := lm.Generate(ctx, messages, options)

// 5. Access enriched history entries
entries := collector.GetAll()
for _, entry := range entries {
    // Provider name from settings
    fmt.Println(entry.Provider) // "openrouter"
    
    // Cache hit status from metadata
    fmt.Println(entry.Cache.Hit) // true/false
    
    // Provider-specific metadata
    fmt.Println(entry.ProviderMeta["x-request-id"])
    fmt.Println(entry.ProviderMeta["x-ratelimit-remaining"])
}
```

## Running the Example

```bash
# Set your API key
export OPENROUTER_API_KEY=your-key-here

# Run the example
cd examples/telemetry_demo
go run main.go
```

**Note**: The example imports `_ "github.com/assagman/dsgo/providers/openrouter"` to register the OpenRouter provider. This blank import is required for provider auto-registration.

## Expected Output

```
=== DSGo Telemetry Demo: Metadata Persistence ===

1. Making API call with metadata collection...
   Response: Hello from DSGo!

2. Making second call (check for cache hit)...
   Response: Hello from DSGo!

3. Inspecting collected telemetry...

   Collected 2 history entries

   --- Entry 1 ---
   ID:          abc-123-def-456
   Timestamp:   2025-11-04 12:00:00
   SessionID:   session-xyz
   Provider:    openrouter (from global settings)
   Model:       google/gemini-2.0-flash-exp:free

   Usage:
     Prompt tokens:     15
     Completion tokens: 5
     Total tokens:      20
     Cost (USD):        $0.000000
     Latency (ms):      450

   Cache:
     Hit:    false

   Provider Metadata (NEW):
     x-request-id: req_abc123xyz
     x-ratelimit-limit: 100
     x-ratelimit-remaining: 99
     x-ratelimit-reset: 1699999999

   Response:
     Content:      Hello from DSGo!
     Finish:       stop
     Length:       17 chars

   --- Entry 2 ---
   [Similar output, possibly with Cache.Hit: true]

   ✓ Saved to telemetry_sample.json
```

## Key Features Demonstrated

### ✅ Automatic Metadata Capture
All provider metadata is automatically transferred from `GenerateResult.Metadata` to `HistoryEntry.ProviderMeta`.

### ✅ Smart Cache Detection
Cache hits are detected from multiple formats:
- `cache_status: "hit"` → sets `Cache.Hit = true`
- `cache_hit: true` → sets `Cache.Hit = true`
- `cache_status: "miss"` → sets `Cache.Hit = false`

### ✅ Flexible Provider Naming
- **Primary**: Uses `settings.DefaultProvider` when set
- **Fallback**: Extracts from model name (gpt-4 → openai, gemini → google, etc.)

### ✅ Complete Observability
Every API call is tracked with:
- Request metadata (messages, options, tools)
- Response metadata (content, finish reason, tool calls)
- Usage tracking (tokens, cost, latency)
- Cache information
- Provider-specific headers and metadata

## Use Cases

1. **Cost Tracking**: Monitor token usage and costs across providers
2. **Performance Analysis**: Track latency and cache hit rates
3. **Rate Limit Management**: Monitor remaining quota and avoid throttling
4. **Debugging**: Inspect request IDs for support tickets
5. **Compliance**: Audit all LM interactions with full metadata

## Integration with Collectors

The example uses `MemoryCollector` for demonstration, but you can use:

- **`JSONLCollector`**: Write to JSONL files for production logging
- **`CompositeCollector`**: Send to multiple destinations simultaneously
- **Custom Collectors**: Implement the `Collector` interface for your needs

```go
// Production example
jsonlCollector := dsgo.NewJSONLCollector("telemetry.jsonl")
defer jsonlCollector.Close()

dsgo.Configure(
    dsgo.WithProvider("openrouter"),
    dsgo.WithModel("google/gemini-2.0-flash-exp:free"),
    dsgo.WithCollector(jsonlCollector),
)

lm, err := dsgo.NewLM(ctx)
// All metadata is now logged to telemetry.jsonl
```

## Next Steps

See ROADMAP.md Phase 4 for upcoming telemetry improvements:
- **4.2**: Streaming telemetry
- **4.3**: Enhanced cache key fidelity
- **4.4**: Provider vs vendor naming
