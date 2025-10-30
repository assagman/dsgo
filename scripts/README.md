# Scripts

## Test Scripts

### test_examples_matrix - Unified Testing Script

**Replaces both `test_examples` and `test_matrix`** with a single, flexible script.

```bash
# Run all examples with single model (default: gpt-4o-mini)
go run scripts/test_examples_matrix/main.go

# Run with 3 random models
go run scripts/test_examples_matrix/main.go -n 3

# Run with all 10 models (full matrix)
go run scripts/test_examples_matrix/main.go -n 0

# Run sequentially (not parallel)
go run scripts/test_examples_matrix/main.go -p=false

# Verbose output
go run scripts/test_examples_matrix/main.go -v

# Custom timeout
go run scripts/test_examples_matrix/main.go -timeout=5m
```

**Flags:**
- `-n <number>` - Number of random models to test
  - `1` (default): Single model, like old `test_examples`
  - `0`: All models, like old `test_matrix`
  - `3`: Random 3 models
- `-p` - Parallel execution (default: true)
- `-v` - Verbose output
- `-timeout` - Timeout per example (default: 3m)

**Environment Variables:**
- `OPENROUTER_MODEL` or `MODEL` - Override default model for `-n 1`

### Legacy Scripts (Deprecated)

- **test_examples/** - Use `test_examples_matrix -n 1` instead
- **test_matrix/** - Use `test_examples_matrix -n 0` instead

## Utility Scripts

### Git Hooks

- **install-hooks.sh** - Install pre-commit hooks
- **pre-commit** - Pre-commit hook (runs `make all`)

### Code Quality

- **fix_fmt.sh** - Format all Go files
- **check-eof.sh** - Check for EOF newlines
- **scan-trailing-spaces.sh** - Scan for trailing whitespace

## Examples

```bash
# Quick test with 1 model (fast)
go run scripts/test_examples_matrix/main.go

# Test with 3 random models (moderate coverage)
go run scripts/test_examples_matrix/main.go -n 3

# Full matrix test (comprehensive)
go run scripts/test_examples_matrix/main.go -n 0

# Debug a failing example
go run scripts/test_examples_matrix/main.go -n 1 -v -p=false
```
