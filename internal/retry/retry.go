package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

const (
	MaxRetries     = 3
	InitialBackoff = 1 * time.Second
	MaxBackoff     = 30 * time.Second
	JitterFactor   = 0.1
)

// IsRetryable checks if an HTTP status code is retryable
func IsRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || // 429
		statusCode == http.StatusInternalServerError || // 500
		statusCode == http.StatusBadGateway || // 502
		statusCode == http.StatusServiceUnavailable || // 503
		statusCode == http.StatusGatewayTimeout // 504
}

// HTTPFunc is a function that performs an HTTP request
type HTTPFunc func() (*http.Response, error)

// WithExponentialBackoff executes an HTTP request with exponential backoff retry logic
func WithExponentialBackoff(ctx context.Context, fn HTTPFunc) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return nil, fmt.Errorf("context cancelled after retries: %w (last error: %v)", err, lastErr)
			}
			return nil, fmt.Errorf("context cancelled: %w", err)
		}

		// Execute the HTTP request
		resp, lastErr = fn()

		// Success - return immediately
		if lastErr == nil && resp != nil && !IsRetryable(resp.StatusCode) {
			return resp, nil
		}

		// Determine if we should retry
		shouldRetry := false
		if lastErr != nil {
			// Network error - retry
			shouldRetry = true
		} else if resp != nil && IsRetryable(resp.StatusCode) {
			// Retryable status code
			shouldRetry = true
			// Close the body to reuse connection
			_ = resp.Body.Close()
		}

		// Don't retry if this was the last attempt
		if !shouldRetry || attempt == MaxRetries {
			if lastErr != nil {
				return nil, fmt.Errorf("request failed after %d attempts: %w", attempt+1, lastErr)
			}
			return resp, nil
		}

		// Calculate backoff with exponential growth and jitter
		backoff := calculateBackoff(attempt)

		// Wait with context awareness
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("context cancelled during backoff: %w (last error: %v)", ctx.Err(), lastErr)
			}
			return nil, fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next retry
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", MaxRetries+1, lastErr)
	}
	return resp, nil
}

// calculateBackoff computes exponential backoff with jitter
func calculateBackoff(attempt int) time.Duration {
	// Exponential: initialBackoff * 2^attempt
	backoff := float64(InitialBackoff) * math.Pow(2, float64(attempt))

	// Cap at MaxBackoff
	if backoff > float64(MaxBackoff) {
		backoff = float64(MaxBackoff)
	}

	// Add jitter: ±10% randomness
	jitter := backoff * JitterFactor * (2*rand.Float64() - 1)
	backoff += jitter

	return time.Duration(backoff)
}
