package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	fmt.Println("=== AI Content Generator ===")
	fmt.Println("Demonstrates: BestOfN with custom scoring functions")
	fmt.Println()

	// Example 1: Blog post titles
	fmt.Println("--- Example 1: Generate best blog title ---")
	generateBlogTitle()

	// Example 2: Product descriptions
	fmt.Println("\n--- Example 2: Generate product descriptions ---")
	generateProductDescription()

	// Example 3: Social media posts
	fmt.Println("\n--- Example 3: Generate social media content ---")
	generateSocialMedia()
}

func generateBlogTitle() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4")

	sig := dsgo.NewSignature("Generate an engaging blog post title").
		AddInput("topic", dsgo.FieldTypeString, "Blog topic").
		AddInput("target_audience", dsgo.FieldTypeString, "Target audience").
		AddOutput("title", dsgo.FieldTypeString, "Blog title").
		AddOutput("hook_strength", dsgo.FieldTypeFloat, "How strong the hook is 0-1").
		AddOutput("seo_score", dsgo.FieldTypeFloat, "SEO friendliness 0-1").
		AddOutput("creativity", dsgo.FieldTypeFloat, "Creativity score 0-1")

	predict := dsgo.NewPredict(sig, lm)

	// Custom scorer: balanced scoring across all dimensions
	customScorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
		hook, ok1 := outputs["hook_strength"].(float64)
		seo, ok2 := outputs["seo_score"].(float64)
		creativity, ok3 := outputs["creativity"].(float64)

		if !ok1 || !ok2 || !ok3 {
			return 0.5, nil
		}

		// Weighted: 40% hook, 30% SEO, 30% creativity
		score := (hook * 0.4) + (seo * 0.3) + (creativity * 0.3)
		return score, nil
	}

	bestOf := dsgo.NewBestOfN(predict, 4).
		WithScorer(customScorer).
		WithReturnAll(true).
		WithParallel(false)

	inputs := map[string]interface{}{
		"topic":            "artificial intelligence in healthcare",
		"target_audience":  "healthcare professionals and tech enthusiasts",
	}

	outputs, err := bestOf.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Topic: %s\n", inputs["topic"])
	fmt.Printf("Audience: %s\n\n", inputs["target_audience"])
	fmt.Printf("🏆 WINNING TITLE:\n%s\n\n", outputs["title"])
	fmt.Printf("Scores:\n")
	fmt.Printf("  Hook Strength: %.2f\n", outputs["hook_strength"])
	fmt.Printf("  SEO Score: %.2f\n", outputs["seo_score"])
	fmt.Printf("  Creativity: %.2f\n", outputs["creativity"])
	fmt.Printf("  Overall Score: %.3f\n", outputs["_best_of_n_score"])

	if allScores, ok := outputs["_best_of_n_all_scores"].([]float64); ok {
		fmt.Printf("\nAll Candidate Scores: ")
		for i, score := range allScores {
			fmt.Printf("%.3f", score)
			if i < len(allScores)-1 {
				fmt.Printf(", ")
			}
		}
		fmt.Println()
	}
}

