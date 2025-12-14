# REFERENCE

Tables and quick lookups for DSGo.

## Environment variables

### Provider keys

| Variable | Purpose |
|---|---|
| `OPENAI_API_KEY` | OpenAI API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |

### Runtime

| Variable | Purpose |
|---|---|
| `DSGO_TIMEOUT` | Request timeout (e.g. `30s`) |
| `DSGO_MAX_RETRIES` | Max retry attempts |
| `DSGO_CACHE_SIZE` | LRU cache size |
| `DSGO_CACHE_TTL` | Cache TTL |

### Structured Outputs

| Variable | Purpose | Default |
|---|---|---|
| `DSGO_STRUCTURED_OUTPUTS` | Enable structured output enforcement (`true`/`false`) | `true` |
| `DSGO_STRUCTURED_MAX_ATTEMPTS` | Max validation retry attempts | `3` |
| `DSGO_STRUCTURED_TEMPERATURE` | Temperature override for structured mode | `0.0` |

### Observability / debugging

| Variable | Purpose |
|---|---|
| `DSGO_LOG` | Logging output mode (see logging docs) |
| `DSGO_DEBUG_PARSE` | Show parse attempts (`1`/`true`) |
| `DSGO_SAVE_RAW_RESPONSES` | Save raw LM outputs (if supported by provider) |
| `DSGO_DEBUG_MARKERS` | Show field markers during streaming |
| `DSGO_COLLECTOR` | Collector type (`memory`, etc.) |
| `DSGO_REQUEST_ID_HEADER` | Incoming request id header name |
| `DSGO_ARTIFACT_DIR` | Where to write debug artifacts (e.g. raw exchanges) |
| `DSGO_EXAMPLE` | Optional label included in saved artifacts |

> Source-of-truth: see package docs and `internal/*`.

## Modules (high level)

| Module | Constructor | Notes |
|---|---|---|
| Predict | `dsgo.NewPredict(sig, lm)` | Structured prediction |
| ChainOfThought | `dsgo.NewChainOfThought(sig, lm)` | Stores reasoning in `Prediction.Rationale` (and/or explicit reasoning fields) |
| ReAct | `dsgo.NewReAct(sig, lm, tools)` | Tool-using agent loop |
| Refine | `dsgo.NewRefine(sig, lm)` | Refines only when `inputs["feedback"]` (or custom refinement field) is provided |
| BestOfN | `dsgo.NewBestOfN(module, n)` | Requires `WithScorer(...)`; can parallelize and optionally return all completions |
| Program | `dsgo.NewProgram(name)` | Sequential composition |
| Parallel | `dsgo.NewParallel(module)` | Concurrent execution; per-item outputs are in `Prediction.Completions` |
| ProgramOfThought | `dsgo.NewProgramOfThought(sig, lm, language)` | Generates code-first solutions; execution is disabled by default |
| MultiChainComparison | `dsgo.NewMultiChainComparison(sig, lm, m)` | Synthesizes from `inputs["completions"]` and prepends `rationale` output |

## Adapters

| Adapter | Description |
|---|---|
| ChatAdapter | Marker-based format `[[ ## field ## ]]` |
| JSONAdapter | JSON output with schema + repair |
| TwoStepAdapter | Two-step reasoning then extraction |
| FallbackAdapter | Chains adapters for robustness |

## GenerateOptions (Advanced)

### ProviderParams

Provider-specific parameters can be passed through to LM providers using `ProviderParams`:

```go
options := core.DefaultGenerateOptions()
options.ProviderParams = map[string]any{
    "reasoning": map[string]any{
        "effort": "high", // OpenRouter reasoning effort
    },
    "custom_param": "value", // Provider-specific field
}
```

**Supported values:**
- **OpenRouter**: `"reasoning": {"effort": "high|medium|low|minimal|none"}`
- **OpenAI**: Provider-specific fields (merged with request)

**Safety rules:**
- DSGo-managed keys (`temperature`, `max_tokens`, etc.) take precedence
- ProviderParams are merged only for non-conflicting keys
- Nested objects are supported

## Links

- Getting started: [`QUICKSTART.md`](QUICKSTART.md)
- Architecture: [`ARCHITECTURE.md`](ARCHITECTURE.md)
- Development: [`DEVELOPMENT.md`](DEVELOPMENT.md)
- Examples index: [`examples/README.md`](examples/README.md)
- Core internals: [`internal/core/README.md`](internal/core/README.md)
- Module internals: [`internal/module/README.md`](internal/module/README.md)
