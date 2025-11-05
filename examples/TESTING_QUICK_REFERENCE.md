# Test Matrix Quick Reference

## One-Liners

```bash
# Quick validation (2-3 min, 1 model)
make test-matrix-quick

# Sample test (5-10 min, 3 models)
make test-matrix-sample N=3

# Full matrix (30-60 min, 14 models)
make test-matrix
```

## Common Scenarios

### Before Committing
```bash
make test-matrix-quick
```

### Before PR
```bash
make test-matrix-sample N=3
```

### Before Release
```bash
make test-matrix
```

### Custom Model
```bash
export EXAMPLES_DEFAULT_MODEL="openrouter/anthropic/claude-haiku-4.5"
make test-matrix-quick
```

### Verbose Debugging
```bash
go run examples/test_matrix/main.go -n 1 -v
```

### Slow Network (Increase Timeout)
```bash
go run examples/test_matrix/main.go -n 1 -timeout=20m
```

### Rate Limiting (Reduce Concurrency)
```bash
go run examples/test_matrix/main.go -n 1 -c 5
```

### Sequential (No Parallelism)
```bash
go run examples/test_matrix/main.go -n 1 -p=false
```

### No Color (CI/CD)
```bash
go run examples/test_matrix/main.go -n 1 -no-color
```

## Test Scope

| Command | Models | Examples | Total Tests | Time | Cost |
|---------|--------|----------|-------------|------|------|
| `make test-matrix-quick` | 1 | 28 | 28 | 2-3 min | $0.02-0.20 |
| `make test-matrix-sample N=3` | 3 | 28 | 84 | 5-10 min | $0.06-0.60 |
| `make test-matrix-sample N=5` | 5 | 28 | 140 | 8-15 min | $0.10-1.00 |
| `make test-matrix` | 14 | 28 | 392 | 30-60 min | $0.30-3.00 |

## Results

### Success Output
```
✅ All 28 tests passed (100.0%)
Duration: 2m15s
```

### Failure Output
```
❌ 3 tests failed

Failed Tests:
  1. 006_program_of_thought [minimax-m2]
     Error: json_schema not supported
     
  2. 003_react [deepseek-v3.1-terminus]
     Error: provider timeout
```

### Circuit Breaker Tripped
```
🚨 CIRCUIT BREAKER TRIPPED 🚨
Reason: Overall failure threshold exceeded
Cancelling remaining tests...
```

## Logs Location

```
test_matrix_logs/
├── passed/
│   └── gemini-2.5-flash_001_predict_*.log
└── failed/
    └── minimax-m2_006_program_of_thought_*.log
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-n` | 1 | Models: 1=single, N=random, 0=all |
| `-v` | false | Verbose output |
| `-timeout` | 10m | Per-example timeout |
| `-p` | true | Parallel execution |
| `-c` | 20 | Max concurrent tests |
| `-no-color` | false | Disable colored output |

## Circuit Breaker Thresholds

- **Overall**: 85% success required (15% max failures)

## Troubleshooting

| Issue | Solution |
|-------|----------|
| All tests fail | Check `$OPENROUTER_API_KEY` |
| Timeouts | Add `-timeout=20m` |
| Rate limits | Add `-c 5` or `-p=false` |
| OOM errors | Add `-c 10` |
| Circuit breaker trips | Check logs, fix issues |

## See Also

Full documentation: [TESTING.md](TESTING.md)
