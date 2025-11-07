# 04 - Structured Programs

**Itinerary Planner with Multi-Step Pipeline and JSON Schemas**

## What This Demonstrates

### Modules
- ✓ **ProgramOfThought** - Generate planning logic/pseudocode
- ✓ **Program** - Multi-step module composition
- ✓ **Predict** - Basic completions with typed outputs

### Adapters
- ✓ **JSON** - Strongly typed structured I/O
- ✓ **Chat** - Natural language interaction

### Features
- ✓ **Typed signatures** - Field type validation (String, Int, Float, JSON)
- ✓ **Module composition** - Chain multiple modules with data flow
- ✓ **Structured outputs** - JSON schemas for complex data
- ✓ **Multi-turn** - Modify structured data across turns

### Observability
- ✓ Program step tracking
- ✓ Data flow visibility
- ✓ JSON serialization events

## Story Flow

### Pipeline Architecture
```
Step 1: ProgramOfThought → Planning Logic
Step 2: Predict (JSON) → Extract Constraints
Step 3: Program Pipeline:
  3a: Predict → Activities (based on interests)
  3b: Predict → Hotels (within budget)
  3c: Predict → Final Itinerary (combine all)
Turn 2: Predict (JSON) → Modify Itinerary
```

### Conversation
1. **Step 1**: Generate pseudocode for trip planning logic
2. **Step 2**: Extract structured constraints (destination, days, budget, interests)
3. **Step 3**: Execute pipeline (activities → hotels → itinerary assembly)
4. **Turn 2**: Modify itinerary to add kid-friendly activities

## Program Data Flow

```
Inputs: {destination, days, budget, interests}
  ↓
Step "activities": activities ← f(destination, interests, days)
  ↓
Step "hotels": hotels ← f(destination, budget, days)
  ↓
Step "itinerary": itinerary ← f(destination, days, budget, activities, hotels)
  ↓
Output: {itinerary: {...}}
```

## Typed Signature Example

```go
sig := dsgo.NewSignature("Extract constraints").
    AddInput("requirements", dsgo.FieldTypeString, "Requirements").
    AddOutput("destination", dsgo.FieldTypeString, "City").
    AddOutput("days", dsgo.FieldTypeInt, "Number of days").
    AddOutput("budget", dsgo.FieldTypeFloat, "Budget USD").
    AddOutput("interests", dsgo.FieldTypeJSON, "Interests list")
```

## Program Composition

```go
program := module.NewProgram().
    AddStep("activities", activitiesModule, map[string]string{
        "destination": "destination",
        "interests": "interests",
    }).
    AddStep("hotels", hotelsModule, map[string]string{
        "destination": "destination",
        "budget": "budget",
    }).
    AddStep("final", finalModule, map[string]string{
        "activities": "activities.activities",  // Access nested field
        "hotels": "hotels.hotels",
    })
```

## Run

```bash
cd examples/04-structured-programs
go run main.go
```

### With event logging
```bash
DSGO_LOG=pretty go run main.go
```

## Expected Output

```
=== Step 1: Generate Planning Logic (ProgramOfThought) ===
▶ step1_planning_logic.start module=program_of_thought

Planning logic:
```
1. Parse requirements (destination, days, budget, interests)
2. Query activities database filtered by interests
3. Query hotels database filtered by budget/night
4. Calculate costs: sum(activities) + hotels * days
5. Verify total_cost <= budget
6. Return itinerary with cost breakdown
```

✓ step1_planning_logic.end 1456ms

=== Step 2: Extract Constraints (JSON Output) ===
▶ step2_constraints.start module=predict adapter=json

Constraints:
  Destination: Kyoto
  Days: 3
  Budget: $1500.00
  Interests: ["temples","food"]

✓ step2_constraints.end 892ms

=== Step 3: Execute Planning Pipeline (Program) ===
▶ step3_pipeline.start steps=3
▶ program.step.start step=activities
✓ program.step.end step=activities output_size=256
▶ program.step.start step=hotels
✓ program.step.end step=hotels output_size=189
▶ program.step.start step=itinerary
✓ program.step.end step=itinerary output_size=512

Final Itinerary:
{
  "destination": "Kyoto",
  "days": 3,
  "budget": 1500,
  "activities": [
    "Fushimi Inari Shrine (free)",
    "Nishiki Market food tour ($45)",
    "Kinkaku-ji Golden Pavilion ($5)"
  ],
  "hotels": [
    "Hotel Gran Ms Kyoto ($120/night)"
  ],
  "total_cost": 1410
}

✓ step3_pipeline.end 4231ms steps_executed=3

=== Turn 2: Modify Itinerary (Add Kid-Friendly Activity) ===
▶ turn2_modify.start

Updated Itinerary:
{
  "destination": "Kyoto",
  "days": 3,
  "budget": 1500,
  "activities": [
    "Fushimi Inari Shrine (free, kid-friendly)",
    "Nishiki Market food tour ($45)",
    "Kinkaku-ji Golden Pavilion ($5)",
    "Kyoto Railway Museum ($10, kid-friendly)",
    "Arashiyama Monkey Park ($6, kid-friendly)"
  ],
  "hotels": [
    "Hotel Gran Ms Kyoto ($120/night, family room)"
  ],
  "total_cost": 1476
}

✓ turn2_modify.end 1123ms
```

## Key Patterns

### JSON Field Types
```go
AddOutput("field", dsgo.FieldTypeJSON, "Description")  // Any JSON structure
AddOutput("count", dsgo.FieldTypeInt, "Count")         // Integer
AddOutput("price", dsgo.FieldTypeFloat, "Price")       // Float
```

### Accessing Nested Program Outputs
```go
// Access output "activities" from step "recommend_activities"
"activities": "recommend_activities.activities"
```

### ProgramOfThought Safety
```go
pot := module.NewProgramOfThought(sig, lm).
    WithAllowExecution(false)  // Generate code but don't execute
```
