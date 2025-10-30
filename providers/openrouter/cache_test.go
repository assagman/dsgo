package openrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assagman/dsgo"
)

func TestOpenRouter_WithCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "openai/gpt-4",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello from OpenRouter!"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 5,
				"total_tokens": 15
			}
		}`))
	}))
	defer server.Close()

	lm := NewOpenRouter("openai/gpt-4")
	lm.BaseURL = server.URL
	lm.Cache = dsgo.NewLMCache(100)

	messages := []dsgo.Message{
		{Role: "user", Content: "Hello"},
	}
	options := dsgo.DefaultGenerateOptions()

	// First call - should hit the server
	result1, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("First generate failed: %v", err)
	}
	if result1.Content != "Hello from OpenRouter!" {
		t.Errorf("Expected 'Hello from OpenRouter!', got '%s'", result1.Content)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call, got %d", callCount)
	}

	// Second call - should use cache
	result2, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("Second generate failed: %v", err)
	}
	if result2.Content != "Hello from OpenRouter!" {
		t.Errorf("Expected 'Hello from OpenRouter!', got '%s'", result2.Content)
	}
	if callCount != 1 {
		t.Errorf("Expected still 1 API call (cache hit), got %d", callCount)
	}

	// Verify cache stats
	stats := lm.Cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats.Misses)
	}
}

func TestOpenRouter_WithoutCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "test",
			"choices": [{"message": {"role": "assistant", "content": "Hi"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
		}`))
	}))
	defer server.Close()

	lm := NewOpenRouter("openai/gpt-4")
	lm.BaseURL = server.URL
	// No cache set

	messages := []dsgo.Message{{Role: "user", Content: "Hi"}}
	options := dsgo.DefaultGenerateOptions()

	// Both calls should hit the server
	_, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("First generate failed: %v", err)
	}

	_, err = lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("Second generate failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 API calls without cache, got %d", callCount)
	}
}

func TestOpenRouter_RetryOn429(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		// Fail with 429 on first two attempts, succeed on third
		if callCount <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "test",
			"choices": [{"message": {"role": "assistant", "content": "Success after retry"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer server.Close()

	lm := NewOpenRouter("openai/gpt-4")
	lm.BaseURL = server.URL

	messages := []dsgo.Message{{Role: "user", Content: "Test retry"}}
	options := dsgo.DefaultGenerateOptions()

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("Generate failed after retries: %v", err)
	}

	if result.Content != "Success after retry" {
		t.Errorf("Expected 'Success after retry', got '%s'", result.Content)
	}

	// Should have made 3 attempts (2 failures + 1 success)
	if callCount != 3 {
		t.Errorf("Expected 3 API calls (2 retries + 1 success), got %d", callCount)
	}
}

func TestOpenRouter_RetryOn503(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		// Fail with 503 on first attempt, succeed on second
		if callCount == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error": {"message": "Service unavailable"}}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
		"id": "test",
		"choices": [{"message": {"role": "assistant", "content": "Recovered from 503"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12}
		}`))
	}))
	defer server.Close()

	lm := NewOpenRouter("openai/gpt-4")
	lm.BaseURL = server.URL

	messages := []dsgo.Message{{Role: "user", Content: "Test 503"}}
	options := dsgo.DefaultGenerateOptions()

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.Content != "Recovered from 503" {
		t.Errorf("Expected 'Recovered from 503', got '%s'", result.Content)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 API calls (1 retry + 1 success), got %d", callCount)
	}
}

func TestOpenRouter_RetryExhaustion(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Always fail with 429
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	}))
	defer server.Close()

	lm := NewOpenRouter("openai/gpt-4")
	lm.BaseURL = server.URL

	messages := []dsgo.Message{{Role: "user", Content: "Test exhaustion"}}
	options := dsgo.DefaultGenerateOptions()

	_, err := lm.Generate(context.Background(), messages, options)
	if err == nil {
		t.Fatal("Expected error after retry exhaustion")
	}

	// Should have made 4 attempts (initial + 3 retries)
	if callCount != 4 {
		t.Errorf("Expected 4 API calls (initial + 3 retries), got %d", callCount)
	}
}

func TestOpenRouter_RetryMixed500And429(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		// Mix of 500 and 429 errors
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": {"message": "Internal server error"}}`))
			return
		}
		if callCount == 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "test",
			"choices": [{"message": {"role": "assistant", "content": "Success"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
		}`))
	}))
	defer server.Close()

	lm := NewOpenRouter("openai/gpt-4")
	lm.BaseURL = server.URL

	messages := []dsgo.Message{{Role: "user", Content: "Test mixed errors"}}
	options := dsgo.DefaultGenerateOptions()

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.Content != "Success" {
		t.Errorf("Expected 'Success', got '%s'", result.Content)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 API calls, got %d", callCount)
	}
}
