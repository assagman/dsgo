package core

import (
	"context"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/cost"
	"github.com/assagman/dsgo/internal/logging"
)

// Option is a functional option for configuring DSGo.
type Option func(*Settings)

// Configure applies the given options to the global settings.
// Environment variables are loaded first, then options are applied in order.
// This function is safe for concurrent use.
func Configure(opts ...Option) {
	globalSettings.mu.Lock()
	defer globalSettings.mu.Unlock()

	loadEnv()

	for _, opt := range opts {
		opt(globalSettings)
	}

	// Propagate logger to logging package if set
	if globalSettings.Logger != nil {
		logging.SetLogger(globalSettings.Logger)
	}
}

// WithProvider sets the default provider name.
func WithProvider(provider string) Option {
	return func(s *Settings) {
		s.DefaultProvider = provider
	}
}

// WithModel sets the default model identifier.
// Automatically strips provider prefixes like "openrouter/" if present.
func WithModel(model string) Option {
	return func(s *Settings) {
		s.DefaultModel = stripProviderPrefix(model)
	}
}

// WithPricingTier sets the pricing tier used for cost estimation for a provider.
//
// This impacts only Usage.Cost computation and observability; it does not change provider behavior.
func WithPricingTier(provider string, tier cost.PricingTier) Option {
	return func(s *Settings) {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			return
		}
		if s.PricingTierByProvider == nil {
			s.PricingTierByProvider = map[string]cost.PricingTier{}
		}
		s.PricingTierByProvider[provider] = cost.PricingTier(strings.ToLower(string(tier)))
	}
}

// WithTimeout sets the default timeout for LM calls.
func WithTimeout(timeout time.Duration) Option {
	return func(s *Settings) {
		s.DefaultTimeout = timeout
	}
}

// WithLM sets the default language model instance.
func WithLM(lm LM) Option {
	return func(s *Settings) {
		s.DefaultLM = lm
	}
}

// WithAPIKey sets the API key for a specific provider.
func WithAPIKey(provider, key string) Option {
	return func(s *Settings) {
		if s.APIKey == nil {
			s.APIKey = make(map[string]string)
		}
		s.APIKey[provider] = key
	}
}

// WithMaxRetries sets the default number of retries for failed LM calls.
func WithMaxRetries(retries int) Option {
	return func(s *Settings) {
		s.MaxRetries = retries
	}
}

// WithTracing enables or disables detailed tracing and diagnostics.
func WithTracing(enable bool) Option {
	return func(s *Settings) {
		s.EnableTracing = enable
	}
}

// WithCollector sets the default collector for LM observability.
func WithCollector(collector Collector) Option {
	return func(s *Settings) {
		s.Collector = collector
	}
}

// WithLogger sets the global logger instance.
func WithLogger(logger logging.Logger) Option {
	return func(s *Settings) {
		s.Logger = logger
	}
}

// WithStructuredOutputEnabled enables or disables structured output enforcement.
func WithStructuredOutputEnabled(enabled bool) Option {
	return func(s *Settings) {
		s.StructuredOutput.Enabled = enabled
	}
}

// WithStructuredOutputMaxAttempts sets the maximum number of attempts for structured output validation.
// Values <= 0 are ignored.
func WithStructuredOutputMaxAttempts(maxAttempts int) Option {
	return func(s *Settings) {
		if maxAttempts > 0 {
			s.StructuredOutput.MaxAttempts = maxAttempts
		}
	}
}

// WithStructuredOutputTemperature sets the temperature override for structured output mode.
func WithStructuredOutputTemperature(temperature float32) Option {
	return func(s *Settings) {
		s.StructuredOutput.Temperature = temperature
	}
}

// WithCache enables caching with the specified capacity.
// A cache with the given capacity will be created and auto-wired to all LM instances.
// Uses the configured CacheTTL if set, otherwise no expiration.
//
// If capacity <= 0, caching is disabled entirely.
func WithCache(capacity int) Option {
	return func(s *Settings) {
		if capacity <= 0 {
			cacheDisabledExplicit.Store(true)
			s.DefaultCache = nil
			logging.GetLogger().Warn(context.Background(), "Caching disabled via WithCache(0)", map[string]any{
				"module": "core.configure",
			})
			return
		}

		cacheDisabledExplicit.Store(false)
		s.DefaultCache = NewLMCacheWithTTL(capacity, s.CacheTTL)
	}
}

// WithCacheTTL sets the cache time-to-live for cached entries.
// After the TTL expires, entries will be considered stale.
func WithCacheTTL(ttl time.Duration) Option {
	return func(s *Settings) {
		s.CacheTTL = ttl
		cacheTTLOverride.Store(int64(ttl))

		// Only recreate memory-only caches; tiered caches are configured via WithTieredCache.
		if lmCache, ok := s.DefaultCache.(*LMCache); ok {
			s.DefaultCache = NewLMCacheWithTTL(lmCache.Capacity(), ttl)
		}
	}
}

// WithTieredCache creates a two-tier cache (memory + disk) with the given options.
// This is the recommended way to configure caching for production use.
// Memory cache provides fast access, disk cache provides persistence across restarts.
func WithTieredCache(opts *TieredCacheOptions) Option {
	return func(s *Settings) {
		if opts == nil {
			opts = DefaultTieredCacheOptions()
		}

		s.CacheConfig.EnableMemory = opts.EnableMemory
		s.CacheConfig.MemoryCapacity = opts.MemoryCapacity
		s.CacheConfig.EnableDisk = opts.EnableDisk
		s.CacheConfig.DiskDir = opts.DiskDir
		s.CacheConfig.DiskSizeLimit = opts.DiskSizeLimit
		s.CacheConfig.DiskShards = opts.DiskShards

		cacheDisabledExplicit.Store(false)

		optsCopy := *opts
		s.DefaultCache = NewLazyCache(func() Cache {
			cache, err := NewTieredCache(&optsCopy)
			if err != nil {
				logging.GetLogger().Warn(context.Background(), "Failed to initialize tiered cache, falling back to memory-only", map[string]any{
					"module": "core.configure",
					"error":  err.Error(),
				})
				return NewLMCacheWithTTL(optsCopy.MemoryCapacity, optsCopy.MemoryTTL)
			}
			return cache
		})
	}
}

// ResetConfig resets all settings to their default values.
func ResetConfig() {
	globalSettings.Reset()
}

// stripProviderPrefix removes known provider prefixes from model names.
// For example: "openrouter/meta-llama/llama-3.3-70b-instruct" -> "meta-llama/llama-3.3-70b-instruct"
func stripProviderPrefix(model string) string {
	prefixes := []string{"openrouter/", "openai/"}
	for _, prefix := range prefixes {
		if after, found := strings.CutPrefix(model, prefix); found {
			return after
		}
	}
	return model
}
