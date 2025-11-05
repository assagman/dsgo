# 027_research_assistant - Advanced Research Assistant with Multi-Tool ReAct

## Overview

Demonstrates DSGo's **complete feature set** in a realistic research assistant application. This advanced example combines complex custom signatures, multiple tool integration, ReAct reasoning, and comprehensive type validation in a production-ready pattern.

## What it demonstrates

- **Complex custom signatures** - Multiple input/output fields with diverse types (string, int, bool, class/enum)
- **Multi-tool integration** - 4 different tools working together (search, statistics, fact-checking, date)
- **ReAct reasoning pattern** - Iterative problem-solving with tool use and chain-of-thought
- **Type safety and validation** - Complete type checking with required/optional/constrained fields
- **Conditional outputs** - Optional fields based on input flags
- **Production patterns** - Realistic research assistant implementation
- Use cases: research assistants, report generation, data synthesis, fact-checking workflows

## Usage

```bash
cd examples/027_research_assistant
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=1
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
=== Advanced Research Assistant Example ===
Demonstrates: Custom Signatures + Tools + ReAct Reasoning

📋 Research Request:
  Topic: Impact of AI on software development productivity
  Focus Areas: code generation, testing automation, developer experience
  Depth Level: 2
  Include Statistics: true

=== ReAct Iteration 1 ===
Thought: I need to research the impact of AI on software development productivity...
Action: search(map[query:AI impact on software development productivity])
Observation: Search Results: Multiple studies show AI tools like GitHub Copilot...

=== ReAct Iteration 2 ===
Thought: I should gather specific information about code generation...
Action: search(map[query:AI code generation productivity])
Observation: Search Results: AI code generation tools can complete up to 40%...

=== ReAct Iteration 3 ===
Thought: Since statistics were requested, I need to get quantitative data...
Action: get_statistics(map[metric:productivity increase])
Observation: Studies show 20-55% productivity increase among developers using AI...

=== ReAct Iteration 4 ===
Thought: I should research testing automation impacts...
Action: search(map[query:AI testing automation])
Observation: Search Results: AI-powered testing tools can generate test cases...

=== ReAct Iteration 5 ===
Thought: Let me verify the key productivity claims...
Action: fact_check(map[claim:AI tools increase developer productivity by 20-55%])
Observation: VERIFIED: Multiple peer-reviewed studies from 2023-2024 confirm...

=== ReAct Iteration 6 ===
Thought: I should get the current date for temporal context...
Action: get_current_date(map[])
Observation: Current date: November 5, 2025 (Studies from 2024 are most recent)

=== ReAct Iteration 7 ===
Thought: I now have enough information to provide a comprehensive research report...
Answer: Proceeding to final output...

======================================================================
📊 RESEARCH RESULTS
======================================================================

📝 SUMMARY:
AI tools have significantly impacted software development productivity, with studies 
showing 20-55% productivity increases. The impact spans code generation (40% 
automated completion), testing automation (30-50% time reduction), and developer 
experience (70% report improved satisfaction). While adoption is growing (46% of 
developers use AI tools regularly), careful code review remains essential as 15-20% 
of AI-generated code needs significant modifications.

🔍 KEY FINDINGS:
• Productivity gains: 20-55% increase in developer productivity
• Code generation: Up to 40% of code can be auto-completed by AI tools
• Testing efficiency: 30-50% reduction in testing time with AI-powered tools
• Adoption rate: 46% of professional developers use AI coding tools regularly
• Developer satisfaction: 70% report improved job satisfaction
• Time savings: Developers save 8-12 hours per week on average
• Code quality: 92% of AI-generated code requires review; 15-20% needs modifications
• Learning curve: 2-4 weeks average onboarding time to proficiency

📈 STATISTICS:
• 20-55% productivity increase (verified by multiple peer-reviewed studies 2023-2024)
• 46% adoption rate among professional developers as of 2024
• 8-12 hours per week average time savings
• 70% improved job satisfaction rate
• 30-50% reduction in testing time
• 40% code auto-completion capability
• 15-20% of AI-generated code needs significant modifications

💡 RECOMMENDATIONS:
1. Adopt AI coding tools gradually, starting with code completion and documentation
2. Establish code review processes specifically for AI-generated code
3. Invest in 2-4 weeks training for developer onboarding on AI tools
4. Focus AI use on routine tasks, boilerplate code, and API integrations
5. Monitor productivity metrics to measure actual impact in your team
6. Balance AI assistance with maintaining developer skills and critical thinking
7. Consider AI-powered testing tools for improved test coverage and efficiency

📊 METADATA:
  Confidence Level: high
  Research Quality: excellent
  Sources Consulted: 4

======================================================================
```

