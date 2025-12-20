# DSGo Logging Package

Structured logging and request tracing for DSGo applications.

## Overview

The `logging` package provides:
- **Request ID Generation**: Auto-generated 16-character hex IDs using `crypto/rand`
- **Request ID Propagation**: Context-based tracking through the entire call chain
- **Structured Logging**: Key-value pairs for easy parsing
- **Configurable Levels**: DEBUG, INFO, WARN, ERROR, FATAL
- **Colored Console Output**: ANSI colors in text mode (auto on TTY)
- **Zero Dependencies**: Uses only Go standard library

## Installation

The logging package is part of DSGo and requires no additional installation:

```go
import "github.com/assagman/dsgo/logging"
```

## Quick Start

### Enable Logging

```go
import "github.com/assagman/dsgo/logging"

func main() {
    // Logging is enabled by default at WARN level
    // Optionally change level:
    logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))
    
    // Your DSGo code here...
}
```

### Color Output (Text Logs)

When `Format` is `text`, log levels can be colorized:

- DEBUG: Cyan
- INFO: Green
- WARN: Yellow
- ERROR/FATAL: Red

Control via environment variable `DSGO_LOG_COLOR`:

- `auto` (default): enable colors when stdout is a TTY and TERM is not "dumb"
- `always`: force ANSI color codes
- `never`: disable ANSI color codes

Auto behavior: Colors are enabled when all conditions are met:
1. Format is "text" (not "json")
2. Color mode is "auto"
3. stdout is a terminal (TTY)
4. TERM environment variable is not "dumb"

### Environment Configuration

`logging.ConfigureLoggerFromEnv()` reads the following variables:

| Variable | Purpose |
|---|---|
| `DSGO_LOG_LEVEL` | Logging level (`debug|info|warn|error|fatal`) |
| `DSGO_LOG_FORMAT` | `text` or `json` |
| `DSGO_LOG_COLOR` | `auto|always|never` (text format) |
| `DSGO_LOG_MODULE_LEVELS` | Per-module overrides (`module.Predict=debug,...`) |
| `DSGO_LOG_BUFFER_SIZE` | Async buffer size |
| `DSGO_LOG_FLUSH_INTERVAL` | Flush interval (e.g. `200ms`) |
| `DSGO_LOG_FLUSH_TIMEOUT` | Flush timeout (e.g. `1s`) |
| `DSGO_LOG_BATCH_SIZE` | Max batch size per flush |
| `DSGO_LOG_DROP_WHEN_FULL` | Drop logs when buffer full (`1`/`true`) |
| `DSGO_LOG_MAX_MEMORY` | Max async buffer memory (bytes) |
| `DSGO_LOG_CACHE_SLOW_THRESHOLD` | Cache slow-log threshold |
| `DSGO_LOG_TOOL_SLOW_THRESHOLD` | Tool slow-log threshold |
| `DSGO_LOG_BLOCK_TIMEOUT` | Max block time when buffer full |

### Use Auto-Generated Request IDs

```go
ctx := context.Background()
result, err := module.Forward(ctx, inputs)
// Request ID is automatically generated and logged
```

### Use Custom Request IDs

```go
ctx := logging.WithRequestID(context.Background(), "user-123-order-456")
result, err := module.Forward(ctx, inputs)
// All logs will show [user-123-order-456]
```

## Request ID Design

### Why Not UUID?

We use **8 random bytes** (16 hex characters) instead of full UUID (36 characters) for:
1. **Conciseness**: Shorter logs, easier to read
2. **Sufficient Entropy**: 2^64 possible IDs (18 quintillion)
3. **Cryptographic Quality**: Uses `crypto/rand`, not `math/rand`

**Collision Probability**: With 1 billion requests, probability of collision is ~0.000000003%

### Format Comparison

```
UUID:    550e8400-e29b-41d4-a716-446655440000  (36 chars)
Our ID:  016e63bd1d0aefd2                      (16 chars)
```

Both are unique enough for request tracking. Our format is more compact.

## API Reference

### Types

```go
type Level int

const (
    LevelDebug Level = iota  // Show everything
    LevelInfo                // API calls only
    LevelWarn                // Warnings and errors
    LevelError               // Errors only
)

type Logger interface {
    Debug(ctx context.Context, msg string, fields map[string]any)
    Info(ctx context.Context, msg string, fields map[string]any)
    Warn(ctx context.Context, msg string, fields map[string]any)
    Error(ctx context.Context, msg string, fields map[string]any)
}
```

### Functions

#### Logger Management

```go
// SetLogger sets the global logger (nil = NoOpLogger)
func SetLogger(logger Logger)

// GetLogger returns the current global logger
func GetLogger() Logger

// NewDefaultLogger creates a logger that writes to stdout
func NewDefaultLogger(level Level) *DefaultLogger
```

#### Request ID Functions

```go
// GenerateRequestID generates a unique 16-character hex ID
func GenerateRequestID() string

// WithRequestID attaches a Request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context

// GetRequestID retrieves the Request ID from the context
func GetRequestID(ctx context.Context) string

// EnsureRequestID ensures context has a Request ID (creates if missing)
func EnsureRequestID(ctx context.Context) context.Context
```

#### Helper Functions

