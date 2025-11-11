package cost

import "strings"

// ModelPricing represents the pricing for a model
type ModelPricing struct {
	PromptPrice     float64 // Price per 1M prompt tokens (USD)
	CompletionPrice float64 // Price per 1M completion tokens (USD)
}

// defaultPricing contains pricing for common models
var defaultPricing = map[string]ModelPricing{
	// OpenAI models
	"openai/gpt-oss-120b:exacto": {
		PromptPrice:     0.05,
		CompletionPrice: 0.24,
	},
	"openai/gpt-4o": {
		PromptPrice:     2.5,
		CompletionPrice: 10,
	},
	"openai/gpt-4o-mini": {
		PromptPrice:     0.15,
		CompletionPrice: 0.60,
	},
	"gpt-3.5-turbo": {
		PromptPrice:     0.50,
		CompletionPrice: 1.50,
	},
	"o1-preview": {
		PromptPrice:     15.00,
		CompletionPrice: 60.00,
	},
	"o1-mini": {
		PromptPrice:     3.00,
		CompletionPrice: 12.00,
	},
	// DeepSeek models
	"deepseek/deepseek-v3.1-terminus": {
		PromptPrice:     0.23,
		CompletionPrice: 0.90,
	},
	// Z-AI models
	"z-ai/glm-4.6:exacto": {
		PromptPrice:     0.60,
		CompletionPrice: 1.90,
	},
	// Minimax models
	"minimax/minimax-m2:free": {
		PromptPrice:     0.00,
		CompletionPrice: 0.00,
	},
	// Meta models
	"meta/llama-3.1-405b": {
		PromptPrice:     2.70,
		CompletionPrice: 2.70,
	},
	"meta/llama-3.1-70b": {
		PromptPrice:     0.35,
		CompletionPrice: 0.40,
	},
	"meta/llama-3.1-8b": {
		PromptPrice:     0.06,
		CompletionPrice: 0.06,
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

// SetModelPricing sets custom pricing for a model
func (c *Calculator) SetModelPricing(model string, pricing ModelPricing) {
	c.pricing[model] = pricing
}

// Calculate calculates the cost for the given usage
// Returns cost in USD
func (c *Calculator) Calculate(model string, promptTokens, completionTokens int) float64 {
	pricing, ok := c.pricing[model]
	if !ok {
		// Try to find a match by prefix or partial match
		pricing = c.findPricingByPattern(model)
	}

	promptCost := float64(promptTokens) * pricing.PromptPrice / 1_000_000
	completionCost := float64(completionTokens) * pricing.CompletionPrice / 1_000_000

	return promptCost + completionCost
}

// findPricingByPattern attempts to find pricing by matching model name patterns
func (c *Calculator) findPricingByPattern(model string) ModelPricing {
	modelLower := strings.ToLower(model)

	// Try exact match first
	if pricing, ok := c.pricing[model]; ok {
		return pricing
	}

	// Try to find by prefix or contains
	for key, pricing := range c.pricing {
		keyLower := strings.ToLower(key)
		if strings.Contains(modelLower, keyLower) || strings.Contains(keyLower, modelLower) {
			return pricing
		}
	}

	// No match found - return zero cost
	return ModelPricing{}
}

// HasPricing checks if pricing is available for a model
func (c *Calculator) HasPricing(model string) bool {
	if _, ok := c.pricing[model]; ok {
		return true
	}
	return c.findPricingByPattern(model).PromptPrice > 0 || c.findPricingByPattern(model).CompletionPrice > 0
}

// GetPricing returns the pricing for a model
func (c *Calculator) GetPricing(model string) (ModelPricing, bool) {
	if pricing, ok := c.pricing[model]; ok {
		return pricing, true
	}
	pricing := c.findPricingByPattern(model)
	if pricing.PromptPrice > 0 || pricing.CompletionPrice > 0 {
		return pricing, true
	}
	return ModelPricing{}, false
}

// DefaultCalculator is the global default calculator instance
var DefaultCalculator = NewCalculator()

// Calculate is a convenience function using the default calculator
func Calculate(model string, promptTokens, completionTokens int) float64 {
	return DefaultCalculator.Calculate(model, promptTokens, completionTokens)
}
