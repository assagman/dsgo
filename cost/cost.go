package cost

import (
	"strings"
	"sync"

	"github.com/assagman/dsgo/modelcatalog"
)

// Pricing is an alias for modelcatalog.Pricing for convenience.
type Pricing = modelcatalog.Pricing

// Calculator calculates costs for LM usage.
//
// It is safe for concurrent use via its methods.
// Callers should not mutate underlying maps directly.
type Calculator struct {
	mu        sync.RWMutex
	overrides map[string]Pricing
}

// NewCalculator creates a new cost calculator.
func NewCalculator() *Calculator {
	return &Calculator{overrides: make(map[string]Pricing)}
}

// SetModelPricing sets custom pricing for a model (overrides catalog pricing).
func (c *Calculator) SetModelPricing(model string, pricing Pricing) {
	modelKey := normalizeModelKey(model)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.overrides == nil {
		c.overrides = make(map[string]Pricing)
	}
	c.overrides[modelKey] = pricing
}

// Calculate calculates the cost for the given usage.
// Returns cost in USD.
func (c *Calculator) Calculate(model string, promptTokens, completionTokens int) float64 {
	pricing, ok := c.GetPricing(model)
	if !ok {
		return 0
	}

	promptCost := float64(promptTokens) * pricing.PromptPrice / 1_000_000
	completionCost := float64(completionTokens) * pricing.CompletionPrice / 1_000_000
	return promptCost + completionCost
}

// GetPricing returns the pricing for a model.
// First checks overrides, then falls back to modelcatalog.
func (c *Calculator) GetPricing(model string) (Pricing, bool) {
	modelKey := normalizeModelKey(model)

	c.mu.RLock()
	if override, ok := c.overrides[modelKey]; ok {
		c.mu.RUnlock()
		return override, true
	}
	c.mu.RUnlock()

	return modelcatalog.GetPricing(modelKey)
}

// HasPricing checks if pricing is available for a model.
func (c *Calculator) HasPricing(model string) bool {
	_, ok := c.GetPricing(model)
	return ok
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
