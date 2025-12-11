# DSGo - Development Guide for AI Agents

> Go framework for building structured LLM applications. DSPy-style composable modules with type-safe signatures.

---

## Quick Commands

```bash
# Essential workflow
make all                    # Full validation: clean + check + lint + test-race
make test                   # Fast: unit + integration (no race)
make check                  # Format + vet + build

# Development
make fmt-fix               # Auto-fix formatting
make lint                  # Run golangci-lint (requires v2.6.0)
go test -v -run TestName   # Run specific test

# Examples
cd examples/codebase_analysis && go run main.go
DSGO_LOG=pretty go run main.go  # With verbose logging
```

---

## Tech Stack

- **Language**: Go 1.25+
- **Dependencies**: Standard library only (zero external deps)
- **Lint**: golangci-lint v2.6.0 (binary install required)
- **Test**: `go test -race` with >90% coverage target

---

## Project Structure

```
dsgo/
├── dsgo.go                    # Main package - public API re-exports
├── internal/
│   ├── core/                  # Layer 2: Primitives
│   │   ├── signature.go       # I/O field definitions
│   │   ├── lm.go              # LM interface
│   │   ├── adapter.go         # Output parsing (Chat, JSON, TwoStep, Fallback)
│   │   ├── module.go          # Module interface
│   │   ├── prediction.go      # Result wrapper with metadata
│   │   ├── history.go         # Conversation tracking
│   │   ├── tool.go            # Function calling
│   │   ├── cache.go           # LRU caching
│   │   ├── settings.go        # Global configuration
│   │   └── collector.go       # Observability collectors
│   │
│   ├── module/                # Layer 3: High-level behaviors
│   │   ├── predict.go         # Basic prediction
│   │   ├── chain_of_thought.go
│   │   ├── react.go           # Tool-using agent
│   │   ├── refine.go
│   │   ├── best_of_n.go
│   │   ├── program.go         # Pipeline composition
│   │   ├── parallel.go        # Concurrent execution
│   │   └── multi_chain_comparison.go
│   │
│   ├── providers/             # Layer 1: LLM implementations
│   │   ├── openai/            # OpenAI API
│   │   └── openrouter/        # OpenRouter (100+ models)
│   │
│   ├── logging/               # Structured logging
│   ├── mcp/                   # Model Context Protocol
│   ├── typed/                 # Generic Func[I,O] wrappers
│   ├── jsonutil/              # JSON extraction & repair
│   ├── cost/                  # Pricing tables
│   ├── retry/                 # Retry logic
│   └── env/                   # Environment loading
│
├── examples/                  # Working examples
├── integration/               # Integration tests
└── scripts/                   # Build scripts
```

**Key principle**: Implementation in `internal/*`, public API via re-exports in `dsgo.go`.

---

## Code Style

### Good Examples

```go
// Signature definition - fluent builder pattern
sig := dsgo.NewSignature("Classify sentiment").
    AddInput("text", dsgo.FieldTypeString, "Text to classify").
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

- Adding new dependencies (currently zero external deps)
- Changing public API signatures in `dsgo.go`
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
make test                      # Unit + integration, no race

# Full validation (CI/final)
make test-race                 # With race detector

# Specific tests
go test -v -run TestPredictForward ./internal/module/
go test -v -run TestAdapter ./internal/core/
```

### Coverage Targets

| Package | Target | Current |
|---------|--------|---------|
| internal/core | >90% | 94.0% |
| internal/module | >85% | 89.0% |
| internal/providers | >85% | 90%+ |
| Total | >90% | 91.8% |

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
4. **FallbackAdapter**: Chains adapters for >95% success rate

```go
// Default: FallbackAdapter (Chat → JSON)
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

1. Create `internal/module/mymodule.go`
2. Implement `Module` interface with `Clone()` method
3. Add builder pattern with `With*()` methods
4. Write tests in `internal/module/mymodule_test.go`
5. Re-export in `dsgo.go` if public
6. Add example in `examples/` if appropriate

### Adding a New Provider

1. Create `internal/providers/myprovider/lm.go`
2. Implement `LM` interface
3. Register via `init()`:
   ```go
   func init() {
       core.RegisterLM("myprovider", Factory)
   }
   ```
4. Add pricing to `internal/cost/cost.go`
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
# API Keys
OPENAI_API_KEY=sk-...
OPENROUTER_API_KEY=sk-or-...

# Runtime
DSGO_TIMEOUT=30s
DSGO_MAX_RETRIES=3
DSGO_CACHE_SIZE=1000

# Debugging
DSGO_LOG=pretty              # none, pretty, events
DSGO_DEBUG_PARSE=1           # Show parse attempts
```

---

## Resources

| Document | Purpose |
|----------|---------|
| [README.md](README.md) | Complete project reference |
| [QUICKSTART.md](QUICKSTART.md) | Step-by-step tutorials |
| [API.md](API.md) | Detailed API documentation |
| [ROADMAP.md](ROADMAP.md) | Implementation status |
| [llms.txt](llms.txt) | LLM-friendly documentation |
| [examples/](examples/) | Working code examples |

---

## Agent-Specific Guidelines

### For AI Coding Assistants

1. **Search first**: Use search tools to understand existing code before changes
2. **Track tasks**: Use todo lists to break down complex work
3. **Test immediately**: Write tests alongside implementation
4. **Validate always**: Run `make all` before marking work complete
5. **Follow patterns**: Look at similar code for consistency

### File Navigation Tips

| Want to... | Look at |
|------------|---------|
| Understand signatures | `internal/core/signature.go` |
| See module patterns | `internal/module/predict.go` |
| Add LLM provider | `internal/providers/openai/lm.go` |
| Debug parsing | `internal/core/adapter.go` |
| Add observability | `internal/core/collector.go` |
| See usage patterns | `examples/codebase_analysis/main.go` |
| Understand tests | `internal/module/*_test.go` |
