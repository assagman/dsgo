# Research Assistant Example

## Overview

This advanced example demonstrates the full power of DSGo by combining:

- **Complex Custom Signatures**: Multiple input/output fields with diverse types
- **Tool Integration**: 4 different tools working together
- **ReAct Reasoning**: Iterative problem-solving with tool use
- **Type Safety**: String, Int, Bool, Class/Enum types with validation
- **Optional Outputs**: Conditional fields based on inputs

## What This Example Demonstrates

### 1. Custom Signature with Multiple Types

```go
sig := dsgo.NewSignature("Research a topic and provide comprehensive analysis").
    // Inputs: Different types
    AddInput("topic", dsgo.FieldTypeString, "The main research topic").
    AddInput("focus_areas", dsgo.FieldTypeString, "Specific aspects to focus on").
    AddInput("depth_level", dsgo.FieldTypeInt, "Research depth: 1=basic, 2=intermediate, 3=deep").
    AddInput("include_statistics", dsgo.FieldTypeBool, "Whether to include statistical data").
    
    // Outputs: Mix of required, optional, and constrained types
    AddOutput("summary", dsgo.FieldTypeString, "Executive summary of findings").
    AddOutput("key_findings", dsgo.FieldTypeString, "Bullet-pointed key discoveries").
    AddClassOutput("confidence_level", []string{"high", "medium", "low"}, "Confidence in research").
    AddOutput("sources_consulted", dsgo.FieldTypeInt, "Number of sources checked").
    AddOptionalOutput("statistics", dsgo.FieldTypeString, "Statistical data if requested").
    AddOutput("recommendations", dsgo.FieldTypeString, "Action items or next steps").
    AddClassOutput("research_quality", []string{"excellent", "good", "fair", "limited"}, "Quality assessment")
```

### 2. Multiple Tools Working Together

- **search**: Web search for information
- **get_statistics**: Retrieve statistical data
- **fact_check**: Verify claims and statements
- **get_current_date**: Temporal context for research

### 3. ReAct Reasoning Pattern

The agent:
1. Analyzes the research topic and focus areas
2. Plans which tools to use and in what order
3. Searches for information on each focus area
4. Gathers statistics when requested
5. Fact-checks important claims
6. Synthesizes findings into structured output
7. Validates outputs match the signature constraints

## Running the Example

```bash
# Set your OpenAI API key
export OPENAI_API_KEY=your_key_here

# Run the example
go run examples/research_assistant/main.go
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

[... more iterations ...]

======================================================================
📊 RESEARCH RESULTS
======================================================================

📝 SUMMARY:
[Executive summary of findings]

🔍 KEY FINDINGS:
[Bullet-pointed discoveries]

📈 STATISTICS:
[Statistical data if requested]

💡 RECOMMENDATIONS:
[Action items and next steps]

📊 METADATA:
  Confidence Level: high
  Research Quality: excellent
  Sources Consulted: 4
======================================================================
```

## Key Learning Points

### Type Diversity
Shows how to use all field types in a single signature:
- `FieldTypeString`: Text inputs/outputs
- `FieldTypeInt`: Numeric values (depth level, source count)
- `FieldTypeBool`: Flags (include_statistics)
- `FieldTypeClass`: Constrained enums (confidence, quality)

### Optional Fields
Demonstrates conditional outputs:
```go
AddOptionalOutput("statistics", dsgo.FieldTypeString, "Stats if requested")
```
The LM includes statistics only when `include_statistics` is true.

### Tool Composition
Shows how multiple tools work together:
- Search provides general information
- Statistics tool adds quantitative data
- Fact checker validates claims
- Date tool provides temporal context

### Validation
All outputs are validated:
- Class fields must match allowed values
- Required fields must be present
- Optional fields are allowed to be missing
- Type checking ensures correct data types

## Extending This Example

You can extend this to:
1. Add more tools (database, API calls, calculations)
2. Use different field types (Image, Datetime, JSON)
3. Increase depth_level to trigger more iterations
4. Add more focus areas to research
5. Integrate with real APIs instead of simulated data

## Real-World Applications

This pattern is useful for:
- Research assistants
- Report generation
- Data analysis and synthesis
- Multi-source information gathering
- Fact-checking and validation workflows
- Content creation with citations
- Due diligence and investigation tasks
