package dsgo

import (
	"time"
)

// Option is a functional option for configuring DSGo.
type Option func(*Settings)

// Configure applies the given options to the global settings.
// Environment variables are loaded first, then options are applied in order.
func Configure(opts ...Option) {
	loadEnv()

	globalSettings.mu.Lock()
	defer globalSettings.mu.Unlock()

	for _, opt := range opts {
		opt(globalSettings)
	}
}

// WithProvider sets the default provider name.
func WithProvider(provider string) Option {
	return func(s *Settings) {
		s.DefaultProvider = provider
	}
}

// WithModel sets the default model identifier.
func WithModel(model string) Option {
	return func(s *Settings) {
		s.DefaultModel = model
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

// ResetConfig resets all settings to their default values.
func ResetConfig() {
	globalSettings.Reset()
}
