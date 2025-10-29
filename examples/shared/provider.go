package shared

import (
	"os"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/providers/openai"
	"github.com/assagman/dsgo/providers/openrouter"
)

// GetLM returns an LM based on environment variables
// Priority: OPENROUTER_API_KEY > OPENAI_API_KEY
// Usage:
//   lm := examples.GetLM("gpt-4") // Uses OpenAI
//   lm := examples.GetLM("anthropic/claude-3.5-sonnet") // Uses OpenRouter
func GetLM(model string) dsgo.LM {
	// Check for OpenRouter API key first
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		return openrouter.NewOpenRouter(model)
	}
	
	// Fall back to OpenAI
	if os.Getenv("OPENAI_API_KEY") != "" {
		return openai.NewOpenAI(model)
	}
	
	// Default to OpenAI (will fail if no key is set)
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
