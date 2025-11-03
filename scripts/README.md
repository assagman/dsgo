# Scripts

## Test Scripts

### test_examples_matrix - Unified Testing Script

**Replaces both `test_examples` and `test_matrix`** with a single, flexible script.

```bash
# Run all examples with single model (default: gpt-4o-mini)
make test-matrix-quick

# Run with 3 random models
make test-matrix-sample N=3

# Run with all 10 models (full matrix)
make test-matrix

# Advanced: Direct script access with custom flags
go run scripts/test_examples_matrix/main.go -p=false -v
```

**Make Targets:**
- `make test-matrix-quick` - Single model, fast (default: gpt-4o-mini)
- `make test-matrix-sample N=<number>` - Test with N random models (e.g., `N=3`)
- `make test-matrix` - All models, comprehensive

**Environment Variables:**
- `OPENROUTER_MODEL` or `MODEL` - Override default model for single-model testing

### Legacy Scripts (Deprecated)

- **test_examples/** - Use `make test-matrix-quick` instead
- **test_matrix/** - Use `make test-matrix` instead

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
make test-matrix-quick

# Test with 3 random models (moderate coverage)
make test-matrix-sample N=3

# Full matrix test (comprehensive)
make test-matrix

# Debug a failing example (advanced)
go run scripts/test_examples_matrix/main.go -n 1 -v -p=false
```
