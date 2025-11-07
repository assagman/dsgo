package core

import (
	"context"
	"testing"
)

func TestRegisterLM(t *testing.T) {
	// Save and restore original registry
	originalRegistry := make(map[string]LMFactory)
	registryLock.Lock()
	for k, v := range lmRegistry {
		originalRegistry[k] = v
	}
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		lmRegistry = originalRegistry
		registryLock.Unlock()
	}()

	t.Run("RegisterNewProvider", func(t *testing.T) {
		testFactory := func(model string) LM {
			return &mockLM{}
		}

		RegisterLM("testprovider", testFactory)

		registryLock.RLock()
		_, ok := lmRegistry["testprovider"]
		registryLock.RUnlock()

		if !ok {
			t.Error("expected provider to be registered")
		}
	})

	t.Run("OverwriteExistingProvider", func(t *testing.T) {
		factory1 := func(model string) LM {
			return &mockLM{}
		}
		factory2 := func(model string) LM {
			return &mockLM{}
		}

		RegisterLM("testprovider2", factory1)
		RegisterLM("testprovider2", factory2)

		registryLock.RLock()
		_, ok := lmRegistry["testprovider2"]
		registryLock.RUnlock()

		if !ok {
			t.Error("expected provider to be registered")
		}
	})
}

func TestNewLM(t *testing.T) {
	// Save and restore original registry and settings
	originalRegistry := make(map[string]LMFactory)
	registryLock.Lock()
	for k, v := range lmRegistry {
		originalRegistry[k] = v
	}
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		lmRegistry = originalRegistry
		registryLock.Unlock()
		ResetConfig()
	}()

	// Register a test provider
	RegisterLM("testprovider", func(model string) LM {
		return &mockLM{}
	})

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ResetConfig()
		Configure(
			WithProvider("testprovider"),
			WithModel("test-model"),
		)

		lm, err := NewLM(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if lm == nil {
			t.Error("expected LM to be created")
		}
		if lm.Name() != "mock" {
			t.Errorf("expected LM name 'mock', got '%s'", lm.Name())
		}
	})

	t.Run("SuccessWithMultipleProviders", func(t *testing.T) {
		RegisterLM("provider1", func(model string) LM {
			return &mockLM{}
		})
		RegisterLM("provider2", func(model string) LM {
			return &mockLM{}
		})

		ResetConfig()
		Configure(
			WithProvider("provider2"),
			WithModel("model-2"),
		)

		lm, err := NewLM(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if lm == nil {
			t.Error("expected LM to be created")
		}
	})

	t.Run("NoProviderConfigured", func(t *testing.T) {
		ResetConfig()
		Configure(
			WithModel("test-model"),
		)

		_, err := NewLM(ctx)
		if err == nil {
			t.Error("expected error when provider not configured")
		}
		if err.Error() != "no default provider configured (use dsgo.Configure with dsgo.WithProvider)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("NoModelConfigured", func(t *testing.T) {
		ResetConfig()
		Configure(
			WithProvider("testprovider"),
		)

		_, err := NewLM(ctx)
		if err == nil {
			t.Error("expected error when model not configured")
		}
		if err.Error() != "no default model configured (use dsgo.Configure with dsgo.WithModel)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ProviderNotRegistered", func(t *testing.T) {
		ResetConfig()
		Configure(
			WithProvider("unknownprovider"),
			WithModel("test-model"),
		)

		_, err := NewLM(ctx)
		if err == nil {
			t.Error("expected error when provider not registered")
		}
		// Check that error contains helpful information about available providers
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ResetConfig()
		Configure(
			WithProvider("testprovider"),
			WithModel("test-model"),
		)

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		// NewLM should still work even with canceled context
		// (context is for future use, not currently enforced)
		lm, err := NewLM(canceledCtx)
		if err != nil {
			t.Errorf("expected no error with canceled context, got %v", err)
		}
		if lm == nil {
			t.Error("expected LM to be created")
		}
	})
}

func TestGetRegisteredProviders(t *testing.T) {
	// Save and restore original registry
	originalRegistry := make(map[string]LMFactory)
	registryLock.Lock()
	for k, v := range lmRegistry {
		originalRegistry[k] = v
	}
	lmRegistry = make(map[string]LMFactory)
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		lmRegistry = originalRegistry
		registryLock.Unlock()
	}()

	t.Run("EmptyRegistry", func(t *testing.T) {
		providers := getRegisteredProviders()
		if len(providers) != 0 {
			t.Errorf("expected 0 providers, got %d", len(providers))
		}
	})

	t.Run("MultipleProviders", func(t *testing.T) {
		RegisterLM("provider1", func(model string) LM { return &mockLM{} })
		RegisterLM("provider2", func(model string) LM { return &mockLM{} })
		RegisterLM("provider3", func(model string) LM { return &mockLM{} })

		providers := getRegisteredProviders()
		if len(providers) != 3 {
			t.Errorf("expected 3 providers, got %d", len(providers))
		}

		providerMap := make(map[string]bool)
		for _, p := range providers {
			providerMap[p] = true
		}

		if !providerMap["provider1"] || !providerMap["provider2"] || !providerMap["provider3"] {
			t.Errorf("expected all providers to be in list, got %v", providers)
		}
	})
}

