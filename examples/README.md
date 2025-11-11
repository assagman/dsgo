# DSGo Examples

**6 Comprehensive Examples Covering All Framework Features**

Each example demonstrates multiple features through natural multi-turn conversations, with clear event logging showing atomic operations.

## Quick Start

```bash
# Set up environment
cp .env.local.example .env.local  # Add your OPENROUTER_API_KEY and EXAMPLES_DEFAULT_MODEL

# Run any example (EXAMPLES_DEFAULT_MODEL must be set)
cd 01-hello-chat
EXAMPLES_DEFAULT_MODEL="anthropic/claude-3.5-sonnet" go run main.go

# Use a different model
EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go

# Enable verbose logging
DSGO_LOG=pretty EXAMPLES_DEFAULT_MODEL="anthropic/claude-3.5-sonnet" go run main.go

# Save events to file
DSGO_LOG=events EXAMPLES_DEFAULT_MODEL="anthropic/claude-3.5-sonnet" go run main.go > events.jsonl

# Use custom .env file location
DSGO_ENV_FILE_PATH="/path/to/custom.env" go run main.go
```

**Note**: DSGo automatically loads `.env` and `.env.local` files from the current directory (or parent directories) when you import the package. You can also set `DSGO_ENV_FILE_PATH` to specify a custom environment file location.

## Examples Overview

### 01 - Hello Chat
**Personal Assistant with Streaming, History, and Caching**

Multi-turn conversation demonstrating core features:
- Streaming responses (token-by-token output)
- History management (context retention)
- LM caching (cache miss → hit)

**Learn**: Basic Predict module, conversation flow, performance optimization

```bash
cd 01-hello-chat && go run main.go
```

---

### 02 - Agent Tools ReAct
**Travel Helper with Multi-Tool Reasoning**

Agentic reasoning with external tools:
- ReAct module (thought → action → observation)
- Multiple typed tools (search, currency, timezone)
- Multi-step planning and execution

**Learn**: Tool definition, agent loops, verbose mode, ReAct patterns

```bash
cd 02-agent-tools-react && go run main.go
```

---

### 03 - Quality Refine BestOf
**Email Drafting with Reasoning and Selection**

Quality optimization pipeline:
- ChainOfThought (structured reasoning with rationale)
- Few-shot learning (style examples)
- BestOfN (generate N candidates, select best)
- Refine (iterative improvement)

**Learn**: Module composition, custom scorers, few-shot patterns, refinement

```bash
cd 03-quality-refine-bestof && go run main.go
```

---

### 04 - Structured Programs
**Itinerary Planner with Multi-Step Pipeline**

Complex program composition with typed data:
- ProgramOfThought (planning logic generation)
- JSON adapter (strongly typed I/O)
- Program module (multi-step composition with data flow)
- Typed signatures (Int, Float, String, JSON)

**Learn**: Structured outputs, program pipelines, field types, composition

```bash
cd 04-structured-programs && go run main.go
```

---

### 05 - Resilience and Observability
**Q&A System with Fallback and Metrics**

Production-grade resilience and monitoring:
- Streaming with chunk tracking
- LM caching with metrics (hit rate, speedup)
- Cache TTL (time-to-live expiry demonstration)
- Fallback adapters (Chat → JSON)
- Provider fallback (primary → secondary)
- Comprehensive event logging

**Learn**: Error handling, caching strategies, cache TTL, metrics, observability

```bash
cd 05-resilience-observability && go run main.go
```

---

### 06 - Parallel
**Concurrent Module Execution**

Parallel processing with multiple modules:
- Parallel module for concurrent execution
- Synchronized results collection
- Error handling across goroutines
- Performance optimization for independent tasks

**Learn**: Concurrency patterns, parallel execution, goroutine management

```bash
cd 06-parallel && go run main.go
```

---

### 07 - Cache and TTL
**Cache Configuration and Performance Testing**

Focused cache demonstration:
- Cache configuration (capacity, TTL)
- Cache hit/miss behavior
- TTL expiry demonstration
- Cache statistics and metrics
- Performance comparison

