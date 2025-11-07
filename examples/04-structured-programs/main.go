package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

// Demonstrates: Program, ProgramOfThought, JSON adapter, Typed signatures
// Story: Itinerary planner with multi-step pipeline and structured outputs

type Itinerary struct {
	Destination string   `json:"destination"`
	Days        int      `json:"days"`
	Budget      float64  `json:"budget"`
	Activities  []string `json:"activities"`
	Hotels      []string `json:"hotels"`
	TotalCost   float64  `json:"total_cost"`
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(2)
	}
	envFilePath := ""
	dir := cwd
	for {
		candidate := filepath.Join(dir, "examples", ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// If not found in examples/, check cwd/.env.local
	if envFilePath == "" {
		candidate := filepath.Join(cwd, ".env.local")
		if _, err := os.Stat(candidate); err == nil {
			envFilePath = candidate
		}
	}
	if envFilePath == "" {
		fmt.Printf("Could not find .env.local file\n")
		os.Exit(3)
	}
	err = godotenv.Load(envFilePath)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(3)
	}

	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "itinerary_planner", map[string]interface{}{
		"scenario": "structured_pipeline",
	})
	defer runSpan.End(nil)

	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("failed to create LM: %v", err)
	}

	// Usage tracking
	var totalPromptTokens, totalCompletionTokens int

	// User request
	userRequest := "Plan a 3-day trip to Kyoto with a budget of $1500. I'm interested in temples and food."
	fmt.Printf("User: %s\n", userRequest)

	// Step 1: Program of Thought - Generate planning logic
	fmt.Println("\n=== Step 1: Generate Planning Logic (ProgramOfThought) ===")
	step1Ctx, step1Span := observe.Start(ctx, observe.SpanKindModule, "step1_planning_logic", map[string]interface{}{
		"module": "program_of_thought",
	})

	potSig := dsgo.NewSignature("Generate step-by-step planning logic for trip itinerary").
		AddInput("requirements", dsgo.FieldTypeString, "Trip requirements").
		AddOutput("code", dsgo.FieldTypeString, "Planning steps in pseudocode").
		AddOutput("explanation", dsgo.FieldTypeString, "Explanation of the planning approach")

	pot := module.NewProgramOfThought(potSig, lm, "python").
		WithAllowExecution(true).  // Enable code execution
		WithExecutionTimeout(10)    // 10 second safety timeout

	planResult, err := pot.Forward(step1Ctx, map[string]interface{}{
		"requirements": "3-day trip to Kyoto, budget $1500, interested in temples and food",
	})
	if err != nil {
		log.Fatal(err)
	}

	steps, _ := planResult.GetString("code")
	explanation, _ := planResult.GetString("explanation")
	fmt.Printf("Planning logic:\n%s\n\nExplanation: %s\n", steps, explanation)
	
	// Show execution result if available
	if execResult, ok := planResult.GetString("execution_result"); ok {
		fmt.Printf("\n✓ Code executed successfully:\n%s\n", execResult)
	} else if execErr, ok := planResult.GetString("execution_error"); ok {
		fmt.Printf("\n✗ Execution failed: %s\n", execErr)
	}
	
	usage1 := planResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage1.PromptTokens, usage1.CompletionTokens)
	totalPromptTokens += usage1.PromptTokens
	totalCompletionTokens += usage1.CompletionTokens
	step1Span.End(nil)

	// Step 2: Extract constraints (Predict with JSON)
	fmt.Println("\n=== Step 2: Extract Constraints (JSON Output) ===")
	step2Ctx, step2Span := observe.Start(ctx, observe.SpanKindModule, "step2_constraints", map[string]interface{}{
		"module":  "predict",
		"adapter": "json",
	})

	constraintsSig := dsgo.NewSignature("Extract structured constraints from requirements").
		AddInput("requirements", dsgo.FieldTypeString, "Trip requirements").
		AddOutput("destination", dsgo.FieldTypeString, "Destination city").
		AddOutput("days", dsgo.FieldTypeInt, "Number of days").
		AddOutput("budget", dsgo.FieldTypeFloat, "Budget in USD").
		AddOutput("interests", dsgo.FieldTypeJSON, "List of interests")

	extractPredict := module.NewPredict(constraintsSig, lm)

	constraintsResult, err := extractPredict.Forward(step2Ctx, map[string]interface{}{
		"requirements": "3-day trip to Kyoto, budget $1500, interested in temples and food",
	})
	if err != nil {
		log.Fatal(err)
	}

	destination, _ := constraintsResult.GetString("destination")
	days, _ := constraintsResult.GetInt("days")
	budget, _ := constraintsResult.GetFloat("budget")
	interests, _ := constraintsResult.Get("interests")
	
	fmt.Printf("Constraints:\n")
	fmt.Printf("  Destination: %s\n", destination)
	fmt.Printf("  Days: %d\n", days)
	fmt.Printf("  Budget: $%.2f\n", budget)
	interestsJSON, _ := json.Marshal(interests)
	fmt.Printf("  Interests: %s\n", string(interestsJSON))
	usage2 := constraintsResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage2.PromptTokens, usage2.CompletionTokens)
	totalPromptTokens += usage2.PromptTokens
	totalCompletionTokens += usage2.CompletionTokens
	step2Span.End(nil)

	// Step 3: Build Program - Chained pipeline
	fmt.Println("\n=== Step 3: Execute Planning Pipeline (Program) ===")
	step3Ctx, step3Span := observe.Start(ctx, observe.SpanKindProgram, "step3_pipeline", map[string]interface{}{
		"steps": 3,
	})

	// Sub-step 3a: Get activities
	activitiesSig := dsgo.NewSignature("Recommend activities based on interests").
		AddInput("destination", dsgo.FieldTypeString, "Destination").
		AddInput("interests", dsgo.FieldTypeJSON, "Interests list").
		AddInput("days", dsgo.FieldTypeInt, "Number of days").
		AddOutput("activities", dsgo.FieldTypeJSON, "List of recommended activities")

	activitiesPredict := module.NewPredict(activitiesSig, lm)

	// Sub-step 3b: Get hotels
	hotelsSig := dsgo.NewSignature("Recommend hotels within budget").
		AddInput("destination", dsgo.FieldTypeString, "Destination").
		AddInput("budget", dsgo.FieldTypeFloat, "Budget per night").
		AddInput("days", dsgo.FieldTypeInt, "Number of days").
		AddOutput("hotels", dsgo.FieldTypeJSON, "List of hotel recommendations")

	hotelsPredict := module.NewPredict(hotelsSig, lm)

	// Sub-step 3c: Build final itinerary
	itinerarySig := dsgo.NewSignature("Create final itinerary with cost breakdown").
		AddInput("destination", dsgo.FieldTypeString, "Destination").
		AddInput("days", dsgo.FieldTypeInt, "Number of days").
		AddInput("budget", dsgo.FieldTypeFloat, "Total budget").
		AddInput("activities", dsgo.FieldTypeJSON, "Activities list").
		AddInput("hotels", dsgo.FieldTypeJSON, "Hotels list").
		AddOutput("itinerary", dsgo.FieldTypeJSON, "Complete itinerary with cost")

	itineraryPredict := module.NewPredict(itinerarySig, lm)

	// Create Program - simplified pipeline
	program := module.NewProgram("itinerary_planner").
		AddModule(activitiesPredict).
		AddModule(hotelsPredict).
		AddModule(itineraryPredict)

	programInputs := map[string]interface{}{
		"destination": destination,
		"days":        days,
		"budget":      budget,
		"interests":   interests,
	}

	programResult, err := program.Forward(step3Ctx, programInputs)
	if err != nil {
		log.Fatal(err)
	}

	itineraryData, _ := programResult.Get("itinerary")
	itineraryJSON, _ := json.MarshalIndent(itineraryData, "", "  ")
	fmt.Printf("\nFinal Itinerary:\n%s\n", string(itineraryJSON))
	usage3 := programResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage3.PromptTokens, usage3.CompletionTokens)
	totalPromptTokens += usage3.PromptTokens
	totalCompletionTokens += usage3.CompletionTokens
	step3Span.End(nil)

	// Turn 2: Modify itinerary
	fmt.Println("\n=== Turn 2: Modify Itinerary (Add Kid-Friendly Activity) ===")
	fmt.Printf("User: Please add one kid-friendly activity per day to this itinerary\n")
	turn2Ctx, turn2Span := observe.Start(ctx, observe.SpanKindModule, "turn2_modify", nil)

	modifySig := dsgo.NewSignature("Modify itinerary to add specific requirement").
		AddInput("current_itinerary", dsgo.FieldTypeJSON, "Current itinerary").
		AddInput("modification", dsgo.FieldTypeString, "Modification request").
		AddOutput("updated_itinerary", dsgo.FieldTypeJSON, "Updated itinerary")

	modifyPredict := module.NewPredict(modifySig, lm)

	modifyResult, err := modifyPredict.Forward(turn2Ctx, map[string]interface{}{
		"current_itinerary": itineraryData,
		"modification":      "Add one kid-friendly activity per day",
	})
	if err != nil {
		log.Fatal(err)
	}

	updatedItinerary, _ := modifyResult.Get("updated_itinerary")
	updatedJSON, _ := json.MarshalIndent(updatedItinerary, "", "  ")
	fmt.Printf("\nUpdated Itinerary:\n%s\n", string(updatedJSON))
	usage4 := modifyResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usage4.PromptTokens, usage4.CompletionTokens)
	totalPromptTokens += usage4.PromptTokens
	totalCompletionTokens += usage4.CompletionTokens
	turn2Span.End(nil)

	// Summary
	fmt.Println("\n=== Itinerary Planner Summary ===")
	fmt.Println("Pipeline: PoT (logic) → Extract (JSON) → Program (activities → hotels → itinerary)")
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  ✓ ProgramOfThought with code execution + timeout")
	fmt.Println("  ✓ JSON adapter (structured I/O)")
	fmt.Println("  ✓ Typed signatures (strong field types)")
	fmt.Println("  ✓ Program (multi-step module composition)")
	fmt.Println("  ✓ Multi-turn modification")
	fmt.Println("  ✓ Event logging for each pipeline step")

	// Usage stats
	fmt.Printf("\n=== Usage Stats ===\n")
	fmt.Printf("Total Prompt Tokens: %d\n", totalPromptTokens)
	fmt.Printf("Total Completion Tokens: %d\n", totalCompletionTokens)
}
