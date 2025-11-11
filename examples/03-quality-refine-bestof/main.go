package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/examples/observe"
	"github.com/assagman/dsgo/module"
)

// Demonstrates: ChainOfThought, BestOfN, Refine, Few-shot learning
// Story: Email drafting with reasoning, quality selection, and refinement

func main() {
	ctx := context.Background()
	ctx, runSpan := observe.Start(ctx, observe.SpanKindRun, "email_drafter", map[string]interface{}{
		"scenario": "quality_optimization",
	})
	defer runSpan.End(nil)

	model := os.Getenv("EXAMPLES_DEFAULT_MODEL")
	if model == "" {
		log.Fatal("EXAMPLES_DEFAULT_MODEL environment variable must be set")
	}
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Usage tracking
	var totalPromptTokens, totalCompletionTokens int

	// Few-shot examples for email style using ExampleSet
	emailExamples := core.NewExampleSet("professional_emails").
		AddPair(
			map[string]interface{}{
				"purpose": "request feedback on design proposal",
				"tone":    "friendly professional",
			},
			map[string]interface{}{
				"outline": "1. Greeting 2. Context (design proposal attached) 3. Specific ask (feedback by Friday) 4. Thank you",
				"email":   "Hi Sarah,\n\nI've attached the new dashboard design proposal we discussed. I'd love your feedback, especially on the navigation flow.\n\nCould you share your thoughts by Friday? Happy to hop on a call if that's easier.\n\nThanks!\nAlex",
			},
		)

	// User request
	userRequest := "Help me draft an email requesting code review feedback from a senior engineer. Make it respectful but concise."
	fmt.Printf("User: %s\n", userRequest)

	// Step A: ChainOfThought for outline with few-shot
	fmt.Println("\n=== Step A: Create Outline (ChainOfThought + Few-shot) ===")
	fmt.Printf("Using %d example(s) for style guidance\n", emailExamples.Len())
	stepACtx, stepASpan := observe.Start(ctx, observe.SpanKindModule, "stepA_outline", map[string]interface{}{
		"module":   "cot",
		"few_shot": emailExamples.Len(),
	})

	outlineSig := dsgo.NewSignature("Create a structured outline for an email").
		AddInput("purpose", dsgo.FieldTypeString, "Email purpose").
		AddInput("tone", dsgo.FieldTypeString, "Desired tone").
		AddOutput("outline", dsgo.FieldTypeString, "Email outline with sections")

	// Convert ExampleSet to slice of Example values
	examplePtrs := emailExamples.Get()
	examples := make([]dsgo.Example, len(examplePtrs))
	for i, ex := range examplePtrs {
		examples[i] = *ex
	}

	cot := module.NewChainOfThought(outlineSig, lm).WithDemos(examples)

	outlineResult, err := cot.Forward(stepACtx, map[string]interface{}{
		"purpose": "request code review feedback from senior engineer",
		"tone":    "respectful but concise",
	})
	if err != nil {
		log.Fatal(err)
	}

	outline, _ := outlineResult.GetString("outline")
	rationale := outlineResult.Rationale
	fmt.Printf("Outline:\n%s\n\nRationale: %s\n", outline, rationale)
	usageA := outlineResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usageA.PromptTokens, usageA.CompletionTokens)
	totalPromptTokens += usageA.PromptTokens
	totalCompletionTokens += usageA.CompletionTokens
	stepASpan.End(nil)

	// Step B: BestOfN for opening section
	fmt.Println("\n=== Step B: Generate Best Opening (BestOfN N=5) ===")
	stepBCtx, stepBSpan := observe.Start(ctx, observe.SpanKindModule, "stepB_bestof", map[string]interface{}{
		"module": "bestofn",
		"n":      5,
	})

	openingSig := dsgo.NewSignature("Write an email opening paragraph").
		AddInput("outline", dsgo.FieldTypeString, "Email outline").
		AddInput("tone", dsgo.FieldTypeString, "Desired tone").
		AddOutput("opening", dsgo.FieldTypeString, "Opening paragraph")

	// Custom scorer: prefer shorter, direct openings
	scorer := func(inputs map[string]interface{}, pred *dsgo.Prediction) (float64, error) {
		opening, ok := pred.GetString("opening")
		if !ok {
			return 0.0, nil
		}
		words := len(strings.Fields(opening))

		// Penalize overly long openings
		lengthScore := 1.0
		if words > 50 {
			lengthScore = 0.5
		} else if words > 30 {
			lengthScore = 0.8
		}

		// Reward directness (check for key phrases)
		directnessScore := 0.5
		lowerOpening := strings.ToLower(opening)
		if strings.Contains(lowerOpening, "review") && strings.Contains(lowerOpening, "feedback") {
			directnessScore = 1.0
		}

		return (lengthScore + directnessScore) / 2.0, nil
	}

	openingPredict := module.NewPredict(openingSig, lm)
	bestof := module.NewBestOfN(openingPredict, 5).
		WithScorer(scorer).
		WithThreshold(0.85). // Early-stop if score >= 0.85
		WithReturnAll(true)  // Return all candidates for analysis

	bestofResult, err := bestof.Forward(stepBCtx, map[string]interface{}{
		"outline": outline,
		"tone":    "respectful but concise",
	})
	if err != nil {
		log.Fatal(err)
	}

	opening, _ := bestofResult.GetString("opening")
	candidateCount := len(bestofResult.Completions)
	stoppedEarly := candidateCount < 5

	fmt.Printf("Generated %d candidate(s), stopped early: %v\n", candidateCount, stoppedEarly)
	fmt.Printf("Best score: %.2f\n\n", bestofResult.Score)
	fmt.Printf("Best opening:\n%s\n", opening)
	usageB := bestofResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usageB.PromptTokens, usageB.CompletionTokens)
	totalPromptTokens += usageB.PromptTokens
	totalCompletionTokens += usageB.CompletionTokens
	stepBSpan.End(nil)

	// Step C: Refine with constraints
	fmt.Println("\n=== Step C: Refine for Tone (Refine) ===")
	stepCCtx, stepCSpan := observe.Start(ctx, observe.SpanKindModule, "stepC_refine", map[string]interface{}{
		"module":     "refine",
		"iterations": 2,
	})

	refineSig := dsgo.NewSignature("Refine email to match tone and style constraints").
		AddInput("draft", dsgo.FieldTypeString, "Draft email").
		AddInput("constraints", dsgo.FieldTypeString, "Style constraints").
		AddOutput("refined", dsgo.FieldTypeString, "Refined email")

	refine := module.NewRefine(refineSig, lm).
		WithMaxIterations(2)

	draftEmail := opening + "\n\n[Body sections...]\n\nBest regards,\nAlex"

	refineResult, err := refine.Forward(stepCCtx, map[string]interface{}{
		"draft":       draftEmail,
		"constraints": "Make it more formal. Add clear deadline. Keep under 100 words.",
	})
	if err != nil {
		log.Fatal(err)
	}

	refined, _ := refineResult.GetString("refined")
	fmt.Printf("\nRefined email:\n%s\n", refined)
	usageC := refineResult.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usageC.PromptTokens, usageC.CompletionTokens)
	totalPromptTokens += usageC.PromptTokens
	totalCompletionTokens += usageC.CompletionTokens
	stepCSpan.End(nil)

	// Step D: Second refinement (multi-turn)
	fmt.Println("\n=== Step D: Further Refinement (Multi-turn) ===")
	stepDCtx, stepDSpan := observe.Start(ctx, observe.SpanKindModule, "stepD_refine2", nil)

	refineResult2, err := refine.Forward(stepDCtx, map[string]interface{}{
		"draft":       refined,
		"constraints": "Add a specific call-to-action for scheduling a review meeting.",
	})
	if err != nil {
		log.Fatal(err)
	}

	final, _ := refineResult2.GetString("refined")
	fmt.Printf("\nFinal email:\n%s\n", final)
	usageD := refineResult2.Usage
	fmt.Printf("Usage: Prompt %d tokens, Completion %d tokens\n", usageD.PromptTokens, usageD.CompletionTokens)
	totalPromptTokens += usageD.PromptTokens
	totalCompletionTokens += usageD.CompletionTokens
	stepDSpan.End(nil)

	// Summary
	fmt.Println("\n=== Email Drafting Pipeline Summary ===")
	fmt.Println("Pipeline: CoT (outline) → BestOfN (opening) → Refine (style) → Refine (CTA)")
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  ✓ ChainOfThought (structured reasoning)")
	fmt.Println("  ✓ Few-shot learning with ExampleSet")
	fmt.Println("  ✓ BestOfN with custom scorer + threshold + returnAll")
	fmt.Println("  ✓ Refine (iterative improvement)")
	fmt.Println("  ✓ Multi-turn refinement (progressive enhancement)")
	fmt.Println("  ✓ Event logging for each pipeline step")

	// Usage stats
	fmt.Printf("\n=== Usage Stats ===\n")
	fmt.Printf("Total Prompt Tokens: %d\n", totalPromptTokens)
	fmt.Printf("Total Completion Tokens: %d\n", totalCompletionTokens)
}
