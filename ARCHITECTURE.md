# ARCHITECTURE

DSGo is a 3-layer framework for building structured LLM applications.

## Layers

```mermaid
graph TB
  subgraph "Layer 3: Modules"
    M1[Predict]
    M2[ChainOfThought]
    M3[ReAct]
    M4[Refine]
    M5[BestOfN]
    M6[Program]
    M7[Parallel]
    M8[ProgramOfThought]
    M9[MultiChainComparison]
  end

  subgraph "Layer 2: Core"
    C1[Signature]
    C2[Adapter]
    C3[Prediction]
    C4[History]
    C5[Tools]
    C6[Cache]
  end

  subgraph "Layer 1: Providers"
    P1[OpenAI]
    P2[OpenRouter]
  end

  M1 --> C1
  M2 --> C1
  M3 --> C5

  C2 --> P1
  C2 --> P2

  C1 --> C2
  C2 --> C3
```

- Public API lives in `dsgo` (see `dsgo.go`).
- Implementations live under `internal/*`.

Links:
- Core: [`internal/core/README.md`](internal/core/README.md)
- Modules: [`internal/module/README.md`](internal/module/README.md)
- MCP: [`internal/mcp/README.md`](internal/mcp/README.md)

## Modules

DSGo modules are higher-level behaviors built on top of the core `Signature` + `Adapter` + `LM` primitives.

- **Predict**: Basic structured input → output prediction.
- **ChainOfThought**: Adds a reasoning step and stores it in `Prediction.Rationale` (plus any explicit `reasoning` output fields).
- **ReAct**: A tool-calling loop agent (think → act → observe), suitable for external integrations.
- **Refine**: Iteratively improves a prediction when `inputs["feedback"]` (or a custom refinement field) is provided.
- **BestOfN**: Runs a wrapped module `N` times and selects the best result using a scorer; can parallelize and optionally return all completions.
- **Program**: Sequential pipeline; merges previous outputs into the next step's inputs.
- **Parallel**: Runs the wrapped module concurrently across expanded inputs; stores per-item outputs in `Prediction.Completions`.
- **ProgramOfThought**: Generates code-first solutions; execution is disabled by default and, when enabled, runs local `python3`/`node` under a timeout (no tool loop).
- **MultiChainComparison**: Synthesizes a final answer from `inputs["completions"]` (multiple attempts) and prepends a `rationale` output.

## Request lifecycle

```mermaid
flowchart LR
  A[Inputs] --> B[Adapter formats prompt]
  B --> C[Provider generates]
  C --> D[Adapter parses]
  D --> E[Validation]
  E --> F[Prediction]
```

## Adapter fallback

```mermaid
graph TD
  A[Raw response] --> B{ChatAdapter}
  B -->|ok| R[Result]
  B -->|fail| C{JSONAdapter}
  C -->|ok| R
  C -->|fail| D{TwoStepAdapter}
  D -->|ok| R
  D -->|fail| X[Error]
```

## Tool calling (ReAct loop)

```mermaid
flowchart TD
  Q[User query] --> T[Thought]
  T --> A[Act: tool call]
  A --> O[Observation]
  O --> T
  T --> F[Final answer]
```

## Streaming pipeline

```mermaid
flowchart LR
  P[Provider stream] --> B[StreamingBuffer]
  B --> M[Marker filter]
  M --> C[Chunks]
  M --> F[Final parsed Prediction]
```
