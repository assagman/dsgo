# DSGo Development Guide

## Quick Reference

```mermaid
graph LR
    subgraph "Development Flow"
        CODE[Write Code] --> TEST[Run Tests]
        TEST --> BUILD[Build Check]
        BUILD --> FIX{Pass?}
        FIX -->|No| CODE
        FIX -->|Yes| DONE[✓ Done]
    end

    classDef dev fill:#06d6a0,stroke:#073b4c,color:#000
    classDef check fill:#ffd166,stroke:#073b4c,color:#000
    classDef pass fill:#118ab2,stroke:#073b4c,color:#fff

    class CODE,DONE dev
    class TEST,BUILD check
    class FIX pass
```

## Testing Commands
- **Everything**: `make test` (runs race detector + coverage + summary)
- **Examples Quick**: `make test-matrix-quick` (single model, fast)
- **Examples Sample**: `make test-matrix-sample N=3` (3 random models)
- **Examples Full**: `make test-matrix` (all models, comprehensive)
- Single test: `go test -run TestName`
- Run example: `go run examples/sentiment/main.go`
- Lint: `make lint` (requires golangci-lint)
- Full check: `make check` (fmt, vet, build)
- Complete: `make all` (clean, check, test, eof-check)

## Current Test Coverage
- **Total**: 79.4% ✅
- **Core**: 61.9%
- **Module**: 84.2% ✅
- **jsonutil**: 100% ✅ (includes RepairJSON tests)
- **OpenAI Provider**: 94.4% ✅
- **OpenRouter Provider**: 94.7% ✅

## Architecture

Go port of DSPy (Declarative Self-improving Language Programs).

### Core Files (Root Directory)
- `signature.go` - I/O field definitions (Field, Signature types, ValidationDiagnostics)
- `lm.go` - LM interface (Message, GenerateOptions, GenerateResult)
- `module.go` - Base Module interface
- `prediction.go` - Prediction wrapper with metadata + parse diagnostics
- `history.go` - Conversation history management
- `example.go` - Few-shot learning support
- `tool.go` - Tool/function calling support
- `adapter.go` - Adapter interface + implementations (JSON, Chat, TwoStep, Fallback)

### Module Implementations (`module/`)
- `predict.go` - Basic prediction
- `chain_of_thought.go` - Reasoning module
- `react.go` - Tool-using agent
- `refine.go` - Iterative refinement
- `best_of_n.go` - Multiple sampling
- `program_of_thought.go` - Code generation
- `program.go` - Module composition

### Internal Utilities (`internal/`)
- `jsonutil/` - JSON extraction and repair (RepairJSON, ExtractJSON)
  - Handles malformed JSON from models
  - 100% test coverage

### Providers (`providers/`)
- `openai/` - OpenAI LM provider
- `openrouter/` - OpenRouter LM provider

### Examples (`examples/`)
- 13+ working examples from basic to advanced
- See QUICKSTART.md for recommended learning path

## Code Style
- Standard Go: `gofmt` formatting, exported types/funcs have doc comments
- Naming: PascalCase exports, camelCase internals, FieldType* constants
- Error handling: Return `error` as last value, wrap with `fmt.Errorf("context: %w", err)`, always check returned errors
- Interfaces: Small, composable (Module, LM)
- Tests: Table-driven with subtests (`t.Run(tt.name, ...)`)
- No external deps except godotenv for examples (use stdlib)
- Linting: All code must pass `golangci-lint` (errcheck, staticcheck, unused, govet, ineffassign)

## Development Workflow
- Always run `go build ./...` and `go test ./...` during development
- All new developments requires unit test update.
- Run `make check` locally (fmt, vet, build); CI handles lint separately
- Use `go test -race` when working with concurrency (e.g., BestOfN parallel)
- Pre-commit hook automatically runs `make all` (tests + checks) when installed via `make install-hooks`
- **Install golangci-lint v2.6.0** (required for linting):
  ```bash
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.6.0
  ```
  Note: `go install` only installs v1.x; v2.x requires binary installation
