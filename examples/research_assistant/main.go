package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/module"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}
	fmt.Println("=== Advanced Research Assistant Example ===")
	fmt.Println("Demonstrates: Custom Signatures + Tools + ReAct Reasoning")
	fmt.Println()

	researchAssistant()
}

func researchAssistant() {
	// Create a complex signature with multiple input/output types
	sig := dsgo.NewSignature("Research a topic and provide comprehensive analysis").
		// Multiple inputs with different types
		AddInput("topic", dsgo.FieldTypeString, "The main research topic").
		AddInput("focus_areas", dsgo.FieldTypeString, "Specific aspects to focus on (comma-separated)").
		AddInput("depth_level", dsgo.FieldTypeInt, "Research depth: 1=basic, 2=intermediate, 3=deep").
		AddInput("include_statistics", dsgo.FieldTypeBool, "Whether to include statistical data").
		// Multiple outputs with different types and constraints
		AddOutput("summary", dsgo.FieldTypeString, "Executive summary of findings").
		AddOutput("key_findings", dsgo.FieldTypeString, "Bullet-pointed key discoveries").
		AddClassOutput("confidence_level", []string{"high", "medium", "low"}, "Confidence in the research").
		AddOutput("sources_consulted", dsgo.FieldTypeInt, "Number of sources checked").
		AddOptionalOutput("statistics", dsgo.FieldTypeString, "Statistical data if requested").
		AddOutput("recommendations", dsgo.FieldTypeString, "Action items or next steps").
		AddClassOutput("research_quality", []string{"excellent", "good", "fair", "limited"}, "Quality assessment")

	// Define research tools
	tools := []dsgo.Tool{
		*createSearchTool(),
		*createStatisticsTool(),
		*createFactCheckerTool(),
		*createDateTool(),
	}

	// Create LM (auto-detects provider from environment)
	lm := shared.GetLM("gpt-4")

	// Create ReAct module for intelligent research
	react := module.NewReAct(sig, lm, tools).
		WithMaxIterations(15).
		WithVerbose(true)

	// Execute with complex inputs
	ctx := context.Background()
	inputs := map[string]any{
		"topic":              "Impact of AI on software development productivity",
		"focus_areas":        "code generation, testing automation, developer experience",
		"depth_level":        2,
		"include_statistics": true,
	}

	fmt.Println("📋 Research Request:")
	fmt.Printf("  Topic: %s\n", inputs["topic"])
	fmt.Printf("  Focus Areas: %s\n", inputs["focus_areas"])
	fmt.Printf("  Depth Level: %d\n", inputs["depth_level"])
	fmt.Printf("  Include Statistics: %v\n\n", inputs["include_statistics"])

	outputs, err := react.Forward(ctx, inputs)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Display results
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("📊 RESEARCH RESULTS")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Println("\n📝 SUMMARY:")
	fmt.Println(outputs["summary"])

	fmt.Println("\n🔍 KEY FINDINGS:")
	fmt.Println(outputs["key_findings"])

	if stats, ok := outputs["statistics"]; ok && stats != nil {
		fmt.Println("\n📈 STATISTICS:")
		fmt.Println(stats)
	}

	fmt.Println("\n💡 RECOMMENDATIONS:")
	fmt.Println(outputs["recommendations"])

	fmt.Println("\n📊 METADATA:")
	fmt.Printf("  Confidence Level: %v\n", outputs["confidence_level"])
	fmt.Printf("  Research Quality: %v\n", outputs["research_quality"])
	fmt.Printf("  Sources Consulted: %v\n", outputs["sources_consulted"])

	fmt.Println("\n" + strings.Repeat("=", 70))
}

