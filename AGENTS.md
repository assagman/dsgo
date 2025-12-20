# DSGo - Development Guide for AI Agents

> Go framework for building structured LLM applications. DSPy-style composable modules with type-safe signatures.

---

## Quick Commands

```bash
# Essential workflow
make all                    # Full validation: clean + check + lint + test-race
make test                   # Fast: unit (no race)
make check                  # verify + fmt + vet + build + check-eof

# Development
make fmt-fix                  # Auto-fix formatting
make lint                     # Run golangci-lint (install via go install ...@latest)
./scripts/check-eof.sh --fix  # Auto-fix missing EOF newlines
go test -v -run TestName      # Run specific test

# Examples
# (Examples directory removed in this layout.)
```

---

## Tech Stack

- **Language**: Go 1.25+
- **Dependencies**: Direct: `github.com/openai/openai-go/v3` (providers); indirect: `github.com/tidwall/gjson`, `github.com/tidwall/sjson`
- **Lint**: golangci-lint (install via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
- **Test**: `go test -race` with >90% coverage target

---

## Project Structure

```
dsgo/
├── core/                      # Layer 2: Primitives
│   ├── signature.go           # I/O field definitions
│   ├── lm.go                  # LM interface
│   ├── adapter.go             # Output parsing (Chat, JSON, TwoStep, Fallback)
│   ├── module.go              # Module interface
│   ├── prediction.go          # Result wrapper with metadata
│   ├── history.go             # Conversation tracking
│   ├── tool.go                # Function calling
│   ├── cache.go               # LRU caching
│   ├── settings.go            # Global configuration
│   └── collector.go           # Observability collectors
│
├── module/                    # Layer 3: High-level behaviors
│   ├── predict.go             # Basic prediction
│   ├── chain_of_thought.go
│   ├── react.go               # Tool-using agent
│   ├── refine.go
│   ├── best_of_n.go
│   ├── program.go             # Pipeline composition
│   ├── parallel.go            # Concurrent execution
│   ├── program_of_thought.go   # Code-first reasoning
│   └── multi_chain_comparison.go
│
├── provider/                  # Layer 1: LLM implementations
│   ├── openai/                # OpenAI API
│   └── openrouter/            # OpenRouter (100+ models)
│   └── mock/                  # Test/mock provider
│
├── logging/                   # Structured logging
├── mcp/                       # Model Context Protocol
├── signature_typed/           # Typed signatures & Func[I,O] implementation
├── cost/                      # Pricing tables
├── modelcatalog/              # Model registry
├── internal/                  # Internal helpers (jsonutil, retry, env, ids)
└── scripts/                   # Build scripts
```

**Key principle**: Public API is package-per-concern (`core`, `module`, `provider/*`, etc.), with internal-only helpers under `internal/*`.

Typed functionality (Func[I,O], typed Predict, struct helpers) lives in `signature_typed` and should be used via that package (e.g., `signature_typed.Func`, `signature_typed.NewPredict`, `signature_typed.StructToMap`).

---

## Code Style

### Good Examples

```go
// Signature definition - fluent builder pattern
sig := core.NewSignature("Classify sentiment").
    AddInput("text", core.FieldTypeString, "Text to classify").
    AddClassOutput("sentiment", []string{"positive", "negative", "neutral"}, "Sentiment")

// Module creation with options
predictor := module.NewPredict(sig, lm).
    WithOptions(&core.GenerateOptions{Temperature: 0.7}).
    WithAdapter(core.NewFallbackAdapter())

// Error handling - always wrap with context
result, err := predictor.Forward(ctx, inputs)
if err != nil {
    return nil, fmt.Errorf("prediction failed: %w", err)
}

// Type-safe getters
sentiment := result.GetString("sentiment")
confidence := result.GetFloat("confidence")
```

### Bad Examples

```go
// BAD: No error handling
result, _ := predictor.Forward(ctx, inputs)

// BAD: Panic instead of error
if err != nil {
    panic(err)
}

// BAD: Not wrapping errors
if err != nil {
    return err  // Missing context
}

// BAD: Direct map access without type safety
sentiment := result.Outputs["sentiment"].(string)  // Use GetString() instead
```

### Naming Conventions

- **PascalCase**: Exported types/functions (`NewPredict`, `FieldTypeString`)
- **camelCase**: Internal variables (`predictor`, `inputFields`)
- **Constants**: `FieldType*` prefix for field types
- **Interfaces**: Small and focused (`Module`, `LM`, `Adapter`)

### Test Structure

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "hello", "HELLO", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Function() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Function() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

## Boundaries

### Always Do

- Run `make all` before completing any task
- Write table-driven tests for new code (>90% coverage)
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Use context.Context for all operations
- Check existing patterns before implementing new features
- Use type-safe getters (`GetString`, `GetInt`, etc.)
- Keep modules stateless or use Clone() for parallel safety

### Ask First

- Adding new dependencies (current direct dep: `github.com/openai/openai-go/v3`; indirect: `github.com/tidwall/gjson`, `github.com/tidwall/sjson`)
- Changing public API signatures in `core`, `module`, `provider/*`, `logging`, `mcp`, `cost`, `modelcatalog`, or `signature_typed`
- Modifying adapter parsing logic
- Adding new field types
- Changing thread-safety guarantees

### Never Do

- Commit API keys or secrets
- Skip error handling (no `_` for errors)
- Add external dependencies to core packages
- Break backward compatibility without discussion
- Modify `internal/` package visibility
- Use `panic()` in production code
- Create SUMMARY.md, MIGRATION.md, or other intermediate docs

---

## Testing Strategy

### Running Tests

```bash
# Fast development cycle
make test                      # Unit, no race

# Full validation (CI/final)
make test-race                 # With race detector

# Specific tests
go test -v -run TestPredictForward ./module/
go test -v -run TestAdapter ./core/
```

### Coverage Targets

| Package | Target |
|---------|--------|
| core | >90% |
| module | >85% |
| provider | >85% |
| Total | >90% |

### What to Test

1. **Success cases**: Normal input → expected output
2. **Error cases**: Invalid input → proper error
3. **Edge cases**: Empty input, nil values, large inputs
4. **Concurrency**: Use `-race` flag for parallel code

---

## Key Implementation Details

### Adapter System

Adapters handle LLM output parsing with automatic fallback:

1. **ChatAdapter**: Field markers `[[ ## field ## ]]` - general purpose
2. **JSONAdapter**: Structured JSON with schema - for APIs
3. **TwoStepAdapter**: Reason then extract - for complex tasks
4. **FallbackAdapter**: Chains adapters for robust parsing

```go
// Default: FallbackAdapter (JSON → Chat)
adapter := core.NewFallbackAdapter()

// Specific adapter
adapter := core.NewJSONAdapter()
```

### Thread Safety

All components are automatically thread-safe:

```go
// Safe for concurrent use
predictor := module.NewPredict(sig, lm).WithHistory(history)
parallel := module.NewParallel(predictor).WithMaxWorkers(50)
```

- **BestOfN**: Auto-clones modules for parallel execution
- **Parallel**: Auto-clones per task for state isolation
- **History**: RWMutex protection with copy-on-read

### Module Interface

All modules implement:

```go
type Module interface {
    Forward(ctx context.Context, inputs map[string]any) (*Prediction, error)
    GetSignature() *Signature
    Clone() Module  // For thread-safe parallelization
}
```

---

## Common Tasks

### Adding a New Module

1. Create `module/mymodule.go`
2. Implement `Module` interface with `Clone()` method
3. Add builder pattern with `With*()` methods
4. Write tests in `module/mymodule_test.go`
5. Add docs if appropriate

### Adding a New Provider

1. Create `provider/myprovider/lm.go`
2. Implement `LM` interface
3. Register via `init()`:
   ```go
   func init() {
       core.RegisterLM("myprovider", Factory)
   }
   ```
4. Add pricing to `cost/cost.go`
5. Write tests with mock responses

### Debugging Parsing Issues

```bash
export DSGO_DEBUG_PARSE=1           # Show parse attempts
export DSGO_SAVE_RAW_RESPONSES=1    # Save raw LLM outputs
export DSGO_DEBUG_MARKERS=1         # Show field markers in streaming
```

```go
result, _ := module.Forward(ctx, inputs)
fmt.Printf("Adapter: %s, Attempts: %d, Fallback: %v\n",
    result.AdapterUsed, result.ParseAttempts, result.FallbackUsed)
```

---

## Git Workflow

### Commit Messages

```
feat(module): add MultiChainComparison module
fix(adapter): handle malformed JSON with trailing commas
test(core): add edge cases for signature validation
docs: update QUICKSTART with streaming example
```

### Branch Naming

- `feat/feature-name` - New features
- `fix/issue-description` - Bug fixes
- `refactor/component-name` - Code improvements
- `test/test-description` - Test additions

### Before Committing

```bash
make all  # Must pass
```

---

## Environment Variables

```bash
# API keys (DSGO_* preferred; OPENAI/OPENROUTER supported)
DSGO_OPENAI_API_KEY=sk-...
OPENAI_API_KEY=sk-...
DSGO_OPENROUTER_API_KEY=sk-or-...
OPENROUTER_API_KEY=sk-or-...

# Runtime defaults
DSGO_TIMEOUT=30              # seconds
DSGO_MAX_RETRIES=3
DSGO_TRACING=true
DSGO_MAX_TOKENS=10000
DSGO_TEMPERATURE=0.7
DSGO_HTTP_TIMEOUT_MS=300000  # provider HTTP timeout (ms)

# Caching
DSGO_CACHE=true
DSGO_CACHE_TTL=5m
DSGO_CACHE_MEMORY=1000
DSGO_CACHE_DISK=true
DSGO_CACHEDIR=~/.dsgo_cache
DSGO_CACHE_LIMIT=32212254720 # bytes (30GB default)

# Structured outputs
DSGO_STRUCTURED_OUTPUTS=true
DSGO_STRUCTURED_MAX_ATTEMPTS=3
DSGO_STRUCTURED_TEMPERATURE=0.0

# ReAct tuning
DSGO_REACT_MAX_PROMPT_BYTES=262144

# Logging
DSGO_LOG_LEVEL=info
DSGO_LOG_FORMAT=text
DSGO_LOG_COLOR=auto           # auto, always, never
DSGO_LOG_MODULE_LEVELS=module.Predict=debug
DSGO_LOG_BUFFER_SIZE=1000
DSGO_LOG_FLUSH_INTERVAL=200ms
DSGO_LOG_FLUSH_TIMEOUT=1s
DSGO_LOG_BATCH_SIZE=50
DSGO_LOG_DROP_WHEN_FULL=1
DSGO_LOG_MAX_MEMORY=10485760
DSGO_LOG_CACHE_SLOW_THRESHOLD=200ms
DSGO_LOG_TOOL_SLOW_THRESHOLD=200ms
DSGO_LOG_BLOCK_TIMEOUT=250ms

# Debugging
DSGO_DEBUG_PARSE=1
DSGO_DEBUG_MARKERS=1

# .env loading
DSGO_ENV_FILE_PATH=/path/to/.env

# Provider extras
OPENROUTER_SITE_NAME=your-app
OPENROUTER_SITE_URL=https://example.com
DSGO_MOCK_BASE_URL=http://localhost:8080
DSGO_MOCK_API_KEY=test

# Example overrides (legacy)
EXAMPLES_MAX_TOKENS=10000
EXAMPLES_TEMPERATURE=0.7
```

---

## Resources

| Document | Purpose |
|----------|---------|
| [README.md](README.md) | Index + diagrams + tiny example |
| [QUICKSTART.md](QUICKSTART.md) | Step-by-step tutorials |
| [REFERENCE.md](REFERENCE.md) | Tables + links (quick reference) |
| [ROADMAP.md](ROADMAP.md) | Implementation status |
| [llms.txt](llms.txt) | LLM-friendly documentation |
| (examples removed) | None |

---

## Agent-Specific Guidelines

### For AI Coding Assistants

1. **Search first**: Use search tools to understand existing code before changes
2. **Track tasks**: Use todo lists to break down complex work
3. **Test immediately**: Write tests alongside implementation
4. **Validate always**: Run `make all` before marking work complete
5. **Follow patterns**: Look at similar code for consistency
6. **Use signature_typed for typed APIs**: prefer `signature_typed.Func`, `signature_typed.NewPredict`, etc.

### File Navigation Tips

| Want to... | Look at |
|------------|---------|
| Understand signatures | `core/signature.go` |
| See module patterns | `module/predict.go` |
| Add LLM provider | `provider/openai/lm.go` |
| Debug parsing | `core/adapter.go` |
| Add observability | `core/collector.go` |
| Understand tests | `module/*_test.go` |
| Work on typed Func[I,O] / typed modules | `signature_typed/*.go` |
