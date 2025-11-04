package main

import (
	"context"
	"fmt"
	"log"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	lm := shared.GetLM(shared.GetModel())

	fmt.Println("=== Refine Module Example: Iterative Improvement ===")

	// Example 1: Basic refine - improve a summary
	basicRefine(lm)

	fmt.Println("\n" + string(make([]rune, 80)) + "\n")

	// Example 2: Refine with custom refinement field
	refineWithCustomField(lm)

	fmt.Println("\n" + string(make([]rune, 80)) + "\n")

	// Example 3: Refine with quality constraints
	refineWithConstraints(lm)
}

func basicRefine(lm dsgo.LM) {
	fmt.Println("Example 1: Basic Iterative Refinement")
	fmt.Println("======================================")

	sig := dsgo.NewSignature("Improve the given text based on feedback.").
		AddInput("text", dsgo.FieldTypeString, "Text to improve").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback for improvement").
		AddOutput("improved_text", dsgo.FieldTypeString, "Improved version of the text").
		AddOutput("changes_made", dsgo.FieldTypeString, "Summary of improvements")

	refine := module.NewRefine(sig, lm).WithMaxIterations(3)

	ctx := context.Background()

	originalText := `The product is good. It works fine. The price is okay.`

	fmt.Printf("📝 Original text:\n%s\n\n", originalText)

	result, err := refine.Forward(ctx, map[string]interface{}{
		"text":     originalText,
		"feedback": "Make it more engaging, professional, and detailed. Add specific benefits.",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✨ Refined text:\n%s\n\n", result.Outputs["improved_text"])
	fmt.Printf("📊 Changes made:\n%s\n", result.Outputs["changes_made"])
}

func refineWithCustomField(lm dsgo.LM) {
	fmt.Println("Example 2: Refine with Custom Refinement Field")
	fmt.Println("===============================================")

	sig := dsgo.NewSignature("Write a compelling product description.").
		AddInput("product_name", dsgo.FieldTypeString, "Name of the product").
		AddInput("key_features", dsgo.FieldTypeString, "Key features").
		AddInput("improvement_notes", dsgo.FieldTypeString, "Notes on how to improve").
		AddOutput("description", dsgo.FieldTypeString, "Product description").
		AddOutput("tone", dsgo.FieldTypeString, "Tone of the description").
		AddOutput("word_count", dsgo.FieldTypeInt, "Word count")

	// Use "improvement_notes" as the refinement field
	refine := module.NewRefine(sig, lm).
		WithMaxIterations(2).
		WithRefinementField("improvement_notes")

	ctx := context.Background()

	fmt.Println("🎯 Product: Smart Coffee Maker")
	fmt.Println("Features: WiFi enabled, voice control, programmable brewing")

	result, err := refine.Forward(ctx, map[string]interface{}{
		"product_name":       "Smart Coffee Maker Pro",
		"key_features":       "WiFi enabled, voice control, programmable brewing, self-cleaning",
		"improvement_notes": "Make it more emotional and highlight the lifestyle benefits. Use power words.",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n✨ Final Description:\n%s\n", result.Outputs["description"])
	fmt.Printf("\n📊 Tone: %s\n", result.Outputs["tone"])
	fmt.Printf("📊 Word Count: %d\n", result.Outputs["word_count"])
}

func refineWithConstraints(lm dsgo.LM) {
	fmt.Println("Example 3: Refine with Quality Constraints")
	fmt.Println("===========================================")

	sig := dsgo.NewSignature("Write a concise, impactful email subject line.").
		AddInput("email_topic", dsgo.FieldTypeString, "Topic of the email").
		AddInput("target_audience", dsgo.FieldTypeString, "Target audience").
		AddInput("feedback", dsgo.FieldTypeString, "Feedback for refinement").
		AddOutput("subject_line", dsgo.FieldTypeString, "Email subject line (max 60 chars)").
		AddOutput("character_count", dsgo.FieldTypeInt, "Character count").
		AddOutput("impact_score", dsgo.FieldTypeFloat, "Estimated impact score (0-1)")

	refine := module.NewRefine(sig, lm).WithMaxIterations(4)

	ctx := context.Background()

	fmt.Println("🎯 Email Topic: Product Launch Announcement")
	fmt.Println("🎯 Audience: Existing customers")

	result, err := refine.Forward(ctx, map[string]interface{}{
		"email_topic":     "New AI-powered feature launch for our analytics platform",
		"target_audience": "Tech-savvy business professionals and data analysts",
		"feedback":        "Keep under 60 characters, create urgency, be specific about the AI feature, use power words",
	})
	if err != nil {
		log.Fatal(err)
	}

	subjectLine := result.Outputs["subject_line"].(string)
	charCount := result.Outputs["character_count"].(int)
	impactScore := result.Outputs["impact_score"].(float64)

	fmt.Printf("\n✨ Final Subject Line:\n\"%s\"\n", subjectLine)
	fmt.Printf("\n📊 Character Count: %d/60\n", charCount)
	fmt.Printf("📊 Impact Score: %.2f/1.0\n", impactScore)

	if charCount <= 60 {
		fmt.Println("✅ Constraint met: Under 60 characters!")
	} else {
		fmt.Println("⚠️  Warning: Exceeds 60 character limit")
	}
}