## Key Concepts

### 1. Complex Custom Signature

Create signatures with multiple input/output types:

```go
import "github.com/assagman/dsgo"

sig := dsgo.NewSignature("Research a topic and provide comprehensive analysis").
    // Multiple inputs with different types
    AddInput("topic", dsgo.FieldTypeString, "The main research topic").
    AddInput("focus_areas", dsgo.FieldTypeString, "Specific aspects to focus on").
    AddInput("depth_level", dsgo.FieldTypeInt, "Research depth: 1=basic, 2=intermediate, 3=deep").
    AddInput("include_statistics", dsgo.FieldTypeBool, "Whether to include statistical data").
    
    // Multiple outputs with different types and constraints
    AddOutput("summary", dsgo.FieldTypeString, "Executive summary of findings").
    AddOutput("key_findings", dsgo.FieldTypeString, "Bullet-pointed key discoveries").
    AddClassOutput("confidence_level", []string{"high", "medium", "low"}, "Confidence in research").
    AddOutput("sources_consulted", dsgo.FieldTypeFloat, "Number of sources checked").
    AddOptionalOutput("statistics", dsgo.FieldTypeString, "Statistical data if requested").
    AddOutput("recommendations", dsgo.FieldTypeString, "Action items or next steps").
    AddClassOutput("research_quality", []string{"excellent", "good", "fair", "limited"}, "Quality assessment")
```

**Field types demonstrated:**
- `FieldTypeString` - Text inputs and outputs
- `FieldTypeInt` - Numeric values (depth level)
- `FieldTypeBool` - Flags (include_statistics)
- `FieldTypeFloat` - Numeric outputs (source count)
- `FieldTypeClass` - Constrained enums (confidence, quality)

**Field categories:**
- **Required inputs** - Must be provided in inputs map
- **Required outputs** - LM must include in response
- **Optional outputs** - May be omitted based on conditions
- **Class outputs** - Must match allowed values

**Benefits:**
- **Type safety** - Compile-time and runtime validation
- **Clear contracts** - Explicit input/output specifications
- **Flexible outputs** - Optional fields for conditional data
- **Validation** - Automatic checking of constraints

**When to use:**
- Complex workflows with multiple data types
- Applications requiring strict output validation
- Conditional output based on input parameters
- Production systems needing type safety

### 2. Multi-Tool Integration

Coordinate multiple tools in a ReAct agent:

```go
import (
    "github.com/assagman/dsgo"
    "github.com/assagman/dsgo/module"
)

// Define 4 different research tools
tools := []dsgo.Tool{
    *createSearchTool(),        // Information retrieval
    *createStatisticsTool(),    // Quantitative data
    *createFactCheckerTool(),   // Claim verification
    *createDateTool(),          // Temporal context
}

// Create ReAct module with all tools
react := module.NewReAct(sig, lm, tools).
    WithMaxIterations(15).
    WithVerbose(true)

// Execute - agent decides which tools to use
result, err := react.Forward(ctx, inputs)
```

**Tool implementations:**

