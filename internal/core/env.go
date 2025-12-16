package core

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/cost"
	"github.com/assagman/dsgo/internal/logging"
)

// loadEnv loads configuration from environment variables.
// This is called automatically by Configure() before applying user options.
// Environment variables supported:
//   - DSGO_TIMEOUT: Default timeout in seconds (e.g., "30")
//   - DSGO_MAX_RETRIES: Default number of retries (e.g., "3")
//   - DSGO_TRACING: Enable tracing ("true" or "false")
//   - DSGO_CACHE: Enable/disable caching ("true"/"false")
//   - DSGO_CACHE_TTL: Cache time-to-live duration (e.g., "5m", "1h", "30s")
//   - DSGO_CACHE_MEMORY: Memory cache capacity in entries
//   - DSGO_CACHEDIR: Disk cache directory (default: ~/.dsgo_cache/proj_<hash>/)
//   - DSGO_CACHE_LIMIT: Disk cache size limit in bytes (default: 30GB)
//   - DSGO_CACHE_DISK: Enable disk caching ("true" or "false", default "true")
//   - DSGO_OPENAI_PRICING_TIER: Pricing tier for OpenAI cost estimation (standard|batch|priority|flex)
//   - DSGO_OPENROUTER_PRICING_TIER: Pricing tier for OpenRouter cost estimation (standard)
//   - DSGO_OPENAI_API_KEY: OpenAI API key
//   - DSGO_OPENROUTER_API_KEY: OpenRouter API key
//   - DSGO_STRUCTURED_OUTPUTS: Enable structured outputs ("true" or "false", default "true")
//   - DSGO_STRUCTURED_MAX_ATTEMPTS: Max attempts for structured output validation (e.g., "3")
//   - DSGO_STRUCTURED_TEMPERATURE: Temperature override for structured mode (e.g., "0.0", "0.1")
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

	// Parse DSGO_CACHE (enable/disable caching)
	if cacheStr := os.Getenv("DSGO_CACHE"); cacheStr != "" {
		if enabled, err := strconv.ParseBool(cacheStr); err == nil && !enabled {
			globalSettings.DefaultCache = nil
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

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" && globalSettings.APIKey["openai"] == "" {
		globalSettings.APIKey["openai"] = apiKey
	}

	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" && globalSettings.APIKey["openrouter"] == "" {
		globalSettings.APIKey["openrouter"] = apiKey
	}

	// Parse DSGO_CACHE_TTL (e.g., "5m", "1h", "30s")
	if ttlStr := os.Getenv("DSGO_CACHE_TTL"); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			globalSettings.CacheTTL = ttl
		}
	}

	// Parse DSGO_CACHE_MEMORY (memory cache capacity)
	if memStr := os.Getenv("DSGO_CACHE_MEMORY"); memStr != "" {
		if mem, err := strconv.Atoi(memStr); err == nil && mem > 0 {
			globalSettings.CacheConfig.MemoryCapacity = mem
		}
	}

	// Parse DSGO_CACHEDIR (disk cache directory)
	if cacheDir := os.Getenv("DSGO_CACHEDIR"); cacheDir != "" {
		globalSettings.CacheConfig.DiskDir = cacheDir
	}

	// Parse DSGO_CACHE_LIMIT (disk cache size limit in bytes)
	if limitStr := os.Getenv("DSGO_CACHE_LIMIT"); limitStr != "" {
		if limit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && limit > 0 {
			globalSettings.CacheConfig.DiskSizeLimit = limit
		}
	}

	// Parse DSGO_CACHE_DISK (enable disk caching)
	if diskStr := os.Getenv("DSGO_CACHE_DISK"); diskStr != "" {
		if disk, err := strconv.ParseBool(diskStr); err == nil {
			globalSettings.CacheConfig.EnableDisk = disk
		}
	}

	// Parse DSGO_STRUCTURED_OUTPUTS (default true)
	if structuredStr := os.Getenv("DSGO_STRUCTURED_OUTPUTS"); structuredStr != "" {
		if structured, err := strconv.ParseBool(structuredStr); err == nil {
			globalSettings.StructuredOutput.Enabled = structured
		}
	}

	// Parse DSGO_STRUCTURED_MAX_ATTEMPTS
	if maxAttemptsStr := os.Getenv("DSGO_STRUCTURED_MAX_ATTEMPTS"); maxAttemptsStr != "" {
		if maxAttempts, err := strconv.Atoi(maxAttemptsStr); err == nil && maxAttempts > 0 {
			globalSettings.StructuredOutput.MaxAttempts = maxAttempts
		}
	}

	// Parse DSGO_STRUCTURED_TEMPERATURE
	if tempStr := os.Getenv("DSGO_STRUCTURED_TEMPERATURE"); tempStr != "" {
		if temp, err := strconv.ParseFloat(tempStr, 32); err == nil {
			globalSettings.StructuredOutput.Temperature = float32(temp)
		}
	}

	// Pricing tiers (cost estimation only)
	if globalSettings.PricingTierByProvider == nil {
		globalSettings.PricingTierByProvider = map[string]cost.PricingTier{}
	}

	if tierStr := os.Getenv("DSGO_OPENAI_PRICING_TIER"); tierStr != "" {
		tier := cost.PricingTier(strings.ToLower(strings.TrimSpace(tierStr)))
		switch tier {
		case cost.TierStandard, cost.TierFlex, cost.TierPriority, cost.TierBatch:
			globalSettings.PricingTierByProvider["openai"] = tier
		}
	}

	if tierStr := os.Getenv("DSGO_OPENROUTER_PRICING_TIER"); tierStr != "" {
		tier := cost.PricingTier(strings.ToLower(strings.TrimSpace(tierStr)))
		// OpenRouter is treated as standard-only.
		if tier == cost.TierStandard {
			globalSettings.PricingTierByProvider["openrouter"] = tier
		}
	}

	// Configure logger from environment if not already set
	if globalSettings.Logger == nil {
		logging.ConfigureLoggerFromEnv()
		globalSettings.Logger = logging.GetLogger()
	}
}
