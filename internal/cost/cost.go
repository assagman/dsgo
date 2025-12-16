package cost

import (
	"maps"
	"strings"

	"github.com/assagman/dsgo/internal/modelcatalog"
)

// PricingTier selects a provider pricing tier.
//
// Not all models support all tiers; when a tier is not available for a model,
// the calculator falls back to TierStandard.
type PricingTier string

const (
	TierStandard PricingTier = "standard"
	TierFlex     PricingTier = "flex"
	TierPriority PricingTier = "priority"
	TierBatch    PricingTier = "batch"
)

func normalizeTier(tier PricingTier) PricingTier {
	switch PricingTier(strings.ToLower(string(tier))) {
	case TierStandard:
		return TierStandard
	case TierFlex:
		return TierFlex
	case TierPriority:
		return TierPriority
	case TierBatch:
		return TierBatch
	default:
		return TierStandard
	}
}

// ModelPricing represents the pricing for a model.
//
// Prices are in USD per 1M tokens.
type ModelPricing struct {
	PromptPrice     float64 // Price per 1M prompt tokens (USD)
	CompletionPrice float64 // Price per 1M completion tokens (USD)
}

var defaultPricingByTier = map[PricingTier]map[string]ModelPricing{
	// OpenAI (source: https://platform.openai.com/docs/pricing)
	TierStandard: {
		"openai/gpt-4o": {
			PromptPrice:     2.50,
			CompletionPrice: 10.00,
		},
		"openai/gpt-4o-mini": {
			PromptPrice:     0.15,
			CompletionPrice: 0.60,
		},
		"openai/gpt-3.5-turbo": {
			PromptPrice:     0.50,
			CompletionPrice: 1.50,
		},
		"openai/o1": {
			PromptPrice:     15.00,
			CompletionPrice: 60.00,
		},
		"openai/o1-mini": {
			PromptPrice:     1.10,
			CompletionPrice: 4.40,
		},

		// OpenRouter curated list (source: https://openrouter.ai/models/*)
		"openrouter/google/gemini-2.5-flash": {
			PromptPrice:     0.30,
			CompletionPrice: 2.50,
		},
		"openrouter/google/gemini-2.5-flash-lite-preview-09-2025": {
			PromptPrice:     0.10,
			CompletionPrice: 0.40,
		},
		"openrouter/amazon/nova-2-lite-v1": {
			PromptPrice:     0.30,
			CompletionPrice: 2.50,
		},
		"openrouter/openai/gpt-4o-mini": {
			PromptPrice:     0.15,
			CompletionPrice: 0.60,
		},
		"openrouter/openai/gpt-5-mini-2025-08-07": {
			PromptPrice:     0.25,
			CompletionPrice: 2.00,
		},
		"openrouter/openai/gpt-5-nano-2025-08-07": {
			PromptPrice:     0.05,
			CompletionPrice: 0.40,
		},
		"openrouter/openai/gpt-4.1-2025-04-14": {
			PromptPrice:     2.00,
			CompletionPrice: 8.00,
		},
		"openrouter/openai/gpt-oss-120b:exacto": {
			PromptPrice:     0.039,
			CompletionPrice: 0.19,
		},
		"openrouter/anthropic/claude-3.5-sonnet": {
			PromptPrice:     6.00,
			CompletionPrice: 30.00,
		},
		"openrouter/anthropic/claude-haiku-4.5": {
			PromptPrice:     1.00,
			CompletionPrice: 5.00,
		},
		"openrouter/x-ai/grok-code-fast-1": {
			PromptPrice:     0.20,
			CompletionPrice: 1.50,
		},
		"openrouter/deepseek/deepseek-v3.2": {
			PromptPrice:     0.24,
			CompletionPrice: 0.38,
		},
		"openrouter/qwen/qwen3-next-80b-a3b-instruct": {
			PromptPrice:     0.09,
			CompletionPrice: 1.10,
		},
		"openrouter/z-ai/glm-4.6:exacto": {
			PromptPrice:     0.44,
			CompletionPrice: 1.76,
		},
		"openrouter/moonshotai/kimi-k2-0905:exacto": {
			PromptPrice:     0.60,
			CompletionPrice: 2.50,
		},
		"openrouter/qwen/qwen3-coder:exacto": {
			PromptPrice:     0.22,
			CompletionPrice: 1.80,
		},
		"openrouter/meta-llama/llama-3.1-8b-instruct": {
			PromptPrice:     0.02,
			CompletionPrice: 0.03,
		},
		"openrouter/meta-llama/llama-3.3-70b-instruct": {
			PromptPrice:     0.10,
			CompletionPrice: 0.32,
		},

		// Mock provider - for testing purposes, uses OpenAI-equivalent pricing
		"mock/gpt-4o-mini": {
			PromptPrice:     0.15,
			CompletionPrice: 0.60,
		},
		"mock/gpt-4o": {
			PromptPrice:     2.50,
			CompletionPrice: 10.00,
		},
	},

	// Batch rates (where available).
	TierBatch: {
		"openai/gpt-4o": {
			PromptPrice:     1.25,
			CompletionPrice: 5.00,
		},
		"openai/gpt-4o-mini": {
			PromptPrice:     0.075,
			CompletionPrice: 0.30,
		},
	},

	// Priority rates (where available).
	TierPriority: {
		"openai/gpt-4o": {
			PromptPrice:     4.25,
			CompletionPrice: 17.00,
		},
		"openai/gpt-4o-mini": {
			PromptPrice:     0.25,
			CompletionPrice: 1.00,
		},
	},

	// Flex rates (where available). Currently empty for our curated set.
	TierFlex: {},
}

