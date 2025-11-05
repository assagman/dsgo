# 008_chat_predict - Multi-Turn Conversations with History

## Overview

Demonstrates how to build multi-turn conversational applications using the Predict module combined with conversation history management. Shows how to maintain context across multiple exchanges for natural, contextual responses.

## What it demonstrates

- Creating and managing conversation history
- Setting history limits to control memory usage
- Adding system, user, and assistant messages
- Formatting conversation context for LM input
- Building multi-turn conversations with Predict
- Context-aware responses that reference previous turns
- Token tracking across conversation turns

## Usage

```bash
cd examples/008_chat_predict
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
User: I'm planning a trip to Japan in spring. What's the best time to visit?
Assistant: Late March to early May is ideal for cherry blossoms...

[Turn 2]
User: What cities should I visit there?
Assistant: Based on your spring visit, I recommend Tokyo, Kyoto, and Osaka...

[Turn 3]
User: How many days should I spend in each city you mentioned?
Assistant: For Tokyo, Kyoto, and Osaka, I'd suggest 4-5 days in Tokyo...

[Turn 4]
User: What's the best way to travel between those cities?
Assistant: The JR Pass is perfect for traveling between Tokyo, Kyoto, and Osaka...

📊 Conversation Summary:
  Total messages: 9
  System messages: 1
  User turns: 4
  Total tokens used: 1250
```

## Key Concepts

- **History Management**: Track conversation context with message history
- **History Limits**: Prevent unbounded growth with `NewHistoryWithLimit(N)`
- **System Messages**: Set assistant personality and behavior guidelines
- **Context Formatting**: Format history for LM consumption
- **Multi-Turn Context**: Each response builds on previous exchanges
- **Token Tracking**: Monitor cumulative token usage across turns

## History API

### Creating History
```go
history := dsgo.NewHistoryWithLimit(20)  // Keep last 20 messages
```

### Adding Messages
```go
history.AddSystemMessage("You are a helpful assistant")
history.AddUserMessage("What is...")
history.AddAssistantMessage("The answer is...")
```

### Retrieving Messages
```go
recent := history.GetLast(6)  // Get last 6 messages
all := history.GetAll()        // Get all messages
count := history.Len()         // Get message count
```

## See Also

- [001_predict](../001_predict/) - Basic Predict module
- [009_chat_cot](../009_chat_cot/) - Multi-turn with ChainOfThought
- [015_history](../015_history/) - Advanced history features
- [QUICKSTART.md](../../QUICKSTART.md) - Getting started guide
