# provider/internal/util

Internal helpers for provider implementations.

## Overview

- `ApplyChatCompletionProviderParams` injects provider-specific parameters into
  `openai.ChatCompletionNewParams`.
- Filters out DSGo-managed fields (model, messages, sampling, tools, streaming,
  and response-shape settings) to avoid conflicts.
- Intended for trusted input only.

## Usage

```go
params := openai.ChatCompletionNewParams{Model: "..."}
util.ApplyChatCompletionProviderParams(&params, options.ProviderParams)
```
