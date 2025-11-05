# 009_chat_cot - Multi-Turn Chain of Thought Reasoning

## Overview

Demonstrates how to build multi-turn conversational applications using the ChainOfThought module combined with conversation history management. Shows how to maintain context across multiple exchanges while providing step-by-step reasoning for complex problem-solving scenarios.

## What it demonstrates

- Creating and managing conversation history with ChainOfThought
- Multi-turn reasoning with context awareness
- Step-by-step problem solving across conversation turns
- Accessing both explanation and answer outputs
- System messages for setting assistant personality
- Formatting conversation context for reasoning tasks
- Token tracking across multi-turn reasoning sessions
- Building educational/tutoring applications

## Usage

```bash
cd examples/009_chat_cot
go run main.go
```

### With Harness Flags

```bash
go run main.go -verbose -format=json
go run main.go -concurrency=5 -timeout=120
```

### Environment Variables

```bash
export HARNESS_VERBOSE=true
export HARNESS_OUTPUT_FORMAT=json
go run main.go
```

## Expected Output

```
[Turn 1]
Student: I need to calculate the total cost of buying 3 shirts at $25 each and 2 pairs of pants. Can you help me set up the problem?
Tutor Reasoning: Let me break this down step by step. First, we need to identify what we're calculating...
Tutor Answer: The problem setup is: (3 × $25) + (2 × pants_price) = total_cost...

[Turn 2]
Student: Great! Now, if each pair of pants costs $40, what's the cost of the pants?
Tutor Reasoning: Now that we know the pants cost $40 each, I'll calculate...
Tutor Answer: 2 pairs of pants × $40 = $80

[Turn 3]
Student: Perfect! Now what's the total cost for everything, and if I have a 15% discount coupon, what's my final price?
Tutor Reasoning: Let me combine all the previous work and apply the discount...
Tutor Answer: Total before discount: $155. With 15% off: $155 × 0.85 = $131.75

📊 Problem-Solving Session:
  Total messages: 7
  System messages: 1
  Problem-solving turns: 3
  Total tokens used: 1450
  ✅ Successfully solved multi-step problem across conversation!
```

## Key Concepts

- **ChainOfThought + History**: Combine reasoning with conversation context
- **Multi-Turn Reasoning**: Each response builds on previous reasoning steps
- **Explanation Access**: Get both the reasoning (`explanation`) and the answer
- **Educational Applications**: Perfect for tutoring, teaching, problem-solving
- **Context-Aware Reasoning**: Use conversation history to inform reasoning
- **Step-by-Step Teaching**: Break down complex problems across turns

## ChainOfThought with Multi-Turn Context

### Signature for Multi-Turn Reasoning
```go
sig := dsgo.NewSignature("Help solve math problems using step-by-step reasoning and conversation context").
    AddInput("question", dsgo.FieldTypeString, "The current question or problem").
    AddInput("context", dsgo.FieldTypeString, "Previous conversation and work done").
    AddOutput("explanation", dsgo.FieldTypeString, "Step-by-step explanation of the reasoning").
    AddOutput("answer", dsgo.FieldTypeString, "Answer or next step")
```

### Creating the Module
```go
cot := module.NewChainOfThought(sig, lm)
```

### Accessing Outputs
```go
result, err := cot.Forward(ctx, inputs)
explanation, _ := result.GetString("explanation")  // The reasoning steps
answer, _ := result.GetString("answer")            // The final answer
```

## Use Cases

- **Educational Tutoring**: Guide students through complex problems step by step
- **Problem Decomposition**: Break down multi-step problems across conversation
- **Technical Support**: Troubleshoot issues with reasoning and context
- **Research Assistance**: Build on previous findings in multi-turn exploration
- **Debugging Help**: Walk through code issues with contextual reasoning

## See Also

- [002_chain_of_thought](../002_chain_of_thought/) - Basic ChainOfThought module
- [008_chat_predict](../008_chat_predict/) - Multi-turn with Predict
- [015_history](../015_history/) - Advanced history features
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
