package core

import (
	"sync"
	"time"
)

// Settings holds global DSGo configuration.
type Settings struct {
	mu sync.RWMutex

	// DefaultLM is the default language model used when none is specified.
	DefaultLM LM

	// DefaultProvider is the default provider name (e.g., "openai", "openrouter").
	DefaultProvider string

	// DefaultModel is the default model identifier (e.g., "gpt-4", "google/gemini-2.5-flash").
	DefaultModel string

	// DefaultTimeout is the default timeout for LM calls.
	DefaultTimeout time.Duration

	// APIKey stores provider-specific API keys.
	APIKey map[string]string

	// MaxRetries sets the default number of retries for failed LM calls.
	MaxRetries int

	// EnableTracing enables detailed tracing and diagnostics.
	EnableTracing bool

	// Collector is the default collector for LM observability.
	Collector Collector

	// DefaultCache is the global cache instance (auto-wired to LM instances).
	DefaultCache Cache

	// CacheTTL is the cache time-to-live (0 = no expiry).
	CacheTTL time.Duration
}

// globalSettings is the singleton instance of Settings.
var globalSettings = &Settings{
	DefaultTimeout: 30 * time.Second,
	APIKey:         make(map[string]string),
	MaxRetries:     3,
	EnableTracing:  false,
	CacheTTL:       0, // No expiry by default
}

// GetSettings returns a copy of the current global settings.
func GetSettings() Settings {
	globalSettings.mu.RLock()
	defer globalSettings.mu.RUnlock()

	apiKeyCopy := make(map[string]string, len(globalSettings.APIKey))
	for k, v := range globalSettings.APIKey {
		apiKeyCopy[k] = v
	}

	return Settings{
		DefaultLM:       globalSettings.DefaultLM,
		DefaultProvider: globalSettings.DefaultProvider,
		DefaultModel:    globalSettings.DefaultModel,
		DefaultTimeout:  globalSettings.DefaultTimeout,
		APIKey:          apiKeyCopy,
		MaxRetries:      globalSettings.MaxRetries,
		EnableTracing:   globalSettings.EnableTracing,
		Collector:       globalSettings.Collector,
		DefaultCache:    globalSettings.DefaultCache,
		CacheTTL:        globalSettings.CacheTTL,
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

// SetCollector sets the default collector for LM observability.
func (s *Settings) SetCollector(collector Collector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Collector = collector
}

// Reset resets the settings to default values.
func (s *Settings) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DefaultLM = nil
	s.DefaultProvider = ""
	s.DefaultModel = ""
	s.DefaultTimeout = 30 * time.Second
	s.APIKey = make(map[string]string)
	s.MaxRetries = 3
	s.EnableTracing = false
	s.Collector = nil
	s.DefaultCache = nil
	s.CacheTTL = 0
}
