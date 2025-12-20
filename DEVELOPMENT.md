# DEVELOPMENT

Contributor workflow for DSGo.

## Commands

```bash
make check      # verify + fmt + vet + build + check-eof
make test       # unit (no race, coverage)
make test-race  # unit + race detector
make all        # clean + check + lint + test-race
```

## Lint

- `make lint` requires `golangci-lint` (install via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`).

## Repo layout (high level)

- `core`: primitives (signatures, adapters, LM interface, prediction)
- `module`: higher-level behaviors (Predict, ReAct, Parallel, etc.)
- `provider`: provider implementations (OpenAI, OpenRouter)
- `signature_typed`: typed APIs
- `internal`: helpers (jsonutil, retry, ids, env)
- `scripts`: helper scripts

## CI expectations

- `make all` must pass before merging.
