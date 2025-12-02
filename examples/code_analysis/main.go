package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

// Constants for field names and packages to avoid hardcoding strings
const (
	// Package targets
	PkgRetry    = "internal/retry"
	PkgJsonUtil = "internal/jsonutil"
	PkgTyped    = "internal/typed"

	// Input/Output Fields
	FieldPkgName     = "package_name"
	FieldCodeContent = "code_content"
	FieldAnalysis    = "analysis"
	FieldKeyFindings = "key_findings"
	FieldCombined    = "combined_analyses"
	FieldExecSummary = "executive_summary"
	FieldArchDiagram = "architecture_diagram"
	FieldImprovement = "improvement_suggestions"
	FieldTask        = "task"
	FieldPlan        = "plan"
	FieldContext     = "context"
	FieldNextSteps   = "next_steps"

	// Model
	ModelName = "openrouter/deepseek/deepseek-v3.2"
)

func main() {
	ctx := context.Background()

	// Initialize LM
	lm, err := dsgo.NewLM(ctx, ModelName)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}

	// Target packages to analyze
	packages := []string{PkgRetry, PkgJsonUtil, PkgTyped}

	// Convert to space-separated string for the task
	packagesStr := strings.Join(packages, ", ")

	fmt.Println("🚀 Starting Tool-Based Code Analysis...")

	// Create filesystem tools for use by the agent
	fmt.Println("\n🛠️  Setting up filesystem tools...")
	fsTools := createFSTools(projectRoot)

	// Define signature for analysis plan
	planSig := dsgo.NewSignature("Generate analysis plan").
		AddInput(FieldTask, dsgo.FieldTypeString, "The analysis task to plan for").
		AddOutput("strategy", dsgo.FieldTypeString, "Step-by-step strategy for analysis").
		AddOutput("target_directories", "array", "List of directories to focus on")

	// Create planner that decides what to do
	planner := dsgo.NewPredict(planSig, lm)

	// Generate analysis plan using the planner
	fmt.Println("\n📋 Generating analysis plan...")
	plan, err := planner.Forward(ctx, map[string]any{
		FieldTask: fmt.Sprintf("Analyze the architecture of the %s packages", packagesStr),
	})
	if err != nil {
		log.Fatalf("Failed to generate plan: %v", err)
	}

	fmt.Printf("\n%s\n", getStringOrZero(plan, "strategy"))

	// Define signature for analysis execution
	analysisSig := dsgo.NewSignature("Perform code analysis").
		AddInput(FieldTask, dsgo.FieldTypeString, "The original analysis request").
		AddInput(FieldPlan, dsgo.FieldTypeString, "The step-by-step analysis strategy").
		AddInput(FieldContext, dsgo.FieldTypeString, "Any relevant context gathered so far").
		AddOutput(FieldAnalysis, dsgo.FieldTypeString, "Detailed analysis of the codebase").
		AddOutput(FieldKeyFindings, dsgo.FieldTypeString, "Key architectural findings or patterns").
		AddOutput(FieldNextSteps, dsgo.FieldTypeString, "Suggested next steps or files to examine")

	// Create a ReAct agent that will use tools to explore the codebase
	fmt.Println("\n🤖 Starting analysis with ReAct agent...")
	react := dsgo.NewReAct(analysisSig, lm, fsTools).WithVerbose(true).WithMaxIterations(10)

	// Execute the analysis using the plan and tools
	result, err := react.Forward(ctx, map[string]any{
		FieldTask:    fmt.Sprintf("Analyze the architecture of the %s packages", packagesStr),
		FieldPlan:    getStringOrZero(plan, "strategy"),
		FieldContext: "Starting analysis. Use tools to explore the codebase as needed.",
	})

	if err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}

	// Extract the completed analysis from the result
	analysis, ok := result.GetString(FieldAnalysis)
	if !ok {
		analysis = "No analysis available"
	}

	findings, ok := result.GetString(FieldKeyFindings)
	if !ok {
		findings = "No key findings available"
	}

	// Format this as package analysis format for the synthesis stage
	combinedAnalyses := fmt.Sprintf("=== Package: Dynamic Analysis ===\nAnalysis:\n%s\nKey Findings:\n%s\n\n", analysis, findings)

	// Stage 2: Synthesis with BestOfN using the gathered analysis
	fmt.Println("\n🔗 Stage: Synthesizing results with BestOfN (ChainOfThought)...")

	synthesisSig := dsgo.NewSignature("Synthesize architecture report").
		AddInput(FieldCombined, dsgo.FieldTypeString, "Analyses of individual packages").
		AddOutput(FieldExecSummary, dsgo.FieldTypeString, "High-level summary of the subsystem").
		AddOutput(FieldArchDiagram, dsgo.FieldTypeString, "Text description of how these packages relate").
		AddOutput(FieldImprovement, dsgo.FieldTypeString, "Suggestions for improvement")

	// Create base module
	baseSynthesizer := dsgo.NewChainOfThought(synthesisSig, lm)

	// Create BestOfN module
	// We generate 3 candidates and pick the best one based on our scorer
	scorer := func(inputs map[string]any, prediction *dsgo.Prediction) (float64, error) {
		score := 0.0

		// Check for presence of key fields
		if val, ok := prediction.GetString(FieldExecSummary); ok && len(val) > 50 {
			score += 1.0
		}
		if val, ok := prediction.GetString(FieldArchDiagram); ok && len(val) > 50 {
			score += 1.0
		}

		// Give more weight to detailed improvement suggestions
		if val, ok := prediction.GetString(FieldImprovement); ok {
			// Score based on length (longer is usually more detailed)
			score += float64(len(val)) / 100.0
		}

		return score, nil
	}

	bestOfN := dsgo.NewBestOfN(baseSynthesizer, 3).
		WithScorer(scorer).
		WithParallel(true) // Run 3 attempts in parallel

	synthesisCtx, cancelSyn := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelSyn()

	fmt.Println("  - Generating 3 candidate reports in parallel...")
	finalResult, err := bestOfN.Forward(synthesisCtx, map[string]any{
		FieldCombined: combinedAnalyses,
	})
	if err != nil {
		log.Fatalf("Synthesis failed: %v", err)
	}

	fmt.Printf("  ✅ Best candidate selected with score: %.2f\n", finalResult.Score)

	// Output results
	fmt.Println("\n📊 FINAL ARCHITECTURE REPORT")
	fmt.Println("===========================")
	fmt.Printf("\n📝 Executive Summary:\n%s\n", getStringOrZero(finalResult, FieldExecSummary))
	fmt.Printf("\n🏗️  Architecture Overview:\n%s\n", getStringOrZero(finalResult, FieldArchDiagram))
	fmt.Printf("\n💡 Improvement Suggestions:\n%s\n", getStringOrZero(finalResult, FieldImprovement))

	// Save report
	saveReport(combinedAnalyses, finalResult)
}

