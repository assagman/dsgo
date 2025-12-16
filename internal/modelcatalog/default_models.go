package modelcatalog

// Default supported model list.
//
// This catalog is authoritative for core.NewLM/dsgo.NewLM.
// Prices are in USD per 1M tokens (source: models.dev API).
func init() {
	defaults := []Model{
		// OpenAI direct
		{ID: "openai/gpt-4o", Aliases: []string{"gpt-4o"}, Pricing: Pricing{PromptPrice: 2.5, CompletionPrice: 10}},
		{ID: "openai/gpt-4o-mini", Aliases: []string{"gpt-4o-mini"}, Pricing: Pricing{PromptPrice: 0.15, CompletionPrice: 0.6}},
		{ID: "openai/gpt-3.5-turbo", Aliases: []string{"gpt-3.5-turbo"}, Pricing: Pricing{PromptPrice: 0.5, CompletionPrice: 1.5}},
		{ID: "openai/o1", Aliases: []string{"o1"}, Pricing: Pricing{PromptPrice: 15, CompletionPrice: 60}},
		{ID: "openai/o1-mini", Aliases: []string{"o1-mini"}, Pricing: Pricing{PromptPrice: 1.1, CompletionPrice: 4.4}},
		{ID: "openai/o3-mini", Pricing: Pricing{PromptPrice: 1.1, CompletionPrice: 4.4}},
		{ID: "openai/o4-mini", Pricing: Pricing{PromptPrice: 1.1, CompletionPrice: 4.4}},
		{ID: "openai/gpt-4.1", Pricing: Pricing{PromptPrice: 2, CompletionPrice: 8}},
		{ID: "openai/gpt-4.1-mini", Pricing: Pricing{PromptPrice: 0.4, CompletionPrice: 1.6}},
		{ID: "openai/gpt-4.1-nano", Pricing: Pricing{PromptPrice: 0.1, CompletionPrice: 0.4}},
		{ID: "openai/gpt-5", Pricing: Pricing{PromptPrice: 1.25, CompletionPrice: 10}},
		{ID: "openai/gpt-5-mini", Pricing: Pricing{PromptPrice: 0.25, CompletionPrice: 2}},
		{ID: "openai/gpt-5-nano", Pricing: Pricing{PromptPrice: 0.05, CompletionPrice: 0.4}},
		{ID: "openai/gpt-4-turbo", Pricing: Pricing{PromptPrice: 10, CompletionPrice: 30}},

		// OpenRouter models with pricing
		{ID: "openrouter/google/gemini-2.5-flash", Pricing: Pricing{PromptPrice: 0.3, CompletionPrice: 2.5}},
		{ID: "openrouter/google/gemini-2.5-pro", Pricing: Pricing{PromptPrice: 1.25, CompletionPrice: 10}},
		{ID: "openrouter/google/gemini-2.5-flash-lite", Pricing: Pricing{PromptPrice: 0.1, CompletionPrice: 0.4}},
		{ID: "openrouter/google/gemini-2.5-flash-lite-preview-09-2025"},
		{ID: "openrouter/openai/gpt-4o-mini", Pricing: Pricing{PromptPrice: 0.15, CompletionPrice: 0.6}},
		{ID: "openrouter/openai/gpt-5", Pricing: Pricing{PromptPrice: 1.25, CompletionPrice: 10}},
		{ID: "openrouter/openai/gpt-5-mini", Pricing: Pricing{PromptPrice: 0.25, CompletionPrice: 2}},
		{ID: "openrouter/openai/gpt-5-mini-2025-08-07"},
		{ID: "openrouter/openai/gpt-5-nano", Pricing: Pricing{PromptPrice: 0.05, CompletionPrice: 0.4}},
		{ID: "openrouter/openai/gpt-5-nano-2025-08-07"},
		{ID: "openrouter/openai/gpt-4.1", Pricing: Pricing{PromptPrice: 2, CompletionPrice: 8}},
		{ID: "openrouter/openai/gpt-4.1-2025-04-14"},
		{ID: "openrouter/openai/gpt-oss-120b:exacto"},
		{ID: "openrouter/anthropic/claude-sonnet-4", Pricing: Pricing{PromptPrice: 3, CompletionPrice: 15}},
		{ID: "openrouter/anthropic/claude-3.5-sonnet"},
		{ID: "openrouter/anthropic/claude-haiku-4.5", Pricing: Pricing{PromptPrice: 1, CompletionPrice: 5}},
		{ID: "openrouter/anthropic/claude-opus-4", Pricing: Pricing{PromptPrice: 15, CompletionPrice: 75}},
		{ID: "openrouter/x-ai/grok-4", Pricing: Pricing{PromptPrice: 3, CompletionPrice: 15}},
		{ID: "openrouter/x-ai/grok-code-fast-1", Pricing: Pricing{PromptPrice: 0.2, CompletionPrice: 1.5}},
		{ID: "openrouter/deepseek/deepseek-v3.2", Pricing: Pricing{PromptPrice: 0.28, CompletionPrice: 0.4}},
		{ID: "openrouter/deepseek/deepseek-chat-v3.1", Pricing: Pricing{PromptPrice: 0.2, CompletionPrice: 0.8}},
		{ID: "openrouter/qwen/qwen3-coder", Pricing: Pricing{PromptPrice: 0.3, CompletionPrice: 1.2}},
		{ID: "openrouter/qwen/qwen3-coder:exacto"},
		{ID: "openrouter/qwen/qwen3-235b-a22b-07-25", Pricing: Pricing{PromptPrice: 0.15, CompletionPrice: 0.85}},
		{ID: "openrouter/qwen/qwen3-next-80b-a3b-instruct"},
		{ID: "openrouter/moonshotai/kimi-k2", Pricing: Pricing{PromptPrice: 0.55, CompletionPrice: 2.2}},
		{ID: "openrouter/moonshotai/kimi-k2-0905:exacto"},
		{ID: "openrouter/z-ai/glm-4.6", Pricing: Pricing{PromptPrice: 0.6, CompletionPrice: 2.2}},
		{ID: "openrouter/z-ai/glm-4.6:exacto"},
		{ID: "openrouter/meta-llama/llama-3.3-70b-instruct:free", Pricing: Pricing{PromptPrice: 0, CompletionPrice: 0}},
		{ID: "openrouter/meta-llama/llama-3.3-70b-instruct"},
		{ID: "openrouter/meta-llama/llama-3.1-8b-instruct"},
		{ID: "openrouter/amazon/nova-2-lite-v1"},

		// Mock provider - for testing purposes, uses OpenAI-equivalent pricing
		{ID: "mock/gpt-4o-mini", Pricing: Pricing{PromptPrice: 0.15, CompletionPrice: 0.6}},
		{ID: "mock/gpt-4o", Pricing: Pricing{PromptPrice: 2.5, CompletionPrice: 10}},
	}

	for _, m := range defaults {
		// Ignore errors in init: duplicates are a programmer error.
		_ = RegisterModel(m)
	}
}