```go
// Search tool with parameter
func createSearchTool() *dsgo.Tool {
    return dsgo.NewTool(
        "search",
        "Search for information on a specific topic or question",
        func(ctx context.Context, args map[string]any) (any, error) {
            query := args["query"].(string)
            // Simulate search logic
            return searchResults, nil
        },
    ).AddParameter("query", "string", "The search query", true)
}

// Statistics tool with parameter
func createStatisticsTool() *dsgo.Tool {
    return dsgo.NewTool(
        "get_statistics",
        "Retrieve statistical data about a specific metric or study",
        func(ctx context.Context, args map[string]any) (any, error) {
            metric := args["metric"].(string)
            // Lookup statistical data
            return statisticsData, nil
        },
    ).AddParameter("metric", "string", "The specific metric to retrieve", true)
}

// Fact checker tool
func createFactCheckerTool() *dsgo.Tool {
    return dsgo.NewTool(
        "fact_check",
        "Verify a claim or statement for accuracy",
        func(ctx context.Context, args map[string]any) (any, error) {
            claim := args["claim"].(string)
            // Verify claim
            return verificationResult, nil
        },
    ).AddParameter("claim", "string", "The claim or statement to verify", true)
}

// Date tool (no parameters)
func createDateTool() *dsgo.Tool {
    return dsgo.NewTool(
        "get_current_date",
        "Get the current date and time for temporal context",
        func(ctx context.Context, args map[string]any) (any, error) {
            now := time.Now()
            return fmt.Sprintf("Current date: %s", now.Format("January 2, 2006")), nil
        },
    )
}
```

**Benefits:**
- **Composition** - Tools work together automatically
- **Flexibility** - Agent chooses which tools to use
- **Extensibility** - Easy to add new capabilities
- **Coordination** - LM orchestrates tool usage

**When to use:**
- Multi-step research and analysis
- Complex information gathering
- Workflows requiring different data sources
- Agents needing diverse capabilities

### 3. ReAct Reasoning Pattern

Iterative reasoning with tool usage:

```go
import "github.com/assagman/dsgo/module"

react := module.NewReAct(sig, lm, tools).
    WithMaxIterations(15).  // Max reasoning steps
    WithVerbose(true)       // Show thought process

result, err := react.Forward(ctx, inputs)
```

**ReAct cycle:**

```
Iteration 1: Think → Decide → Act (search for general info)
Iteration 2: Think → Decide → Act (search specific focus area)
Iteration 3: Think → Decide → Act (get statistics if requested)
Iteration 4: Think → Decide → Act (search another focus area)
Iteration 5: Think → Decide → Act (fact check key claims)
Iteration 6: Think → Decide → Act (get date for context)
Iteration 7: Think → Decide → Answer (synthesize final output)
```

**Verbose output shows:**
- **Thought** - Agent's reasoning
- **Action** - Tool call with parameters
- **Observation** - Tool execution result
- **Answer** - Final structured output

**Benefits:**
- **Transparency** - See agent's decision-making
- **Debugging** - Understand why agent chose specific tools
- **Control** - Limit iterations to prevent runaway
- **Quality** - Iterative refinement improves results

**When to use:**
- Complex multi-step problems
- Research and investigation tasks
- Workflows requiring multiple data sources
- Applications needing explainable decisions

### 4. Type Safety and Validation

Comprehensive validation of inputs and outputs:

```go
// Execution
result, err := react.Forward(ctx, map[string]any{
    "topic":              "Impact of AI on software development productivity",
    "focus_areas":        "code generation, testing automation, developer experience",
    "depth_level":        2,                    // Must be int
    "include_statistics": true,                 // Must be bool
})

// Access validated outputs
summary := result.Outputs["summary"]               // Required string
confidence := result.Outputs["confidence_level"]   // Required class (high/medium/low)
stats := result.Outputs["statistics"]              // Optional string (may be nil)
```

**Validation rules:**

| Validation | Example | Error If |
|-----------|---------|----------|
| **Required Input** | `topic`, `depth_level` | Missing from inputs map |
| **Type Checking** | `depth_level: int` | Wrong type provided |
| **Required Output** | `summary`, `key_findings` | LM doesn't provide |
| **Class Constraints** | `confidence_level` ∈ {high, medium, low} | Invalid value |
| **Optional Fields** | `statistics` may be absent | No error if omitted |

**Benefits:**
- **Reliability** - Catch errors early
- **Documentation** - Types serve as contracts
- **Safety** - Prevent invalid data
- **Debugging** - Clear error messages

