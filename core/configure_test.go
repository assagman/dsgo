package core

import (
	"testing"
	"time"
)

func TestConfigure(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	t.Run("WithProvider", func(t *testing.T) {
		ResetConfig()
		Configure(WithProvider("openai"))
		settings := GetSettings()
		if settings.DefaultProvider != "openai" {
			t.Errorf("expected provider 'openai', got '%s'", settings.DefaultProvider)
		}
	})

	t.Run("WithModel", func(t *testing.T) {
		ResetConfig()
		Configure(WithModel("gpt-4"))
		settings := GetSettings()
		if settings.DefaultModel != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got '%s'", settings.DefaultModel)
		}
	})

	t.Run("WithTimeout", func(t *testing.T) {
		ResetConfig()
		timeout := 45 * time.Second
		Configure(WithTimeout(timeout))
		settings := GetSettings()
		if settings.DefaultTimeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, settings.DefaultTimeout)
		}
	})

	t.Run("WithLM", func(t *testing.T) {
		ResetConfig()
		var lm LM = &mockLM{}
		Configure(WithLM(lm))
		settings := GetSettings()
		if settings.DefaultLM == nil {
			t.Error("expected LM to be set")
		}
	})

	t.Run("WithAPIKey", func(t *testing.T) {
		ResetConfig()
		Configure(WithAPIKey("openai", "test-key"))
		settings := GetSettings()
		key, ok := settings.APIKey["openai"]
		if !ok {
			t.Error("expected API key to be set")
		}
		if key != "test-key" {
			t.Errorf("expected API key 'test-key', got '%s'", key)
		}
	})

	t.Run("WithMaxRetries", func(t *testing.T) {
		ResetConfig()
		Configure(WithMaxRetries(5))
		settings := GetSettings()
		if settings.MaxRetries != 5 {
			t.Errorf("expected max retries 5, got %d", settings.MaxRetries)
		}
	})

	t.Run("WithTracing", func(t *testing.T) {
		ResetConfig()
		Configure(WithTracing(true))
		settings := GetSettings()
		if !settings.EnableTracing {
			t.Error("expected tracing to be enabled")
		}
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		ResetConfig()
		Configure(
			WithProvider("openrouter"),
			WithModel("meta-llama/llama-3.3-70b-instruct"),
			WithTimeout(60*time.Second),
			WithAPIKey("openrouter", "or-key"),
			WithMaxRetries(7),
			WithTracing(true),
		)

		settings := GetSettings()
		if settings.DefaultProvider != "openrouter" {
			t.Errorf("expected provider 'openrouter', got '%s'", settings.DefaultProvider)
		}
		if settings.DefaultModel != "meta-llama/llama-3.3-70b-instruct" {
			t.Errorf("expected model 'meta-llama/llama-3.3-70b-instruct', got '%s'", settings.DefaultModel)
		}
		if settings.DefaultTimeout != 60*time.Second {
			t.Errorf("expected timeout 60s, got %v", settings.DefaultTimeout)
		}
		if key, ok := settings.APIKey["openrouter"]; !ok || key != "or-key" {
			t.Error("expected OpenRouter API key to be set")
		}
		if settings.MaxRetries != 7 {
			t.Errorf("expected max retries 7, got %d", settings.MaxRetries)
		}
		if !settings.EnableTracing {
			t.Error("expected tracing to be enabled")
		}
	})

	t.Run("OptionsOverride", func(t *testing.T) {
		ResetConfig()
		Configure(WithProvider("openai"))
		Configure(WithProvider("openrouter"))
		settings := GetSettings()
		if settings.DefaultProvider != "openrouter" {
			t.Errorf("expected provider to be overridden to 'openrouter', got '%s'", settings.DefaultProvider)
		}
	})
}

func TestResetConfig(t *testing.T) {
	Configure(
		WithProvider("openai"),
		WithModel("gpt-4"),
		WithTimeout(45*time.Second),
		WithAPIKey("openai", "test-key"),
		WithMaxRetries(5),
		WithTracing(true),
	)

	ResetConfig()

	settings := GetSettings()
	if settings.DefaultProvider != "" {
		t.Error("expected DefaultProvider to be reset")
	}
	if settings.DefaultModel != "" {
		t.Error("expected DefaultModel to be reset")
	}
	if settings.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout to be reset to 30s, got %v", settings.DefaultTimeout)
	}
	if len(settings.APIKey) != 0 {
		t.Error("expected APIKey to be reset")
	}
	if settings.MaxRetries != 3 {
		t.Errorf("expected MaxRetries to be reset to 3, got %d", settings.MaxRetries)
	}
	if settings.EnableTracing {
		t.Error("expected EnableTracing to be reset to false")
	}
}

