# Research Assistant - Feature Showcase

## Complete Feature Matrix

This example demonstrates **every major DSGo capability** in a single, realistic application.

### ✅ Signature Features

| Feature | Example in Code | Description |
|---------|----------------|-------------|
| **String Input** | `topic` | Main research topic |
| **String Output** | `summary`, `key_findings`, `recommendations` | Text outputs |
| **Integer Input** | `depth_level` | Numeric parameter (1-3) |
| **Integer Output** | `sources_consulted` | Numeric result |
| **Boolean Input** | `include_statistics` | Flag for conditional behavior |
| **Class/Enum Output** | `confidence_level`, `research_quality` | Constrained values |
| **Optional Output** | `statistics` | Conditionally included field |
| **Field Descriptions** | All fields | Human-readable documentation |

### ✅ Module Features

| Feature | Implementation | Benefit |
|---------|---------------|---------|
| **ReAct Pattern** | `NewReAct(sig, lm, tools)` | Reasoning + Acting |
| **Max Iterations** | `WithMaxIterations(7)` | Control execution depth |
| **Verbose Mode** | `WithVerbose(true)` | Debug agent behavior |
| **Tool Integration** | 4 different tools | Multi-capability agent |

### ✅ Tool Features

| Tool | Parameters | Purpose | Demonstrates |
|------|-----------|---------|-------------|
| **search** | `query: string` | Information retrieval | Basic tool with string param |
| **get_statistics** | `metric: string` | Statistical data | Domain-specific lookup |
| **fact_check** | `claim: string` | Verification | Quality assurance |
| **get_current_date** | None | Temporal context | Parameterless tools |

### ✅ Type System Features

```go
// ALL field types used in one signature:
.AddInput("topic", FieldTypeString, "...")           // String
.AddInput("depth_level", FieldTypeInt, "...")        // Integer
.AddInput("include_statistics", FieldTypeBool, "...") // Boolean
.AddClassOutput("confidence", []string{...}, "...")   // Enum/Class
.AddOptionalOutput("statistics", FieldTypeString, "...") // Optional
```

### ✅ Validation Features

| Validation Type | Example | Error If |
|----------------|---------|----------|
| **Required Input** | `topic`, `depth_level`, etc. | Missing from inputs map |
| **Required Output** | `summary`, `key_findings`, etc. | LM doesn't provide |
| **Class Constraints** | `confidence_level` ∈ {high, medium, low} | Invalid value |
| **Optional Fields** | `statistics` may be absent | LM omits when not needed |
| **Type Checking** | Int fields must be numeric | Wrong type provided |

## ReAct Loop Demonstration

The example shows the complete ReAct cycle:

```
Iteration 1: Think → Search for general info
Iteration 2: Think → Search for specific focus area
Iteration 3: Think → Get statistics (if requested)
Iteration 4: Think → Search another focus area
Iteration 5: Think → Fact check key claims
Iteration 6: Think → Get date for context
Iteration 7: Think → Synthesize final answer
```

## Input/Output Flow

```
Inputs (Multiple Types):
  topic: string
  focus_areas: string
  depth_level: int (1-3)
  include_statistics: bool
         ↓
    [ReAct Agent]
    - Uses search tool
    - Uses stats tool
    - Uses fact checker
    - Uses date tool
         ↓
Outputs (Multiple Types):
  summary: string
  key_findings: string
  confidence_level: class (high/medium/low)
  sources_consulted: int
  statistics: string (optional)
  recommendations: string
  research_quality: class (excellent/good/fair/limited)
```

## Real API Calls

This example makes **real OpenAI API calls**:

1. Initial prompt with signature
2. LM decides to call tools
3. Tools execute and return results
4. LM processes tool outputs
5. Iterates until final answer
6. Returns structured, validated output

## Code Patterns Demonstrated

### 1. Tool Creation Pattern
```go
tool := dsgo.NewTool(name, description, handler).
    AddParameter("param1", "string", "desc", required).
    AddParameter("param2", "string", "desc", required)
```

### 2. Signature Building Pattern
```go
sig := dsgo.NewSignature("description").
    AddInput(...).
    AddInput(...).
    AddOutput(...).
    AddClassOutput(...).
    AddOptionalOutput(...)
```

### 3. ReAct Configuration Pattern
```go
react := dsgo.NewReAct(sig, lm, tools).
    WithMaxIterations(n).
    WithVerbose(debug)
```

### 4. Execution Pattern
```go
outputs, err := react.Forward(ctx, map[string]interface{}{
    "field1": value1,
    "field2": value2,
    ...
})
```

## Why This Example Matters

1. **Production-Ready Pattern**: Shows realistic research assistant implementation
2. **Complete Type Coverage**: Uses all available field types
3. **Tool Composition**: Demonstrates multi-tool coordination
4. **Conditional Logic**: Optional outputs based on input flags
5. **Validation**: Full type checking and constraint validation
6. **Debugging**: Verbose mode shows agent reasoning
7. **Error Handling**: Graceful tool failures
8. **Structured Output**: Complex, validated JSON responses

## Extending This Example

Use this as a template for:

- **Document Analysis**: Add PDF/text extraction tools
- **Data Pipeline**: Add database/API query tools
- **Content Generation**: Add formatting/template tools
- **Code Analysis**: Add AST parsing/linting tools
- **Multi-Agent**: Spawn sub-agents as tools
- **Real APIs**: Replace simulated tools with real integrations

## Learning Path

1. **First**: Run `examples/sentiment/` - Basic Predict
2. **Second**: Run `examples/react_agent/` - Simple tools
3. **Third**: Run this example - Full complexity
4. **Then**: Build your own using these patterns!
