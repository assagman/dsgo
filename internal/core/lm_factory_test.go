package core

import (
	"context"
	"strings"
	"testing"

	"github.com/assagman/dsgo/internal/modelcatalog"
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

	// Register test providers
	RegisterLM("testprovider", func(model string) LM {
		return &mockLM{}
	})
	RegisterLM("openrouter", func(model string) LM {
		return &mockLM{}
	})

	// Authoritative model catalog requires explicit registration.
	if err := modelcatalog.RegisterModel(modelcatalog.Model{ID: "testprovider/test-model", Limits: modelcatalog.Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: modelcatalog.Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: modelcatalog.Metadata{Name: "Test Model", Family: "test"}}); err != nil {
		t.Fatalf("RegisterModel(testprovider/test-model) err = %v", err)
	}
	if err := modelcatalog.RegisterModel(modelcatalog.Model{ID: "provider2/model-2", Limits: modelcatalog.Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: modelcatalog.Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: modelcatalog.Metadata{Name: "Model 2", Family: "test"}}); err != nil {
		t.Fatalf("RegisterModel(provider2/model-2) err = %v", err)
	}

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		lm, err := NewLM(ctx, "testprovider/test-model")
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

		lm, err := NewLM(ctx, "provider2/model-2")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if lm == nil {
			t.Error("expected LM to be created")
		}
	})

	t.Run("NoProviderConfigured", func(t *testing.T) {
		_, err := NewLM(ctx, "openai/gpt-4") // openai provider is not registered in this test
		if err == nil {
			t.Error("expected error when provider not registered")
		}
		// Error should be about provider not registered
		if err == nil {
			t.Error("expected error for unregistered provider")
		}
	})

	t.Run("EmptyModelString", func(t *testing.T) {
		_, err := NewLM(ctx, "")
		if err == nil {
			t.Error("expected error when model string is empty")
		}
		if !strings.Contains(err.Error(), "model string is required") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ProviderNotRegistered", func(t *testing.T) {
		_, err := NewLM(ctx, "unknownprovider/model") // unknown provider is not registered
		if err == nil {
			t.Error("expected error when provider not registered")
		}
		// Check that error contains helpful information about available providers
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("expected non-empty error message")
		}
		if !strings.Contains(errMsg, "unknownprovider") {
			t.Errorf("expected error to mention the missing provider, got: %s", errMsg)
		}
	})

	t.Run("MixedCaseProviderRegistration", func(t *testing.T) {
		// Register with mixed case
		RegisterLM("MixedCaseProvider", func(model string) LM {
			return &mockLM{}
		})

		// Should be able to look up with lowercase
		lm, err := NewLM(ctx, "mixedcaseprovider/some-model")
		if err == nil {
			// This fails if model validation is strict and model not in catalog
			// But for this test we mainly care that provider lookup succeeded.
			// However, NewLM will fail on model validation.
			// So we must register the model too or check error type.
			// Let's check error. If it says "provider not registered", test fails.
			// If it says "model not supported", test passes (provider found).
			_ = lm // Silence unused variable error
		}

		if err != nil && strings.Contains(err.Error(), "provider 'mixedcaseprovider' not registered") {
			t.Errorf("expected provider to be found, but got error: %v", err)
		}
	})

	t.Run("SkipModelValidation", func(t *testing.T) {
		// Enable skip validation
		Configure(WithSkipModelValidation(true))

		// Register provider
		RegisterLM("skipprovider", func(model string) LM {
			// Use mockLM (no fields) as simple mock
			return &mockLM{}
		})

		// Try to create unknown model
		lm, err := NewLM(ctx, "skipprovider/unknown-model")
		if err != nil {
			t.Errorf("expected success when skipping validation, got error: %v", err)
		}
		if lm == nil {
			t.Error("expected LM to be created")
		}

		// Reset config
		Configure(WithSkipModelValidation(false))

		// Should fail now
		_, err = NewLM(ctx, "skipprovider/unknown-model-2")
		if err == nil {
			t.Error("expected error when validation is enabled")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()

		// NewLM should still work even with canceled context
		// (context is for future use, not currently enforced)
		lm, err := NewLM(canceledCtx, "testprovider/test-model")
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
	RegisterLM("openrouter", testLMFactory)
	if err := modelcatalog.RegisterModel(modelcatalog.Model{ID: "test-provider/test-model", Limits: modelcatalog.Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: modelcatalog.Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: modelcatalog.Metadata{Name: "Test Model", Family: "test"}}); err != nil {
		t.Fatalf("RegisterModel(test-provider/test-model) err = %v", err)
	}

	ctx := context.Background()
	collector := NewMemoryCollector(10)

	// Configure with collector
	ResetConfig()
	Configure(WithCollector(collector))

	// Create LM - should be wrapped automatically
	lm, err := NewLM(ctx, "test-provider/test-model")
	if err != nil {
		t.Fatalf("Failed to create LM: %v", err)
	}

	// Verify it works and produces history entries when collector is configured
	// Generate a response to trigger history collection
	result, err := lm.Generate(ctx, []Message{{Role: "user", Content: "test"}}, DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	// Verify that history was collected
	if collector.Count() == 0 {
		t.Error("Expected history to be collected when collector is configured")
	}

	// Verify the result is valid
	if result == nil {
		t.Error("Expected result to be non-nil")
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
	RegisterLM("openrouter", testLMFactory)
	if err := modelcatalog.RegisterModel(modelcatalog.Model{ID: "test-provider/test-model", Limits: modelcatalog.Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: modelcatalog.Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: modelcatalog.Metadata{Name: "Test Model", Family: "test"}}); err != nil {
		t.Fatalf("RegisterModel(test-provider/test-model) err = %v", err)
	}

	ctx := context.Background()

	// Configure without collector
	ResetConfig()

	// Create LM - should still work without collector
	lm, err := NewLM(ctx, "test-provider/test-model")
	if err != nil {
		t.Fatalf("Failed to create LM: %v", err)
	}

	// Verify it works but does NOT produce history entries when no collector is configured
	// Generate a response - should not panic
	result, err := lm.Generate(ctx, []Message{{Role: "user", Content: "test"}}, DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	// Verify the result is valid
	if result == nil {
		t.Error("Expected result to be non-nil")
	}

	// Note: We can't easily verify that no history is collected since there's no collector configured
}

// TestNewLM_WithCache tests that cache is auto-wired when configured
func TestNewLM_WithCache(t *testing.T) {
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

	// Register a test LM that supports caching
	RegisterLM("test-provider", func(model string) LM {
		return NewMockLM()
	})
	RegisterLM("openrouter", func(model string) LM {
		return NewMockLM()
	})
	if err := modelcatalog.RegisterModel(modelcatalog.Model{ID: "test-provider/test-model", Limits: modelcatalog.Limits{ContextTokens: 1, OutputTokens: 1}, Modalities: modelcatalog.Modalities{Input: []string{"text"}, Output: []string{"text"}}, Metadata: modelcatalog.Metadata{Name: "Test Model", Family: "test"}}); err != nil {
		t.Fatalf("RegisterModel(test-provider/test-model) err = %v", err)
	}

	ctx := context.Background()

	// Configure with cache
	ResetConfig()
	Configure(WithCache(50))

	// Create LM - should have cache auto-wired
	lm, err := NewLM(ctx, "test-provider/test-model")
	if err != nil {
		t.Fatalf("Failed to create LM: %v", err)
	}

	// Verify LM was created
	if lm == nil {
		t.Error("Expected LM to be created")
	}

	// Get settings to verify cache is set
	settings := GetSettings()
	if settings.DefaultCache == nil {
		t.Error("Expected cache to be configured in settings")
	}
}

// TestNewLM_WithModelStringArg tests NewLM with explicit model string
func TestNewLM_WithModelStringArg(t *testing.T) {
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

	// Register providers
	RegisterLM("openai", func(model string) LM {
		return NewMockLM()
	})
	RegisterLM("openrouter", func(model string) LM {
		return NewMockLM()
	})

	ctx := context.Background()

	tests := []struct {
		name      string
		model     string
		wantError bool
	}{
		{"explicit openai gpt-4o", "openai/gpt-4o", false},
		{"explicit openai gpt-4o-mini", "openai/gpt-4o-mini", false},
		{"explicit meta model via openrouter", "openrouter/meta-llama/llama-3.3-70b-instruct:free", false},
		{"explicit unknown model", "unknownprovider/model", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lm, err := NewLM(ctx, tt.model)
			if (err != nil) != tt.wantError {
				t.Errorf("NewLM() error = %v, wantError = %v", err, tt.wantError)
			}
			if !tt.wantError && lm == nil {
				t.Error("Expected LM to be created")
			}
		})
	}
}
