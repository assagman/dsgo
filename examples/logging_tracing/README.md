# Logging & Tracing Example

This example demonstrates DSGo's built-in logging and tracing capabilities with Request ID propagation for observability.

## Features Demonstrated

1. **Automatic Request ID Generation** - DSGo automatically generates unique Request IDs for each prediction
2. **Custom Request IDs** - You can set your own Request IDs for correlation across multiple calls
3. **Request ID Propagation** - Request IDs are propagated through the entire call chain (module → provider → API)
4. **Structured Logging** - All logs include contextual information (model, tokens, duration, etc.)
5. **Configurable Log Levels** - Control verbosity with DEBUG, INFO, WARN, ERROR levels

## What Gets Logged

### At INFO Level:
- **API Request Start**: Model name, prompt length, Request ID
- **API Response**: Status code, duration, token usage (prompt/completion/total)
- **API Errors**: Error messages with Request ID

### At DEBUG Level (includes INFO + DEBUG):
- **Prediction Start**: Module name, signature description
- **Prediction End**: Duration, success/failure status

## Log Format

```
[DSGo] 2025-10-31 00:16:36.155 [INFO] [cdc1e21eef392ce6] API request started | model=gpt-4 prompt_length=100
[DSGo] 2025-10-31 00:16:36.655 [INFO] [cdc1e21eef392ce6] API request completed | status_code=200 duration_ms=500 prompt_tokens=100 completion_tokens=50 total_tokens=150
```

Format: `[Prefix] Timestamp [Level] [RequestID] Message | key=value ...`

## Usage

### Enable Logging

By default, logging is disabled (uses `NoOpLogger`). Enable it with:

```go
import "github.com/assagman/dsgo/logging"

// Set log level (LevelDebug, LevelInfo, LevelWarn, LevelError)
logging.SetLogger(logging.NewDefaultLogger(logging.LevelInfo))
```

### Automatic Request ID

Request IDs are automatically generated for each `Forward()` or `Stream()` call:

```go
ctx := context.Background()
result, err := module.Forward(ctx, inputs)
// Logs will show an auto-generated Request ID like [a1b2c3d4e5f6g7h8]
```

### Custom Request ID

Set your own Request ID for correlation:

```go
import "github.com/assagman/dsgo/logging"

ctx := logging.WithRequestID(context.Background(), "user-request-12345")
result, err := module.Forward(ctx, inputs)
// All logs will show [user-request-12345]
```

### Multiple Calls with Same Request ID

Use the same Request ID for related operations:

```go
requestID := "batch-job-001"
ctx := logging.WithRequestID(context.Background(), requestID)

for _, item := range items {
    result, err := module.Forward(ctx, map[string]any{"text": item})
    // All calls share the same Request ID for easy tracing
}
```

## Running the Example

```bash
# From repository root
go run examples/logging_tracing/main.go
```

## Use Cases

1. **Debugging Production Issues** - Trace specific requests through your system
2. **Performance Monitoring** - Track API latency and token usage
3. **Cost Tracking** - Monitor token consumption per request
4. **Distributed Tracing** - Correlate logs across services using Request IDs
5. **API Rate Limiting** - Identify which requests trigger rate limits

## Log Levels

- **DEBUG**: All logs (prediction flow + API calls)
- **INFO**: API request/response logs only
- **WARN**: Warnings and errors only
- **ERROR**: Errors only

## Disabling Logging

```go
logging.SetLogger(nil) // Reverts to NoOpLogger
```

## Integration with External Logging

You can implement your own logger by satisfying the `logging.Logger` interface:

```go
type Logger interface {
    Debug(ctx context.Context, msg string, fields map[string]any)
    Info(ctx context.Context, msg string, fields map[string]any)
    Warn(ctx context.Context, msg string, fields map[string]any)
    Error(ctx context.Context, msg string, fields map[string]any)
}
```

Then set it as the global logger:

```go
logging.SetLogger(myCustomLogger)
```

This allows integration with structured logging libraries like zap, zerolog, or logrus.
