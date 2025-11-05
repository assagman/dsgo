# Examples Reorganization - Implementation Summary

## Status: Phase 4 Complete ✅ | All Examples Migrated 🎉

### Recent Consolidation

**011 + 015 Reorganization** (Completed):
- **015_fewshot**: Deep dive on few-shot learning with WithDemos()
  - Zero-shot vs few-shot comparison
  - 5 example demonstrations
  - Multiple test cases proving effectiveness
  - Advanced patterns and use cases
- **011_history_prediction**: Focused on History and Prediction primitives
  - Removed few-shot demo (now in 015)
  - Focuses on conversation history and metadata
  - Clearer separation of concerns

### Completed Tasks

#### 1. Shared Harness Infrastructure ✅
Created `examples/shared/_harness/` with:
- **harness.go**: Core runner with worker pool (50 concurrent executions)
- **config.go**: CLI flags and environment variable handling

**Features Implemented:**
- ✅ Concurrent execution with semaphore-based worker pool
- ✅ Detailed execution statistics (tokens, timing, cache hits, retries)
- ✅ Automatic error dumps to `examples/errors/` with full Prediction data
- ✅ Multiple output formats (summary, JSON, NDJSON)
- ✅ Configurable via CLI flags and environment variables
- ✅ Thread-safe result collection

**CLI Flags:**
```bash
-concurrency=50        # Number of concurrent executions
-timeout=30            # Timeout in seconds
-error-dir=path        # Error dump directory
-format=summary        # Output format: summary, json, ndjson
-verbose               # Verbose output
```

**Environment Variables:**
```bash
HARNESS_CONCURRENCY    # Override concurrency
HARNESS_TIMEOUT        # Override timeout
HARNESS_ERROR_DIR      # Override error directory
HARNESS_OUTPUT_FORMAT  # Override output format
HARNESS_VERBOSE        # Override verbose flag
```

#### 2. Pilot Examples Migrated ✅

**001_predict** - Basic Prediction
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Signature, Predict module, class outputs
- ✅ Builds successfully

**013_sentiment** - Chain of Thought
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: ChainOfThought, rationale access, multi-output
- ✅ Builds successfully

**022_caching** - LM Cache
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: LMCache, performance metrics, cache statistics
- ✅ Builds successfully

#### 3. Documentation Updates ✅

**QUICKSTART.md Updates:**
- ✅ Updated example references (001_predict, 013_sentiment, 022_caching)
- ✅ Added "New Harness Infrastructure" section
- ✅ Added harness features list
- ✅ Updated "Next Steps" with numbered examples
- ✅ Updated "Examples by Use Case" section

### Example Template Structure

Each numbered example follows this structure:

```
examples/NNN_name/
├── main.go          # Example implementation with harness
└── README.md        # Documentation with usage, concepts, links
```

**main.go Template:**
```go
package main

import (
    "context"
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/examples/shared"
    "github.com/assagman/dsgo/examples/shared/_harness"
    "github.com/assagman/dsgo/module"
)

func main() {
    shared.LoadEnv()
    config, _ := harness.ParseFlags()
    h := harness.NewHarness(config)
    
    err := h.Run(context.Background(), "NNN_name", runExample)
    if err != nil {
        log.Fatal(err)
    }
    
    h.OutputResults()
}

func runExample(ctx context.Context) (*dsgo.Prediction, *harness.ExecutionStats, error) {
    stats := &harness.ExecutionStats{
        Metadata: make(map[string]any),
    }
    
    // Example implementation
    // ...
    
    stats.TokensUsed = result.Usage.TotalTokens
    stats.Metadata["key"] = "value"
    
    return result, stats, nil
}
```

#### 4. Core Module Examples Migrated ✅

**002_chain_of_thought** - Chain of Thought Reasoning
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: ChainOfThought module, reasoning via Rationale, math problems
- ✅ Builds successfully

**003_react** - ReAct Agent with Tools
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: ReAct module, tool usage (search, calculator), verbose mode
- ✅ Builds successfully

**004_refine** - Iterative Refinement
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Refine module, iterative improvement, feedback-based refinement
- ✅ Builds successfully

**005_best_of_n** - Best of N Sampling
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: BestOfN module, parallel execution, custom scoring, early stopping
- ✅ Builds successfully

**006_program_of_thought** - Code Generation
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: ProgramOfThought module, Python code generation, execution control
- ✅ Builds successfully

**007_program_composition** - Module Composition & Pipelines
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Program composition, chaining modules, ChainOfThought + BestOfN, hybrid workflows
- ✅ Builds successfully

**008_chat_predict** - Multi-Turn Conversations with History
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Conversation history, multi-turn context, system messages, token tracking
- ✅ Builds successfully

**009_chat_cot** - Multi-Turn Chain of Thought Reasoning
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: ChainOfThought with history, multi-turn reasoning, step-by-step problem solving, educational applications
- ✅ Builds successfully

**010_typed_signatures** - Type-Safe API with Generics
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Typed input/output structs, generics API, typed few-shot, typed CoT/ReAct, compile-time safety
- ✅ Builds successfully

