package core

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/assagman/dsgo/internal/cost"
	"github.com/assagman/dsgo/internal/logging"
)

// CacheConfiguration holds advanced cache configuration options
type CacheConfiguration struct {
	// EnableMemory enables in-memory LRU caching (default: true)
	EnableMemory bool

	// MemoryCapacity is the maximum number of entries in memory cache
	MemoryCapacity int

	// EnableDisk enables persistent disk caching (default: false for backward compat)
	EnableDisk bool

	// DiskDir is the directory for disk cache (default: ~/.dsgo_cache)
	DiskDir string

	// DiskSizeLimit is the maximum size of disk cache in bytes (default: 30GB)
	DiskSizeLimit int64

	// DiskShards is the number of shards for concurrent disk access (default: 16)
	DiskShards int
}

// DefaultCacheConfiguration returns sensible defaults for cache configuration
func DefaultCacheConfiguration() CacheConfiguration {
	return CacheConfiguration{
		EnableMemory:   true,
		MemoryCapacity: 1000,
		EnableDisk:     true,                    // Enabled by default (DSPy parity), use DSGO_CACHE_DISK=false to disable
		DiskDir:        "",                      // Will use ~/.dsgo_cache
		DiskSizeLimit:  30 * 1024 * 1024 * 1024, // 30GB like DSPy
		DiskShards:     16,
	}
}

// StructuredOutputConfig holds configuration for structured output enforcement.
type StructuredOutputConfig struct {
	// Enabled controls whether structured output enforcement is active.
	// Default: true (structured outputs enabled by default).
	Enabled bool

	// MaxAttempts is the maximum number of retries for structured output validation.
	// Default: 3 (attempt 1 initial, then up to 2 retries).
	MaxAttempts int

	// Temperature override for structured output mode.
	// When structured mode is active, this temperature is forced to ensure deterministic output.
	// Default: 0.0 (deterministic). Set to 0.1 for slight variation.
	Temperature float32
}

// Settings holds global DSGo configuration.
type Settings struct {
	mu sync.RWMutex

	// DefaultLM is the default language model used when none is specified.
	DefaultLM LM

	// DefaultProvider is the default provider name (e.g., "openai", "openrouter").
	DefaultProvider string

	// DefaultModel is the default model identifier (e.g., "gpt-4", "meta-llama/llama-3.3-70b-instruct").
	DefaultModel string

	// DefaultTimeout is the default timeout for LM calls.
	DefaultTimeout time.Duration

	// APIKey stores provider-specific API keys.
	APIKey map[string]string

	// PricingTierByProvider selects pricing tiers per provider.
	//
	// Used for cost estimation/observability only; it does not affect provider behavior.
	PricingTierByProvider map[string]cost.PricingTier

	// MaxRetries sets the default number of retries for failed LM calls.
	MaxRetries int

	// EnableTracing enables detailed tracing and diagnostics.
	EnableTracing bool

	// SkipModelValidation disables the strict model catalog validation in NewLM.
	//
	// If true, NewLM will accept any model string as long as it has a provider prefix,
	// bypassing the IsValidCanonical check.
	SkipModelValidation bool

	// Collector is the default collector for LM observability.
	Collector Collector

	// DefaultCache is the global cache instance (auto-wired to LM instances).
	DefaultCache Cache

	// CacheTTL is the cache time-to-live (0 = no expiry).
	CacheTTL time.Duration

	// CacheConfig holds advanced cache configuration options
	CacheConfig CacheConfiguration

	// Logger is the global logger instance.
	Logger logging.Logger

	// StructuredOutputConfig controls structured output enforcement.
	StructuredOutput StructuredOutputConfig
}

var (
	cacheDisabledExplicit atomic.Bool
	cacheTTLOverride      atomic.Int64 // nanoseconds; 0 means unset
)

// globalSettings is the singleton instance of Settings.
var globalSettings = &Settings{
	DefaultTimeout: 30 * time.Second,
	APIKey:         make(map[string]string),
	PricingTierByProvider: map[string]cost.PricingTier{
		"openai":     cost.TierStandard,
		"openrouter": cost.TierStandard,
	},
	MaxRetries:          3,
	EnableTracing:       false,
	SkipModelValidation: false,
	CacheTTL:            0, // No expiry by default
	CacheConfig:         DefaultCacheConfiguration(),
	Logger:              nil, // Will be set by logging package initialization
	StructuredOutput: StructuredOutputConfig{
		Enabled:     true, // Structured outputs enabled by default
		MaxAttempts: 3,    // 1 initial + up to 2 retries
		Temperature: 0.0,  // Deterministic
	},

	// Lazy by default: no disk writes until first LM call.
	DefaultCache: NewLazyCache(defaultCacheInitializer),
}

func defaultCacheInitializer() Cache {
	if cacheDisabledExplicit.Load() {
		return nil
	}

	if cacheEnv := getEnv("DSGO_CACHE"); cacheEnv == "false" || cacheEnv == "0" {
		return nil
	}

	opts := DefaultTieredCacheOptions()

	// TTL: prefer explicit Configure() value, otherwise env.
	if ttlNanos := cacheTTLOverride.Load(); ttlNanos > 0 {
		opts.MemoryTTL = time.Duration(ttlNanos)
	} else if ttlStr := getEnv("DSGO_CACHE_TTL"); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			opts.MemoryTTL = ttl
		}
	}

	// Disk enable/disable
	if diskEnv := getEnv("DSGO_CACHE_DISK"); diskEnv == "false" || diskEnv == "0" {
		opts.EnableDisk = false
	}

	// Custom cache directory
	if cacheDir := getEnv("DSGO_CACHEDIR"); cacheDir != "" {
		opts.DiskDir = cacheDir
	}

	// Custom disk size limit
	if limitStr := getEnv("DSGO_CACHE_LIMIT"); limitStr != "" {
		if limit := parseInt(limitStr); limit > 0 {
			opts.DiskSizeLimit = int64(limit)
		}
	}

	// Custom memory capacity
	if memStr := getEnv("DSGO_CACHE_MEMORY"); memStr != "" {
		if mem := parseInt(memStr); mem > 0 {
			opts.MemoryCapacity = mem
		}
	}

	cache, err := NewTieredCache(opts)
	if err != nil {
		logging.GetLogger().Warn(context.Background(), "Failed to initialize tiered cache, falling back to memory-only", map[string]any{
			"module": "core.settings",
			"error":  err.Error(),
		})
		return NewLMCacheWithTTL(opts.MemoryCapacity, opts.MemoryTTL)
	}

	return cache
}