func TestLMFactory_Concurrency(t *testing.T) {
	// Save and restore original registry
	originalRegistry := make(map[string]LMFactory)
	registryLock.Lock()
	for k, v := range lmRegistry {
		originalRegistry[k] = v
	}
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		lmRegistry = originalRegistry
		registryLock.Unlock()
	}()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			RegisterLM("testprovider", func(model string) LM {
				return &mockLM{}
			})
			getRegisteredProviders()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestLMFactory_WithCollector(t *testing.T) {
	// Save and restore original registry
	originalRegistry := make(map[string]LMFactory)
	registryLock.Lock()
	for k, v := range lmRegistry {
		originalRegistry[k] = v
	}
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		lmRegistry = originalRegistry
		registryLock.Unlock()
		ResetConfig()
	}()

	// Register a test LM
	testLMFactory := func(model string) LM {
		return &mockLM{}
	}
	RegisterLM("test-provider", testLMFactory)

	ctx := context.Background()
	collector := NewMemoryCollector(10)

	// Configure with collector
	ResetConfig()
	Configure(
		WithProvider("test-provider"),
		WithModel("test-model"),
		WithCollector(collector),
	)

	// Create LM - should be wrapped automatically
	lm, err := NewLM(ctx)
	if err != nil {
		t.Fatalf("Failed to create LM: %v", err)
	}

	// Verify it's wrapped by checking the type
	if _, ok := lm.(*LMWrapper); !ok {
		t.Error("Expected LM to be wrapped with LMWrapper when collector is configured")
	}

	// Verify the wrapped LM still works
	if lm.Name() != "mock" {
		t.Errorf("Expected wrapped LM name 'mock', got '%s'", lm.Name())
	}
}

func TestLMFactory_WithoutCollector(t *testing.T) {
	// Save and restore original registry
	originalRegistry := make(map[string]LMFactory)
	registryLock.Lock()
	for k, v := range lmRegistry {
		originalRegistry[k] = v
	}
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		lmRegistry = originalRegistry
		registryLock.Unlock()
		ResetConfig()
	}()

	// Register a test LM
	testLMFactory := func(model string) LM {
		return &mockLM{}
	}
	RegisterLM("test-provider", testLMFactory)

	ctx := context.Background()

	// Configure without collector
	ResetConfig()
	Configure(
		WithProvider("test-provider"),
		WithModel("test-model"),
	)

	// Create LM - should NOT be wrapped
	lm, err := NewLM(ctx)
	if err != nil {
		t.Fatalf("Failed to create LM: %v", err)
	}

	// Verify it's NOT wrapped (should be the base mockLM)
	if _, ok := lm.(*LMWrapper); ok {
		t.Error("Expected LM to NOT be wrapped when no collector is configured")
	}

	if _, ok := lm.(*mockLM); !ok {
		t.Error("Expected LM to be base mockLM when no collector is configured")
	}
}

// TestDetectProvider tests the detectProvider function for auto-detection logic
func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "gpt-4 model",
			model:    "gpt-4",
			expected: "openai",
		},
		{
			name:     "gpt-4-turbo model",
			model:    "gpt-4-turbo",
			expected: "openai",
		},
		{
			name:     "gpt-3.5-turbo model",
			model:    "gpt-3.5-turbo",
			expected: "openai",
		},
		{
			name:     "o1-preview model",
			model:    "o1-preview",
			expected: "openai",
		},
		{
			name:     "o3-mini model",
			model:    "o3-mini",
			expected: "openrouter",
		},
		{
			name:     "text-davinci-003 model",
			model:    "text-davinci-003",
			expected: "openai",
		},
		{
			name:     "davinci-003 model",
			model:    "davinci-003",
			expected: "openai",
		},
		{
			name:     "google gemini via openrouter",
			model:    "google/gemini-2.5-flash",
			expected: "openrouter",
		},
		{
			name:     "anthropic claude via openrouter",
			model:    "anthropic/claude-3-opus-20250219",
			expected: "openrouter",
		},
		{
			name:     "meta llama via openrouter",
			model:    "meta-llama/llama-3.3-70b-instruct",
			expected: "openrouter",
		},
		{
			name:     "unknown vendor with slash",
			model:    "unknownvendor/some-model",
			expected: "openrouter",
		},
		{
			name:     "unknown model without slash",
			model:    "unknown-model",
			expected: "openrouter",
		},
		{
			name:     "empty string",
			model:    "",
			expected: "openrouter",
		},
		{
			name:     "colon in model name",
			model:    "google/gemini:flash",
			expected: "openrouter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectProvider(tt.model)
			if result != tt.expected {
				t.Errorf("detectProvider(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}
