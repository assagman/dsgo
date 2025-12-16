package retry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"200 OK", http.StatusOK, false},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"404 Not Found", http.StatusNotFound, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetryable(tt.statusCode); got != tt.want {
				t.Errorf("IsRetryable(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestWithExponentialBackoff_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	resp, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		return client.Get(server.URL)
	})

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestWithExponentialBackoff_RetryOn429(t *testing.T) {
	t.Parallel()
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	resp, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		return client.Get(server.URL)
	})

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
	_ = resp.Body.Close()
}

func TestWithExponentialBackoff_NetworkError(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	resp, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		if callCount <= 2 {
			return nil, errors.New("network error")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
		}, nil
	})

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestWithExponentialBackoff_MaxRetriesExceeded(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	_, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		return nil, errors.New("persistent error")
	})

	if err == nil {
		t.Fatal("Expected error after max retries")
	}
	if callCount != DefaultMaxRetries+1 {
		t.Errorf("Expected %d calls (initial + %d retries), got %d", DefaultMaxRetries+1, DefaultMaxRetries, callCount)
	}
}

func TestWithExponentialBackoff_ContextCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		time.Sleep(100 * time.Millisecond)
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	if err == nil {
		t.Fatal("Expected error due to canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestWithExponentialBackoff_ContextTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callCount := 0
	_, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		time.Sleep(100 * time.Millisecond)
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       http.NoBody,
		}, nil
	})

	if err == nil {
		t.Fatal("Expected error due to context timeout")
	}
	// We expect fewer calls due to timeout
	if callCount > 2 {
		t.Errorf("Expected at most 2 calls due to timeout, got %d", callCount)
	}
}

func TestWithExponentialBackoff_Non200Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &http.Client{}
	ctx := context.Background()

	resp, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		return client.Get(server.URL)
	})

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{0, 500 * time.Millisecond, 2 * time.Second},                               // 1s ± jitter
		{1, 1 * time.Second, 3 * time.Second},                                      // 2s ± jitter
		{2, 2 * time.Second, 6 * time.Second},                                      // 4s ± jitter
		{3, 4 * time.Second, 12 * time.Second},                                     // 8s ± jitter
		{10, DefaultMaxBackoff - 5*time.Second, DefaultMaxBackoff + 5*time.Second}, // capped at MaxBackoff
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			backoff := calculateBackoff(tt.attempt, nil)
			if backoff < tt.minExpected || backoff > tt.maxExpected {
				t.Errorf("calculateBackoff(%d) = %v, want between %v and %v",
					tt.attempt, backoff, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestWithExponentialBackoff_MixedErrors(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	resp, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("network error")
		}
		if callCount == 2 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       http.NoBody,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
		}, nil
	})

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestIsQuotaExhausted_InsufficientQuota(t *testing.T) {
	t.Parallel()
	body := `{"error":{"code":"insufficient_quota","message":"You exceeded your current quota"}}`
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	if !isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return true for insufficient_quota code")
	}
}

func TestIsQuotaExhausted_BillingHardLimit(t *testing.T) {
	t.Parallel()
	body := `{"error":{"code":"billing_hard_limit_reached","message":"Billing limit reached"}}`
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	if !isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return true for billing_hard_limit_reached")
	}
}

func TestIsQuotaExhausted_TypeInsufficientQuota(t *testing.T) {
	t.Parallel()
	body := `{"error":{"type":"insufficient_quota","message":"Quota exceeded"}}`
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	if !isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return true for type=insufficient_quota")
	}
}

func TestIsQuotaExhausted_RateLimitNotQuota(t *testing.T) {
	t.Parallel()
	body := `{"error":{"code":"rate_limit_exceeded","message":"Rate limit"}}`
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	if isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return false for rate_limit_exceeded")
	}
}