- No temporary UPPER_CASE.md files (SUMMARY.md, CHANGES.md, etc.) - update existing docs only
- Keep responses concise
- Ask user for feedback/choices at important checkpoints
- Never create .md files unless user told you so. Always update git included documentation files.
- If any issue, error, or unexpected result detected, always add it to test cases, then fix the codebase, and
    make sure that all success.
- Always execute examples/ to one-by-one and scan for unexpected behaviors, errors, problematic results etc. If any issue encountered, plan for adding them to test cases and fixing it.
    execute examples from top-level dir since the .env loading is designed in that way.
- run `make all` when you're done


## Production-Grade Robustness Features

✅ **Automatic JSON Repair** - Models often emit malformed JSON; DSGo fixes it automatically:
- Single quotes → double quotes: `{'key': 'val'}` → `{"key": "val"}`
- Unquoted keys: `{key: "val"}` → `{"key": "val"}`
- Trailing commas: `{"a": 1,}` → `{"a": 1}`
- Smart quotes normalization
- Tracked with `__json_repair` metadata

✅ **Partial Output Validation** - For training/optimization:
- `ValidateOutputsPartial()` returns diagnostics instead of failing
- Missing fields set to `nil` with detailed tracking
- `ParseDiagnostics` attached to predictions for telemetry

✅ **Class/Enum Normalization** - Flexible matching:
- Case-insensitive: `"POSITIVE"` → `"positive"`
- Configurable aliases: `"pos"` → `"positive"`
- Applied automatically in validation

✅ **Smart Numeric Extraction** - From text descriptions:
- `"High (95%)"` → `95`
- `"Medium"` → `0.7` (qualitative mapping)

See [ROBUSTNESS_ENHANCEMENTS.md](ROBUSTNESS_ENHANCEMENTS.md) and [FRAMEWORK_COMPARISON.md](FRAMEWORK_COMPARISON.md) for details.

## Known Issues & Warnings

⚠️ **BestOfN Parallel Safety**: When using `WithParallel(true)`, ensure modules are stateless or use N independent instances. Modules with History cause data races.

⚠️ **Concurrency**: History is not thread-safe. Use separate instances for parallel execution.

See [CODEBASE_ANALYSIS_REPORT.md](CODEBASE_ANALYSIS_REPORT.md) for complete issue list and fixes.

## Documentation Structure

```mermaid
graph TD
    R[README.md<br/>Complete Overview] --> Q[QUICKSTART.md<br/>Get Started Fast]
    R --> RD[ROADMAP.md<br/>Implementation Status]
    R --> A[AGENTS.md<br/>Dev Guide]
    Q --> EX[examples/<br/>13+ Examples]

    classDef main fill:#ef476f,stroke:#073b4c,color:#fff
    classDef quick fill:#06d6a0,stroke:#073b4c,color:#000
    classDef ref fill:#118ab2,stroke:#073b4c,color:#fff
    classDef code fill:#ffd166,stroke:#073b4c,color:#000

    class R main
    class Q quick
    class RD,A ref
    class EX code
```

- **README.md** - Main entry point, features, architecture, examples
- **QUICKSTART.md** - Fast onboarding, core concepts, common patterns
- **ROADMAP.md** - Implementation progress, future features
- **AGENTS.md** - Development guide, testing, code style (this file)


## API Conventions

When writing code or examples:
- Modules return `*dsgo.Prediction`, not `map[string]any`
- Access outputs via `result.GetString("field")`, `result.GetInt("field")`, etc.
- BestOfN scorer signature: `func(inputs map[string]any, pred *dsgo.Prediction) (float64, error)`
- Few-shot examples use `module.WithDemos([]dsgo.Example{...})`, not `Signature.WithExamples()`
