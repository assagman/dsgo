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
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.statusCode); got != tt.want {
				t.Errorf("IsRetryable(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestWithExponentialBackoff_Success(t *testing.T) {
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
	callCount := 0
	ctx := context.Background()

	_, err := WithExponentialBackoff(ctx, func() (*http.Response, error) {
		callCount++
		return nil, errors.New("persistent error")
	})

	if err == nil {
		t.Fatal("Expected error after max retries")
	}
	if callCount != MaxRetries+1 {
		t.Errorf("Expected %d calls (initial + %d retries), got %d", MaxRetries+1, MaxRetries, callCount)
	}
}

func TestWithExponentialBackoff_ContextCanceled(t *testing.T) {
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
	tests := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{0, 500 * time.Millisecond, 2 * time.Second},                 // 1s ± jitter
		{1, 1 * time.Second, 3 * time.Second},                        // 2s ± jitter
		{2, 2 * time.Second, 6 * time.Second},                        // 4s ± jitter
		{3, 4 * time.Second, 12 * time.Second},                       // 8s ± jitter
		{10, MaxBackoff - 5*time.Second, MaxBackoff + 5*time.Second}, // capped at MaxBackoff
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			backoff := calculateBackoff(tt.attempt)
			if backoff < tt.minExpected || backoff > tt.maxExpected {
				t.Errorf("calculateBackoff(%d) = %v, want between %v and %v",
					tt.attempt, backoff, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestWithExponentialBackoff_MixedErrors(t *testing.T) {
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
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}

	if isQuotaExhausted(resp) {
		t.Error("Expected isQuotaExhausted to return false for empty body")
	}
}

func TestWithExponentialBackoff_QuotaExhausted(t *testing.T) {
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

func TestWithExponentialBackoff_ContextCanceledDuringBackoff(t *testing.T) {
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