func TestIsQuotaExhausted_Non429Status(t *testing.T) {
	t.Parallel()
	body := `{"error":{"code":"insufficient_quota"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	if isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return false for non-429 status")
	}
}

func TestIsQuotaExhausted_InvalidJSON(t *testing.T) {
	t.Parallel()
	body := `not valid json`
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	if isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return false for invalid JSON")
	}
}

func TestIsQuotaExhausted_EmptyBody(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}

	if isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return false for empty body")
	}
}

func TestWithExponentialBackoff_QuotaExhausted(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	body := `{"error":{"code":"insufficient_quota","message":"Quota exceeded"}}`
	resp, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	})

	if err != nil {
		t.Fatalf("Expected no error for quota exhaustion, got: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("Expected only 1 call (no retries for quota exhaustion), got %d", callCount)
	}
	_ = resp.Body.Close()
}

func TestWithExponentialBackoff_ContextCanceledAfterRetries(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx, cancel := context.WithCancel(context.Background())

	_, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		if callCount == 2 {
			cancel() // Cancel after first retry
		}
		return nil, errors.New("network error")
	})

	if err == nil {
		t.Fatal("Expected error due to canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// TestWithExponentialBackoff_ContextCancelledAfterRetriesWithLastErr covers line 42-44
// Tests context cancellation at start of loop iteration after fn() has failed once (lastErr is set)
func TestWithExponentialBackoff_ContextCancelledAfterRetriesWithLastErr(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after backoff completes but before next loop iteration
	go func() {
		// First backoff is ~1s, cancel slightly after that
		time.Sleep(1100 * time.Millisecond)
		cancel()
	}()

	_, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		// Always fail to ensure lastErr is set
		return nil, errors.New("network error")
	})

	if err == nil {
		t.Fatal("Expected error due to canceled context after retries")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
	// Should contain both context cancellation and last error
	errStr := err.Error()
	// Line 42-44 returns "context cancelled after retries" when ctx.Err() at loop start and lastErr != nil
	// Line 87-89 returns "context cancelled during backoff" when cancelled during backoff
	if !bytes.Contains([]byte(errStr), []byte("context cancelled after retries")) &&
		!bytes.Contains([]byte(errStr), []byte("context cancelled during backoff")) {
		t.Errorf("Expected error to contain context cancellation message, got: %v", err)
	}
	if !bytes.Contains([]byte(errStr), []byte("last error")) {
		t.Errorf("Expected error to contain 'last error', got: %v", err)
	}
}

// TestWithExponentialBackoff_LastAttemptRetryable covers line 78
// Tests that when last attempt returns retryable status, we return resp without error
func TestWithExponentialBackoff_LastAttemptRetryable(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	// Always return 503 (retryable) - not quota exhaustion
	resp, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       http.NoBody,
		}, nil
	})

	// Should not error - returns 503 response
	if err != nil {
		t.Errorf("Expected no error on last retryable attempt, got: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response to be returned")
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}
	// Should have tried DefaultMaxRetries+1 times
	if callCount != DefaultMaxRetries+1 {
		t.Errorf("Expected %d calls, got %d", DefaultMaxRetries+1, callCount)
	}
}

// errorReader is an io.Reader that always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (e *errorReader) Close() error {
	return nil
}

// TestIsQuotaExhausted_ReadError covers line 128-130
// Tests that isQuotaExhausted returns false when io.ReadAll fails
func TestIsQuotaExhausted_ReadError(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       &errorReader{},
	}

	if isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return false when body read fails")
	}
}

func TestWithExponentialBackoff_ContextCanceledDuringBackoff(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       http.NoBody,
		}, nil
	})

	if err == nil {
		t.Fatal("Expected error due to context timeout during backoff")
	}
	// Should only make 1-2 calls before timing out during backoff
	if callCount > 2 {
		t.Errorf("Expected at most 2 calls, got %d", callCount)
	}
}

// BenchmarkRetryLogic_RateLimit tests retry logic under sustained rate limiting
// Simulates thousands of requests hitting continuous 429 responses
func BenchmarkRetryLogic_RateLimit(b *testing.B) {
	ctx := context.Background()
	rateLimitCount := 0

	// Simulate sustained rate limiting - always return 429
	fn := func() (*http.Response, error) {
		rateLimitCount++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"code":"rate_limit_exceeded"}}`)),
		}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := WithExponentialBackoff(ctx, fn)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	b.ReportMetric(float64(rateLimitCount)/float64(b.N), "retries/op")
}

