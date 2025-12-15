package cost

import "strings"

// ModelPricing represents the pricing for a model
type ModelPricing struct {
	PromptPrice     float64 // Price per 1M prompt tokens (USD)
	CompletionPrice float64 // Price per 1M completion tokens (USD)
}

// defaultPricing contains pricing for common models.
// Keys use the format "provider/model" where provider is the routing provider
// (e.g., "openai", "openrouter") and model is the model identifier.
//
// This explicit format ensures accurate pricing when the same model is available
// through different providers with different pricing (e.g., openrouter/google/gemini-2.5-flash
// may have different costs).
var defaultPricing = map[string]ModelPricing{
	// OpenAI provider - direct API access
	"openai/gpt-oss-120b:exacto": {
		PromptPrice:     0.05,
		CompletionPrice: 0.24,
	},
	"openai/gpt-4": {
		PromptPrice:     30.00,
		CompletionPrice: 60.00,
	},
	"openai/gpt-4o": {
		PromptPrice:     2.5,
		CompletionPrice: 10,
	},
	"openai/gpt-4o-mini": {
		PromptPrice:     0.15,
		CompletionPrice: 0.60,
	},
	"openai/gpt-3.5-turbo": {
		PromptPrice:     0.50,
		CompletionPrice: 1.50,
	},
	"openai/o1-preview": {
		PromptPrice:     15.00,
		CompletionPrice: 60.00,
	},
	"openai/o1-mini": {
		PromptPrice:     3.00,
		CompletionPrice: 12.00,
	},

	// OpenRouter provider - pricing reflects OpenRouter's rates
	"openrouter/deepseek/deepseek-v3.1-terminus": {
		PromptPrice:     0.23,
		CompletionPrice: 0.90,
	},
	"openrouter/z-ai/glm-4.6:exacto": {
		PromptPrice:     0.60,
		CompletionPrice: 1.90,
	},
	"openrouter/minimax/minimax-m2:free": {
		PromptPrice:     0.00,
		CompletionPrice: 0.00,
	},
	"openrouter/google/gemini-2.5-flash": {
		PromptPrice:     0.30,
		CompletionPrice: 2.50,
	},
	"openrouter/meta/llama-3.1-405b": {
		PromptPrice:     2.70,
		CompletionPrice: 2.70,
	},
	"openrouter/meta/llama-3.1-70b": {
		PromptPrice:     0.35,
		CompletionPrice: 0.40,
	},
	"openrouter/meta/llama-3.1-8b": {
		PromptPrice:     0.06,
		CompletionPrice: 0.06,
	},

	// Mock provider - for testing purposes, uses OpenAI-equivalent pricing
	"mock/gpt-4o-mini": {
		PromptPrice:     0.15,
		CompletionPrice: 0.60,
	},

	// OpenRouter provider - OpenAI models accessed via OpenRouter
	// Note: Pricing may differ from direct OpenAI access
	"openrouter/openai/gpt-4": {
		PromptPrice:     30.00,
		CompletionPrice: 60.00,
	},
	"openrouter/openai/gpt-4o": {
		PromptPrice:     2.5,
		CompletionPrice: 10,
	},
	"openrouter/openai/gpt-4o-mini": {
		PromptPrice:     0.15,
		CompletionPrice: 0.60,
	},
}

// Calculator calculates costs for LM usage
type Calculator struct {
	pricing map[string]ModelPricing
}

// NewCalculator creates a new cost calculator
func NewCalculator() *Calculator {
	// Copy default pricing
	pricing := make(map[string]ModelPricing)
	for k, v := range defaultPricing {
		pricing[k] = v
	}
	return &Calculator{
		pricing: pricing,
	}
}

// SetModelPricing sets custom pricing for a provider/model combination.
// The key should be in format "provider/model" (e.g., "openrouter/google/gemini-2.5-flash").
func (c *Calculator) SetModelPricing(key string, pricing ModelPricing) {
	c.pricing[key] = pricing
}

// buildKey constructs a pricing lookup key from provider and model.
// Returns "provider/model" format for consistent lookups.
func buildKey(provider, model string) string {
	if provider == "" {
		return model
	}
	return provider + "/" + model
}

// Calculate calculates the cost for the given usage.
// The provider parameter specifies the routing provider (e.g., "openai", "openrouter").
// The model parameter is the model identifier (e.g., "gpt-4o", "google/gemini-2.5-flash").
// Returns cost in USD.
//
// If provider is empty, pricing is resolved across all providers by pattern matching
// for backwards compatibility. Prefer passing an explicit provider for correct,
// provider-specific pricing.
//
// Note: If the model has no pricing information, this returns 0.
func (c *Calculator) Calculate(provider, model string, promptTokens, completionTokens int) float64 {
	key := buildKey(provider, model)
	pricing, ok := c.pricing[key]
	if !ok {
		// Try to find a match by prefix or partial match
		pricing = c.findPricingByPattern(provider, model)
	}

	promptCost := float64(promptTokens) * pricing.PromptPrice / 1_000_000
	completionCost := float64(completionTokens) * pricing.CompletionPrice / 1_000_000

	return promptCost + completionCost
}

