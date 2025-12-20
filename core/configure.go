package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/env"
	"github.com/assagman/dsgo/logging"
)

// Option is a functional option for configuring DSGo.
type Option func(*Settings)

// Configure applies the given options to the global settings.
// .env files are loaded first, then environment variables, then options are applied in order.
// Returns an error if required API key environment variables are missing.
// This function is safe for concurrent use.
func Configure(opts ...Option) error {
	if err := env.AutoLoad(); err != nil {
		return err
	}
	if err := requireAPIKeyEnv(); err != nil {
		return err
	}

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
	return nil
}

func requireAPIKeyEnv() error {
	if hasAPIKeyEnv() {
		return nil
	}
	return fmt.Errorf("missing required API key env var: set DSGO_OPENAI_API_KEY or DSGO_OPENROUTER_API_KEY (or OPENAI_API_KEY/OPENROUTER_API_KEY)")
}

func hasAPIKeyEnv() bool {
	keys := []string{
		"DSGO_OPENAI_API_KEY",
		"DSGO_OPENROUTER_API_KEY",
		"OPENAI_API_KEY",
		"OPENROUTER_API_KEY",
	}
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
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

// WithTimeout sets the default timeout for LM calls.
func WithTimeout(timeout time.Duration) Option {
	return func(s *Settings) {
		s.DefaultTimeout = timeout
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

// WithSkipModelValidation enables or disables strict model validation in NewLM.
// Use this as an escape hatch for using models not yet in the catalog.
func WithSkipModelValidation(skip bool) Option {
	return func(s *Settings) {
		s.SkipModelValidation = skip
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
