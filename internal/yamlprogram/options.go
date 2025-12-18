package yamlprogram

import (
	"github.com/assagman/dsgo"
)

func buildGenerateOptions(spec GenerateOptionsSpec) *dsgo.GenerateOptions {
	opts := dsgo.DefaultGenerateOptions()

	if spec.Temperature != nil {
		opts.Temperature = *spec.Temperature
	}
	if spec.MaxTokens != nil {
		opts.MaxTokens = *spec.MaxTokens
	}
	if spec.TopP != nil {
		opts.TopP = *spec.TopP
	}
	if spec.Stop != nil {
		opts.Stop = append([]string(nil), spec.Stop...)
	}
	if spec.ResponseFormat != nil {
		opts.ResponseFormat = *spec.ResponseFormat
	}
	if spec.ResponseSchema != nil {
		opts.ResponseSchema = spec.ResponseSchema
	}
	if spec.ToolChoice != nil {
		opts.ToolChoice = *spec.ToolChoice
	}
	if spec.FrequencyPenalty != nil {
		opts.FrequencyPenalty = *spec.FrequencyPenalty
	}
	if spec.PresencePenalty != nil {
		opts.PresencePenalty = *spec.PresencePenalty
	}
	if spec.ProviderParams != nil {
		opts.ProviderParams = dsgo.DeepCopyMap(spec.ProviderParams)
	}

	// Retry
	if spec.Retry.MaxRetries != nil || spec.Retry.InitialBackoff != nil || spec.Retry.MaxBackoff != nil || spec.Retry.JitterFactor != nil {
		rc := &dsgo.RetryConfig{}
		if spec.Retry.MaxRetries != nil {
			rc.MaxRetries = *spec.Retry.MaxRetries
		}
		if spec.Retry.InitialBackoff != nil {
			rc.InitialBackoff = spec.Retry.InitialBackoff.Duration
		}
		if spec.Retry.MaxBackoff != nil {
			rc.MaxBackoff = spec.Retry.MaxBackoff.Duration
		}
		if spec.Retry.JitterFactor != nil {
			rc.JitterFactor = *spec.Retry.JitterFactor
		}
		opts.RetryConfig = rc
	}

	return opts
}
