package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigure(t *testing.T) {
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)

	t.Run("WithProvider", func(t *testing.T) {
		ResetConfig()
		if err := Configure(WithProvider("openai")); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		settings := GetSettings()
		if settings.DefaultProvider != "openai" {
			t.Errorf("expected provider 'openai', got '%s'", settings.DefaultProvider)
		}
	})

	t.Run("WithModel", func(t *testing.T) {
		ResetConfig()
		if err := Configure(WithModel("gpt-4")); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		settings := GetSettings()
		if settings.DefaultModel != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got '%s'", settings.DefaultModel)
		}
	})

	t.Run("WithTimeout", func(t *testing.T) {
		ResetConfig()
		timeout := 45 * time.Second
		if err := Configure(WithTimeout(timeout)); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		settings := GetSettings()
		if settings.DefaultTimeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, settings.DefaultTimeout)
		}
	})

	t.Run("WithMaxRetries", func(t *testing.T) {
		ResetConfig()
		if err := Configure(WithMaxRetries(5)); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		settings := GetSettings()
		if settings.MaxRetries != 5 {
			t.Errorf("expected max retries 5, got %d", settings.MaxRetries)
		}
	})

	t.Run("WithTracing", func(t *testing.T) {
		ResetConfig()
		if err := Configure(WithTracing(true)); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		settings := GetSettings()
		if !settings.EnableTracing {
			t.Error("expected tracing to be enabled")
		}
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		ResetConfig()
		if err := Configure(
			WithProvider("openrouter"),
			WithModel("meta-llama/llama-3.3-70b-instruct"),
			WithTimeout(60*time.Second),
			WithMaxRetries(7),
			WithTracing(true),
		); err != nil {
			t.Fatalf("Configure: %v", err)
		}

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
		if settings.MaxRetries != 7 {
			t.Errorf("expected max retries 7, got %d", settings.MaxRetries)
		}
		if !settings.EnableTracing {
			t.Error("expected tracing to be enabled")
		}
	})

	t.Run("OptionsOverride", func(t *testing.T) {
		ResetConfig()
		if err := Configure(WithProvider("openai")); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		if err := Configure(WithProvider("openrouter")); err != nil {
			t.Fatalf("Configure: %v", err)
		}
		settings := GetSettings()
		if settings.DefaultProvider != "openrouter" {
			t.Errorf("expected provider to be overridden to 'openrouter', got '%s'", settings.DefaultProvider)
		}
	})
}

func TestConfigure_RequiresAPIKeyEnv(t *testing.T) {
	ResetConfig()
	defer ResetConfig()

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("DSGO_ENV_FILE_PATH", envPath)
	t.Setenv("DSGO_OPENAI_API_KEY", "")
	t.Setenv("DSGO_OPENROUTER_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	if err := Configure(); err == nil {
		t.Fatal("expected Configure to fail without API key env vars")
	}
}

func TestResetConfig(t *testing.T) {
	// Note: Cannot use t.Parallel() because ResetConfig() modifies global state
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)
	if err := Configure(
		WithProvider("openai"),
		WithModel("gpt-4"),
		WithTimeout(45*time.Second),
		WithMaxRetries(5),
		WithTracing(true),
	); err != nil {
		t.Fatalf("Configure: %v", err)
	}

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
	// Note: Cannot use t.Parallel() because ResetConfig() modifies global state
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			if err := Configure(
				WithProvider("provider"),
				WithModel("model"),
				WithTimeout(30*time.Second),
			); err != nil {
				t.Errorf("Configure: %v", err)
			}
			GetSettings()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestWithCache enables caching with specific capacity
func TestWithCache(t *testing.T) {
	// Note: Cannot use t.Parallel() because ResetConfig() modifies global state
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)

	if err := Configure(WithCache(100)); err != nil {
		t.Fatalf("Configure: %v", err)
	}

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
	// Note: Cannot use t.Parallel() because ResetConfig() modifies global state
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)

	ttl := 5 * time.Second

	if err := Configure(
		WithCache(50),
		WithCacheTTL(ttl),
	); err != nil {
		t.Fatalf("Configure: %v", err)
	}

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
	// Note: Cannot use t.Parallel() because ResetConfig() modifies global state
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)

	if err := Configure(WithCache(50)); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	settings := GetSettings()
	oldCache := settings.DefaultCache

	newTTL := 10 * time.Second
	if err := Configure(WithCacheTTL(newTTL)); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	settings = GetSettings()
	if settings.CacheTTL != newTTL {
		t.Errorf("expected TTL to be updated to %v, got %v", newTTL, settings.CacheTTL)
	}

	// Cache should be recreated
	if settings.DefaultCache == oldCache {
		t.Error("expected cache to be recreated with new TTL")
	}
}

// TestWithCacheTTL_WithDefaultCache tests setting TTL with the default cache
func TestWithCacheTTL_WithDefaultCache(t *testing.T) {
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)

	ttl := 5 * time.Second
	if err := Configure(WithCacheTTL(ttl)); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	settings := GetSettings()
	if settings.CacheTTL != ttl {
		t.Errorf("expected TTL to be set to %v, got %v", ttl, settings.CacheTTL)
	}
	// Cache is created by default now (DSPy parity)
	if settings.DefaultCache == nil {
		t.Error("expected default cache to exist")
	}
}

// TestWithCollector sets custom collector
func TestWithCollector(t *testing.T) {
	// Note: Cannot use t.Parallel() because ResetConfig() modifies global state
	ResetConfig()
	defer ResetConfig()
	ensureAPIKeyEnv(t)

	collector := NewMemoryCollector(100)
	if err := Configure(WithCollector(collector)); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	settings := GetSettings()
	if settings.Collector != collector {
		t.Error("expected collector to be set")
	}
}

// TestStripProviderPrefix tests the stripProviderPrefix helper function
func TestStripProviderPrefix(t *testing.T) {
	t.Parallel()
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := stripProviderPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("stripProviderPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
