# DSGo Development Guide

## Testing Commands
- All tests: `go test $(go list ./... | grep -v /examples/)`
- Single test: `go test -run TestName`
- With coverage: `go test -v -cover $(go list ./... | grep -v /examples/)`
- Build check: `go build ./...`

## Architecture
Go port of DSPy (Declarative Self-improving Language Programs). Core modules in root:
- `signature.go` - I/O field definitions (Field, Signature types)
- `lm.go` - LM interface (Message, GenerateOptions, GenerateResult)
- `module.go` - Base Module interface & Predict implementation
- `chain_of_thought.go`, `react.go`, `best_of_n.go`, `program_of_thought.go`, `refine.go` - Advanced modules
- `tool.go` - Tool/function calling support
- `program.go` - Program composition
- `examples/` - LM providers (openai/) and usage examples (sentiment/, react_agent/, research_assistant/, etc.)

## Code Style
- Standard Go: `gofmt` formatting, exported types/funcs have doc comments
- Naming: PascalCase exports, camelCase internals, FieldType* constants
- Error handling: Return `error` as last value, wrap with `fmt.Errorf("context: %w", err)`
- Interfaces: Small, composable (Module, LM)
- Tests: Table-driven with subtests (`t.Run(tt.name, ...)`)
- No external deps except godotenv for examples (use stdlib)

## Development Workflow
- Always run `go build ./...` and `go test ./...` during development
- No temporary UPPER_CASE.md files (SUMMARY.md, CHANGES.md, etc.) - update existing docs only
- Keep responses concise
- Ask user for feedback/choices at important checkpoints
