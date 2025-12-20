# provider/openai

OpenAI LM implementation using the official `openai-go/v3` SDK.

## Overview

- Registered under provider name `openai` via `core.RegisterLM`.
- Supports tool calling and native JSON response formats.
- Uses exponential backoff for 429/5xx responses with `Retry-After` handling.
- Integrates with `core.Cache` when configured.
- Applies provider-specific parameters via `GenerateOptions.ProviderParams` (filtered to avoid DSGo-managed fields).

## Environment

- `OPENAI_API_KEY` (required)
- `DSGO_HTTP_TIMEOUT_MS` (optional, request timeout in milliseconds; default 300000)

## Usage

```go
lm, err := core.NewLM(ctx, "openai/gpt-4o")
if err != nil {
    return err
}

predictor := module.NewPredict(sig, lm)
```