func generateProductDescription() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4")

	sig := dsgo.NewSignature("Create compelling product description").
		AddInput("product_name", dsgo.FieldTypeString, "Product name").
		AddInput("key_features", dsgo.FieldTypeString, "Key features").
		AddInput("tone", dsgo.FieldTypeString, "Desired tone").
		AddOutput("description", dsgo.FieldTypeString, "Product description").
		AddOutput("persuasiveness", dsgo.FieldTypeFloat, "How persuasive 0-1").
		AddOutput("clarity", dsgo.FieldTypeFloat, "How clear 0-1")

	predict := dsgo.NewPredict(sig, lm)

	// Length-aware quality scorer
	qualityScorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
		description, ok := outputs["description"].(string)
		if !ok {
			return 0, fmt.Errorf("description not found")
		}

		persuasiveness, ok1 := outputs["persuasiveness"].(float64)
		clarity, ok2 := outputs["clarity"].(float64)

		if !ok1 || !ok2 {
			return 0.5, nil
		}

		// Word count scoring (prefer 50-150 words)
		wordCount := len(strings.Fields(description))
		lengthScore := 1.0
		if wordCount < 50 {
			lengthScore = float64(wordCount) / 50.0
		} else if wordCount > 150 {
			lengthScore = 1.0 - ((float64(wordCount-150) / 100.0))
			if lengthScore < 0 {
				lengthScore = 0
			}
		}

		// Combined: 40% persuasiveness, 40% clarity, 20% length
		score := (persuasiveness * 0.4) + (clarity * 0.4) + (lengthScore * 0.2)
		return score, nil
	}

	bestOf := dsgo.NewBestOfN(predict, 3).
		WithScorer(qualityScorer).
		WithReturnAll(true)

	inputs := map[string]interface{}{
		"product_name":  "EcoBottle Pro",
		"key_features":  "insulated, keeps drinks cold 24h/hot 12h, made from recycled materials, leak-proof",
		"tone":          "eco-conscious and premium",
	}

	outputs, err := bestOf.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	description := outputs["description"].(string)
	wordCount := len(strings.Fields(description))

	fmt.Printf("Product: %s\n", inputs["product_name"])
	fmt.Printf("Features: %s\n\n", inputs["key_features"])
	fmt.Printf("📝 BEST DESCRIPTION (%d words):\n%s\n\n", wordCount, description)
	fmt.Printf("Persuasiveness: %.2f\n", outputs["persuasiveness"])
	fmt.Printf("Clarity: %.2f\n", outputs["clarity"])
	fmt.Printf("Overall Score: %.3f\n", outputs["_best_of_n_score"])
}

func generateSocialMedia() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4")

	sig := dsgo.NewSignature("Create social media post").
		AddInput("message", dsgo.FieldTypeString, "Core message").
		AddInput("platform", dsgo.FieldTypeString, "Platform (twitter/linkedin/instagram)").
		AddInput("hashtags_count", dsgo.FieldTypeInt, "Number of hashtags").
		AddOutput("post", dsgo.FieldTypeString, "Social media post").
		AddOutput("engagement_potential", dsgo.FieldTypeFloat, "Engagement potential 0-1").
		AddOutput("character_count", dsgo.FieldTypeInt, "Character count")

	predict := dsgo.NewPredict(sig, lm)

	// Platform-specific scorer
	platformScorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
		platform := inputs["platform"].(string)
		post := outputs["post"].(string)
		engagement, ok1 := outputs["engagement_potential"].(float64)
		charCount, ok2 := outputs["character_count"]

		if !ok1 || !ok2 {
			return 0.5, nil
		}

		// Convert character count to int
		var count int
		switch v := charCount.(type) {
		case int:
			count = v
		case float64:
			count = int(v)
		default:
			count = len(post)
		}

		// Platform-specific length scoring
		lengthScore := 1.0
		switch platform {
		case "twitter":
			if count > 280 {
				lengthScore = 0.5
			}
		case "linkedin":
			if count < 100 || count > 1300 {
				lengthScore = 0.7
			}
		case "instagram":
			if count > 2200 {
				lengthScore = 0.6
			}
		}

		// 70% engagement, 30% length appropriateness
		score := (engagement * 0.7) + (lengthScore * 0.3)
		return score, nil
	}

	bestOf := dsgo.NewBestOfN(predict, 3).
		WithScorer(platformScorer).
		WithReturnAll(true)

	inputs := map[string]interface{}{
		"message":        "Launching our new AI-powered productivity tool that helps teams collaborate better",
		"platform":       "twitter",
		"hashtags_count": 3,
	}

	outputs, err := bestOf.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Message: %s\n", inputs["message"])
	fmt.Printf("Platform: %s\n\n", inputs["platform"])
	fmt.Printf("🐦 BEST POST:\n%s\n\n", outputs["post"])
	fmt.Printf("Character Count: %v\n", outputs["character_count"])
	fmt.Printf("Engagement Potential: %.2f\n", outputs["engagement_potential"])
	fmt.Printf("Overall Score: %.3f\n", outputs["_best_of_n_score"])
}