// Calculator calculates costs for LM usage.
//
// It is safe for concurrent reads once constructed, but callers should not mutate
// pricing maps without external synchronization.
type Calculator struct {
	pricingByTier map[PricingTier]map[string]ModelPricing
}

// NewCalculator creates a new cost calculator with the default pricing tables.
func NewCalculator() *Calculator {
	pricing := make(map[PricingTier]map[string]ModelPricing, len(defaultPricingByTier))
	for tier, tierMap := range defaultPricingByTier {
		copyTier := make(map[string]ModelPricing, len(tierMap))
		maps.Copy(copyTier, tierMap)
		pricing[tier] = copyTier
	}
	return &Calculator{pricingByTier: pricing}
}

// SetModelPricing sets custom pricing for a model on the TierStandard tier.
func (c *Calculator) SetModelPricing(model string, pricing ModelPricing) {
	c.SetModelPricingForTier(model, TierStandard, pricing)
}

// SetModelPricingForTier sets custom pricing for a model on a specific tier.
func (c *Calculator) SetModelPricingForTier(model string, tier PricingTier, pricing ModelPricing) {
	tier = normalizeTier(tier)
	modelKey := normalizeModelKey(model)

	if c.pricingByTier == nil {
		c.pricingByTier = make(map[PricingTier]map[string]ModelPricing)
	}
	if c.pricingByTier[tier] == nil {
		c.pricingByTier[tier] = make(map[string]ModelPricing)
	}
	c.pricingByTier[tier][modelKey] = pricing
}

// Calculate calculates the cost for the given usage using TierStandard.
// Returns cost in USD.
func (c *Calculator) Calculate(model string, promptTokens, completionTokens int) float64 {
	return c.CalculateWithTier(model, TierStandard, promptTokens, completionTokens)
}

// CalculateWithTier calculates the cost for the given usage using the given tier.
// Returns cost in USD.
func (c *Calculator) CalculateWithTier(model string, tier PricingTier, promptTokens, completionTokens int) float64 {
	tier = normalizeTier(tier)
	modelKey := normalizeModelKey(model)

	pricing, ok := c.getPricingExact(modelKey, tier)
	if !ok {
		// Fall back to standard tier if the requested tier isn't available.
		pricing, ok = c.getPricingExact(modelKey, TierStandard)
	}
	if !ok {
		return 0
	}

	promptCost := float64(promptTokens) * pricing.PromptPrice / 1_000_000
	completionCost := float64(completionTokens) * pricing.CompletionPrice / 1_000_000
	return promptCost + completionCost
}

func (c *Calculator) getPricingExact(model string, tier PricingTier) (ModelPricing, bool) {
	if c.pricingByTier == nil {
		return ModelPricing{}, false
	}
	tierMap, ok := c.pricingByTier[tier]
	if !ok {
		return ModelPricing{}, false
	}
	pricing, ok := tierMap[model]
	return pricing, ok
}

// HasPricing checks if pricing is available for a model on TierStandard.
func (c *Calculator) HasPricing(model string) bool {
	_, ok := c.GetPricing(model)
	return ok
}

// HasPricingForTier checks if pricing is available for a model on the given tier.
// If the tier is not available, TierStandard is used as a fallback.
func (c *Calculator) HasPricingForTier(model string, tier PricingTier) bool {
	_, ok := c.GetPricingForTier(model, tier)
	return ok
}

// GetPricing returns the TierStandard pricing for a model.
func (c *Calculator) GetPricing(model string) (ModelPricing, bool) {
	return c.GetPricingForTier(model, TierStandard)
}

// GetPricingForTier returns the pricing for a model on the given tier.
// If the tier is not available, TierStandard is used as a fallback.
func (c *Calculator) GetPricingForTier(model string, tier PricingTier) (ModelPricing, bool) {
	tier = normalizeTier(tier)
	modelKey := normalizeModelKey(model)

	pricing, ok := c.getPricingExact(modelKey, tier)
	if ok {
		return pricing, true
	}
	if tier != TierStandard {
		pricing, ok = c.getPricingExact(modelKey, TierStandard)
		if ok {
			return pricing, true
		}
	}
	return ModelPricing{}, false
}

func normalizeModelKey(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}

	if canonical, ok := modelcatalog.Resolve(model); ok {
		return canonical
	}

	return strings.ToLower(model)
}

// DefaultCalculator is the global default calculator instance.
var DefaultCalculator = NewCalculator()

// Calculate is a convenience function using the default calculator.
func Calculate(model string, promptTokens, completionTokens int) float64 {
	return DefaultCalculator.Calculate(model, promptTokens, completionTokens)
}

// CalculateWithTier is a convenience function using the default calculator.
func CalculateWithTier(model string, tier PricingTier, promptTokens, completionTokens int) float64 {
	return DefaultCalculator.CalculateWithTier(model, tier, promptTokens, completionTokens)
}