// Create filesystem tools with project root context
func createFSTools(projectRoot string) []dsgo.Tool {
	// ListFiles lists directory contents recursively up to a specified depth
	listFiles := func(ctx context.Context, args map[string]any) (any, error) {
		fmt.Printf("🛠️  Tool Call: list_files %v\n", args)
		directory, ok := args["directory"].(string)
		if !ok {
			// Default to project root if no directory specified
			directory = projectRoot
		} else {
			// If relative path provided, make it absolute from project root
			if !filepath.IsAbs(directory) {
				directory = filepath.Join(projectRoot, directory)
			}
		}

		depthVal, ok := args["depth"].(float64)
		if !ok {
			depthVal = 3 // default depth
		}
		depth := int(depthVal)

		var files []string
		err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Calculate relative depth
			relPath, err := filepath.Rel(projectRoot, path)
			if err != nil {
				return err
			}
			currentDepth := len(strings.Split(relPath, string(os.PathSeparator)))

			if currentDepth > depth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking directory: %w", err)
		}

		return map[string]interface{}{
			"files":     files,
			"directory": directory,
		}, nil
	}

	// ReadFile reads the content of a specific file
	readFile := func(ctx context.Context, args map[string]any) (any, error) {
		fmt.Printf("🛠️  Tool Call: read_file %v\n", args)
		filepathArg, ok := args["filepath"].(string)
		if !ok {
			return nil, fmt.Errorf("filepath parameter is required")
		}

		// If relative path provided, make it absolute from project root
		if !filepath.IsAbs(filepathArg) {
			filepathArg = filepath.Join(projectRoot, filepathArg)
		}

		content, err := os.ReadFile(filepathArg)
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		// Limit content size to prevent overwhelming the LM
		contentStr := string(content)
		if len(contentStr) > 10000 { // 10KB limit
			contentStr = contentStr[:10000] + "\n... [content truncated]"
		}

		return map[string]interface{}{
			"content":  contentStr,
			"filepath": filepathArg,
		}, nil
	}

	// SearchFiles searches for files matching a glob pattern
	searchFiles := func(ctx context.Context, args map[string]any) (any, error) {
		fmt.Printf("🛠️  Tool Call: search_files %v\n", args)
		directory, ok := args["directory"].(string)
		if !ok {
			directory = projectRoot
		} else {
			if !filepath.IsAbs(directory) {
				directory = filepath.Join(projectRoot, directory)
			}
		}

		pattern, ok := args["pattern"].(string)
		if !ok {
			return nil, fmt.Errorf("pattern parameter is required")
		}

		pattern = filepath.Join(directory, pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("error searching files: %w", err)
		}

		// Convert to relative paths for cleaner output
		relativeMatches := make([]string, len(matches))
		for i, match := range matches {
			relativeMatches[i], _ = filepath.Rel(projectRoot, match)
		}

		return map[string]interface{}{
			"files":     relativeMatches,
			"directory": directory,
			"pattern":   pattern,
		}, nil
	}

	// Tool for listing directory contents
	listFilesTool := dsgo.NewTool("list_files", "List files and directories in a given path up to a specified depth", listFiles).
		AddParameter("directory", "string", "The directory path to list (relative to project root, defaults to project root)", false).
		AddParameter("depth", "int", "Maximum depth to traverse (default: 3)", false)

	// Tool for reading file content
	readFileTool := dsgo.NewTool("read_file", "Read the content of a specific file", readFile).
		AddParameter("filepath", "string", "The path to the file to read (relative to project root)", true)

	// Tool for searching files by glob pattern
	searchFilesTool := dsgo.NewTool("search_files", "Search for files matching a glob pattern", searchFiles).
		AddParameter("directory", "string", "The directory to search in (relative to project root, defaults to project root)", false).
		AddParameter("pattern", "string", "Glob pattern to match (e.g., *.go, **/*.txt)", true)

	return []dsgo.Tool{*listFilesTool, *readFileTool, *searchFilesTool}
}

