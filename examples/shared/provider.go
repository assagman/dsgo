package shared

import (
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/providers/openai"
	"github.com/assagman/dsgo/providers/openrouter"
)

// GetLM returns an LM based on model name prefix
// Usage:
//
//	lm := examples.GetLM("gpt-3.5-turbo") // Uses OpenAI with "gpt-3.5-turbo"
//	lm := examples.GetLM("openrouter/minimax/minimax-m2") // Uses OpenRouter with "minimax/minimax-m2"
//	lm := examples.GetLM("openrouter/openai/gpt-3.5-turbo") // Uses OpenRouter with "openai/gpt-3.5-turbo"
func GetLM(model string) dsgo.LM {
	// Check if model name starts with "openrouter/"
	if strings.HasPrefix(model, "openrouter/") {
		// Strip "openrouter/" prefix
		actualModel := strings.TrimPrefix(model, "openrouter/")
		return openrouter.NewOpenRouter(actualModel)
	}

	// Check if model name starts with "openai/"
	if strings.HasPrefix(model, "openai/") {
		// Strip "openai/" prefix
		actualModel := strings.TrimPrefix(model, "openai/")
		return openai.NewOpenAI(actualModel)
	}

	// Default to OpenAI without prefix stripping
	return openai.NewOpenAI(model)
}

// GetOpenAI explicitly returns an OpenAI LM
func GetOpenAI(model string) dsgo.LM {
	return openai.NewOpenAI(model)
}

// GetOpenRouter explicitly returns an OpenRouter LM
func GetOpenRouter(model string) dsgo.LM {
	return openrouter.NewOpenRouter(model)
}
