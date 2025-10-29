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

	fmt.Println("=== AI Code Reviewer ===")
	fmt.Println("Demonstrates: Program pipeline for multi-stage code review")
	fmt.Println()

	// Example 1: Simple pipeline
	fmt.Println("--- Example 1: Code analysis pipeline ---")
	codeAnalysisPipeline()

	// Example 2: Comprehensive review
	fmt.Println("\n--- Example 2: Comprehensive code review ---")
	comprehensiveReview()
}

func codeAnalysisPipeline() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

	// Step 1: Analyze code structure
	structureSig := dsgo.NewSignature("Analyze code structure and complexity").
		AddInput("code", dsgo.FieldTypeString, "The code to analyze").
		AddOutput("structure_analysis", dsgo.FieldTypeString, "Structure analysis").
		AddOutput("complexity", dsgo.FieldTypeString, "Complexity assessment").
		AddOutput("maintainability_score", dsgo.FieldTypeFloat, "Maintainability score 0-1")

	structureModule := dsgo.NewPredict(structureSig, lm)

	// Step 2: Find issues
	issuesSig := dsgo.NewSignature("Identify code issues and improvements").
		AddInput("code", dsgo.FieldTypeString, "The code").
		AddInput("structure_analysis", dsgo.FieldTypeString, "Structure analysis").
		AddInput("complexity", dsgo.FieldTypeString, "Complexity").
		AddOutput("issues", dsgo.FieldTypeString, "List of issues found").
		AddOutput("suggestions", dsgo.FieldTypeString, "Improvement suggestions").
		AddClassOutput("severity", []string{"low", "medium", "high", "critical"}, "Overall severity")

	issuesModule := dsgo.NewPredict(issuesSig, lm)

	// Step 3: Generate recommendations
	recommendSig := dsgo.NewSignature("Generate actionable recommendations").
		AddInput("issues", dsgo.FieldTypeString, "Issues").
		AddInput("suggestions", dsgo.FieldTypeString, "Suggestions").
		AddInput("severity", dsgo.FieldTypeString, "Severity").
		AddOutput("recommendations", dsgo.FieldTypeJSON, "Prioritized recommendations as JSON array").
		AddOutput("refactoring_priority", dsgo.FieldTypeString, "What to refactor first")

	recommendModule := dsgo.NewChainOfThought(recommendSig, lm)

	// Create pipeline
	pipeline := dsgo.NewProgram("Code Review Pipeline").
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

	inputs := map[string]interface{}{
		"code": code,
	}

	outputs, err := pipeline.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Code to Review:\n%s\n", code)
	fmt.Printf("Structure Analysis: %s\n", outputs["structure_analysis"])
	fmt.Printf("Complexity: %s\n", outputs["complexity"])
	fmt.Printf("Maintainability Score: %.2f\n\n", outputs["maintainability_score"])
	fmt.Printf("Issues Found:\n%s\n\n", outputs["issues"])
	fmt.Printf("Severity: %s\n\n", outputs["severity"])
	fmt.Printf("Recommendations:\n%s\n", outputs["recommendations"])
	fmt.Printf("\nRefactoring Priority: %s\n", outputs["refactoring_priority"])
}

func comprehensiveReview() {
	ctx := context.Background()
	lm := examples.GetLM("gpt-4o-mini")

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

	review := dsgo.NewChainOfThought(reviewSig, lm)

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

	inputs := map[string]interface{}{
		"code":     code,
		"language": "JavaScript",
	}

	outputs, err := review.Forward(ctx, inputs)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Code Under Review:\n%s\n", code)
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("COMPREHENSIVE CODE REVIEW REPORT")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Printf("\n🔒 SECURITY ISSUES:\n%s\n", outputs["security_issues"])
	fmt.Printf("\n⚡ PERFORMANCE ISSUES:\n%s\n", outputs["performance_issues"])
	fmt.Printf("\n✅ BEST PRACTICES:\n%s\n", outputs["best_practices"])
	fmt.Printf("\n👃 CODE SMELLS:\n%s\n", outputs["code_smell"])
	fmt.Printf("\n📊 OVERALL QUALITY: %.2f/1.0\n", outputs["overall_quality"])
	fmt.Printf("\n📝 SUMMARY:\n%s\n", outputs["summary"])
	fmt.Println(strings.Repeat("=", 70))
}