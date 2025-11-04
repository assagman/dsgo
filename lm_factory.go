package dsgo

import (
	"context"
	"fmt"
	"sync"
)

// LMFactory is a function that creates an LM instance for a given model.
type LMFactory func(model string) LM

var (
	lmRegistry   = make(map[string]LMFactory)
	registryLock sync.RWMutex
)

// RegisterLM registers an LM factory for a specific provider.
// This should be called by provider packages during init.
func RegisterLM(provider string, factory LMFactory) {
	registryLock.Lock()
	defer registryLock.Unlock()
	lmRegistry[provider] = factory
}

// NewLM creates a new LM instance based on the global settings.
// It reads DefaultProvider and DefaultModel from settings and uses the registered factory.
// If a Collector is configured, the LM is automatically wrapped with LMWrapper for observability.
// Returns an error if the provider is not set or not registered.
func NewLM(ctx context.Context) (LM, error) {
	settings := GetSettings()

	if settings.DefaultProvider == "" {
		return nil, fmt.Errorf("no default provider configured (use dsgo.Configure with dsgo.WithProvider)")
	}

	registryLock.RLock()
	factory, ok := lmRegistry[settings.DefaultProvider]
	registryLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider '%s' not registered (available: %v)", settings.DefaultProvider, getRegisteredProviders())
	}

	model := settings.DefaultModel
	if model == "" {
		return nil, fmt.Errorf("no default model configured for provider '%s' (use dsgo.Configure with dsgo.WithModel)", settings.DefaultProvider)
	}

	baseLM := factory(model)

	// Automatically wrap with LMWrapper if a Collector is configured
	if settings.Collector != nil {
		return NewLMWrapper(baseLM, settings.Collector), nil
	}

	return baseLM, nil
}

// getRegisteredProviders returns a list of registered provider names.
func getRegisteredProviders() []string {
	registryLock.RLock()
	defer registryLock.RUnlock()

	providers := make([]string, 0, len(lmRegistry))
	for p := range lmRegistry {
		providers = append(providers, p)
	}
	return providers
}
