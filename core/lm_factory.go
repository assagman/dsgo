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

// NewLM creates a new LM instance with automatic provider detection.
// It can work in two modes:
// 1. With model parameter: auto-detects provider from model name (e.g., "google/gemini-2.5-flash" uses openrouter)
// 2. Without model (empty string): uses DefaultProvider and DefaultModel from global settings
//
// The method automatically:
// - Strips provider prefixes (e.g., "openrouter/google/gemini" -> "google/gemini")
// - Detects the correct provider (openai for gpt-*, openrouter for vendor/model format)
// - Wraps with LMWrapper if a Collector is configured for observability
//
// Examples:
//   - NewLM(ctx, "google/gemini-2.5-flash") -> auto-uses openrouter
//   - NewLM(ctx, "gpt-4") -> auto-uses openai
//   - NewLM(ctx, "") -> uses global settings
func NewLM(ctx context.Context, model ...string) (LM, error) {
	settings := GetSettings()

	var targetModel string
	var provider string

	// Determine model and provider
	if len(model) > 0 && model[0] != "" {
		// Model specified - auto-detect provider
		targetModel = stripProviderPrefix(model[0])
		provider = detectProvider(targetModel)
	} else {
		// Use global settings
		if settings.DefaultProvider == "" {
			return nil, fmt.Errorf("no default provider configured (use dsgo.Configure with dsgo.WithProvider)")
		}
		if settings.DefaultModel == "" {
			return nil, fmt.Errorf("no default model configured (use dsgo.Configure with dsgo.WithModel)")
		}
		provider = settings.DefaultProvider
		targetModel = settings.DefaultModel
	}

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

// detectProvider automatically detects which provider to use based on the model name.
// Returns the provider name that should be used.
func detectProvider(model string) string {
	// OpenAI models (gpt-*, o1-*, etc.)
	if strings.HasPrefix(model, "gpt-") ||
		strings.HasPrefix(model, "o1-") ||
		strings.HasPrefix(model, "text-") ||
		strings.HasPrefix(model, "davinci-") {
		return "openai"
	}

	// All other models (with vendor prefix like "google/", "anthropic/", etc.) use openrouter
	// This includes: google/gemini-*, anthropic/claude-*, meta-llama/*, etc.
	if strings.Contains(model, "/") {
		return "openrouter"
	}

	// Default to openrouter for unknown models
	return "openrouter"
}