// GetSettings returns a copy of the current global settings.
func GetSettings() Settings {
	globalSettings.mu.RLock()
	defer globalSettings.mu.RUnlock()

	apiKeyCopy := make(map[string]string, len(globalSettings.APIKey))
	maps.Copy(apiKeyCopy, globalSettings.APIKey)

	pricingTierCopy := make(map[string]cost.PricingTier, len(globalSettings.PricingTierByProvider))
	maps.Copy(pricingTierCopy, globalSettings.PricingTierByProvider)

	return Settings{
		DefaultLM:             globalSettings.DefaultLM,
		DefaultProvider:       globalSettings.DefaultProvider,
		DefaultModel:          globalSettings.DefaultModel,
		DefaultTimeout:        globalSettings.DefaultTimeout,
		APIKey:                apiKeyCopy,
		PricingTierByProvider: pricingTierCopy,
		MaxRetries:            globalSettings.MaxRetries,
		EnableTracing:         globalSettings.EnableTracing,
		SkipModelValidation:   globalSettings.SkipModelValidation,
		Collector:             globalSettings.Collector,
		DefaultCache:          globalSettings.DefaultCache,
		CacheTTL:              globalSettings.CacheTTL,
		CacheConfig:           globalSettings.CacheConfig,
		Logger:                globalSettings.Logger,
		StructuredOutput:      globalSettings.StructuredOutput,
	}
}

// SetDefaultLM sets the default language model.
func (s *Settings) SetDefaultLM(lm LM) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DefaultLM = lm
}

// SetDefaultProvider sets the default provider name.
func (s *Settings) SetDefaultProvider(provider string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DefaultProvider = provider
}

// SetDefaultModel sets the default model identifier.
func (s *Settings) SetDefaultModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DefaultModel = model
}

// SetDefaultTimeout sets the default timeout for LM calls.
func (s *Settings) SetDefaultTimeout(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DefaultTimeout = timeout
}

// SetAPIKey sets the API key for a specific provider.
func (s *Settings) SetAPIKey(provider, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.APIKey == nil {
		s.APIKey = make(map[string]string)
	}
	s.APIKey[provider] = key
}

// GetAPIKey retrieves the API key for a specific provider.
func (s *Settings) GetAPIKey(provider string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, ok := s.APIKey[provider]
	return key, ok
}

// SetMaxRetries sets the default number of retries.
func (s *Settings) SetMaxRetries(retries int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MaxRetries = retries
}

// SetEnableTracing enables or disables tracing.
func (s *Settings) SetEnableTracing(enable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EnableTracing = enable
}

// SetSkipModelValidation enables or disables strict model validation.
func (s *Settings) SetSkipModelValidation(skip bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SkipModelValidation = skip
}

// SetCollector sets the default collector for LM observability.
func (s *Settings) SetCollector(collector Collector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Collector = collector
}

// SetLogger sets the global logger instance.
func (s *Settings) SetLogger(logger logging.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Logger = logger
}

// SetStructuredOutputConfig sets the structured output configuration.
func (s *Settings) SetStructuredOutputConfig(config StructuredOutputConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StructuredOutput = config
}

// SetStructuredOutputEnabled enables or disables structured output enforcement.
func (s *Settings) SetStructuredOutputEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StructuredOutput.Enabled = enabled
}

// SetStructuredOutputMaxAttempts sets the maximum number of attempts for structured output validation.
func (s *Settings) SetStructuredOutputMaxAttempts(maxAttempts int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StructuredOutput.MaxAttempts = maxAttempts
}

// SetStructuredOutputTemperature sets the temperature override for structured output mode.
func (s *Settings) SetStructuredOutputTemperature(temperature float32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StructuredOutput.Temperature = temperature
}

// Reset resets the settings to default values.
func (s *Settings) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cacheDisabledExplicit.Store(false)
	cacheTTLOverride.Store(0)

	s.DefaultLM = nil
	s.DefaultProvider = ""
	s.DefaultModel = ""
	s.DefaultTimeout = 30 * time.Second
	s.APIKey = make(map[string]string)
	s.PricingTierByProvider = map[string]cost.PricingTier{
		"openai":     cost.TierStandard,
		"openrouter": cost.TierStandard,
	}
	s.MaxRetries = 3
	s.EnableTracing = false
	s.SkipModelValidation = false
	s.Collector = nil
	s.CacheTTL = 0
	s.CacheConfig = DefaultCacheConfiguration()
	s.Logger = nil
	s.StructuredOutput = StructuredOutputConfig{
		Enabled:     true,
		MaxAttempts: 3,
		Temperature: 0.0,
	}

	// Lazy by default: no disk writes until first LM call.
	s.DefaultCache = NewLazyCache(defaultCacheInitializer)
}

// SetCacheConfig sets the cache configuration.
func (s *Settings) SetCacheConfig(config CacheConfiguration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CacheConfig = config
}
