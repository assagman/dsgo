# 01 - Hello Chat

**Personal Assistant with Streaming, History, and Caching**

## What This Demonstrates

### Modules
- ✓ **Predict** - Basic prediction with conversational flow

### Adapters  
- ✓ **Chat** - Natural dialogue format

### Features
- ✓ **Streaming** - Real-time token-by-token output
- ✓ **History** - Multi-turn context retention (10 message limit)
- ✓ **Caching** - LM cache for repeated queries

### Observability
- ✓ Event logging (set `DSGO_LOG=pretty`)
- ✓ Cache hit/miss tracking
- ✓ Timing and token metrics

## Story Flow

1. **Turn 1**: User introduces themselves (name="Alex", interest="hiking") with streaming response
2. **Turn 2**: Follow-up question that relies on conversation history
3. **Turn 3a**: New question (cache miss expected)
3. **Turn 3b**: Repeat same question (cache hit expected)

## Run

**Note:** `EXAMPLES_DEFAULT_MODEL` environment variable is required.

```bash
cd examples/01-hello-chat
EXAMPLES_DEFAULT_MODEL="anthropic/claude-3.5-sonnet" go run main.go
```

### With different model
```bash
EXAMPLES_DEFAULT_MODEL="anthropic/claude-3-haiku" go run main.go
```

### With verbose logging
```bash
DSGO_LOG=pretty EXAMPLES_DEFAULT_MODEL="anthropic/claude-3.5-sonnet" go run main.go
```

### With JSON events
```bash
DSGO_LOG=events EXAMPLES_DEFAULT_MODEL="anthropic/claude-3.5-sonnet" go run main.go > events.jsonl
```

## Expected Output

```
=== Turn 1: Introduction (Streaming) ===
▶ turn1.start streaming=true
Assistant: [streaming tokens appear progressively]
✓ turn1.end 1243ms

=== Turn 2: Follow-up (Using History) ===
▶ turn2.start history_entries=2
Assistant: [response references earlier context about Alex being a beginner]
✓ turn2.end 876ms

=== Turn 3a: New Question (Cache Miss) ===
▶ turn3a.start
• cache.miss
Assistant: [gear recommendations]
✓ turn3a.end 1102ms

=== Turn 3b: Repeat Question (Cache Hit Expected) ===
▶ turn3b.start
• cache.hit
Assistant: [same gear recommendations, near-instant]
✓ turn3b.end 12ms
```
