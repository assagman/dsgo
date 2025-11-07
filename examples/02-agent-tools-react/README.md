# 02 - Agent Tools ReAct

**Travel Helper Agent with Multi-Tool Reasoning**

## What This Demonstrates

### Modules
- ✓ **ReAct** - Reasoning and Acting with tools

### Adapters
- ✓ **JSON** - Structured tool arguments and results
- ✓ **Chat** - Natural conversation format

### Features
- ✓ **Tools** - Multiple typed tools (search, currency, timezone)
- ✓ **Typed signatures** - Strongly typed tool parameters
- ✓ **Verbose mode** - See agent's reasoning process
- ✓ **Multi-step planning** - Agent chains multiple tool calls

### Observability
- ✓ Tool call tracking with arguments
- ✓ ReAct iteration visibility (thought → action → observation)
- ✓ Event logging for each tool invocation

## Story Flow

1. **Turn 1**: "What's a good weekend trip from London to Barcelona? Convert price from USD to EUR"
   - Agent searches for Barcelona trips
   - Converts currency using tool
   - Synthesizes answer

2. **Turn 2**: "If it's 9 AM in London on Saturday, what time is it in Barcelona?"
   - Agent queries both timezones
   - Calculates time difference
   - Provides answer

## Tools Provided

1. **search(query: string)** - Web search simulation
2. **convert_currency(amount: number, from: string, to: string)** - Currency conversion
3. **local_time(city: string)** - Timezone and current time lookup

## Run

```bash
cd examples/02-agent-tools-react
go run main.go
```

### With verbose mode (see agent thinking)
```bash
go run main.go  # Already enabled via WithVerbose(true)
```

### With event logging
```bash
DSGO_LOG=pretty go run main.go
```

## Expected Output

```
=== Turn 1: Weekend Trip Planning ===
▶ turn1.start tools_available=3

Thought: I need to search for Barcelona weekend trips and get pricing
Action: search(query="Barcelona weekend trips from London price")
Observation: Barcelona weekend trips: avg $450-650 (flights+hotel)...

Thought: Now I need to convert USD to EUR
Action: convert_currency(amount=550, from="USD", to="EUR")
Observation: {"result": "506.00", "rate": 0.92}

Thought: I have all the information needed
✓ Final Answer:
A weekend trip to Barcelona from London typically costs $450-650 
(approximately €414-598 EUR). The best times are shoulder seasons 
(April-May or September-October)...

✓ turn1.end 2341ms tools_used=2

=== Turn 2: Timezone Query ===
▶ turn2.start

Thought: I need to check the local time in both cities
Action: local_time(city="London")
Observation: {"time": "09:00", "timezone": "UTC+0"}

Action: local_time(city="Barcelona")  
Observation: {"time": "10:00", "timezone": "UTC+1"}

✓ Final Answer:
If it's 9 AM in London, it's 10 AM in Barcelona (1 hour ahead).

✓ turn2.end 1876ms
```

## Key Patterns

### Tool Definition
```go
tool := dsgo.NewTool(name, description, function).
    AddParameter(name, type, description, required)
```

### ReAct Configuration
```go
react := module.NewReAct(sig, lm, tools).
    WithMaxIterations(8).
    WithVerbose(true)
```

### Observability Integration
```go
ctx, span := observe.Start(ctx, observe.SpanKindTool, "tool_name", args)
defer span.End(nil)
```
