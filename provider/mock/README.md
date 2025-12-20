# provider/mock

Mock LM provider for tests and local integration.

## Overview

- Implements an OpenAI-compatible chat completions client against a test server.
- Registered under provider name `mock` via `core.RegisterLM`.
- Registers mock models (e.g. `mock/gpt-4o`, `mock/gpt-4o-mini`, `mock/test-model`).
- Supports streaming and uses `core.Cache` when configured.

## Environment

- `DSGO_MOCK_BASE_URL` (required unless a custom transport is set)
- `DSGO_MOCK_API_KEY` (optional, default `test`)
- `DSGO_HTTP_TIMEOUT_MS` (optional, request timeout in milliseconds; default 30000)

## Scripted Transport

Use a scripted transport to avoid a real HTTP server:

```go
reset := mock.SetHTTPTransport(mock.NewScriptedTransport(
    mock.HTTPResponseStep{Body: `{"id":"1","choices":[{"message":{"content":"ok"}}]}`},
))

defer reset()
```

## Usage

```go
lm, err := core.NewLM(ctx, "mock/test-model")
if err != nil {
    return err
}

predictor := module.NewPredict(sig, lm)
```
