package core

import (
	"os"
	"testing"
	"time"
)

func TestLoadEnv(t *testing.T) {
	setupEnv := func() {
		_ = os.Setenv("DSGO_PROVIDER", "openai")
		_ = os.Setenv("DSGO_MODEL", "gpt-4")
		_ = os.Setenv("DSGO_TIMEOUT", "45")
		_ = os.Setenv("DSGO_MAX_RETRIES", "5")
		_ = os.Setenv("DSGO_TRACING", "true")
		_ = os.Setenv("DSGO_OPENAI_API_KEY", "test-openai-key")
		_ = os.Setenv("DSGO_OPENROUTER_API_KEY", "test-openrouter-key")
		_ = os.Setenv("DSGO_ANTHROPIC_API_KEY", "test-anthropic-key")
	}

	cleanupEnv := func() {
		_ = os.Unsetenv("DSGO_PROVIDER")
		_ = os.Unsetenv("DSGO_MODEL")
		_ = os.Unsetenv("DSGO_TIMEOUT")
		_ = os.Unsetenv("DSGO_MAX_RETRIES")
		_ = os.Unsetenv("DSGO_TRACING")
		_ = os.Unsetenv("DSGO_OPENAI_API_KEY")
		_ = os.Unsetenv("DSGO_OPENROUTER_API_KEY")
		_ = os.Unsetenv("DSGO_ANTHROPIC_API_KEY")
		_ = os.Unsetenv("OPENAI_API_KEY")
		_ = os.Unsetenv("OPENROUTER_API_KEY")
		_ = os.Unsetenv("ANTHROPIC_API_KEY")
	}

	t.Run("LoadAllEnvVars", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()
		setupEnv()

		ResetConfig()
		Configure()

		settings := GetSettings()

		if settings.DefaultProvider != "openai" {
			t.Errorf("expected provider 'openai', got '%s'", settings.DefaultProvider)
		}
		if settings.DefaultModel != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got '%s'", settings.DefaultModel)
		}
		if settings.DefaultTimeout != 45*time.Second {
			t.Errorf("expected timeout 45s, got %v", settings.DefaultTimeout)
		}
		if settings.MaxRetries != 5 {
			t.Errorf("expected max retries 5, got %d", settings.MaxRetries)
		}
		if !settings.EnableTracing {
			t.Error("expected tracing to be enabled")
		}

		if key, ok := settings.APIKey["openai"]; !ok || key != "test-openai-key" {
			t.Errorf("expected OpenAI API key 'test-openai-key', got '%s'", key)
		}
		if key, ok := settings.APIKey["openrouter"]; !ok || key != "test-openrouter-key" {
			t.Errorf("expected OpenRouter API key 'test-openrouter-key', got '%s'", key)
		}
		if key, ok := settings.APIKey["anthropic"]; !ok || key != "test-anthropic-key" {
			t.Errorf("expected Anthropic API key 'test-anthropic-key', got '%s'", key)
		}
	})

	t.Run("FallbackAPIKeys", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("OPENAI_API_KEY", "fallback-openai-key")
		_ = os.Setenv("OPENROUTER_API_KEY", "fallback-openrouter-key")
		_ = os.Setenv("ANTHROPIC_API_KEY", "fallback-anthropic-key")

		ResetConfig()
		Configure()

		settings := GetSettings()

		if key, ok := settings.APIKey["openai"]; !ok || key != "fallback-openai-key" {
			t.Errorf("expected OpenAI API key 'fallback-openai-key', got '%s'", key)
		}
		if key, ok := settings.APIKey["openrouter"]; !ok || key != "fallback-openrouter-key" {
			t.Errorf("expected OpenRouter API key 'fallback-openrouter-key', got '%s'", key)
		}
		if key, ok := settings.APIKey["anthropic"]; !ok || key != "fallback-anthropic-key" {
			t.Errorf("expected Anthropic API key 'fallback-anthropic-key', got '%s'", key)
		}
	})

	t.Run("PrefixedAPIKeysOverrideFallback", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("DSGO_OPENAI_API_KEY", "prefixed-key")
		_ = os.Setenv("OPENAI_API_KEY", "fallback-key")

		ResetConfig()
		Configure()

		settings := GetSettings()

		if key, ok := settings.APIKey["openai"]; !ok || key != "prefixed-key" {
			t.Errorf("expected prefixed key to override fallback, got '%s'", key)
		}
	})

	t.Run("OptionsOverrideEnv", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("DSGO_PROVIDER", "openai")
		_ = os.Setenv("DSGO_MODEL", "gpt-4")

		ResetConfig()
		Configure(
			WithProvider("openrouter"),
			WithModel("google/gemini-2.5-flash"),
		)

		settings := GetSettings()

		if settings.DefaultProvider != "openrouter" {
			t.Errorf("expected options to override env, got provider '%s'", settings.DefaultProvider)
		}
		if settings.DefaultModel != "google/gemini-2.5-flash" {
			t.Errorf("expected options to override env, got model '%s'", settings.DefaultModel)
		}
	})

	t.Run("InvalidTimeout", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("DSGO_TIMEOUT", "invalid")

		ResetConfig()
		Configure()

		settings := GetSettings()

		if settings.DefaultTimeout != 30*time.Second {
			t.Errorf("expected default timeout for invalid value, got %v", settings.DefaultTimeout)
		}
	})

	t.Run("InvalidMaxRetries", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("DSGO_MAX_RETRIES", "invalid")

		ResetConfig()
		Configure()

		settings := GetSettings()

		if settings.MaxRetries != 3 {
			t.Errorf("expected default max retries for invalid value, got %d", settings.MaxRetries)
		}
	})

	t.Run("InvalidTracing", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("DSGO_TRACING", "invalid")

		ResetConfig()
		Configure()

		settings := GetSettings()

		if settings.EnableTracing {
			t.Error("expected tracing to be false for invalid value")
		}
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("DSGO_TIMEOUT", "0")

		ResetConfig()
		Configure()

		settings := GetSettings()

		if settings.DefaultTimeout != 30*time.Second {
			t.Errorf("expected default timeout for zero value, got %v", settings.DefaultTimeout)
		}
	})

	t.Run("NegativeMaxRetries", func(t *testing.T) {
		cleanupEnv()
		defer cleanupEnv()

		_ = os.Setenv("DSGO_MAX_RETRIES", "-1")

		ResetConfig()
		Configure()

		settings := GetSettings()

		if settings.MaxRetries != 3 {
			t.Errorf("expected default max retries for negative value, got %d", settings.MaxRetries)
		}
	})
}