**When to use:**
- Production systems requiring reliability
- Complex workflows with multiple fields
- Applications with strict output requirements
- Systems needing clear data contracts

## Common Patterns

### Pattern 1: Conditional Outputs

Make outputs optional based on input flags:

```go
// Signature with optional field
sig := dsgo.NewSignature("...").
    AddInput("include_statistics", dsgo.FieldTypeBool, "Whether to include stats").
    AddOptionalOutput("statistics", dsgo.FieldTypeString, "Stats if requested")

// LM includes statistics only when flag is true
inputs := map[string]any{
    "include_statistics": true,  // Statistics will be included
}

result, _ := module.Forward(ctx, inputs)

// Safe access to optional field
if stats, ok := result.Outputs["statistics"]; ok && stats != nil {
    fmt.Println("Statistics:", stats)
}
```

### Pattern 2: Multi-Stage Research

Use iterative refinement for comprehensive analysis:

```go
// Stage 1: Broad search
Action: search(query: "AI impact on software development")

// Stage 2: Specific focus areas
Action: search(query: "AI code generation productivity")
Action: search(query: "AI testing automation")

// Stage 3: Quantitative data
Action: get_statistics(metric: "productivity increase")

// Stage 4: Verification
Action: fact_check(claim: "AI tools increase productivity by 20-55%")

// Stage 5: Context
Action: get_current_date()

// Stage 6: Synthesis
Answer: [Comprehensive structured output]
```

### Pattern 3: Tool Composition

Combine different tool types:

```go
tools := []dsgo.Tool{
    *createSearchTool(),        // Information retrieval
    *createStatisticsTool(),    // Quantitative data
    *createFactCheckerTool(),   // Validation
    *createDateTool(),          // Context
}

// Agent automatically coordinates tool usage
react := module.NewReAct(sig, lm, tools).
    WithMaxIterations(15)

result, _ := react.Forward(ctx, inputs)
```

### Pattern 4: Structured Output Validation

Validate complex output structures:

```go
// Access and validate outputs
summary := result.Outputs["summary"].(string)
confidence := result.Outputs["confidence_level"].(string)  // "high", "medium", or "low"
quality := result.Outputs["research_quality"].(string)     // "excellent", "good", "fair", "limited"
sources := result.Outputs["sources_consulted"].(float64)

// Optional field handling
var statistics string
if stats, ok := result.Outputs["statistics"]; ok && stats != nil {
    statistics = stats.(string)
}

// Validation is automatic - invalid values cause errors
```

## Troubleshooting

### Missing Required Output

**Symptom:** Error about missing required field

**Diagnosis:**
```go
// LM didn't include all required outputs
// Check signature definition
```

**Solution:**
```go
// Ensure all outputs are marked correctly
sig.AddOutput("field", dsgo.FieldTypeString, "Clear description")

// For optional fields
sig.AddOptionalOutput("optional_field", dsgo.FieldTypeString, "May be omitted")
```

### Invalid Class Value

**Symptom:** Error about invalid enum value

**Diagnosis:**
```go
// LM returned value not in allowed list
// Check class output definition
```

**Solution:**
```go
// Define all possible values
sig.AddClassOutput("confidence_level", 
    []string{"high", "medium", "low"},  // Include all valid values
    "Confidence in research")

// Use clear, distinct values
// Avoid similar-sounding options
```

### Tool Not Called

**Symptom:** Agent finishes without using expected tool

**Diagnosis:**
```go
// Tool description unclear
// LM decided tool wasn't needed
```

**Solution:**
```go
// Write clear, specific tool descriptions
tool := dsgo.NewTool(
    "search",
    "Search for CURRENT information on a specific topic or question",  // Clear purpose
    handler,
)

// Make tool necessity clear in signature/inputs
```

### Too Many Iterations

**Symptom:** Agent uses all iterations without finishing

**Diagnosis:**
```go
// MaxIterations too low
// Agent can't complete task
```

