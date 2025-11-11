package core

import (
	"context"
	"testing"
	"time"
)

type mockLM struct{}

func (m *mockLM) Generate(ctx context.Context, messages []Message, opts *GenerateOptions) (*GenerateResult, error) {
	return &GenerateResult{Content: "test"}, nil
}

func (m *mockLM) Stream(ctx context.Context, messages []Message, opts *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunks := make(chan Chunk)
	errs := make(chan error)
	close(chunks)
	close(errs)
	return chunks, errs
}

func (m *mockLM) Name() string {
	return "mock"
}

func (m *mockLM) SupportsJSON() bool {
	return true
}

func (m *mockLM) SupportsTools() bool {
	return true
}

func TestSettings_SetAndGet(t *testing.T) {
	s := &Settings{
		APIKey: make(map[string]string),
	}

	t.Run("SetDefaultProvider", func(t *testing.T) {
		s.SetDefaultProvider("openai")
		if s.DefaultProvider != "openai" {
			t.Errorf("expected provider 'openai', got '%s'", s.DefaultProvider)
		}
	})

	t.Run("SetDefaultModel", func(t *testing.T) {
		s.SetDefaultModel("gpt-4")
		if s.DefaultModel != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got '%s'", s.DefaultModel)
		}
	})

	t.Run("SetDefaultTimeout", func(t *testing.T) {
		timeout := 45 * time.Second
		s.SetDefaultTimeout(timeout)
		if s.DefaultTimeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, s.DefaultTimeout)
		}
	})

	t.Run("SetDefaultLM", func(t *testing.T) {
		var lm LM = &mockLM{}
		s.SetDefaultLM(lm)
		if s.DefaultLM == nil {
			t.Error("expected LM to be set")
		}
	})

	t.Run("SetAPIKey", func(t *testing.T) {
		s.SetAPIKey("openai", "test-key")
		key, ok := s.GetAPIKey("openai")
		if !ok {
			t.Error("expected API key to be set")
		}
		if key != "test-key" {
			t.Errorf("expected API key 'test-key', got '%s'", key)
		}
	})

	t.Run("GetAPIKey_NotFound", func(t *testing.T) {
		_, ok := s.GetAPIKey("nonexistent")
		if ok {
			t.Error("expected API key to not be found")
		}
	})

	t.Run("SetMaxRetries", func(t *testing.T) {
		s.SetMaxRetries(5)
		if s.MaxRetries != 5 {
			t.Errorf("expected max retries 5, got %d", s.MaxRetries)
		}
	})

	t.Run("SetEnableTracing", func(t *testing.T) {
		s.SetEnableTracing(true)
		if !s.EnableTracing {
			t.Error("expected tracing to be enabled")
		}
	})
}

func TestSettings_Reset(t *testing.T) {
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

	Configure(
		WithProvider("openai"),
		WithModel("gpt-4"),
		WithTimeout(45*time.Second),
		WithAPIKey("openai", "test-key"),
		WithMaxRetries(5),
		WithTracing(true),
	)

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
	if key, ok := settings.APIKey["openai"]; !ok || key != "test-key" {
		t.Error("expected API key to be set")
	}
	if settings.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", settings.MaxRetries)
	}
	if !settings.EnableTracing {
		t.Error("expected tracing to be enabled")
	}
}

func TestSettings_SetAPIKey_NilMap(t *testing.T) {
	s := &Settings{
		// Intentionally nil to test initialization
		APIKey: nil,
	}

	// SetAPIKey should initialize the map if nil
	s.SetAPIKey("provider1", "key1")

	key, ok := s.GetAPIKey("provider1")
	if !ok {
		t.Error("expected API key to be set")
	}
	if key != "key1" {
		t.Errorf("expected key 'key1', got %q", key)
	}
}

func TestSettings_SetAPIKey_Overwrite(t *testing.T) {
	s := &Settings{
		APIKey: make(map[string]string),
	}

	s.SetAPIKey("provider", "old-key")
	s.SetAPIKey("provider", "new-key")

	key, ok := s.GetAPIKey("provider")
	if !ok {
		t.Error("expected API key to exist")
	}
	if key != "new-key" {
		t.Errorf("expected key to be overwritten to 'new-key', got %q", key)
	}
}

func TestSettings_SetAPIKey_MultipleProviders(t *testing.T) {
	s := &Settings{
		APIKey: nil,
	}

	s.SetAPIKey("openai", "key-openai")
	s.SetAPIKey("openrouter", "key-openrouter")
	s.SetAPIKey("anthropic", "key-anthropic")

	if len(s.APIKey) != 3 {
		t.Errorf("expected 3 providers, got %d", len(s.APIKey))
	}

	tests := []struct {
		provider string
		want     string
	}{
		{"openai", "key-openai"},
		{"openrouter", "key-openrouter"},
		{"anthropic", "key-anthropic"},
	}

	for _, tt := range tests {
		key, ok := s.GetAPIKey(tt.provider)
		if !ok {
			t.Errorf("expected key for %q to exist", tt.provider)
		}
		if key != tt.want {
			t.Errorf("expected key for %q to be %q, got %q", tt.provider, tt.want, key)
		}
	}
}

func TestSettings_Concurrency(t *testing.T) {
	s := &Settings{
		APIKey: make(map[string]string),
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			s.SetAPIKey("provider", "key")
			s.GetAPIKey("provider")
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
