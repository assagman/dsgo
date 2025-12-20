# internal/retry

HTTP retry helpers with exponential backoff.

## Overview

- `WithExponentialBackoff` / `WithExponentialBackoffOpts` wrap an HTTP function
  and retry on retryable failures.
- `IsRetryable` covers 429 and common 5xx responses.
- Honors `Retry-After` headers when present.
- Stops early for quota exhaustion errors detected in 429 responses.

## Options

```go
opts := retry.NewOptions(
    retry.DefaultMaxRetries,
    retry.DefaultInitialBackoff,
    retry.DefaultMaxBackoff,
    retry.DefaultJitterFactor,
)
```

## Usage

```go
resp, err := retry.WithExponentialBackoff(ctx, func() (*http.Response, error) {
    return client.Do(req)
})
```
