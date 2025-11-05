# 022_caching - LM Cache Performance

## Overview

Demonstrates LM caching for performance optimization and cost reduction. Shows cache hits, misses, and performance metrics.

## What it demonstrates

- LMCache setup and configuration
- Cache hit vs miss behavior
- Performance improvements from caching
- Cache statistics tracking
- Token savings calculation

## Usage

```bash
cd examples/022_caching
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
```

## Expected Output

```
--- Request 1: First Translation (Cache Miss) ---
Translation: Hola, ¿cómo estás?
Tokens: 150

--- Request 2: Same Translation (Cache Hit) ---
Translation: Hola, ¿cómo estás?
Tokens: 150 (cached)

--- Cache Statistics ---
Cache Hits: 1
Cache Misses: 1
Hit Rate: 50.0%
```

## Key Concepts

- **LMCache**: In-memory cache for identical requests
- **Cache Key**: Generated from model, messages, and all parameters
- **LRU Eviction**: Oldest entries removed when cache is full
- **Deep Copying**: Prevents mutation of cached results

## Performance Benefits

- **Speed**: 10-100x faster for cache hits
- **Cost**: 50%+ token savings on repeated queries
- **Thread-safe**: Safe for concurrent use

## See Also

- [001_predict](../001_predict/) - Basic prediction
- [README.md](../../README.md) - LMCache documentation
