package core

import (
	"testing"
	"time"
)

func TestSettings_SetAndGet(t *testing.T) {
	t.Parallel()
	s := &Settings{
		APIKey: make(map[string]string),
	}

	t.Run("SetDefaultProvider", func(t *testing.T) {
		t.Parallel()
		s.SetDefaultProvider("openai")
		if s.DefaultProvider != "openai" {
			t.Errorf("expected provider 'openai', got '%s'", s.DefaultProvider)
		}
	})

	t.Run("SetDefaultModel", func(t *testing.T) {
		t.Parallel()
		s.SetDefaultModel("gpt-4")
		if s.DefaultModel != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got '%s'", s.DefaultModel)
		}
	})

	t.Run("SetDefaultTimeout", func(t *testing.T) {
		t.Parallel()
		timeout := 45 * time.Second
		s.SetDefaultTimeout(timeout)
		if s.DefaultTimeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, s.DefaultTimeout)
		}
	})

	t.Run("SetMaxRetries", func(t *testing.T) {
		t.Parallel()
		s.SetMaxRetries(5)
		if s.MaxRetries != 5 {
			t.Errorf("expected max retries 5, got %d", s.MaxRetries)
		}
	})

	t.Run("SetEnableTracing", func(t *testing.T) {
		t.Parallel()
		s.SetEnableTracing(true)
		if !s.EnableTracing {
			t.Error("expected tracing to be enabled")
		}
	})
}

func TestSettings_Reset(t *testing.T) {
	t.Parallel()
	s := &Settings{
		DefaultProvider: "openai",
		DefaultModel:    "gpt-4",
		DefaultTimeout:  45 * time.Second,
		APIKey:          map[string]string{"openai": "test-key"},
		MaxRetries:      5,
		EnableTracing:   true,
	}

	s.Reset()

	if s.DefaultProvider != "" {
		t.Error("expected DefaultProvider to be reset")
	}
	if s.DefaultModel != "" {
		t.Error("expected DefaultModel to be reset")
	}
	if s.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout to be reset to 30s, got %v", s.DefaultTimeout)
	}
	if len(s.APIKey) != 0 {
		t.Error("expected APIKey to be reset")
	}
	if s.MaxRetries != 3 {
		t.Errorf("expected MaxRetries to be reset to 3, got %d", s.MaxRetries)
	}
	if s.EnableTracing {
		t.Error("expected EnableTracing to be reset to false")
	}
}

func TestGetSettings(t *testing.T) {
	ResetConfig()
	defer ResetConfig()
	t.Setenv("DSGO_OPENAI_API_KEY", "test-openai-key")

	if err := Configure(
		WithProvider("openai"),
		WithModel("gpt-4"),
		WithTimeout(45*time.Second),
		WithMaxRetries(5),
		WithTracing(true),
	); err != nil {
		t.Fatalf("Configure: %v", err)
	}

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
	if key, ok := settings.APIKey["openai"]; !ok || key != "test-openai-key" {
		t.Errorf("expected OpenAI API key 'test-openai-key', got '%s'", key)
	}
	if settings.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", settings.MaxRetries)
	}
	if !settings.EnableTracing {
		t.Error("expected tracing to be enabled")
	}
}

func TestSettings_Concurrency(t *testing.T) {
	t.Parallel()
	s := &Settings{
		APIKey: make(map[string]string),
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			s.SetDefaultProvider("provider")
			s.SetDefaultModel("model")
			s.SetDefaultTimeout(30 * time.Second)
			s.SetMaxRetries(3)
			s.SetEnableTracing(true)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
