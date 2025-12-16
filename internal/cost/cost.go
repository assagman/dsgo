package cost

import (
	"maps"
	"strings"
	"sync"

	"github.com/assagman/dsgo/internal/modelcatalog"
)

// ModelPricing represents the pricing for a model.
//
// Prices are in USD per 1M tokens.
type ModelPricing struct {
	PromptPrice     float64 // Price per 1M prompt tokens (USD)
	CompletionPrice float64 // Price per 1M completion tokens (USD)
}

var defaultPricing = map[string]ModelPricing{
	// OpenAI (source: models.dev API)
	"openai/gpt-4o":        {PromptPrice: 2.5, CompletionPrice: 10},
	"openai/gpt-4o-mini":   {PromptPrice: 0.15, CompletionPrice: 0.6},
	"openai/gpt-3.5-turbo": {PromptPrice: 0.5, CompletionPrice: 1.5},
	"openai/o1":            {PromptPrice: 15, CompletionPrice: 60},
	"openai/o1-mini":       {PromptPrice: 1.1, CompletionPrice: 4.4},
	"openai/o3-mini":       {PromptPrice: 1.1, CompletionPrice: 4.4},
	"openai/o4-mini":       {PromptPrice: 1.1, CompletionPrice: 4.4},
	"openai/gpt-4.1":       {PromptPrice: 2, CompletionPrice: 8},
	"openai/gpt-4.1-mini":  {PromptPrice: 0.4, CompletionPrice: 1.6},
	"openai/gpt-4.1-nano":  {PromptPrice: 0.1, CompletionPrice: 0.4},
	"openai/gpt-5":         {PromptPrice: 1.25, CompletionPrice: 10},
	"openai/gpt-5-mini":    {PromptPrice: 0.25, CompletionPrice: 2},
	"openai/gpt-5-nano":    {PromptPrice: 0.05, CompletionPrice: 0.4},
	"openai/gpt-4-turbo":   {PromptPrice: 10, CompletionPrice: 30},

	// OpenRouter (source: models.dev API)
	"openrouter/google/gemini-2.5-flash":                {PromptPrice: 0.3, CompletionPrice: 2.5},
	"openrouter/google/gemini-2.5-pro":                  {PromptPrice: 1.25, CompletionPrice: 10},
	"openrouter/google/gemini-2.5-flash-lite":           {PromptPrice: 0.1, CompletionPrice: 0.4},
	"openrouter/openai/gpt-4o-mini":                     {PromptPrice: 0.15, CompletionPrice: 0.6},
	"openrouter/openai/gpt-5":                           {PromptPrice: 1.25, CompletionPrice: 10},
	"openrouter/openai/gpt-5-mini":                      {PromptPrice: 0.25, CompletionPrice: 2},
	"openrouter/openai/gpt-5-nano":                      {PromptPrice: 0.05, CompletionPrice: 0.4},
	"openrouter/openai/gpt-4.1":                         {PromptPrice: 2, CompletionPrice: 8},
	"openrouter/anthropic/claude-sonnet-4":              {PromptPrice: 3, CompletionPrice: 15},
	"openrouter/anthropic/claude-haiku-4.5":             {PromptPrice: 1, CompletionPrice: 5},
	"openrouter/anthropic/claude-opus-4":                {PromptPrice: 15, CompletionPrice: 75},
	"openrouter/x-ai/grok-4":                            {PromptPrice: 3, CompletionPrice: 15},
	"openrouter/x-ai/grok-code-fast-1":                  {PromptPrice: 0.2, CompletionPrice: 1.5},
	"openrouter/deepseek/deepseek-v3.2":                 {PromptPrice: 0.28, CompletionPrice: 0.4},
	"openrouter/deepseek/deepseek-chat-v3.1":            {PromptPrice: 0.2, CompletionPrice: 0.8},
	"openrouter/qwen/qwen3-coder":                       {PromptPrice: 0.3, CompletionPrice: 1.2},
	"openrouter/qwen/qwen3-235b-a22b-07-25":             {PromptPrice: 0.15, CompletionPrice: 0.85},
	"openrouter/moonshotai/kimi-k2":                     {PromptPrice: 0.55, CompletionPrice: 2.2},
	"openrouter/z-ai/glm-4.6":                           {PromptPrice: 0.6, CompletionPrice: 2.2},
	"openrouter/meta-llama/llama-3.3-70b-instruct:free": {PromptPrice: 0, CompletionPrice: 0},

	// Mock provider - for testing purposes, uses OpenAI-equivalent pricing
	"mock/gpt-4o-mini": {PromptPrice: 0.15, CompletionPrice: 0.6},
	"mock/gpt-4o":      {PromptPrice: 2.5, CompletionPrice: 10},
}

// Calculator calculates costs for LM usage.
//
// It is safe for concurrent use via its methods.
// Callers should not mutate underlying maps directly.
type Calculator struct {
	mu      sync.RWMutex
	pricing map[string]ModelPricing
}

// NewCalculator creates a new cost calculator with the default pricing tables.
func NewCalculator() *Calculator {
	pricing := make(map[string]ModelPricing, len(defaultPricing))
	maps.Copy(pricing, defaultPricing)
	return &Calculator{pricing: pricing}
}

// SetModelPricing sets custom pricing for a model.
func (c *Calculator) SetModelPricing(model string, pricing ModelPricing) {
	modelKey := normalizeModelKey(model)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pricing == nil {
		c.pricing = make(map[string]ModelPricing)
	}
	c.pricing[modelKey] = pricing
}

// Calculate calculates the cost for the given usage.
// Returns cost in USD.
func (c *Calculator) Calculate(model string, promptTokens, completionTokens int) float64 {
	modelKey := normalizeModelKey(model)

	pricing, ok := c.getPricing(modelKey)
	if !ok {
		return 0
	}

	promptCost := float64(promptTokens) * pricing.PromptPrice / 1_000_000
	completionCost := float64(completionTokens) * pricing.CompletionPrice / 1_000_000
	return promptCost + completionCost
}

func (c *Calculator) getPricing(model string) (ModelPricing, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.pricing == nil {
		return ModelPricing{}, false
	}
	pricing, ok := c.pricing[model]
	return pricing, ok
}

// HasPricing checks if pricing is available for a model.
func (c *Calculator) HasPricing(model string) bool {
	_, ok := c.GetPricing(model)
	return ok
}

// GetPricing returns the pricing for a model.
func (c *Calculator) GetPricing(model string) (ModelPricing, bool) {
	modelKey := normalizeModelKey(model)
	return c.getPricing(modelKey)
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
