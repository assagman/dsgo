# internal/jsonutil

JSON extraction helpers for LM responses.

## Overview

- `ExtractJSON` pulls a JSON object from markdown or raw text.
- `ParseJSON` extracts and unmarshals into `map[string]any`.
- Options allow newline repair and simplified brace matching.

## Options

- `WithFixNewlines` repairs unescaped newlines inside JSON strings.
- `WithSimpleBraceMatching` ignores string context and uses simple brace matching.

## Usage

```go
raw, err := jsonutil.ExtractJSON(content, jsonutil.WithFixNewlines())
parsed, err := jsonutil.ParseJSON(content)
```
