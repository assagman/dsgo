package yamlprogram

// Normalize applies schema defaults and normalizes absent fields.
//
// This is intentionally separate from Validate() so that validation can assume
// defaults are already applied.
func Normalize(s *Spec) {
	if s == nil {
		return
	}

	// Defaults
	if s.Defaults.Gen.Temperature == nil {
		v := 0.6
		s.Defaults.Gen.Temperature = &v
	}
	if s.Defaults.Adapter.Kind == "" {
		s.Defaults.Adapter.Kind = "fallback"
	}

	// Module defaults
	for name, mod := range s.Modules {
		applyGenDefaults(&mod.Gen, &s.Defaults.Gen)
		applyAdapterDefaults(&mod.Adapter, &s.Defaults.Adapter)
		s.Modules[name] = mod
	}
}

func applyGenDefaults(target *GenerateOptionsSpec, defaults *GenerateOptionsSpec) {
	if target == nil || defaults == nil {
		return
	}
	if target.Temperature == nil {
		target.Temperature = defaults.Temperature
	}
	if target.MaxTokens == nil {
		target.MaxTokens = defaults.MaxTokens
	}
	if target.TopP == nil {
		target.TopP = defaults.TopP
	}
	if target.ResponseFormat == nil {
		target.ResponseFormat = defaults.ResponseFormat
	}
	if target.ToolChoice == nil {
		target.ToolChoice = defaults.ToolChoice
	}
	if target.FrequencyPenalty == nil {
		target.FrequencyPenalty = defaults.FrequencyPenalty
	}
	if target.PresencePenalty == nil {
		target.PresencePenalty = defaults.PresencePenalty
	}
	if target.ProviderParams == nil && defaults.ProviderParams != nil {
		target.ProviderParams = defaults.ProviderParams
	}
	if target.ResponseSchema == nil && defaults.ResponseSchema != nil {
		target.ResponseSchema = defaults.ResponseSchema
	}
	if target.Stop == nil && defaults.Stop != nil {
		target.Stop = defaults.Stop
	}

	// Retry
	applyRetryDefaults(&target.Retry, &defaults.Retry)
}

func applyRetryDefaults(target *RetrySpec, defaults *RetrySpec) {
	if target == nil || defaults == nil {
		return
	}
	if target.MaxRetries == nil {
		target.MaxRetries = defaults.MaxRetries
	}
	if target.InitialBackoff == nil {
		target.InitialBackoff = defaults.InitialBackoff
	}
	if target.MaxBackoff == nil {
		target.MaxBackoff = defaults.MaxBackoff
	}
	if target.JitterFactor == nil {
		target.JitterFactor = defaults.JitterFactor
	}
}

func applyAdapterDefaults(target *AdapterSpec, defaults *AdapterSpec) {
	if target == nil || defaults == nil {
		return
	}
	if target.Kind == "" {
		target.Kind = defaults.Kind
	}
	if target.Reasoning == nil {
		target.Reasoning = defaults.Reasoning
	}
	if target.ExtractionModel == "" {
		target.ExtractionModel = defaults.ExtractionModel
	}
}