func TestConfigure_Concurrent(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			Configure(
				WithProvider("provider"),
				WithModel("model"),
				WithTimeout(30*time.Second),
			)
			GetSettings()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestWithAPIKey_MultipleProviders tests adding multiple API keys
func TestWithAPIKey_MultipleProviders(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	Configure(
		WithAPIKey("openai", "key-openai"),
		WithAPIKey("openrouter", "key-openrouter"),
	)

	settings := GetSettings()

	if len(settings.APIKey) != 2 {
		t.Errorf("expected 2 API keys, got %d", len(settings.APIKey))
	}

	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "key-openai"},
		{"openrouter", "key-openrouter"},
	}

	for _, tt := range tests {
		if key, ok := settings.APIKey[tt.provider]; !ok || key != tt.want {
			t.Errorf("expected key for %q to be %q, got %q (ok=%v)", tt.provider, tt.want, key, ok)
		}
	}
}

// TestWithAPIKey_Overwrite tests overwriting existing API keys
func TestWithAPIKey_Overwrite(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	Configure(WithAPIKey("openai", "old-key"))
	Configure(WithAPIKey("openai", "new-key"))

	settings := GetSettings()
	key, ok := settings.APIKey["openai"]
	if !ok || key != "new-key" {
		t.Errorf("expected overwritten key to be 'new-key', got %q", key)
	}
}

// TestWithCache enables caching with specific capacity
func TestWithCache(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	Configure(WithCache(100))

	settings := GetSettings()
	if settings.DefaultCache == nil {
		t.Fatal("expected cache to be set")
	}

	// Test cache functionality
	result := &GenerateResult{Content: "test"}
	result.Usage.TotalTokens = 50

	// Cache should work
	testKey := "test-key-123"
	if _, found := settings.DefaultCache.Get(testKey); found {
		t.Error("expected cache to be empty initially")
	}

	settings.DefaultCache.Set(testKey, result)

	if cached, found := settings.DefaultCache.Get(testKey); !found {
		t.Error("expected to retrieve cached value")
	} else if cached.Content != "test" {
		t.Errorf("expected cached content to be 'test', got %q", cached.Content)
	}
}

// TestWithCacheTTL sets cache TTL and affects cache recreation
func TestWithCacheTTL(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	ttl := 5 * time.Second

	Configure(
		WithCache(50),
		WithCacheTTL(ttl),
	)

	settings := GetSettings()
	if settings.CacheTTL != ttl {
		t.Errorf("expected TTL to be %v, got %v", ttl, settings.CacheTTL)
	}

	if settings.DefaultCache == nil {
		t.Fatal("expected cache to be set")
	}
}

// TestWithCacheTTL_UpdatesTTL tests updating TTL on existing cache
func TestWithCacheTTL_UpdatesTTL(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	Configure(WithCache(50))

	settings := GetSettings()
	oldCache := settings.DefaultCache

	newTTL := 10 * time.Second
	Configure(WithCacheTTL(newTTL))

	settings = GetSettings()
	if settings.CacheTTL != newTTL {
		t.Errorf("expected TTL to be updated to %v, got %v", newTTL, settings.CacheTTL)
	}

	// Cache should be recreated
	if settings.DefaultCache == oldCache {
		t.Error("expected cache to be recreated with new TTL")
	}
}

// TestWithCacheTTL_WithoutExistingCache tests setting TTL without an existing cache
func TestWithCacheTTL_WithoutExistingCache(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	ttl := 5 * time.Second
	Configure(WithCacheTTL(ttl))

	settings := GetSettings()
	if settings.CacheTTL != ttl {
		t.Errorf("expected TTL to be set to %v, got %v", ttl, settings.CacheTTL)
	}
	// Cache should not be created if only TTL is set
	if settings.DefaultCache != nil {
		t.Error("expected cache to not be created when only TTL is set")
	}
}

// TestWithCollector sets custom collector
func TestWithCollector(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	collector := NewMemoryCollector(100)
	Configure(WithCollector(collector))

	settings := GetSettings()
	if settings.Collector != collector {
		t.Error("expected collector to be set")
	}
}

// TestStripProviderPrefix tests the stripProviderPrefix helper function
func TestStripProviderPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "openrouter prefix",
			input:    "openrouter/meta-llama/llama-3.3-70b-instruct",
			expected: "meta-llama/llama-3.3-70b-instruct",
		},
		{
			name:     "openai prefix",
			input:    "openai/gpt-4",
			expected: "gpt-4",
		},
		{
			name:     "no prefix",
			input:    "gpt-4-turbo",
			expected: "gpt-4-turbo",
		},
		{
			name:     "meta prefix without openrouter",
			input:    "meta-llama/llama-3.3-70b-instruct",
			expected: "meta-llama/llama-3.3-70b-instruct",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just prefix",
			input:    "openrouter/",
			expected: "",
		},
		{
			name:     "multiple slashes",
			input:    "openrouter/vendor/model/variant",
			expected: "vendor/model/variant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripProviderPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("stripProviderPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
