package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared"
	"github.com/assagman/dsgo/examples/shared/_harness"
	"github.com/assagman/dsgo/module"
)

func main() {
	shared.LoadEnv()

	config, _ := harness.ParseFlags()
	h := harness.NewHarness(config)

	err := h.Run(context.Background(), "028_code_reviewer", runExample)
	if err != nil {
		log.Fatal(err)
	}

	if err := h.OutputResults(); err != nil {
		log.Fatal(err)
	}
}

func runExample(ctx context.Context) (*dsgo.Prediction, *harness.ExecutionStats, error) {
	stats := &harness.ExecutionStats{
		Metadata: make(map[string]any),
	}

	fmt.Println("=== AI Code Reviewer Example ===")
	fmt.Println("Demonstrates: Multi-Stage Code Review Pipeline with Program Composition")
	fmt.Println()

	// Example 1: Simple pipeline
	fmt.Println("--- Example 1: Code Analysis Pipeline ---")
	result1, err := codeAnalysisPipeline(ctx)
	if err != nil {
		return nil, stats, fmt.Errorf("code analysis pipeline failed: %w", err)
	}

	// Example 2: Comprehensive review
	fmt.Println("\n--- Example 2: Comprehensive Code Review ---")
	result2, err := comprehensiveReview(ctx)
	if err != nil {
		return nil, stats, fmt.Errorf("comprehensive review failed: %w", err)
	}

	// Use the comprehensive review as the main result
	stats.TokensUsed = result1.Usage.TotalTokens + result2.Usage.TotalTokens
	stats.Metadata["example1_maintainability"] = result1.Outputs["maintainability_score"]
	stats.Metadata["example1_severity"] = result1.Outputs["severity"]
	stats.Metadata["example2_quality"] = result2.Outputs["overall_quality"]
	stats.Metadata["total_examples"] = 2

	return result2, stats, nil
}

func codeAnalysisPipeline(ctx context.Context) (*dsgo.Prediction, error) {
	lm := shared.GetLM(shared.GetModel())

	// Step 1: Analyze code structure
	structureSig := dsgo.NewSignature("Analyze code structure and complexity").
		AddInput("code", dsgo.FieldTypeString, "The code to analyze").
		AddOutput("structure_analysis", dsgo.FieldTypeString, "Structure analysis").
		AddOutput("complexity", dsgo.FieldTypeString, "Complexity assessment").
		AddOutput("maintainability_score", dsgo.FieldTypeFloat, "Maintainability score 0-1")

	structureModule := module.NewPredict(structureSig, lm)

	// Step 2: Find issues
	issuesSig := dsgo.NewSignature("Identify code issues and improvements").
		AddInput("code", dsgo.FieldTypeString, "The code").
		AddInput("structure_analysis", dsgo.FieldTypeString, "Structure analysis").
		AddInput("complexity", dsgo.FieldTypeString, "Complexity").
		AddOutput("issues", dsgo.FieldTypeString, "List of issues found").
		AddOutput("suggestions", dsgo.FieldTypeString, "Improvement suggestions").
		AddClassOutput("severity", []string{"low", "medium", "high", "critical"}, "Overall severity")

	issuesModule := module.NewPredict(issuesSig, lm)

	// Step 3: Generate recommendations
	recommendSig := dsgo.NewSignature("Generate actionable recommendations").
		AddInput("issues", dsgo.FieldTypeString, "Issues").
		AddInput("suggestions", dsgo.FieldTypeString, "Suggestions").
		AddInput("severity", dsgo.FieldTypeString, "Severity").
		AddOutput("recommendations", dsgo.FieldTypeJSON, "Prioritized recommendations as JSON array").
		AddOutput("refactoring_priority", dsgo.FieldTypeString, "What to refactor first")

	recommendModule := module.NewChainOfThought(recommendSig, lm)

	// Create pipeline
	pipeline := module.NewProgram("Code Review Pipeline").
		AddModule(structureModule).
		AddModule(issuesModule).
		AddModule(recommendModule)

	code := `
func processData(data []int) []int {
    result := []int{}
    for i := 0; i < len(data); i++ {
        if data[i] > 0 {
            result = append(result, data[i] * 2)
        }
    }
    return result
}
`

	inputs := map[string]any{
		"code": code,
	}

	outputs, err := pipeline.Forward(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("pipeline forward failed: %w", err)
	}

	fmt.Printf("Code to Review:\n%s\n", code)
	fmt.Printf("Structure Analysis: %s\n", outputs.Outputs["structure_analysis"])
	fmt.Printf("Complexity: %s\n", outputs.Outputs["complexity"])
	fmt.Printf("Maintainability Score: %.2f\n\n", outputs.Outputs["maintainability_score"])
	fmt.Printf("Issues Found:\n%s\n\n", outputs.Outputs["issues"])
	fmt.Printf("Severity: %s\n\n", outputs.Outputs["severity"])
	fmt.Printf("Recommendations:\n%s\n", outputs.Outputs["recommendations"])
	fmt.Printf("\nRefactoring Priority: %s\n", outputs.Outputs["refactoring_priority"])

	return outputs, nil
}

func comprehensiveReview(ctx context.Context) (*dsgo.Prediction, error) {
	lm := shared.GetLM(shared.GetModel())

	// Multi-aspect review signature
	reviewSig := dsgo.NewSignature("Perform comprehensive code review").
		AddInput("code", dsgo.FieldTypeString, "Code to review").
		AddInput("language", dsgo.FieldTypeString, "Programming language").
		AddOutput("security_issues", dsgo.FieldTypeString, "Security concerns").
		AddOutput("performance_issues", dsgo.FieldTypeString, "Performance concerns").
		AddOutput("best_practices", dsgo.FieldTypeString, "Best practice violations").
		AddOutput("code_smell", dsgo.FieldTypeString, "Code smells detected").
		AddOutput("overall_quality", dsgo.FieldTypeFloat, "Overall quality score 0-1").
		AddOutput("summary", dsgo.FieldTypeString, "Executive summary")

	review := module.NewChainOfThought(reviewSig, lm)

	code := `
function authenticateUser(username, password) {
    var query = "SELECT * FROM users WHERE username='" + username +
                "' AND password='" + password + "'";
    var result = db.execute(query);
    if (result.length > 0) {
        return result[0];
    }
    return null;
}
`

	inputs := map[string]any{
		"code":     code,
		"language": "JavaScript",
	}

	outputs, err := review.Forward(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("review forward failed: %w", err)
	}

	fmt.Printf("Code Under Review:\n%s\n", code)
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("COMPREHENSIVE CODE REVIEW REPORT")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Printf("\n🔒 SECURITY ISSUES:\n%s\n", outputs.Outputs["security_issues"])
	fmt.Printf("\n⚡ PERFORMANCE ISSUES:\n%s\n", outputs.Outputs["performance_issues"])
	fmt.Printf("\n✅ BEST PRACTICES:\n%s\n", outputs.Outputs["best_practices"])
	fmt.Printf("\n👃 CODE SMELLS:\n%s\n", outputs.Outputs["code_smell"])
	fmt.Printf("\n📊 OVERALL QUALITY: %.2f/1.0\n", outputs.Outputs["overall_quality"])
	fmt.Printf("\n📝 SUMMARY:\n%s\n", outputs.Outputs["summary"])
	fmt.Println(strings.Repeat("=", 70))

	return outputs, nil
}
