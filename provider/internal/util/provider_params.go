package util

import "github.com/openai/openai-go/v3"

// ApplyChatCompletionProviderParams applies provider-specific parameters to the request.
//
// DSGo manages a core set of parameters (model, messages, sampling options, tool calling,
// response format, etc). ProviderParams is intended only for provider-specific extensions
// (e.g. top_k, seed, reasoning.effort) and must not override DSGo-managed fields.
//
// For security reasons, ProviderParams should only be used with trusted input.
func ApplyChatCompletionProviderParams(params *openai.ChatCompletionNewParams, providerParams map[string]any) {
	extra := filterChatCompletionProviderParams(providerParams)
	if len(extra) == 0 {
		return
	}

	params.SetExtraFields(extra)
}

func filterChatCompletionProviderParams(providerParams map[string]any) map[string]any {
	if len(providerParams) == 0 {
		return nil
	}

	extra := make(map[string]any, len(providerParams))
	for key, value := range providerParams {
		// Don't override DSGo-managed keys to maintain consistency.
		//
		// Also block fields that change response shape (e.g. n) or transport mode (e.g. stream),
		// because DSGo assumes single-choice, non-streaming responses in Generate().
		switch key {
		case "model", "messages",
			"temperature", "top_p", "stop",
			"max_tokens", "max_completion_tokens",
			"response_format",
			"frequency_penalty", "presence_penalty",
			"tools", "tool_choice",
			"n", "stream", "stream_options",
			"logprobs", "top_logprobs":
			continue
		default:
			extra[key] = value
		}
	}

	return extra
}
