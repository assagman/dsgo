# provider/openrouter

OpenRouter LM implementation using the OpenAI-compatible `openai-go/v3` SDK.

## Overview

- Registered under provider name `openrouter` via `core.RegisterLM`.
- Supports tool calling and native JSON response formats.
- Falls back when JSON schema or JSON mode is unsupported by a model.
- Integrates with `core.Cache` when configured.
- Applies provider-specific parameters via `GenerateOptions.ProviderParams` (filtered to avoid DSGo-managed fields).

## Environment

- `OPENROUTER_API_KEY` (required)
- `OPENROUTER_SITE_NAME` (optional, sent as `X-Title` header)
- `OPENROUTER_SITE_URL` (optional, sent as `HTTP-Referer` header)
- `DSGO_HTTP_TIMEOUT_MS` (optional, request timeout in milliseconds; default 300000)

## Usage

```go
lm, err := core.NewLM(ctx, "openrouter/google/gemini-2.5-flash")
if err != nil {
    return err
}

predictor := module.NewPredict(sig, lm)
```
