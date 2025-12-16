package modelcatalog

// Default supported model list.
//
// This catalog is authoritative for core.NewLM/dsgo.NewLM.
func init() {
	defaults := []Model{
		// OpenAI direct
		{ID: "openai/gpt-4o", Aliases: []string{"gpt-4o"}},
		{ID: "openai/gpt-4o-mini", Aliases: []string{"gpt-4o-mini"}},
		{ID: "openai/gpt-3.5-turbo", Aliases: []string{"gpt-3.5-turbo"}},
		{ID: "openai/o1", Aliases: []string{"o1"}},
		{ID: "openai/o1-mini", Aliases: []string{"o1-mini"}},

		// OpenRouter curated list (static snapshot)
		{ID: "openrouter/amazon/nova-2-lite-v1"},
		{ID: "openrouter/openai/gpt-4o-mini"},
		{ID: "openrouter/openai/gpt-5-mini-2025-08-07"},
		{ID: "openrouter/openai/gpt-5-nano-2025-08-07"},
		{ID: "openrouter/openai/gpt-4.1-2025-04-14"},
		{ID: "openrouter/openai/gpt-oss-120b:exacto"},
		{ID: "openrouter/google/gemini-2.5-flash"},
		{ID: "openrouter/google/gemini-2.5-flash-lite-preview-09-2025"},
		{ID: "openrouter/anthropic/claude-3.5-sonnet"},
		{ID: "openrouter/anthropic/claude-haiku-4.5"},
		{ID: "openrouter/x-ai/grok-code-fast-1"},
		{ID: "openrouter/deepseek/deepseek-v3.2"},
		{ID: "openrouter/qwen/qwen3-next-80b-a3b-instruct"},
		{ID: "openrouter/z-ai/glm-4.6:exacto"},
		{ID: "openrouter/moonshotai/kimi-k2-0905:exacto"},
		{ID: "openrouter/qwen/qwen3-coder:exacto"},
		{ID: "openrouter/meta-llama/llama-3.1-8b-instruct"},
		{ID: "openrouter/meta-llama/llama-3.3-70b-instruct"},
	}

	for _, m := range defaults {
		// Ignore errors in init: duplicates are a programmer error.
		_ = RegisterModel(m)
	}
}