// BenchmarkRetryLogic_SporadicFailures tests retry logic with intermittent failures
func BenchmarkRetryLogic_SporadicFailures(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attemptCount := 0
		fn := func() (*http.Response, error) {
			attemptCount++
			// Fail first 2 attempts, succeed on 3rd
			if attemptCount <= 2 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       http.NoBody,
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
			}, nil
		}

		resp, _ := WithExponentialBackoff(ctx, fn)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
}

// BenchmarkRetryLogic_ImmediateSuccess tests baseline performance with no retries
func BenchmarkRetryLogic_ImmediateSuccess(b *testing.B) {
	ctx := context.Background()

	fn := func() (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
		}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := WithExponentialBackoff(ctx, fn)
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
}

// BenchmarkRetryLogic_NetworkErrors tests retry logic with persistent network errors
func BenchmarkRetryLogic_NetworkErrors(b *testing.B) {
	ctx := context.Background()

	fn := func() (*http.Response, error) {
		return nil, errors.New("network connection refused")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = WithExponentialBackoff(ctx, fn)
	}
}

// BenchmarkRetryLogic_Concurrent tests retry logic under concurrent load
func BenchmarkRetryLogic_Concurrent(b *testing.B) {
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		attemptNum := 0
		for pb.Next() {
			attemptNum++
			localAttempt := attemptNum
			fn := func() (*http.Response, error) {
				// Vary behavior: some succeed, some fail transiently
				if localAttempt%3 == 0 {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       http.NoBody,
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"code":"rate_limit_exceeded"}}`)),
				}, nil
			}

			resp, _ := WithExponentialBackoff(ctx, fn)
			if resp != nil {
				_ = resp.Body.Close()
			}
		}
	})
}

func TestParseRetryAfter_Seconds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{"empty", "", 0},
		{"5 seconds", "5", 5 * time.Second},
		{"30 seconds", "30", 30 * time.Second},
		{"120 seconds", "120", 120 * time.Second},
		{"negative", "-5", 0},
		{"zero", "0", 0},
		{"whitespace", "  10  ", 10 * time.Second},
		{"invalid", "abc", 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			header := http.Header{}
			if tt.value != "" {
				header.Set("Retry-After", tt.value)
			}
			got := parseRetryAfter(header)
			if got != tt.expected {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.expected)
			}
		})
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	t.Parallel()
	futureTime := time.Now().Add(10 * time.Second).UTC()
	header := http.Header{}
	header.Set("Retry-After", futureTime.Format(http.TimeFormat))

	got := parseRetryAfter(header)
	if got < 8*time.Second || got > 12*time.Second {
		t.Errorf("parseRetryAfter(HTTP date) = %v, want ~10s", got)
	}
}

func TestParseRetryAfter_PastDate(t *testing.T) {
	t.Parallel()
	pastTime := time.Now().Add(-10 * time.Second).UTC()
	header := http.Header{}
	header.Set("Retry-After", pastTime.Format(http.TimeFormat))

	got := parseRetryAfter(header)
	if got != 0 {
		t.Errorf("parseRetryAfter(past date) = %v, want 0", got)
	}
}

func TestDefaultOptions(t *testing.T) {
	t.Parallel()
	opts := DefaultOptions()

	if opts.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", opts.MaxRetries, DefaultMaxRetries)
	}
	if opts.InitialBackoff != DefaultInitialBackoff {
		t.Errorf("InitialBackoff = %v, want %v", opts.InitialBackoff, DefaultInitialBackoff)
	}
	if opts.MaxBackoff != DefaultMaxBackoff {
		t.Errorf("MaxBackoff = %v, want %v", opts.MaxBackoff, DefaultMaxBackoff)
	}
	if opts.JitterFactor != DefaultJitterFactor {
		t.Errorf("JitterFactor = %v, want %v", opts.JitterFactor, DefaultJitterFactor)
	}
}

func TestOptions_Copy(t *testing.T) {
	t.Parallel()
	original := &Options{
		MaxRetries:     5,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     60 * time.Second,
		JitterFactor:   0.2,
	}

	copied := original.Copy()

	if copied.MaxRetries != original.MaxRetries {
		t.Errorf("copied.MaxRetries = %d, want %d", copied.MaxRetries, original.MaxRetries)
	}
	if copied.InitialBackoff != original.InitialBackoff {
		t.Errorf("copied.InitialBackoff = %v, want %v", copied.InitialBackoff, original.InitialBackoff)
	}

	original.MaxRetries = 10
	if copied.MaxRetries == original.MaxRetries {
		t.Error("Copy should be independent of original")
	}
}

func TestOptions_Copy_Nil(t *testing.T) {
	t.Parallel()
	var nilOpts *Options
	copied := nilOpts.Copy()

	if copied == nil {
		t.Fatal("Copy of nil should return default options")
	}
	if copied.MaxRetries != DefaultMaxRetries {
		t.Errorf("nil copy should have default MaxRetries, got %d", copied.MaxRetries)
	}
}

func TestWithExponentialBackoffOpts_CustomMaxRetries(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	opts := &Options{
		MaxRetries:     1,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		JitterFactor:   0.0,
	}

	_, err := WithExponentialBackoffOpts(ctx, func() (*http.Response, error) {
		callCount++
		return nil, errors.New("persistent error")
	}, opts)

	if err == nil {
		t.Fatal("Expected error after max retries")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls (1 initial + 1 retry), got %d", callCount)
	}
}

func TestWithExponentialBackoffOpts_ZeroRetries(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	opts := &Options{
		MaxRetries:     0,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		JitterFactor:   0.0,
	}

	_, err := WithExponentialBackoffOpts(ctx, func() (*http.Response, error) {
		callCount++
		return nil, errors.New("error")
	}, opts)

	if err == nil {
		t.Fatal("Expected error")
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call (no retries), got %d", callCount)
	}
}

func TestWithExponentialBackoffOpts_NilOptions(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	resp, err := WithExponentialBackoffOpts(ctx, func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
		}, nil
	}, nil)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestWithExponentialBackoffOpts_RetryAfterHonored(t *testing.T) {
	t.Parallel()
	callCount := 0
	ctx := context.Background()

	opts := &Options{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Second,
		MaxBackoff:     30 * time.Second,
		JitterFactor:   0.0,
	}

	start := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{}
	resp, err := WithExponentialBackoffOpts(ctx, func() (*http.Response, error) {
		return client.Get(server.URL)
	}, opts)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if elapsed > 3*time.Second {
		t.Errorf("Expected backoff ~1s (from Retry-After), but took %v", elapsed)
	}
	_ = resp.Body.Close()
}

func TestCalculateBackoffWithOpts_CustomOptions(t *testing.T) {
	t.Parallel()
	opts := &Options{
		MaxRetries:     5,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		JitterFactor:   0.0,
	}

	backoff0 := calculateBackoffWithOpts(0, opts)
	if backoff0 != 500*time.Millisecond {
		t.Errorf("Expected 500ms for attempt 0, got %v", backoff0)
	}

	backoff1 := calculateBackoffWithOpts(1, opts)
	if backoff1 != 1*time.Second {
		t.Errorf("Expected 1s for attempt 1, got %v", backoff1)
	}

	backoff10 := calculateBackoffWithOpts(10, opts)
	if backoff10 != 5*time.Second {
		t.Errorf("Expected 5s (max) for attempt 10, got %v", backoff10)
	}
}

func TestIsQuotaExhausted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		statusCode     int
		body           string
		expectedResult bool
		bodyReadable   bool
	}{
		{
			name:           "insufficient_quota code",
			statusCode:     http.StatusTooManyRequests,
			body:           `{"error":{"code":"insufficient_quota"}}`,
			expectedResult: true,
			bodyReadable:   true,
		},
		{
			name:           "insufficient_quota type",
			statusCode:     http.StatusTooManyRequests,
			body:           `{"error":{"type":"insufficient_quota"}}`,
			expectedResult: true,
			bodyReadable:   true,
		},
		{
			name:           "billing_hard_limit_reached",
			statusCode:     http.StatusTooManyRequests,
			body:           `{"error":{"code":"billing_hard_limit_reached"}}`,
			expectedResult: true,
			bodyReadable:   true,
		},
		{
			name:           "rate_limit_exceeded - not quota",
			statusCode:     http.StatusTooManyRequests,
			body:           `{"error":{"code":"rate_limit_exceeded"}}`,
			expectedResult: false,
			bodyReadable:   true,
		},
		{
			name:           "non-429 status",
			statusCode:     http.StatusInternalServerError,
			body:           `{"error":{"code":"insufficient_quota"}}`,
			expectedResult: false,
			bodyReadable:   true,
		},
		{
			name:           "invalid JSON",
			statusCode:     http.StatusTooManyRequests,
			body:           `not json`,
			expectedResult: false,
			bodyReadable:   true,
		},
		{
			name:           "empty body",
			statusCode:     http.StatusTooManyRequests,
			body:           ``,
			expectedResult: false,
			bodyReadable:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(tt.body)),
			}

			got := isQuotaExhausted(resp)
			if got != tt.expectedResult {
				t.Errorf("isQuotaExhausted() = %v, want %v", got, tt.expectedResult)
			}

			if tt.bodyReadable {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("body should be readable after isQuotaExhausted, got error: %v", err)
				}
				if tt.statusCode == http.StatusTooManyRequests && string(bodyBytes) != tt.body {
					t.Errorf("body content changed, got %q, want %q", string(bodyBytes), tt.body)
				}
			}
		})
	}
}

func TestIsQuotaExhausted_BodyClosed(t *testing.T) {
	t.Parallel()

	closeCount := 0
	body := &trackingReadCloser{
		Reader: bytes.NewBufferString(`{"error":{"code":"insufficient_quota"}}`),
		onClose: func() {
			closeCount++
		},
	}

	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       body,
	}

	got := isQuotaExhausted(resp)
	if !got {
		t.Error("expected isQuotaExhausted to return true")
	}

	if closeCount != 1 {
		t.Errorf("expected original body to be closed exactly once, got %d closes", closeCount)
	}
}

type trackingReadCloser struct {
	io.Reader
	onClose func()
	closed  bool
}

func (t *trackingReadCloser) Close() error {
	if !t.closed {
		t.closed = true
		if t.onClose != nil {
			t.onClose()
		}
	}
	return nil
}

func TestCalculateBackoffWithOpts_ExtremeJitter(t *testing.T) {
	t.Parallel()

	opts := &Options{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		JitterFactor:   2.0,
	}

	for i := 0; i < 100; i++ {
		backoff := calculateBackoffWithOpts(0, opts)
		if backoff < 0 {
			t.Errorf("backoff should never be negative, got %v", backoff)
		}
		if backoff > opts.MaxBackoff {
			t.Errorf("backoff should not exceed MaxBackoff, got %v > %v", backoff, opts.MaxBackoff)
		}
	}
}

func TestCalculateBackoffWithOpts_NegativeJitterFactor(t *testing.T) {
	t.Parallel()

	opts := &Options{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		JitterFactor:   -0.5,
	}

	for i := 0; i < 100; i++ {
		backoff := calculateBackoffWithOpts(0, opts)
		if backoff < 0 {
			t.Errorf("backoff should never be negative even with negative jitter factor, got %v", backoff)
		}
	}
}

func TestWithExponentialBackoffOpts_OptionsCopied(t *testing.T) {
	t.Parallel()

	opts := &Options{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		JitterFactor:   0.0,
	}

	ctx := context.Background()
	callCount := 0
	var retriesSeen int

	_, _ = WithExponentialBackoffOpts(ctx, func() (*http.Response, error) {
		callCount++
		if callCount == 1 {
			opts.MaxRetries = 0
		}
		retriesSeen = callCount
		if callCount < 3 {
			return nil, errors.New("transient error")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
		}, nil
	}, opts)

	if retriesSeen < 3 {
		t.Errorf("expected at least 3 retries (options should be copied), got %d", retriesSeen)
	}
}

func TestWithExponentialBackoffOpts_QuotaExhaustedNoRetry(t *testing.T) {
	t.Parallel()

	callCount := 0
	ctx := context.Background()

	opts := &Options{
		MaxRetries:     5,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		JitterFactor:   0.0,
	}

	resp, err := WithExponentialBackoffOpts(ctx, func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"code":"insufficient_quota"}}`)),
		}, nil
	}, opts)

	if err != nil {
		t.Fatalf("expected no error for quota exhausted, got: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 response, got %d", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("expected exactly 1 call (no retries for quota exhausted), got %d", callCount)
	}
	_ = resp.Body.Close()
}
