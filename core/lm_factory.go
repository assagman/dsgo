package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// LMFactory is a function that creates an LM instance for a given model.
type LMFactory func(model string) LM

var (
	lmRegistry   = make(map[string]LMFactory)
	registryLock sync.RWMutex
)

// RegisterLM registers an LM factory for a specific provider.
// This is called automatically during package initialization for built-in providers.
func RegisterLM(provider string, factory LMFactory) {
	registryLock.Lock()
	defer registryLock.Unlock()
	lmRegistry[provider] = factory
}

// NewLM creates a new LM instance with explicit provider specification in model string.
// Users must provide a valid model string that includes provider as first part.
//
// The model string format is: "provider/model" or "provider/org/model"
// - First part (before first slash) = provider name
// - Remaining parts = model name (may contain slashes)
//
// Examples:
//   - NewLM(ctx, "openai/gpt-4o") -> uses openai provider with model "gpt-4o"
//   - NewLM(ctx, "openrouter/z-ai/glm-4.6") -> uses openrouter provider with model "z-ai/glm-4.6"
//   - NewLM(ctx, "openrouter/google/gemini-2.5-flash") -> uses openrouter provider with model "google/gemini-2.5-flash"
func NewLM(ctx context.Context, model string) (LM, error) {
	if model == "" {
		return nil, fmt.Errorf("model string is required - provide a valid model like 'openai/gpt-4o' or 'openrouter/z-ai/glm-4.6'")
	}

	// Parse provider and model from model string
	parts := strings.SplitN(model, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("model string must include provider: format 'provider/model' (e.g., 'openai/gpt-4o' or 'openrouter/z-ai/glm-4.6')")
	}

	provider := parts[0]
	targetModel := parts[1]

	// Get factory for provider
	registryLock.RLock()
	factory, ok := lmRegistry[provider]
	registryLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider '%s' not registered for model '%s' (available: %v)", provider, targetModel, getRegisteredProviders())
	}

	// Create base LM
	baseLM := factory(targetModel)

	// Auto-wire cache if configured
	settings := GetSettings()
	if settings.DefaultCache != nil {
		// Use type assertion to check if provider supports SetCache
		if cacheableLM, ok := baseLM.(interface{ SetCache(Cache) }); ok {
			cacheableLM.SetCache(settings.DefaultCache)
		}
	}

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