**011_history_prediction** - History and Prediction Primitives
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: History management, multi-turn conversations, rich predictions with metadata, type-safe getters
- ✅ Builds successfully
- ✅ Consolidated: Removed few-shot demo (now in 015_fewshot)

**012_math_solver** - Math Solver with Program of Thought
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: ProgramOfThought for code-based reasoning, Python code generation, financial/statistical/physics problems, safety controls
- ✅ Builds successfully

#### 5. Advanced Features Examples Started ✅

**014_adapter_fallback** - Resilient Response Parsing with Adapter Fallback
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: FallbackAdapter system, ChatAdapter → JSONAdapter fallback, adapter metrics, parse robustness, >95% success rate
- ✅ Builds successfully

**015_fewshot** - Few-Shot Learning with Example Demonstrations
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Few-shot learning with WithDemos(), zero-shot vs few-shot comparison, dsgo.NewExample(), improved accuracy through demonstrations
- ✅ Builds successfully

**016_history** - Advanced History Management
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: WithHistory(), NewHistoryWithLimit(), Clone(), GetLast(), Clear(), manual message addition, conversation branching, context window management
- ✅ Builds successfully

**017_tools** - Tool Definition & Integration
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: NewTool(), required/optional parameters, multiple parameter types, tool integration with ReAct, error handling, stateless and stateful tools
- ✅ Builds successfully

**018_adapters** - Adapter System Overview
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: JSONAdapter, ChatAdapter, FallbackAdapter, TwoStepAdapter, custom adapter chains, adapter metrics, when to use each adapter type
- ✅ Builds successfully

**019_retry_resilience** - Automatic Retry & Resilience
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Automatic retry on 429/5xx errors, exponential backoff with jitter, max 3 retries, context-aware retry, works across all modules
- ✅ Builds successfully

**020_streaming** - Real-Time Streaming Output
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Real-time streaming with Stream(), chunk-by-chunk processing, better UX, error handling, works with Predict module
- ✅ Builds successfully

**021_best_of_n_parallel** - Parallel Candidate Generation
- ✅ main.go with harness integration
- ✅ README.md with usage and documentation
- ✅ Demonstrates: Parallel execution with WithParallel(true), 2-3x speedup, custom scoring, early stopping, WithReturnAll(), concurrency safety
- ✅ Builds successfully

### Next Phase: Production Examples

The following examples need to be migrated to the numbered structure:

**Advanced Features (014-021)**
- ~~014_adapter_fallback~~ ✅
- ~~015_fewshot~~ ✅
- ~~016_history~~ ✅
- ~~017_tools~~ ✅
- ~~018_adapters~~ ✅
- ~~019_retry_resilience~~ ✅
- ~~020_streaming~~ ✅
- ~~021_best_of_n_parallel~~ ✅

**Production (023-028)**
- ~~023_global_config~~ ✅
- ~~024_lm_factory~~ ✅
- ~~025_logging_tracing~~ ✅
- ~~026_observability~~ ✅
- ~~027_research_assistant~~ ✅
- ~~028_code_reviewer~~ ✅

### Directory Structure

```
examples/
├── shared/
│   ├── _harness/         # ✅ New harness infrastructure
│   │   ├── harness.go
│   │   └── config.go
│   ├── env.go
│   └── provider.go
├── errors/               # Auto-created by harness for error dumps
├── 001_predict/          # ✅ Migrated - Basic Prediction
│   ├── main.go
│   └── README.md
├── 002_chain_of_thought/ # ✅ Migrated - CoT Reasoning
│   ├── main.go
│   └── README.md
├── 003_react/            # ✅ Migrated - ReAct Agent
│   ├── main.go
│   └── README.md
├── 004_refine/           # ✅ Migrated - Iterative Refinement
│   ├── main.go
│   └── README.md
├── 005_best_of_n/        # ✅ Migrated - Best of N Sampling
│   ├── main.go
│   └── README.md
├── 006_program_of_thought/ # ✅ Migrated - Code Generation
│   ├── main.go
│   └── README.md
├── 007_program_composition/ # ✅ Migrated - Module Composition & Pipelines
│   ├── main.go
│   └── README.md
├── 008_chat_predict/     # ✅ Migrated - Multi-Turn Conversations
│   ├── main.go
│   └── README.md
├── 009_chat_cot/         # ✅ Migrated - Multi-Turn Chain of Thought
│   ├── main.go
│   └── README.md
├── 010_typed_signatures/ # ✅ Migrated - Type-Safe API with Generics
│   ├── main.go
│   └── README.md
├── 011_history_prediction/ # ✅ Migrated - History and Prediction Primitives
│   ├── main.go
│   └── README.md
├── 012_math_solver/      # ✅ Migrated - Math Solver with Program of Thought
│   ├── main.go
│   └── README.md
├── 013_sentiment/        # ✅ Migrated - Sentiment Analysis
│   ├── main.go
│   └── README.md
├── 014_adapter_fallback/ # ✅ Migrated - Resilient Response Parsing
│   ├── main.go
│   └── README.md
├── 015_fewshot/          # ✅ Migrated - Few-Shot Learning
│   ├── main.go
│   └── README.md
├── 016_history/          # ✅ Migrated - Advanced History Management
│   ├── main.go
│   └── README.md
├── 017_tools/            # ✅ Migrated - Tool Definition & Integration
│   ├── main.go
│   └── README.md
├── 018_adapters/         # ✅ Migrated - Adapter System Overview
│   ├── main.go
│   └── README.md
├── 019_retry_resilience/ # ✅ Migrated - Automatic Retry & Resilience
│   ├── main.go
│   └── README.md
├── 020_streaming/        # ✅ Migrated - Real-Time Streaming Output
│   ├── main.go
│   └── README.md
├── 021_best_of_n_parallel/ # ✅ Migrated - Parallel Candidate Generation
│   ├── main.go
│   └── README.md
├── 022_caching/          # ✅ Migrated - LM Cache
│   ├── main.go
│   └── README.md
├── 023_global_config/    # ✅ Migrated - Global Configuration System
│   ├── main.go
│   └── README.md
├── 024_lm_factory/       # ✅ Migrated - LM Factory Pattern
│   ├── main.go
│   └── README.md
├── 025_logging_tracing/  # ✅ Migrated - Logging & Tracing with Request ID
│   ├── main.go
│   └── README.md
├── 026_observability/    # ✅ Migrated - Comprehensive Observability
│   ├── main.go
│   └── README.md
├── 027_research_assistant/ # ✅ Migrated - Advanced Research Assistant with Multi-Tool ReAct
│   ├── main.go
│   └── README.md
├── 028_code_reviewer/    # ✅ Migrated - AI-Powered Multi-Stage Code Review
│   ├── main.go
│   └── README.md
└── MIGRATION_PLAN.md     # This file
```

