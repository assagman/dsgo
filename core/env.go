package core

import (
	"os"
	"strconv"
	"time"
)

// loadEnv loads configuration from environment variables.
// This is called automatically by Configure() before applying user options.
// Environment variables supported:
//   - DSGO_PROVIDER: Default provider name (e.g., "openai", "openrouter")
//   - DSGO_MODEL: Default model identifier (e.g., "gpt-4", "google/gemini-2.5-flash")
//   - DSGO_TIMEOUT: Default timeout in seconds (e.g., "30")
//   - DSGO_MAX_RETRIES: Default number of retries (e.g., "3")
//   - DSGO_TRACING: Enable tracing ("true" or "false")
//   - DSGO_OPENAI_API_KEY: OpenAI API key
//   - DSGO_OPENROUTER_API_KEY: OpenRouter API key
//   - DSGO_ANTHROPIC_API_KEY: Anthropic API key
func loadEnv() {
	if provider := os.Getenv("DSGO_PROVIDER"); provider != "" {
		globalSettings.DefaultProvider = provider
	}

	if model := os.Getenv("DSGO_MODEL"); model != "" {
		globalSettings.DefaultModel = stripProviderPrefix(model)
	}

	if timeoutStr := os.Getenv("DSGO_TIMEOUT"); timeoutStr != "" {
		if timeoutSec, err := strconv.Atoi(timeoutStr); err == nil && timeoutSec > 0 {
			globalSettings.DefaultTimeout = time.Duration(timeoutSec) * time.Second
		}
	}

	if retriesStr := os.Getenv("DSGO_MAX_RETRIES"); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil && retries >= 0 {
			globalSettings.MaxRetries = retries
		}
	}

	if tracingStr := os.Getenv("DSGO_TRACING"); tracingStr != "" {
		if tracing, err := strconv.ParseBool(tracingStr); err == nil {
			globalSettings.EnableTracing = tracing
		}
	}

	if globalSettings.APIKey == nil {
		globalSettings.APIKey = make(map[string]string)
	}

	if apiKey := os.Getenv("DSGO_OPENAI_API_KEY"); apiKey != "" {
		globalSettings.APIKey["openai"] = apiKey
	}

	if apiKey := os.Getenv("DSGO_OPENROUTER_API_KEY"); apiKey != "" {
		globalSettings.APIKey["openrouter"] = apiKey
	}

	if apiKey := os.Getenv("DSGO_ANTHROPIC_API_KEY"); apiKey != "" {
		globalSettings.APIKey["anthropic"] = apiKey
	}

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" && globalSettings.APIKey["openai"] == "" {
		globalSettings.APIKey["openai"] = apiKey
	}

	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" && globalSettings.APIKey["openrouter"] == "" {
		globalSettings.APIKey["openrouter"] = apiKey
	}

	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" && globalSettings.APIKey["anthropic"] == "" {
		globalSettings.APIKey["anthropic"] = apiKey
	}
}