func getStringOrZero(prediction *dsgo.Prediction, key string) string {
	if val, ok := prediction.GetString(key); ok {
		return val
	}
	return ""
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func saveReport(rawAnalyses string, finalResult *dsgo.Prediction) {
	filename := fmt.Sprintf("architecture_report_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create report file: %v", err)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("Failed to close report file: %v", closeErr)
		}
	}()

	if _, err := fmt.Fprintf(file, "ARCHITECTURE REPORT\n===================\n\n"); err != nil {
		log.Printf("Failed to write to report file: %v", err)
		return
	}
	if _, err := fmt.Fprintf(file, "Executive Summary:\n%s\n\n", getStringOrZero(finalResult, FieldExecSummary)); err != nil {
		log.Printf("Failed to write to report file: %v", err)
		return
	}
	if _, err := fmt.Fprintf(file, "Architecture Overview:\n%s\n\n", getStringOrZero(finalResult, FieldArchDiagram)); err != nil {
		log.Printf("Failed to write to report file: %v", err)
		return
	}
	if _, err := fmt.Fprintf(file, "Suggestions:\n%s\n\n", getStringOrZero(finalResult, FieldImprovement)); err != nil {
		log.Printf("Failed to write to report file: %v", err)
		return
	}
	if _, err := fmt.Fprintf(file, "===================\nRAW PACKAGE ANALYSES\n===================\n%s", rawAnalyses); err != nil {
		log.Printf("Failed to write to report file: %v", err)
		return
	}

	fmt.Printf("\n💾 Full report saved to: %s\n", filename)
}