### Testing

All migrated examples compile successfully:
```bash
✅ examples/001_predict
✅ examples/002_chain_of_thought
✅ examples/003_react
✅ examples/004_refine
✅ examples/005_best_of_n
✅ examples/006_program_of_thought
✅ examples/007_program_composition
✅ examples/008_chat_predict
✅ examples/009_chat_cot
✅ examples/010_typed_signatures
✅ examples/011_history_prediction
✅ examples/012_math_solver
✅ examples/013_sentiment
✅ examples/014_adapter_fallback
✅ examples/015_fewshot
✅ examples/016_history
✅ examples/017_tools
✅ examples/018_adapters
✅ examples/019_retry_resilience
✅ examples/020_streaming
✅ examples/021_best_of_n_parallel
✅ examples/022_caching
✅ examples/023_global_config
✅ examples/024_lm_factory
✅ examples/025_logging_tracing
✅ examples/026_observability
✅ examples/027_research_assistant
✅ examples/028_code_reviewer
```

### Usage Examples

**Run single example:**
```bash
cd examples/001_predict
go run main.go -verbose
```

**JSON output:**
```bash
go run main.go -format=json
```

**Batch execution (future):**
```bash
# When implemented
make test-examples  # Run all examples with harness
```

## Benefits Achieved

1. **Unified Interface**: All examples use consistent harness API
2. **Better Observability**: Automatic stats collection and error dumps
3. **Production-Ready**: Thread-safe, concurrent execution support
4. **Developer Experience**: Clear numbering, consistent structure
5. **Testing**: Easy to batch-run and collect metrics
6. **Documentation**: Each example is self-documenting with README

## Rollout Plan

**Phase 1** (Complete ✅): Infrastructure + 3 pilots (001, 013, 022)
**Phase 2** (Complete ✅): Core modules migrated (002-012)
**Phase 3** (Complete ✅): Advanced features migrated (014-021)
**Phase 4** (Complete ✅): Production examples migrated (023-028)
**Phase 5** (Complete ✅): Testing infrastructure
  - ✅ Moved test matrix to examples/test_matrix/ (first-class tool)
  - ✅ Rewrote with improved logging and colored output
  - ✅ Updated to use EXAMPLES_DEFAULT_MODEL env var
  - ✅ Set default model to gemini-2.5-flash
  - ✅ Removed incompatible combos (let natural failures happen)
  - ✅ Created comprehensive TESTING.md documentation
  - ✅ Created TESTING_QUICK_REFERENCE.md
  - ✅ Updated Makefile targets
**Phase 6** (Next): Deprecate old examples (keep for reference)

## Testing

All numbered examples (001-028) can be tested individually or in batch using the test matrix system.

**Quick test** (single model, ~2-3 minutes):
```bash
make test-matrix-quick
```

**Sample test** (N random models):
```bash
make test-matrix-sample N=3
```

**Full matrix** (all 14 models × 28 examples = 392 tests):
```bash
make test-matrix
```

See [TESTING.md](TESTING.md) for complete documentation on:
- Test matrix architecture and usage
- Model compatibility matrix
- Circuit breaker system
- CI/CD integration
- Cost estimation and best practices
