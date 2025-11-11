# DSGo Development Guide

**For AI Agents and Human Contributors**

This guide provides all the information needed to understand, develop, and contribute to DSGo.

## Table of Contents

- [Quick Reference](#quick-reference)
- [Architecture Overview](#architecture-overview)
- [Testing Commands](#testing-commands)
- [Development Workflow](#development-workflow)
- [Code Style and Conventions](#code-style-and-conventions)
- [Key Implementation Details](#key-implementation-details)
- [Common Tasks](#common-tasks)
- [Agent Guidelines](#agent-guidelines)

## Quick Reference

### Essential Commands

```bash
# Development cycle
make test          # Run all tests with race detector + coverage
make check         # Format, vet, and build
make all           # Complete validation (clean, check, test, eof-check)

# Specific tasks
make lint          # Run golangci-lint (requires v2.6.0)
make fmt-fix       # Auto-fix formatting
make clean         # Remove coverage files and test cache

# Example testing
make test-matrix-quick        # Test examples with 1 model
make test-matrix-sample N=3   # Test with 3 random models
make test-matrix              # Test all models (comprehensive)
```

### Current Test Coverage

- **Total**: 91.8% ✅
- **Core**: 94.0% ✅
- **Module**: 89.0% ✅
- **jsonutil**: 88.8% ✅
- **retry**: 87.2% ✅
- **OpenAI Provider**: 92.9% ✅
- **OpenRouter Provider**: 88.8% ✅

**Target**: Maintain >90% coverage for all packages.

## Architecture Overview

DSGo is a three-layer architecture implementing the DSPy framework in Go.

### Layer 1: Core (`/` root directory)

**Purpose**: Foundational primitives and interfaces

**Key Files**:
- `signature.go` - Input/output field definitions (Field, Signature, ValidationDiagnostics)
- `lm.go` - Language model interface (LM, Message, GenerateOptions, GenerateResult)
- `module.go` - Base Module interface
- `prediction.go` - Prediction wrapper with metadata and diagnostics
- `adapter.go` - Adapter interface + implementations (JSON, Chat, TwoStep, Fallback)
- `history.go` - Conversation history management
- `example.go` - Few-shot learning support
- `tool.go` - Tool/function calling support
- `cache.go` - LRU caching layer
- `settings.go` - Global configuration
- `configure.go` - Configuration API

**Field Types Supported**:
- `FieldTypeString` - Text data
- `FieldTypeInt` - Integer numbers
- `FieldTypeFloat` - Floating-point numbers
- `FieldTypeBool` - Boolean values
- `FieldTypeJSON` - Structured JSON data
- `FieldTypeClass` - Enum/classification (with aliases)
- `FieldTypeImage` - Image data (partial support)
- `FieldTypeDatetime` - Date/time values

### Layer 2: Modules (`module/`)

**Purpose**: High-level LM behaviors and orchestration

**Implementations**:
- `predict.go` - Basic prediction with structured I/O
- `chain_of_thought.go` - Reasoning with rationale extraction
- `react.go` - Tool-using agent (ReAct pattern: Reason + Act)
- `refine.go` - Iterative output refinement
- `best_of_n.go` - Multiple sampling with scoring
- `program_of_thought.go` - Code generation and optional execution
- `program.go` - Module composition and pipelines

**All modules implement**:
```go
type Module interface {
    Forward(ctx context.Context, inputs map[string]any) (*Prediction, error)
    GetSignature() *Signature
}
```

### Layer 3: Providers (`providers/`)

**Purpose**: LM API implementations

**Current Providers**:
- `openai/` - OpenAI API (GPT-3.5, GPT-4, GPT-4 Turbo, GPT-4o)
- `openrouter/` - OpenRouter API (100+ models)

**Auto-registration**: Providers register themselves via `init()` functions.

### Internal Utilities (`internal/`)

- `jsonutil/` - JSON extraction and repair for malformed LM outputs
- `cost/` - Model pricing tables for usage tracking
- `ids/` - UUID generation for request tracking

### Infrastructure (`logging/`)

- Structured logging with request ID propagation
- Span-based observability
- Multiple collectors (Memory, JSONL, Composite)

## Testing Commands

### Unit Tests

```bash
# Run all tests with race detector and coverage
make test

# Run specific package tests
go test -v ./core/...
go test -v ./module/...
go test -v ./providers/openai/...

# Run single test by name
go test -v -run TestPredictForward
go test -v -run TestReActWithTools ./module/
```

### Example Testing

```bash
# Quick validation (1 model, fast)
make test-matrix-quick

# Sample validation (N random models)
make test-matrix-sample N=3

# Full validation (all models, comprehensive)
make test-matrix
```

### Linting

**Install golangci-lint v2.6.0** (required):
```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.6.0
```

**Run linter**:
```bash
make lint
```

**Note**: `go install` only installs v1.x; v2.x requires binary installation.

## Development Workflow

### Standard Workflow

1. **Understand the task** - Read existing code, use search tools, consult oracle for complex analysis
2. **Plan changes** - Use `todo_write` to break down tasks
3. **Implement** - Write code following conventions (see below)
4. **Write tests** - All new code requires unit tests (target >90% coverage)
5. **Run checks** - `make test` and `make check` during development
6. **Validate** - Run `make all` before completing work
7. **Update docs** - Update README, QUICKSTART, examples as needed

### Pre-commit Hook

Install the pre-commit hook to automatically run checks:
```bash
make install-hooks
```

### When to Use Oracle

Use the `oracle` tool for:
- Planning complex implementations or refactoring
- Reviewing architecture decisions
- Debugging multi-file issues
- Understanding intricate code behavior
- Analyzing test failures

### When Working with Concurrency

- Always run `make test` (includes race detector)
- **Important**: `History` is NOT thread-safe - use separate instances for parallel execution
- **BestOfN parallel safety**: When using `WithParallel(true)`, ensure modules are stateless or use N independent instances

## Code Style and Conventions

### General Go Style

- **Formatting**: Use `gofmt` (run `make fmt-fix` to auto-fix)
- **Naming**:
  - PascalCase for exports
  - camelCase for internals
  - `FieldType*` constants for field types
- **Documentation**: All exported types and functions have doc comments
- **Error handling**:
  - Return `error` as last value
  - Wrap with `fmt.Errorf("context: %w", err)`
  - Always check returned errors

### Interfaces

Keep interfaces small and composable:
```go
// Good: Single responsibility
type Module interface {
    Forward(ctx context.Context, inputs map[string]any) (*Prediction, error)
    GetSignature() *Signature
}

// Good: Composable
type LM interface {
    Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error)
    Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error)
    SupportsJSON() bool
    SupportsTools() bool
}
```

### Test Structure

Use table-driven tests with subtests:
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

### Dependencies

- **Production code**: No external dependencies (stdlib only)
- **Examples**: Only `github.com/joho/godotenv` for `.env` file loading
- **Tests**: Can use additional test libraries if absolutely necessary

### Linting Rules

All code must pass:
- `errcheck` - Check for unchecked errors
- `staticcheck` - Static analysis
- `unused` - Detect unused code
- `govet` - Go vet checks
- `ineffassign` - Detect ineffectual assignments

## Key Implementation Details

### Adapter System

DSGo uses adapters to handle prompt formatting and output parsing. Adapters gracefully handle real-world LM output issues.

**Adapter Types**:

1. **ChatAdapter** - Field marker format `[[ ## field_name ## ]]`
   - Robust fallback parsing
   - Handles malformed markers
   - Used by default for most modules

2. **JSONAdapter** - Structured JSON with schema validation
   - Automatic JSON repair (quotes, commas, trailing commas)
   - Schema generation from signatures
   - Best for structured data

3. **TwoStepAdapter** - Two-phase reasoning
   - First step: Think/reason
   - Second step: Structured extraction
   - Used by modules requiring reasoning

4. **FallbackAdapter** - Chain of adapters (default: Chat → JSON)
   - Automatically retries with different strategies
   - >95% success rate
   - Used by default in most modules

**Metadata Tracking**:
All adapters track:
- Which adapter succeeded
- Number of parse attempts
- Whether fallback was used

### Production-Grade Robustness

#### JSON Repair
Models often emit malformed JSON. DSGo fixes automatically:
- `{'key': 'val'}` → `{"key": "val"}` (single to double quotes)
- `{key: "val"}` → `{"key": "val"}` (unquoted keys)
- `{"a": 1,}` → `{"a": 1}` (trailing commas)
- Smart quote normalization
- Tracked via `__json_repair` metadata

#### Partial Validation
For training/optimization workflows:
- `ValidateOutputsPartial()` returns diagnostics instead of failing
- Missing fields set to `nil` with detailed tracking
- `ParseDiagnostics` attached to predictions for observability

#### Class/Enum Normalization
Flexible matching for classification tasks:
- Case-insensitive: `"POSITIVE"` → `"positive"`
- Configurable aliases: `"pos"` → `"positive"`
- Applied automatically in validation

#### Smart Numeric Extraction
Extract numbers from text descriptions:
- `"High (95%)"` → `95`
- `"Medium"` → `0.7` (qualitative mapping)

### Observability

#### History Entries
All LM calls generate `HistoryEntry` with:
- Request/response content
- Token usage (prompt, completion, total)
- Cost calculation
- Latency tracking
- Provider metadata (rate limits, cache status, request IDs)
- Session/request IDs for tracing

#### Collectors
- `MemoryCollector` - Ring buffer for debugging
- `JSONLCollector` - Production logging to JSONL files
- `CompositeCollector` - Multiple sinks simultaneously

#### Usage Tracking
Every prediction includes:
```go
prediction.Usage.PromptTokens     // Input tokens
prediction.Usage.CompletionTokens // Output tokens
prediction.Usage.TotalTokens      // Total
prediction.Usage.Cost             // USD cost
prediction.Usage.Latency          // Milliseconds
```

### Streaming

Modules support streaming with automatic marker filtering:
```go
result, err := predictor.Stream(ctx, inputs)
for chunk := range result.Chunks {
    fmt.Print(chunk.Content) // Clean content (markers filtered)
}
finalPrediction := <-result.Prediction
err := <-result.Errors
```

**Important**: Streaming emits complete observability data including usage and cost.

### Caching

LRU cache with deterministic keys:
- Includes all parameters (messages, options, tools, penalties, etc.)
- Map canonicalization for consistent keys
- TTL support
- Deep copy to avoid mutation
- Cache hit tracking via metadata

**Cache key components**:
- Messages (role + content)
- Model name
- Temperature, MaxTokens, TopP
- Tools, ToolChoice
- FrequencyPenalty, PresencePenalty
- ResponseFormat, ResponseSchema

## Common Tasks

### Adding a New Module

1. **Create file** in `module/` directory
2. **Implement Module interface**:
   ```go
   type MyModule struct {
       Signature *core.Signature
       LM        core.LM
       Options   *core.GenerateOptions
       Adapter   core.Adapter
   }

   func NewMyModule(sig *core.Signature, lm core.LM) *MyModule {
       return &MyModule{
           Signature: sig,
           LM:        lm,
           Options:   core.DefaultGenerateOptions(),
           Adapter:   core.NewFallbackAdapter(),
       }
   }

   func (m *MyModule) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
       // Implementation
   }

   func (m *MyModule) GetSignature() *core.Signature {
       return m.Signature
   }
   ```
3. **Write tests** in `module/mymodule_test.go`
4. **Add example** in `examples/` if appropriate
5. **Update docs** - README.md and QUICKSTART.md

### Adding a New Provider

1. **Create directory** `providers/myprovider/`
2. **Implement LM interface** in `providers/myprovider/myprovider.go`
3. **Register provider** via `init()`:
   ```go
   func init() {
       core.RegisterLM("myprovider", Factory)
   }

   func Factory(ctx context.Context, model string, options *core.GenerateOptions) (core.LM, error) {
       return NewMyProvider(model, options), nil
   }
   ```
4. **Add pricing** to `internal/cost/pricing.go`
5. **Write tests** with mock responses
6. **Update docs** with supported models

### Adding a New Field Type

1. **Add constant** to `signature.go`:
   ```go
   const FieldTypeMyType FieldType = "mytype"
   ```
2. **Update validation** in `signature.go` `ValidateOutputs()`
3. **Add getter** to `prediction.go`:
   ```go
   func (p *Prediction) GetMyType(field string) (MyType, bool) {
       // Implementation
   }
   ```
4. **Update adapters** if special parsing needed
5. **Write tests** for validation and getters
6. **Add example** demonstrating usage

### Debugging Parsing Issues

Set environment variables:
```bash
export DSGO_DEBUG_PARSE=1           # Show parse attempts
export DSGO_SAVE_RAW_RESPONSES=1    # Save raw LM outputs
export DSGO_DEBUG_MARKERS=1         # Show field markers in streaming
```

Check adapter metadata:
```go
result, _ := module.Forward(ctx, inputs)
fmt.Printf("Adapter used: %s\n", result.AdapterUsed)
fmt.Printf("Parse attempts: %d\n", result.ParseAttempts)
fmt.Printf("Fallback used: %v\n", result.FallbackUsed)

if result.ParseDiagnostics != nil {
    fmt.Printf("Missing fields: %v\n", result.ParseDiagnostics.MissingFields)
    fmt.Printf("Type errors: %v\n", result.ParseDiagnostics.TypeErrors)
}
```

### Running Examples

```bash
# From project root
cd examples/01-hello-chat
EXAMPLES_DEFAULT_MODEL="gpt-4o-mini" go run main.go

# With verbose logging
DSGO_LOG=pretty EXAMPLES_DEFAULT_MODEL="gpt-4o-mini" go run main.go

# Save events to file
DSGO_LOG=events EXAMPLES_DEFAULT_MODEL="gpt-4o-mini" go run main.go > events.jsonl
```

## Agent Guidelines

**For AI agents working with this codebase:**

### DO

✅ **Always use search tools first** - Use `finder` and `Grep` to understand the codebase before making changes

✅ **Use oracle for complex tasks** - Planning, debugging, reviewing architecture

✅ **Use todo_write for tracking** - Break down tasks and check them off as you complete them

✅ **Write comprehensive tests** - All new code requires unit tests with >90% coverage target

✅ **Run validation before finishing** - Always run `make all` after making changes

✅ **Update documentation** - Keep README, QUICKSTART, and examples synchronized

✅ **Follow existing patterns** - Look at similar code before implementing new features

✅ **Check for errors** - Never ignore returned errors, always wrap with context

### DON'T

❌ **Don't create summary documents** - No SUMMARY.md, MIGRATION.md, COVERAGE_*.md files

❌ **Don't assume libraries exist** - Always check if a library is already used before importing

❌ **Don't ignore test failures** - Address all errors and warnings

❌ **Don't skip documentation** - Changes affecting user-facing APIs require doc updates

❌ **Don't batch TODO completion** - Mark TODOs as completed immediately after finishing each one

❌ **Don't use background processes** - Never use `&` operator in shell commands

❌ **Don't suppress errors in production code** - No `as any` or `@ts-expect-error` equivalents unless explicitly requested

### Workflow for Agents

1. **Understand the request** - Read user requirements carefully
2. **Search the codebase** - Use finder/Grep to locate relevant code
3. **Consult oracle if needed** - For complex planning or debugging
4. **Plan with todo_write** - Break down tasks into steps
5. **Implement incrementally** - Complete one TODO at a time, marking each done
6. **Write tests immediately** - Don't defer test writing
7. **Run checks frequently** - `make test` during development
8. **Validate before completion** - `make all` must pass
9. **Update docs if needed** - Reflect changes in user-facing documentation
10. **Report concisely** - Summarize what was done without excessive detail

### Testing Strategy for Agents

When implementing features:
1. Check existing test files for patterns (`*_test.go`)
2. Use table-driven tests with descriptive names
3. Cover success cases, error cases, and edge cases
4. Ensure >90% coverage for new code
5. Run `make test` to verify with race detector
6. Check `make test-matrix-quick` for examples if relevant

### Documentation Strategy for Agents

When features affect users:
1. **README.md** - Update if architecture or core concepts change
2. **QUICKSTART.md** - Add examples for new modules or major features
3. **examples/** - Create or update examples demonstrating new functionality
4. **AGENTS.md** - Update if development workflow changes (this file)
5. **ROADMAP.md** - Mark items complete, update status

Don't create intermediate docs like:
- COVERAGE_*.md (coverage is tracked in tests)
- MIGRATION.md (migration info goes in README or examples)
- SUMMARY.md (summaries belong in README)
- ERROR_HANDLING.md (error handling documented in code)

## Known Issues & Warnings

⚠️ **BestOfN Parallel Safety**: When using `WithParallel(true)`, ensure modules are stateless or use N independent instances. Modules with History cause data races.

⚠️ **Concurrency**: History is NOT thread-safe. Use separate instances for parallel execution.

⚠️ **Streaming**: StreamResult channels must be fully consumed to avoid goroutine leaks.

⚠️ **Cache Mutations**: Cache entries are deep-copied to prevent mutations from affecting cached values.

## Project Status

### Implementation Progress

**Core Modules**: 7/7 complete ✅
- Predict, ChainOfThought, ReAct, Refine, BestOfN, ProgramOfThought, Program

**Adapters**: 4/4 complete ✅
- JSON, Chat, TwoStep, Fallback

**Providers**: 2/4 planned
- OpenAI ✅, OpenRouter ✅
- Groq (planned), Cerebras (planned)

**Overall DSPy Parity**: ~70%

See [ROADMAP.md](ROADMAP.md) for detailed implementation status and future plans.

## Resources

- **Main README**: [README.md](README.md) - Project overview and features
- **Quick Start**: [QUICKSTART.md](QUICKSTART.md) - Get started in minutes
- **Roadmap**: [ROADMAP.md](ROADMAP.md) - Implementation status and future plans
- **LLM Documentation**: [llms.txt](llms.txt) - AI-friendly documentation index
- **Examples**: [examples/](examples/) - Working examples for all features

## Questions or Issues?

- Check existing tests for usage patterns
- Review examples for real-world usage
- Use oracle for complex analysis
- Open an issue on GitHub for bugs or feature requests