**Learn**: Cache optimization, TTL configuration, hit rate analysis, performance tuning

```bash
cd 07-cache-ttl && go run main.go
```

## Feature Coverage Matrix

| Feature | Ex 01 | Ex 02 | Ex 03 | Ex 04 | Ex 05 | Ex 06 | Ex 07 |
|---------|-------|-------|-------|-------|-------|-------|-------|
| **Modules** |
| Predict | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| ChainOfThought | | | ✓ | | | | |
| ReAct | | ✓ | | | | | |
| Refine | | | ✓ | | | | |
| BestOfN | | | ✓ | | | | |
| ProgramOfThought | | | | ✓ | | | |
| Program | | | | ✓ | | | |
| Parallel | | | | | | ✓ | |
| **Adapters** |
| Chat | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| JSON | | ✓ | | ✓ | ✓ | | |
| Fallback | | | | | ✓ | | |
| **Features** |
| Streaming | ✓ | | | | ✓ | | |
| History | ✓ | | | | | | |
| Caching | ✓ | | | | ✓ | | ✓ |
| Cache TTL | | | | | ✓ | | ✓ |
| Tools | | ✓ | | | | | |
| Few-shot | | | ✓ | | | | |
| Typed signatures | | ✓ | | ✓ | | | |
| Observability | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Multi-turn | ✓ | ✓ | ✓ | ✓ | ✓ | | |
| Concurrency | | | | | | ✓ | |

## Logging and Observability

All examples support structured event logging:

### Environment Variables

**Required:**
```bash
EXAMPLES_DEFAULT_MODEL=anthropic/claude-3.5-sonnet  # Required: Model to use (any OpenRouter model)
```

**Optional:**
```bash
# Generation options
EXAMPLES_MAX_TOKENS=10000      # Max tokens (default: 10000)
EXAMPLES_TEMPERATURE=0.7       # Temperature (default: 0.7)

# Caching configuration
DSGO_CACHE_TTL=5m              # Cache TTL (e.g., "5m", "1h", "30s", "0" for infinite)

# Logging mode
DSGO_LOG=off       # No logging
DSGO_LOG=events    # JSON events to stdout
DSGO_LOG=pretty    # Human-readable tree (default)
DSGO_LOG=both      # Both formats

# Output file (events mode)
DSGO_LOG_FILE=events.jsonl

# Environment file location (automatic loading)
DSGO_ENV_FILE_PATH=/path/to/custom.env  # Custom .env file path (default: auto-discover .env/.env.local)
```

**Automatic .env Loading:**
DSGo automatically loads environment variables from `.env` and `.env.local` files when you import the package. The framework searches for these files in:
1. Current working directory
2. Parent directories (walking up the tree)

Files are loaded with this precedence (later files override earlier ones):
1. `.env` (base configuration)
2. `.env.local` (local overrides)

You can also set `DSGO_ENV_FILE_PATH` to specify a custom environment file location. If the specified file doesn't exist, DSGo will return an error.

If no `.env` files are found, DSGo continues normally and it's your responsibility to set required environment variables.

### Event Types

Examples emit structured events showing atomic operations:

- **run.start / run.end** - Overall conversation
- **module.start / module.end** - Module execution (Predict, CoT, ReAct, etc.)
- **tool.call.start / tool.call.end** - Tool invocations with args
- **cache.check / cache.hit / cache.miss** - Cache operations
- **stream.chunk** - Streaming tokens
- **adapter.selected** - Which adapter succeeded
- **program.step.start / step.end** - Program pipeline steps
- **react.thought / action / observation** - ReAct iterations
- **bestofn.candidate / select** - BestOfN scoring
- **refine.iteration** - Refinement steps

### Pretty Output Example
```
▶ run.start scenario=chat_assistant
  ▶ turn1.start streaming=true
    • cache.miss
    • stream.chunk seq=1
    • stream.chunk seq=2
  ✓ turn1.end 1234ms tokens=156
  ▶ turn2.start history_entries=2
    • cache.hit
  ✓ turn2.end 12ms
✓ run.end 1246ms
```

