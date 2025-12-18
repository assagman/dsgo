# YAML Pipeline Builder

Demonstrates how to define DSGo pipelines declaratively using YAML configuration files.

## Overview

This example shows how to:
- Define signatures (inputs/outputs) in YAML
- Configure modules (Predict, ChainOfThought, ReAct) declaratively
- Define MCP clients for external tool access (Exa, Jina, Tavily, Filesystem)
- Configure custom function/filesystem tools separately from MCP
- Specify MCP tools per-module with tool filtering
- Compose multi-stage pipelines without writing Go code for the pipeline structure
- Execute pipelines with automatic data flow between stages

## Files

| File | Purpose |
|------|---------|
| `pipeline.yaml` | Basic YAML pipeline definition (Predict, ChainOfThought) |
| `pipeline_react.yaml` | ReAct pipeline with filesystem MCP tools |
| `pipeline_functions.yaml` | ReAct with native function tools + filesystem MCP |
| `pipeline_mcp.yaml` | ReAct pipeline with MCP web search tools (requires API key) |
| `pipeline_web_summary.yaml` | Search/Extract/Scrape and summarize web content elegantly (requires TAVILY_API_KEY) |
| `pipeline_deep_researcher.yaml` | Multi-stage deep researcher (requires API key) |
| `pipeline_deep_review.yaml` | 2-iteration implement/test/review loop (requires TAVILY_API_KEY) |
| `pipeline_todo.yaml` | Task -> web research -> codebase analysis -> numbered TODO list (requires TAVILY_API_KEY) |
| `builder.go` | YAML parsing and validation |
| `converter.go` | YAML to DSGo Signature conversion |
| `factory.go` | Module creation from specs |
| `tools.go` | MCP client, function, and tool registries |
| `program.go` | Program composition |
| `main.go` | Entry point and execution |

## YAML Schema

### Basic Pipeline

```yaml
name: pipeline_name
description: Pipeline description

settings:
  temperature: 0.7
  max_tokens: 1024

signatures:
  signature_name:
    description: What this signature does
    inputs:
      - name: field_name
        type: string|int|float|bool|json|class|image|datetime
        description: Field description
        optional: false
    outputs:
      - name: field_name
        type: class
        description: Field description
        classes: [option1, option2]

modules:
  module_name:
    type: Predict|ChainOfThought
    signature: signature_name
    options:
      temperature: 0.3
      max_tokens: 512

pipeline:
  - module: module_name_1
  - module: module_name_2
```

### Custom Tools (Function and Filesystem)

Custom tools are defined in the top-level `tools:` section and referenced by modules:

```yaml
# Custom tools (function/filesystem types only)
tools:
  get_datetime:
    type: function
    name: current_datetime
  
  calculator:
    type: function
    name: calculate

  list_files:
    type: filesystem
    name: list_files

modules:
  assistant:
    type: ReAct
    signature: assistant
    options:
      max_iterations: 6
      tools:  # Reference custom tools here
        - get_datetime
        - calculator
        - list_files
```

### MCP Tools (Per-Module Configuration)

MCP clients are defined globally, but tool selection is configured per-module:

```yaml
# Define MCP clients (global registry)
mcp:
  tavily:
    type: tavily
    # api_key: optional, defaults to env var (TAVILY_API_KEY)
  filesystem:
    type: filesystem

modules:
  researcher:
    type: ReAct
    signature: research
    options:
      temperature: 0.5
      max_iterations: 10
    mcp:
      tavily:
        tools:
          - "*"  # Use all tools from tavily
      filesystem:
        tools:
          - read_file
          - list_directory

  fact_checker:
    type: ReAct
    signature: fact_check
    options:
      max_iterations: 6
    mcp:
      tavily:
        tools:
          - tavily_search  # Use only specific tool
```

### Combining Custom Tools and MCP Tools

Modules can use both custom tools and MCP tools:

```yaml
mcp:
  filesystem:
    type: filesystem

tools:
  get_datetime:
    type: function
    name: current_datetime
  calculator:
    type: function
    name: calculate

modules:
  assistant:
    type: ReAct
    signature: assistant
    options:
      max_iterations: 8
      tools:  # Custom tools
        - get_datetime
        - calculator
    mcp:  # MCP tools
      filesystem:
        tools:
          - "*"
```

## Module Types

| Type | Description | Options |
|------|-------------|---------|
| `Predict` | Basic prediction | `temperature`, `max_tokens` |
| `ChainOfThought` | Step-by-step reasoning | `temperature`, `max_tokens` |
| `ReAct` | Reasoning + Acting with tools | `temperature`, `max_tokens`, `max_iterations`, `tools` (custom), `mcp` (per-module) |

## Tool Types (Custom)

| Type | Description | Fields |
|------|-------------|--------|
| `filesystem` | Built-in file tools | `name`: `list_files`, `read_file`, `search_files` |
| `function` | Native Go function tools | `name`: function name (see below) |

## Function Tools

Native Go function tools that work without any API keys:

| Name | Description |
|------|-------------|
| `current_datetime` | Get current date/time with timezone support |
| `calculate` | Basic arithmetic (add, subtract, multiply, divide) |
| `random_number` | Generate random numbers in a range |
| `string_length` | Get string length and character counts |
| `word_count` | Count words and analyze text statistics |
| `environment_info` | Get environment information |

## MCP Client Types

| Type | Env Variable | Description |
|------|--------------|-------------|
| `exa` | `EXA_API_KEY` | Exa search and web content |
| `jina` | `JINA_API_KEY` | Jina URL reading and extraction |
| `tavily` | `TAVILY_API_KEY` | Tavily web search and extraction |
| `filesystem` | - | Local filesystem operations |
| `shell` | - | Shell command execution (apply_patch, shell_run) |
| `custom` | - | Custom MCP server (requires `url` field) |

## Run

```bash
cd examples/yaml_program

# Basic pipeline (Predict, ChainOfThought)
go run . 

# ReAct with filesystem tools
go run . pipeline_react.yaml

# ReAct with native function tools (no API keys needed!)
go run . pipeline_functions.yaml

# ReAct with MCP web search (requires TAVILY_API_KEY)
TAVILY_API_KEY=your-key go run . pipeline_mcp.yaml

# Search and summarize web content (requires TAVILY_API_KEY)
TAVILY_API_KEY=your-key go run . pipeline_web_summary.yaml

# Deep researcher pipeline (requires TAVILY_API_KEY)
TAVILY_API_KEY=your-key go run . pipeline_deep_researcher.yaml

# Deep review pipeline (requires TAVILY_API_KEY)
TAVILY_API_KEY=your-key go run . pipeline_deep_review.yaml

# Or with custom YAML file:
go run . path/to/custom.yaml
```

## Pipeline Flow

### Basic Pipeline
```
┌─────────────────────┐
│   Input: text       │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ sentiment_analyzer  │  Predict
│ → sentiment         │
│ → confidence        │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ key_points_extractor│  ChainOfThought
│ → key_points        │
│ → word_count        │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ summary_generator   │  ChainOfThought
│ → summary           │
│ → recommendations   │
└─────────────────────┘
```

### ReAct Pipeline with MCP
```
┌─────────────────────┐
│   Input: question   │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   code_explorer     │  ReAct
│   ┌─────────────┐   │
│   │ MCP:        │   │
│   │  filesystem │   │  ← MCP tools (per-module)
│   │    - "*"    │   │    (all filesystem tools)
│   └─────────────┘   │
│ → answer            │
│ → files_examined    │
└─────────────────────┘
```

Each module receives all outputs from previous modules plus the original inputs.