**Solution:**
```go
// Increase iteration limit
react := module.NewReAct(sig, lm, tools).
    WithMaxIterations(20)  // Increase from default

// Or simplify the task
// Or provide better tool descriptions
```

## Performance Considerations

### Iteration Count

**Impact:**
- More iterations = more API calls = higher cost
- Each iteration: 1-2 API calls (thought + action)
- Typical research task: 5-15 iterations

**Best practices:**
- Start with conservative limits (10-15 iterations)
- Monitor actual usage in verbose mode
- Adjust based on task complexity
- Use early stopping when possible

### Token Usage

**Typical usage:**
- Per iteration: 500-2000 tokens
- Full research (10 iterations): 5,000-20,000 tokens
- Cost estimate: $0.01-$0.10 per research request

**Optimization:**
- Clear, concise tool descriptions
- Focused input focus areas
- Efficient tool implementations
- Caching for repeated queries

### Tool Execution Time

**Considerations:**
- Simulated tools: <10ms per call
- Real API tools: 100ms-2s per call
- Database queries: 10-500ms per call
- Network requests: 100ms-5s per call

**Best practices:**
- Implement timeouts for all tools
- Cache frequent queries
- Use async tools when possible
- Monitor tool latency

## Comparison with Alternatives

**vs. Simple Predict:**
- **Research Assistant**: Multi-tool, iterative, comprehensive
- **Predict**: Single call, direct answer, faster

**vs. Chain of Thought:**
- **Research Assistant**: Tool usage, information gathering
- **CoT**: Pure reasoning, no external data

**vs. Manual orchestration:**
- **Research Assistant**: Automatic tool selection, adaptive
- **Manual**: More control, more code, less flexible

**vs. Traditional search:**
- **Research Assistant**: Synthesis, validation, structured
- **Search**: Raw results, no analysis, unstructured

## See Also

- [003_react](../003_react/) - Basic ReAct agent with tools
- [017_tools](../017_tools/) - Tool definition & integration
- [002_chain_of_thought](../002_chain_of_thought/) - CoT reasoning
- [001_predict](../001_predict/) - Basic prediction
- [010_typed_signatures](../010_typed_signatures/) - Type-safe API with generics
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide

## Production Tips

1. **Tool Timeout**: Implement timeouts for all external tools
2. **Error Handling**: Tools should never throw, return error messages as strings
3. **Caching**: Cache tool results for repeated queries
4. **Monitoring**: Track tool usage and success rates
5. **Validation**: Validate tool inputs and outputs
6. **Documentation**: Clear tool descriptions improve agent decisions
7. **Testing**: Test each tool independently before integration
8. **Rate Limits**: Respect API rate limits in tools
9. **Fallbacks**: Provide fallback data when tools fail
10. **Logging**: Log all tool calls for debugging and analysis

## Architecture Notes

Research assistant flow:

```
┌────────────────────────────────────────────────────────────┐
│                    Application Code                         │
│  inputs: topic, focus_areas, depth_level, include_stats     │
└──────────────────────────┬─────────────────────────────────┘
                           │
                           ▼
                  ┌────────────────┐
                  │   ReAct Agent  │
                  │ (max 15 iters) │
                  └────────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Iteration 1  │  │ Iteration 2  │  │ Iteration N  │
│ Think → Act  │  │ Think → Act  │  │ Think → Answer│
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       ▼                 ▼                 ▼
┌──────────────────────────────────────────────────┐
│                  Tool Selection                   │
│  search | get_statistics | fact_check | get_date │
└──────────────────┬───────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────┐
│              Tool Execution Results               │
│  Observations fed back to next iteration          │
└──────────────────┬───────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────┐
│           Validated Structured Output             │
│  summary, key_findings, statistics, metadata      │
└──────────────────────────────────────────────────┘
```

**Design Principles:**
- **Declarative** - Signature defines contract, not implementation
- **Composable** - Tools are independent, reusable components
- **Adaptive** - Agent decides strategy based on inputs
- **Validated** - All outputs checked against signature
- **Transparent** - Verbose mode shows decision-making
- **Production-ready** - Error handling, timeouts, validation