```go
// LogAPIRequest logs the start of an API request
func LogAPIRequest(ctx context.Context, provider string, model string, promptLength int)

// LogAPIResponse logs the completion of an API request
func LogAPIResponse(ctx context.Context, provider string, model string, statusCode int, duration time.Duration, usage core.Usage)

// LogAPIError logs an API error
func LogAPIError(ctx context.Context, provider string, model string, err error)

// LogPredictionStart logs the start of a prediction
func LogPredictionStart(ctx context.Context, moduleName string, signature string)

// LogPredictionEnd logs the end of a prediction
func LogPredictionEnd(ctx context.Context, moduleName string, duration time.Duration, err error)
```

## Usage Examples

### Basic Logging

```go
import "github.com/assagman/dsgo/logging"

// Enable logging
logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))

// Use DSGo as normal
ctx := context.Background()
result, err := predict.Forward(ctx, inputs)
```

### Custom Request ID for User Sessions

```go
func handleUserRequest(userID string, sessionID string) {
    requestID := fmt.Sprintf("user-%s-session-%s", userID, sessionID)
    ctx := logging.WithRequestID(context.Background(), requestID)
    
    // All API calls in this function share the same Request ID
    result1, _ := module1.Forward(ctx, inputs1)
    result2, _ := module2.Forward(ctx, inputs2)
}
```

### Batch Processing

```go
func processBatch(batchID string, items []Item) {
    ctx := logging.WithRequestID(context.Background(), "batch-"+batchID)
    
    for i, item := range items {
        result, err := module.Forward(ctx, map[string]any{"text": item.Text})
        // All items in batch share the Request ID for easy tracing
    }
}
```

### Different Log Levels

```go
// Default: WARN and ERROR only (enabled automatically)
// No SetLogger() needed for basic logging

// Development: See everything
logging.SetLogger(logging.NewDefaultLogger(logging.LevelDebug))

// Production: INFO, WARN, and ERROR
logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))

// High-volume production: Errors only
logging.SetLogger(logging.NewDefaultLogger(logging.LevelError))

// Testing: Disable logging
logging.SetLogger(nil)  // or &logging.NoOpLogger{}
```

## Custom Logger Implementation

Implement the `Logger` interface for integration with structured logging libraries:

```go
import (
    "go.uber.org/zap"
    "github.com/assagman/dsgo/logging"
)

type ZapLogger struct {
    logger *zap.Logger
}

func (z *ZapLogger) Info(ctx context.Context, msg string, fields map[string]any) {
    zapFields := []zap.Field{
        zap.String("request_id", logging.GetRequestID(ctx)),
    }
    for k, v := range fields {
        zapFields = append(zapFields, zap.Any(k, v))
    }
    z.logger.Info(msg, zapFields...)
}

// Implement Debug, Warn, Error...

// Use it
zapLogger := &ZapLogger{logger: zap.NewProduction()}
logging.SetLogger(zapLogger)
```

## Log Format

```
[Prefix] Timestamp [Level] [RequestID] Message | key=value ...
```

Example:
```
[DSGo] 2025-10-31 00:24:07.625 [INFO] [016e63bd1d0aefd2] API request started | model=gpt-4 prompt_length=367
```

## Performance

- **NoOpLogger**: Zero overhead (when explicitly set)
- **DefaultLogger**: ~1-2µs per log line (default at WARN level)
- **Request ID**: 8 bytes allocation, cryptographically random
- **Context Propagation**: No copying, uses Go's context.Value

## Thread Safety & Resource Management

### Thread Safety

- `SetLogger()` and `GetLogger()` are **not thread-safe**. Set the logger once at startup.
- `Logger` interface methods **must be thread-safe** (DefaultLogger is thread-safe via stdout writes)
- Request IDs in context are **immutable** (safe to share across goroutines)

### Resource Cleanup

`DefaultLogger` runs a background goroutine for async log processing. To prevent resource leaks:

**Option 1: Automatic cleanup when replacing logger**
```go
oldLogger := logging.NewDefaultLogger(logging.LevelInfo)
logging.SetLogger(oldLogger)

// ... later ...

newLogger := logging.NewDefaultLogger(logging.LevelDebug)
logging.SetLogger(newLogger)  // Automatically stops oldLogger
```

**Option 2: Explicit cleanup**
```go
logger := logging.NewDefaultLogger(logging.LevelInfo)
defer logger.Stop()  // Always call Stop() before exit

// Use logger...
```

**Option 3: Global logger (production)**
```go
func init() {
    logging.ConfigureLoggerFromEnv()  // Registers cleanup handler
}

func main() {
    defer logging.StopLogger()  // Stop global logger on exit
    // Rest of application...
}
```

**Important**: Calling `Stop()` is idempotent (safe to call multiple times)

## Best Practices

1. **Set logger once at startup**, not during request handling
2. **Use custom Request IDs** for multi-step workflows
3. **Start with LevelInfo** in production, increase to DEBUG only when debugging
4. **Don't log sensitive data** in fields (API keys, passwords, etc.)
5. **Implement custom Logger** for production logging infrastructure

## See Also

- [LOGGING_IMPLEMENTATION.md](../LOGGING_IMPLEMENTATION.md) - Implementation details
