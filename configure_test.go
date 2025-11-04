package dsgo

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
			WithModel("google/gemini-2.5-flash"),
			WithTimeout(60*time.Second),
			WithAPIKey("openrouter", "or-key"),
			WithMaxRetries(7),
			WithTracing(true),
		)

		settings := GetSettings()
		if settings.DefaultProvider != "openrouter" {
			t.Errorf("expected provider 'openrouter', got '%s'", settings.DefaultProvider)
		}
		if settings.DefaultModel != "google/gemini-2.5-flash" {
			t.Errorf("expected model 'google/gemini-2.5-flash', got '%s'", settings.DefaultModel)
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