### JSON Events Example
```json
{"ts":"2024-11-07T10:00:00Z","level":"INFO","span_id":"abc","run_id":"xyz","kind":"module","operation":"turn1.start","fields":{"streaming":true}}
{"ts":"2024-11-07T10:00:01Z","level":"INFO","span_id":"abc","run_id":"xyz","kind":"cache","operation":"cache.miss"}
{"ts":"2024-11-07T10:00:02Z","level":"INFO","span_id":"abc","run_id":"xyz","kind":"module","operation":"turn1.end","latency_ms":1234,"tokens":{"total":156}}
```

## Learning Path

**New to DSGo?** Follow this sequence:

1. **01-hello-chat** - Learn basics: Predict, streaming, history
2. **02-agent-tools-react** - Add tools and agentic reasoning
3. **03-quality-refine-bestof** - Explore quality optimization
4. **04-structured-programs** - Master complex pipelines
5. **05-resilience-observability** - Production patterns
6. **06-parallel** - Concurrent execution patterns
7. **07-cache-ttl** - Cache optimization and performance tuning

**Building a specific use case?**

- **Chatbot/Assistant** → Start with 01, add tools from 02
- **Content Generation** → See 03 for quality patterns
- **Data Pipeline** → Check 04 for structured flows
- **Production System** → Review 05 for resilience
- **High Performance** → Check 06 for parallelization, 07 for caching

## Migration from Old Examples

The previous 28 numbered examples have been consolidated:

| Old Examples | New Location | What Moved |
|--------------|--------------|------------|
| 001, 008, 011, 016, 020, 022 | **01-hello-chat** | Predict, History, Streaming, Caching |
| 003, 017, 027 | **02-agent-tools-react** | ReAct, Tools, Agents |
| 002, 004, 005, 009, 013, 015, 021 | **03-quality-refine-bestof** | CoT, Refine, BestOfN, Few-shot |
| 006, 007, 010, 012 | **04-structured-programs** | PoT, Program, Typed sigs |
| 014, 018, 019, 023, 024, 025, 026 | **05-resilience-observability** | Adapters, Config, Logging |

All features from the original 28 examples are preserved in the new 6 consolidated examples.

## Directory Structure

```
examples/
├── 01-hello-chat/           # Predict + Streaming + History + Caching
├── 02-agent-tools-react/    # ReAct + Tools + Multi-step reasoning
├── 03-quality-refine-bestof/ # CoT + BestOfN + Refine + Few-shot
├── 04-structured-programs/  # Program + PoT + JSON + Typed sigs
├── 05-resilience-observability/ # Fallback + Metrics + Logging
├── 06-parallel/             # Concurrent module execution
├── 07-cache-ttl/            # Cache configuration and TTL testing
├── _shared/                 # Shared utilities
│   └── observe/             # Event logging infrastructure
├── .env.local               # API keys (gitignored)
└── README.md                # This file
```

## Running Tests

Each example can be tested:

```bash
# Run specific example
cd 01-hello-chat
go run main.go

# Run with test matrix (multiple models)
make test-matrix-quick   # Fast (1 model)
make test-matrix-sample N=3  # Sample (3 random models)
make test-matrix         # Full (all models)
```

## Next Steps

- **Read the code**: Each example is ~100-200 lines, well-commented
- **Check READMEs**: Each directory has detailed docs
- **Enable logging**: Set `DSGO_LOG=pretty` to see what's happening
- **Modify and experiment**: Change prompts, add tools, adjust pipelines
- **Build your own**: Use examples as templates for your use case

## Questions?

- **Main docs**: [README.md](../README.md) - Framework overview
- **Quick start**: [QUICKSTART.md](../QUICKSTART.md) - Fast onboarding
- **Development**: [AGENTS.md](../AGENTS.md) - Testing and contributing
- **Progress**: [ROADMAP.md](../ROADMAP.md) - Implementation status