// createSearchTool simulates web search for research
func createSearchTool() *dsgo.Tool {
	return dsgo.NewTool(
		"search",
		"Search for information on a specific topic or question",
		func(ctx context.Context, args map[string]any) (any, error) {
			query := args["query"].(string)

			// Simulate search results based on query
			if strings.Contains(strings.ToLower(query), "ai") && strings.Contains(strings.ToLower(query), "productivity") {
				return `Search Results: Multiple studies show AI tools like GitHub Copilot and ChatGPT have increased developer productivity by 20-55%. Developers report faster code completion, reduced time on boilerplate code, and improved ability to learn new technologies. However, concerns exist about code quality and over-reliance on AI suggestions.`, nil
			}

			if strings.Contains(strings.ToLower(query), "code generation") {
				return `Search Results: AI code generation tools can complete up to 40% of code automatically. Most effective for routine tasks, API integrations, and boilerplate code. Developers still need to review and refine AI-generated code for production use.`, nil
			}

			if strings.Contains(strings.ToLower(query), "testing") || strings.Contains(strings.ToLower(query), "automation") {
				return `Search Results: AI-powered testing tools can generate test cases automatically, identify edge cases, and predict bugs. Early adopters report 30-50% reduction in testing time and improved test coverage.`, nil
			}

			if strings.Contains(strings.ToLower(query), "developer experience") {
				return `Search Results: Surveys indicate 70% of developers who use AI tools report improved job satisfaction. AI handles repetitive tasks, allowing developers to focus on creative problem-solving. Learning curve exists but most adapt within 2-4 weeks.`, nil
			}

			return fmt.Sprintf("Search results for: %s - General information about software development and AI.", query), nil
		},
	).AddParameter("query", "string", "The search query", true)
}

// createStatisticsTool provides statistical data
func createStatisticsTool() *dsgo.Tool {
	return dsgo.NewTool(
		"get_statistics",
		"Retrieve statistical data about a specific metric or study",
		func(ctx context.Context, args map[string]any) (any, error) {
			metric := args["metric"].(string)

			stats := map[string]string{
				"productivity_increase": "Studies show 20-55% productivity increase among developers using AI coding assistants",
				"adoption_rate":         "As of 2024, 46% of professional developers use AI coding tools regularly",
				"time_savings":          "Developers save average of 8-12 hours per week using AI assistance",
				"code_quality":          "92% of AI-generated code requires review; 15-20% needs significant modifications",
				"learning_time":         "Average onboarding time for AI tools: 2-4 weeks to proficiency",
				"satisfaction":          "70% of developers report improved job satisfaction with AI tools",
			}

			for key, value := range stats {
				if strings.Contains(strings.ToLower(metric), key) || strings.Contains(key, strings.ToLower(metric)) {
					return value, nil
				}
			}

			return fmt.Sprintf("Statistical data for %s: Limited data available, estimate based on industry trends", metric), nil
		},
	).AddParameter("metric", "string", "The specific metric to retrieve statistics for", true)
}

// createFactCheckerTool validates claims
func createFactCheckerTool() *dsgo.Tool {
	return dsgo.NewTool(
		"fact_check",
		"Verify a claim or statement for accuracy",
		func(ctx context.Context, args map[string]any) (any, error) {
			claim := args["claim"].(string)

			// Simulate fact checking
			if strings.Contains(strings.ToLower(claim), "productivity") {
				return "VERIFIED: Multiple peer-reviewed studies from 2023-2024 confirm productivity improvements with AI coding tools. Confidence: High", nil
			}

			if strings.Contains(strings.ToLower(claim), "github copilot") {
				return "VERIFIED: GitHub's internal study (2023) showed significant productivity gains. External studies confirm findings. Confidence: High", nil
			}

			return fmt.Sprintf("Fact check for '%s': Plausible based on available information. Recommend additional verification. Confidence: Medium", claim), nil
		},
	).AddParameter("claim", "string", "The claim or statement to verify", true)
}

// createDateTool provides current date context
func createDateTool() *dsgo.Tool {
	return dsgo.NewTool(
		"get_current_date",
		"Get the current date and time for temporal context",
		func(ctx context.Context, args map[string]any) (any, error) {
			now := time.Now()
			return fmt.Sprintf("Current date: %s (Studies from 2024 are most recent)", now.Format("January 2, 2006")), nil
		},
	)
}