// CalculateIfKnown calculates cost only when pricing is available.
// The provider parameter specifies the routing provider (e.g., "openai", "openrouter").
// The model parameter is the model identifier (e.g., "gpt-4o", "google/gemini-2.5-flash").
//
// If provider is empty, pricing is resolved across all providers by pattern matching.
//
// It returns (cost, true) when the model's pricing is known (including free models
// with 0 pricing), and (0, false) when the model is unknown.
func (c *Calculator) CalculateIfKnown(provider, model string, promptTokens, completionTokens int) (float64, bool) {
	pricing, ok := c.GetPricing(provider, model)
	if !ok {
		return 0, false
	}

	promptCost := float64(promptTokens) * pricing.PromptPrice / 1_000_000
	completionCost := float64(completionTokens) * pricing.CompletionPrice / 1_000_000

	return promptCost + completionCost, true
}

// extractMatchTarget extracts the model portion from a pricing key for comparison.
// If providerPrefix is set, it strips that prefix. Otherwise, it extracts everything
// after the first "/" (e.g., "openai/gpt-4o" → "gpt-4o").
func extractMatchTarget(pricingKeyLower, providerPrefix string) string {
	if providerPrefix != "" {
		return strings.TrimPrefix(pricingKeyLower, providerPrefix)
	}
	parts := strings.SplitN(pricingKeyLower, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return pricingKeyLower
}

// findPricingByPattern attempts to find pricing by matching provider/model patterns.
// It falls back to pattern matching within the same provider's models (if provider is specified)
// or across all providers (if provider is empty).
func (c *Calculator) findPricingByPattern(provider, model string) ModelPricing {
	modelLower := strings.ToLower(model)

	// Build provider prefix for scoped matching
	providerPrefix := ""
	if provider != "" {
		providerPrefix = strings.ToLower(provider) + "/"
	}

	// First pass: try exact matches within the provider scope
	for pricingKey, pricing := range c.pricing {
		pricingKeyLower := strings.ToLower(pricingKey)

		// If provider is specified, only match within that provider's namespace
		if providerPrefix != "" && !strings.HasPrefix(pricingKeyLower, providerPrefix) {
			continue
		}

		matchTarget := extractMatchTarget(pricingKeyLower, providerPrefix)

		// Check for exact match first
		if matchTarget == modelLower {
			return pricing
		}
	}

	// Second pass: try prefix matches or contains matches - find the best matching model
	// Case A: incoming "gpt-4o-something" could match "gpt-4o" (incoming is longer)
	// Case B: incoming "gpt-4" could match "openai/gpt-4" (incoming is contained in match target)
	// Prefer longer/more specific matches over shorter ones
	var bestMatch ModelPricing
	var longestMatchLen int

	for pricingKey, pricing := range c.pricing {
		pricingKeyLower := strings.ToLower(pricingKey)

		// If provider is specified, only match within that provider's namespace
		if providerPrefix != "" && !strings.HasPrefix(pricingKeyLower, providerPrefix) {
			continue
		}

		matchTarget := extractMatchTarget(pricingKeyLower, providerPrefix)

		// Case A: Incoming model is longer - check if it starts with a known model
		// e.g., incoming "gpt-4o-something" starts with "gpt-4o"
		if len(matchTarget) < len(modelLower) && strings.HasPrefix(modelLower, matchTarget) {
			// Track the longest match for maximum specificity
			if len(matchTarget) > longestMatchLen {
				longestMatchLen = len(matchTarget)
				bestMatch = pricing
			}
			continue
		}

		// Case B: Incoming model is an exact component match in the multi-component target
		// e.g., incoming "gpt-4" should match target "openai/gpt-4" (after provider prefix is stripped)
		// Split matchTarget by / and check if any component matches exactly
		if strings.Contains(matchTarget, "/") {
			parts := strings.Split(matchTarget, "/")
			for _, part := range parts {
				if part == modelLower {
					// Exact component match - prefer longer targets for specificity
					if len(matchTarget) > longestMatchLen {
						longestMatchLen = len(matchTarget)
						bestMatch = pricing
					}
					break
				}
			}
		}
	}

	if longestMatchLen > 0 {
		return bestMatch
	}

	// No match found - return zero cost
	return ModelPricing{}
}

// HasPricing checks if pricing is available for a provider/model combination.
func (c *Calculator) HasPricing(provider, model string) bool {
	key := buildKey(provider, model)
	if _, ok := c.pricing[key]; ok {
		return true
	}
	pricing := c.findPricingByPattern(provider, model)
	return pricing.PromptPrice > 0 || pricing.CompletionPrice > 0
}

// GetPricing returns the pricing for a provider/model combination.
// If provider is empty, pricing is resolved across all providers by pattern matching.
func (c *Calculator) GetPricing(provider, model string) (ModelPricing, bool) {
	key := buildKey(provider, model)
	if pricing, ok := c.pricing[key]; ok {
		return pricing, true
	}
	pricing := c.findPricingByPattern(provider, model)
	if pricing.PromptPrice > 0 || pricing.CompletionPrice > 0 {
		return pricing, true
	}
	return ModelPricing{}, false
}

// DefaultCalculator is the global default calculator instance
var DefaultCalculator = NewCalculator()

// Calculate is a convenience function using the default calculator.
// The provider parameter specifies the routing provider (e.g., "openai", "openrouter").
// The model parameter is the model identifier (e.g., "gpt-4o", "google/gemini-2.5-flash").
func Calculate(provider, model string, promptTokens, completionTokens int) float64 {
	return DefaultCalculator.Calculate(provider, model, promptTokens, completionTokens)
}
