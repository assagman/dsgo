# DEVELOPMENT

Contributor workflow for DSGo.

## Commands

```bash
make check      # verify + fmt + vet + build
make test       # unit + integration (no race)
make all        # clean + check + lint + test-race
```

## Lint

- `make lint` requires `golangci-lint`.

## Repo layout (high level)

- `dsgo.go`: public API re-exports
- `internal/core`: primitives (signatures, adapters, LM interface, prediction)
- `internal/module`: higher-level behaviors (Predict, ReAct, Parallel, etc.)
- `internal/providers`: provider implementations (OpenAI, OpenRouter)
- `examples/`: runnable examples
- `integration/`: integration tests
- `scripts/`: helper scripts

## CI expectations

- `make all` must pass before merging.
