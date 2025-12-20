package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/assagman/dsgo/modelcatalog"
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

	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return
	}

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
//   - NewLM(ctx, "openrouter/meta-llama/llama-3.3-70b-instruct") -> uses openrouter provider with model "meta-llama/llama-3.3-70b-instruct"
func NewLM(ctx context.Context, model string) (LM, error) {
	if model == "" {
		return nil, fmt.Errorf("model string is required - provide a valid model like 'openai/gpt-4o' or 'openrouter/google/gemini-2.5-flash'. Example: core.NewLM(ctx, \"openai/gpt-4o\")")
	}

	// Parse provider and model from model string
	parts := strings.SplitN(model, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("model string must include provider: format 'provider/model' (e.g., 'openai/gpt-4o' or 'openrouter/google/gemini-2.5-flash'). Example: core.NewLM(ctx, \"openai/gpt-4o\")")
	}

	provider := strings.ToLower(parts[0])
	targetModel := parts[1]

	// Get factory for provider
	registryLock.RLock()
	factory, ok := lmRegistry[provider]
	registryLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider '%s' not registered for model '%s'. Available providers: %v. Example: core.NewLM(ctx, \"openai/gpt-4o\")", provider, targetModel, getRegisteredProviders())
	}

	canonicalModel := provider + "/" + targetModel

	// Check if model is valid (unless validation is skipped)
	if !GetSettings().SkipModelValidation && !modelcatalog.IsValidCanonical(canonicalModel) {
		candidates := modelcatalog.ListModelsByProvider(provider)
		if len(candidates) > 0 {
			max := 5
			if len(candidates) < max {
				max = len(candidates)
			}
			examples := make([]string, 0, max)
			for i := 0; i < max; i++ {
				examples = append(examples, candidates[i].ID)
			}
			return nil, fmt.Errorf("model '%s' is not supported. Use modelcatalog.ListModels() to see supported models. Examples for provider '%s': %v", canonicalModel, provider, examples)
		}
		return nil, fmt.Errorf("model '%s' is not supported. Use modelcatalog.RegisterModel() to add custom models, or modelcatalog.ListModels() to see supported models", canonicalModel)
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

	// Always wrap with lmWrapper so cost/latency work by default.
	// History collection remains a no-op when settings.Collector is nil.
	return newLMWrapperWithProvider(baseLM, settings.Collector, provider), nil
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
