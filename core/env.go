package core

import (
	"os"
	"strconv"
	"time"
)

// loadEnv loads configuration from environment variables.
// This is called automatically by Configure() before applying user options.
// Environment variables supported:
//   - DSGO_TIMEOUT: Default timeout in seconds (e.g., "30")
//   - DSGO_MAX_RETRIES: Default number of retries (e.g., "3")
//   - DSGO_TRACING: Enable tracing ("true" or "false")
//   - DSGO_CACHE_TTL: Cache time-to-live duration (e.g., "5m", "1h", "30s")
//   - DSGO_OPENAI_API_KEY: OpenAI API key
//   - DSGO_OPENROUTER_API_KEY: OpenRouter API key
//   - DSGO_ANTHROPIC_API_KEY: Anthropic API key
func loadEnv() {

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

	// Parse DSGO_CACHE_TTL (e.g., "5m", "1h", "30s")
	if ttlStr := os.Getenv("DSGO_CACHE_TTL"); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			globalSettings.CacheTTL = ttl
		}
	}
}
