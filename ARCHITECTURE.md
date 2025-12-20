# ARCHITECTURE

DSGo is a 3-layer framework for building structured LLM applications.

## Layers

```mermaid
flowchart TB
  classDef module fill:#ffd166,stroke:#c25f00,stroke-width:1px,color:#2b2b2b;
  classDef core fill:#9bf6ff,stroke:#0077b6,stroke-width:1px,color:#1f2d3d;
  classDef provider fill:#caffbf,stroke:#2d6a4f,stroke-width:1px,color:#1b4332;
  classDef typed fill:#ffc6ff,stroke:#9d4edd,stroke-width:1px,color:#2b193f;

  subgraph L3["Layer 3: Modules"]
    M1["Predict"]
    M2["ChainOfThought"]
    M3["ReAct"]
    M4["Refine"]
    M5["BestOfN"]
    M6["Program"]
    M7["Parallel"]
    M8["ProgramOfThought"]
    M9["MultiChainComparison"]
  end

  subgraph L2["Layer 2: Core"]
    C1["Signature"]
    C2["LM Interface"]
    C3["Adapter"]
    C4["Prediction"]
    C5["History"]
    C6["Tooling"]
    C7["Cache"]
    C8["Settings/Config"]
    C9["Collectors"]
    C10["Streaming Buffer & Marker Filter"]
    C11["Structured Output"]
  end

  subgraph L1["Layer 1: Providers"]
    P1["OpenAI"]
    P2["OpenRouter"]
    P3["Mock"]
  end

  subgraph L0["Typed API (Facade)"]
    T1["signature_typed.Func / Predict"]
  end

  M1 --> C1
  M1 --> C3
  M1 --> C2
  M3 --> C6
  M6 --> C4

  T1 --> M1
  T1 --> C1

  C3 --> C11
  C10 --> C3
  C2 --> P1
  C2 --> P2
  C2 --> P3

  class M1,M2,M3,M4,M5,M6,M7,M8,M9 module;
  class C1,C2,C3,C4,C5,C6,C7,C8,C9,C10,C11 core;
  class P1,P2,P3 provider;
  class T1 typed;
```

- Public API is package-per-concern (e.g., `core`, `module`, `provider/*`).
- Internal helpers live under `internal/*` (jsonutil, retry, ids, env).

Links:
- Core: [`core/README.md`](core/README.md)
- Modules: [`module/README.md`](module/README.md)
- MCP: [`mcp/README.md`](mcp/README.md)

## Modules

DSGo modules are higher-level behaviors built on top of the core `Signature` + `Adapter` + `LM` primitives.

- **Predict**: Basic structured input → output prediction.
- **ChainOfThought**: Adds a reasoning step and stores it in `Prediction.Rationale` (plus any explicit `reasoning` output fields).
- **ReAct**: A native tool-calling loop agent with an auto-injected `finish` tool and a post-loop extractor fallback to produce signature-valid outputs.
- **Refine**: Iteratively improves a prediction when `inputs["feedback"]` (or a custom refinement field) is provided.
- **BestOfN**: Runs a wrapped module `N` times and selects the best result using a scorer; can parallelize and optionally return all completions.
- **Program**: Sequential pipeline; merges previous outputs into the next step's inputs.
- **Parallel**: Runs the wrapped module concurrently across expanded inputs; stores per-item outputs in `Prediction.Completions`.
- **ProgramOfThought**: Generates code-first solutions; execution is disabled by default and, when enabled, runs local `python3`/`node` under a timeout (Go execution is not supported yet; no tool loop).
- **MultiChainComparison**: Synthesizes a final answer from `inputs["completions"]` (multiple attempts) and prepends a `rationale` output.

## Request lifecycle

```mermaid
flowchart LR
  classDef step fill:#ffe5a5,stroke:#f59f00,stroke-width:1px,color:#2b2b2b;
  classDef core fill:#a7c7ff,stroke:#3b5bdb,stroke-width:1px,color:#1f2d3d;
  classDef output fill:#b2f2bb,stroke:#2f9e44,stroke-width:1px,color:#1b4332;

  A["Inputs"] --> B["Signature & Adapter\nprompt assembly"]
  B --> C["LM (provider) generation"]
  C --> D["Adapter parsing"]
  D --> E["Validation"]
  E --> F["Prediction"]

  class A step;
  class B,D,E core;
  class C core;
  class F output;
```

## Adapter fallback

```mermaid
flowchart TD
  classDef raw fill:#ffd6a5,stroke:#e07a5f,stroke-width:1px,color:#2b2b2b;
  classDef adapter fill:#cde4ff,stroke:#4361ee,stroke-width:1px,color:#1f2d3d;
  classDef ok fill:#b7efc5,stroke:#2d6a4f,stroke-width:1px,color:#1b4332;
  classDef err fill:#ffadad,stroke:#c92a2a,stroke-width:1px,color:#2b2b2b;

  A["Raw response"] --> B{"JSONAdapter"}
  B -->|ok| R["Result"]
  B -->|fail| C{"ChatAdapter"}
  C -->|ok| R
  C -->|fail| X["Error"]

  class A raw;
  class B,C adapter;
  class R ok;
  class X err;
```

Notes:
- The default `FallbackAdapter` chain is JSON → Chat.
- `TwoStepAdapter` is opt-in for specialized workflows.

## Tool calling (ReAct loop)

```mermaid
flowchart TD
  classDef input fill:#ffe066,stroke:#f08c00,stroke-width:1px,color:#2b2b2b;
  classDef action fill:#a5d8ff,stroke:#1c7ed6,stroke-width:1px,color:#1f2d3d;
  classDef tool fill:#d0bfff,stroke:#7048e8,stroke-width:1px,color:#2b193f;
  classDef output fill:#b2f2bb,stroke:#2f9e44,stroke-width:1px,color:#1b4332;
  classDef warn fill:#ffadad,stroke:#c92a2a,stroke-width:1px,color:#2b2b2b;

  Q["Inputs"] --> M["LM step"]
  M -->|"tool call(s)"| A["Act: tool execution"]
  A --> O["Observation(s)"]
  O --> M
  M -->|"finish(args) or direct answer"| F["Final outputs (validated)"]
  F -->|"if invalid or limits hit"| X["Extractor (schema enforced)"]

  class Q input;
  class M action;
  class A,O tool;
  class F output;
  class X warn;
```

Notes:
- DSGo auto-injects a synthetic `finish` tool that mirrors the output `Signature`.
- If the loop can’t produce a valid final output (or exceeds limits), DSGo runs an extraction step over the full trajectory to salvage a schema-valid answer.

## Streaming pipeline

```mermaid
flowchart LR
  classDef source fill:#ffd6a5,stroke:#e07a5f,stroke-width:1px,color:#2b2b2b;
  classDef pipe fill:#bde0fe,stroke:#4ea8de,stroke-width:1px,color:#1f2d3d;
  classDef out fill:#b7efc5,stroke:#2d6a4f,stroke-width:1px,color:#1b4332;

  P["Provider stream"] --> B["StreamingBuffer"]
  B --> M["Marker filter"]
  M --> C["Chunks"]
  M --> F["Final parsed Prediction"]

  class P source;
  class B,M pipe;
  class C,F out;
```
