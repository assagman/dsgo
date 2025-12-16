package retry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMaxRetries     = 3
	DefaultInitialBackoff = 1 * time.Second
	DefaultMaxBackoff     = 30 * time.Second
	DefaultJitterFactor   = 0.1
)

type Options struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	JitterFactor   float64
}

func DefaultOptions() *Options {
	return &Options{
		MaxRetries:     DefaultMaxRetries,
		InitialBackoff: DefaultInitialBackoff,
		MaxBackoff:     DefaultMaxBackoff,
		JitterFactor:   DefaultJitterFactor,
	}
}

func (o *Options) Copy() *Options {
	if o == nil {
		return DefaultOptions()
	}
	return &Options{
		MaxRetries:     o.MaxRetries,
		InitialBackoff: o.InitialBackoff,
		MaxBackoff:     o.MaxBackoff,
		JitterFactor:   o.JitterFactor,
	}
}

func NewOptions(maxRetries int, initialBackoff, maxBackoff time.Duration, jitterFactor float64) *Options {
	return &Options{
		MaxRetries:     maxRetries,
		InitialBackoff: initialBackoff,
		MaxBackoff:     maxBackoff,
		JitterFactor:   jitterFactor,
	}
}

// MergeFrom applies non-zero values from the provided overrides to this Options.
// This allows partial configuration where callers only override specific fields.
//
// NOTE: This method mutates the receiver. It is NOT safe for concurrent use on
// a shared *Options. Callers should use it only on freshly created instances
// (e.g., from DefaultOptions() or Copy()) before sharing.
func (o *Options) MergeFrom(maxRetries int, initialBackoff, maxBackoff time.Duration, jitterFactor float64) {
	if maxRetries > 0 {
		o.MaxRetries = maxRetries
	}
	if initialBackoff > 0 {
		o.InitialBackoff = initialBackoff
	}
	if maxBackoff > 0 {
		o.MaxBackoff = maxBackoff
	}
	if jitterFactor > 0 {
		o.JitterFactor = jitterFactor
	}
}

func IsRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || // 429
		statusCode == http.StatusInternalServerError || // 500
		statusCode == http.StatusBadGateway || // 502
		statusCode == http.StatusServiceUnavailable || // 503
		statusCode == http.StatusGatewayTimeout // 504
}

type HTTPFunc func() (*http.Response, error)

func WithExponentialBackoff(ctx context.Context, fn HTTPFunc) (*http.Response, error) {
	return WithExponentialBackoffOpts(ctx, fn, nil)
}

func WithExponentialBackoffOpts(ctx context.Context, fn HTTPFunc, opts *Options) (*http.Response, error) {
	if opts == nil {
		opts = DefaultOptions()
	} else {
		opts = opts.Copy()
	}

	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return nil, fmt.Errorf("context cancelled after retries: %w (last error: %v)", err, lastErr)
			}
			return nil, fmt.Errorf("context cancelled: %w", err)
		}

		resp, lastErr = fn()

		if lastErr == nil && resp != nil && !IsRetryable(resp.StatusCode) {
			return resp, nil
		}

		shouldRetry := false
		var retryAfter time.Duration

		if lastErr != nil {
			shouldRetry = true
		} else if resp != nil && IsRetryable(resp.StatusCode) {
			if isQuotaExhausted(resp) {
				return resp, nil
			}
			shouldRetry = true
			retryAfter = parseRetryAfter(resp.Header)
			_ = resp.Body.Close()
		}

		if !shouldRetry || attempt == opts.MaxRetries {
			if lastErr != nil {
				return nil, fmt.Errorf("request failed after %d attempts: %w", attempt+1, lastErr)
			}
			return resp, nil
		}

		backoff := calculateBackoffWithOpts(attempt, opts)
		if retryAfter > 0 && retryAfter < opts.MaxBackoff {
			backoff = retryAfter
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("context cancelled during backoff: %w (last error: %v)", ctx.Err(), lastErr)
			}
			return nil, fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", opts.MaxRetries+1, lastErr)
	}
	return resp, nil
}

func calculateBackoffWithOpts(attempt int, opts *Options) time.Duration {
	return calculateBackoff(attempt, opts)
}

func calculateBackoff(attempt int, opts *Options) time.Duration {
	if opts == nil {
		opts = DefaultOptions()
	}

	backoff := float64(opts.InitialBackoff) * math.Pow(2, float64(attempt))
	if backoff > float64(opts.MaxBackoff) {
		backoff = float64(opts.MaxBackoff)
	}

	if opts.JitterFactor != 0 {
		jitter := backoff * opts.JitterFactor * (2*rand.Float64() - 1)
		backoff += jitter
	}

	if backoff < 0 {
		backoff = 0
	}
	if backoff > float64(opts.MaxBackoff) {
		backoff = float64(opts.MaxBackoff)
	}

	return time.Duration(backoff)
}

func parseRetryAfter(header http.Header) time.Duration {
	value := header.Get("Retry-After")
	if value == "" {
		return 0
	}

	value = strings.TrimSpace(value)

	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	if t, err := http.ParseTime(value); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			return delay
		}
	}

	return 0
}

func isQuotaExhausted(resp *http.Response) bool {
	if resp.StatusCode != http.StatusTooManyRequests {
		return false
	}

	originalBody := resp.Body
	bodyBytes, err := io.ReadAll(originalBody)
	_ = originalBody.Close()

	if err != nil {
		return false
	}
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var errorResp struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &errorResp); err != nil {
		return false
	}

	return errorResp.Error.Code == "insufficient_quota" ||
		errorResp.Error.Type == "insufficient_quota" ||
		errorResp.Error.Code == "billing_hard_limit_reached"
}
