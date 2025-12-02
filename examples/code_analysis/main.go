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
	ModelName = "openrouter/amazon/nova-2-lite-v1"
)

func main() {
	ctx := context.Background()

	// Initialize LM with enhanced error reporting
	fmt.Println("🔧 Initializing Language Model...")
	lm, err := dsgo.NewLM(ctx, ModelName)
	if err != nil {
		log.Fatalf("❌ Failed to create LM: %v\n\n💡 This error typically means:\n   • Missing or invalid API key in environment variables\n   • Model name '%s' is not available\n   • Network connectivity issues to the LLM provider\n   • Rate limiting from the provider\n\nPlease check:\n   • OPENROUTER_API_KEY (or other provider API key) environment variable\n   • Internet connectivity\n   • Provider status page for any outages", err, ModelName)
	}
	fmt.Printf("✅ Successfully initialized LM: %s\n", ModelName)

	// Find project root
	fmt.Println("📂 Locating project root...")
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("❌ Failed to find project root: %v\n\n💡 This error typically means:\n   • No go.mod file found in current or parent directories\n   • Permission issues accessing parent directories", err)
	}
	fmt.Printf("✅ Project root located: %s\n", projectRoot)

	// Target packages to analyze
	packages := []string{PkgRetry, PkgJsonUtil, PkgTyped}

	// Convert to space-separated string for the task
	packagesStr := strings.Join(packages, ", ")
	fmt.Printf("📦 Packages to analyze: [%s]\n", packagesStr)

	fmt.Println("🚀 Starting Multi-Stage Tool-Based Code Analysis...")

	// Create filesystem tools for use by the agent
	fmt.Println("\n🛠️  Setting up filesystem tools...")
	fsTools := createFSTools(projectRoot)

	// Define signature for analysis plan
	fmt.Println("\n📋 Defining analysis plan signature...")
	planSig := dsgo.NewSignature("Generate comprehensive analysis plan").
		AddInput(FieldTask, dsgo.FieldTypeString, "The analysis task to plan for").
		AddOutput("strategy", dsgo.FieldTypeString, "Step-by-step strategy for comprehensive analysis").
		AddOutput("target_directories", "array", "List of directories to focus on").
		AddOutput("analysis_phases", "array", "List of analysis phases to execute")

	// Create planner that decides what to do
	planner := dsgo.NewPredict(planSig, lm)

	// Generate analysis plan using the planner
	fmt.Println("\n📝 Generating comprehensive analysis plan...")
	planStartTime := time.Now()
	plan, err := planner.Forward(ctx, map[string]any{
		FieldTask: fmt.Sprintf("Generate a comprehensive analysis plan for the %s packages that includes multiple phases: 1) Initial exploration and structure mapping, 2) Detailed code analysis, 3) Relationship and dependency mapping, 4) Pattern and architecture identification, 5) Quality assessment and improvement opportunities", packagesStr),
	})
	planDuration := time.Since(planStartTime)
	if err != nil {
		log.Fatalf("❌ Failed to generate analysis plan after %.2f seconds: %v\n\n💡 This error typically means:\n   • API key issues (check environment variables)\n   • Model connectivity problems\n   • Model rate limiting\n   • Network connectivity issues",
			planDuration.Seconds(), err)
	}
	fmt.Printf("✅ Analysis plan generated in %.2f seconds\n", planDuration.Seconds())
	fmt.Printf("\n🎯 Analysis Strategy:\n%s\n", getStringOrZero(plan, "strategy"))

	// Multi-stage analysis approach
	fmt.Println("\n🔄 Executing Multi-Stage Analysis...")

	// Stage 1: Structure and Exploration
	fmt.Println("\n🔍 STAGE 1: Structural Exploration")
	structureAnalysis, err := performStructuralAnalysis(ctx, lm, fsTools, packages, getStringOrZero(plan, "strategy"))
	if err != nil {
		log.Fatalf("❌ Structural analysis failed: %v", err)
	}

	// Stage 2: Detailed Code Analysis
	fmt.Println("\n🔍 STAGE 2: Detailed Code Analysis")
	detailedAnalysis, err := performDetailedAnalysis(ctx, lm, fsTools, packages, structureAnalysis)
	if err != nil {
		log.Fatalf("❌ Detailed analysis failed: %v", err)
	}

	// Stage 3: Relationship and Dependency Mapping
	fmt.Println("\n🔍 STAGE 3: Relationship and Dependency Mapping")
	relationshipAnalysis, err := performRelationshipAnalysis(ctx, lm, fsTools, packages, detailedAnalysis)
	if err != nil {
		log.Fatalf("❌ Relationship analysis failed: %v", err)
	}

	// Combine all analysis results for synthesis
	combinedAnalyses := fmt.Sprintf(`=== MULTI-STAGE COMPREHENSIVE PACKAGE ANALYSIS ===

STAGE 1: STRUCTURAL EXPLORATION
%s

STAGE 2: DETAILED CODE ANALYSIS
%s

STAGE 3: RELATIONSHIP AND DEPENDENCY MAPPING
%s

=== END OF MULTI-STAGE COMPREHENSIVE ANALYSIS ===
`, structureAnalysis, detailedAnalysis, relationshipAnalysis)

	// Log information about the combined analysis
	analysisSize := len(combinedAnalyses)
	fmt.Printf("📊 Combined multi-stage analysis size: %d characters\n", analysisSize)
	if analysisSize > 15000 {
		fmt.Printf("⚠️  Large analysis input detected (%d chars) - this may impact synthesis performance\n", analysisSize)
	}
	if analysisSize < 2000 {
		fmt.Printf("⚠️  Small analysis input detected (%d chars) - may not have gathered sufficient information\n", analysisSize)
	}

	// Stage 2: Synthesis with BestOfN using the gathered analysis
	fmt.Println("\n🔗 Stage: Synthesizing comprehensive architecture report with BestOfN (ChainOfThought)...")
	fmt.Printf("  • Input size: %d characters\n", len(combinedAnalyses))
	fmt.Printf("  • Will generate 10 attempts in parallel, keeping the best result\n")
	fmt.Printf("  • Max allowed failures: %d of %d attempts\n", 8, 10) // Updated MaxFailures

	synthesisSig := dsgo.NewSignature("Synthesize comprehensive architecture report").
		AddInput(FieldCombined, dsgo.FieldTypeString, "Comprehensive analyses of individual packages including structure, dependencies, and design patterns").
		AddOutput(FieldExecSummary, dsgo.FieldTypeString, "High-level summary of the architecture, including purpose, main components, and key design decisions").
		AddOutput(FieldArchDiagram, dsgo.FieldTypeString, "Text-based architecture diagram showing relationships between packages, data flow, and component interactions").
		AddOutput(FieldImprovement, dsgo.FieldTypeString, "Detailed suggestions for improvement including design patterns, performance, maintainability, and scalability").
		AddOutput("detailed_analysis", dsgo.FieldTypeString, "In-depth analysis of implementation details, trade-offs, and design rationale").
		AddOutput("package_interactions", dsgo.FieldTypeString, "Analysis of how packages interact with each other and external dependencies").
		AddOutput("potential_issues", dsgo.FieldTypeString, "Identification of potential issues, bottlenecks, or areas of concern").
		AddOutput("recommendations", dsgo.FieldTypeString, "Specific, actionable recommendations for enhancements and best practices")

	// Create base module
	baseSynthesizer := dsgo.NewChainOfThought(synthesisSig, lm)

	// Create BestOfN module
	// We generate 10 candidates and pick the best one based on our enhanced scorer
	scorer := func(inputs map[string]any, prediction *dsgo.Prediction) (float64, error) {
		score := 0.0

		// Check for presence of key fields with more granular scoring
		execSummary, hasExecSummary := prediction.GetString(FieldExecSummary)
		archDiagram, hasArchDiagram := prediction.GetString(FieldArchDiagram)
		improvement, hasImprovement := prediction.GetString(FieldImprovement)
		detailedAnalysis, hasDetailedAnalysis := prediction.GetString("detailed_analysis")
		packageInteractions, hasPackageInteractions := prediction.GetString("package_interactions")
		potentialIssues, hasPotentialIssues := prediction.GetString("potential_issues")
		recommendations, hasRecommendations := prediction.GetString("recommendations")

		// Base scoring for required fields
		if hasExecSummary && len(execSummary) > 100 {
			score += 2.0  // Higher weight for comprehensive executive summary
		} else if hasExecSummary && len(execSummary) > 50 {
			score += 1.0  // Partial credit for basic summary
		}

		if hasArchDiagram && len(archDiagram) > 100 {
			score += 2.0  // Higher weight for detailed architecture diagram
		} else if hasArchDiagram && len(archDiagram) > 50 {
			score += 1.0  // Partial credit for basic diagram
		}

		if hasImprovement && len(improvement) > 150 {
			score += 2.0  // Higher weight for comprehensive improvements
		} else if hasImprovement && len(improvement) > 50 {
			score += 1.0  // Partial credit for basic improvements
		}

		// Scoring for additional comprehensive fields
		if hasDetailedAnalysis && len(detailedAnalysis) > 200 {
			score += 1.5
		}

		if hasPackageInteractions && len(packageInteractions) > 100 {
			score += 1.5
		}

		if hasPotentialIssues && len(potentialIssues) > 50 {
			score += 1.0
		}

		if hasRecommendations && len(recommendations) > 100 {
			score += 1.5
		}

		// Log scoring details for debugging
		fmt.Printf("    └─ Scoring Details:\n")
		fmt.Printf("      └─ ExecSummary: %t (%d chars)\n", hasExecSummary, len(execSummary))
		fmt.Printf("      └─ ArchDiagram: %t (%d chars)\n", hasArchDiagram, len(archDiagram))
		fmt.Printf("      └─ Improvement: %t (%d chars)\n", hasImprovement, len(improvement))
		fmt.Printf("      └─ DetailedAnalysis: %t (%d chars)\n", hasDetailedAnalysis, len(detailedAnalysis))
		fmt.Printf("      └─ PackageInteractions: %t (%d chars)\n", hasPackageInteractions, len(packageInteractions))
		fmt.Printf("      └─ PotentialIssues: %t (%d chars)\n", hasPotentialIssues, len(potentialIssues))
		fmt.Printf("      └─ Recommendations: %t (%d chars)\n", hasRecommendations, len(recommendations))
		fmt.Printf("      └─ TotalScore: %.2f\n", score)

		return score, nil
	}

	bestOfN := dsgo.NewBestOfN(baseSynthesizer, 10).
		WithScorer(scorer).
		WithParallel(true). // Run attempts in parallel
		WithMaxFailures(8)  // Allow up to 8 failures out of 10 attempts before giving up

	synthesisStartTime := time.Now()
	fmt.Printf("  - Starting comprehensive synthesis with enhanced scoring (%.1f min timeout)...\n", 2.0)
	finalResult, err := bestOfN.Forward(ctx, map[string]any{
		FieldCombined: combinedAnalyses,
	})
	synthesisDuration := time.Since(synthesisStartTime)
	if err != nil {
		// Provide detailed error context
		fmt.Printf("\n❌ Synthesis failed after %.2f seconds: %v\n\n", synthesisDuration.Seconds(), err)
		fmt.Println("💡 This error typically means one of the following:")
		fmt.Println("   • API key issues (insufficient_quota, invalid_api_key, etc.)")
		fmt.Println("   • Model connectivity problems or timeouts")
		fmt.Println("   • Model rate limiting from provider")
		fmt.Println("   • Input too large for model's context window")
		fmt.Println("   • Model failed to produce outputs matching signature requirements")
		fmt.Println("   • Parsing/validation failures in ChainOfThought module")
		fmt.Println("   • Too many synthesis attempts failed (more than MaxFailures threshold)")
		fmt.Println("\nTo debug further:")
		fmt.Println("   • Check environment variables for API keys")
		fmt.Println("   • Try a different model that supports structured output")
		fmt.Println("   • Reduce input size if analysis is very large")
		fmt.Println("   • Increase MaxFailures if model is having trouble with format")
		log.Fatalf("Synthesis failed: %v", err)
	}

	fmt.Printf("✅ Synthesis completed in %.2f seconds\n", synthesisDuration.Seconds())
	fmt.Printf("  ✅ Best candidate selected with score: %.2f\n", finalResult.Score)

	// Extract comprehensive synthesis results
	execSummary := getStringOrZero(finalResult, FieldExecSummary)
	archDiagram := getStringOrZero(finalResult, FieldArchDiagram)
	improvement := getStringOrZero(finalResult, FieldImprovement)
	detailedAnalysisResult := getStringOrZero(finalResult, "detailed_analysis")
	packageInteractionsResult := getStringOrZero(finalResult, "package_interactions")
	potentialIssuesResult := getStringOrZero(finalResult, "potential_issues")
	recommendationsResult := getStringOrZero(finalResult, "recommendations")

	// Output comprehensive results
	fmt.Println("\n📊 FINAL COMPREHENSIVE ARCHITECTURE REPORT")
	fmt.Println("=========================================")

	if execSummary != "" {
		fmt.Printf("\n📋 EXECUTIVE SUMMARY:\n%s\n", execSummary)
	} else {
		fmt.Println("\n📋 EXECUTIVE SUMMARY: No executive summary generated")
	}

	if archDiagram != "" {
		fmt.Printf("\n🏗️  ARCHITECTURE DIAGRAM & RELATIONSHIPS:\n%s\n", archDiagram)
	} else {
		fmt.Println("\n🏗️  ARCHITECTURE DIAGRAM & RELATIONSHIPS: No architecture diagram generated")
	}

	if improvement != "" {
		fmt.Printf("\n💡 IMPROVEMENT SUGGESTIONS:\n%s\n", improvement)
	} else {
		fmt.Println("\n💡 IMPROVEMENT SUGGESTIONS: No improvement suggestions generated")
	}

	if detailedAnalysisResult != "" {
		fmt.Printf("\n🔍 DETAILED ANALYSIS:\n%s\n", detailedAnalysisResult)
	} else {
		fmt.Println("\n🔍 DETAILED ANALYSIS: No detailed analysis generated")
	}

	if packageInteractionsResult != "" {
		fmt.Printf("\n🔗 PACKAGE INTERACTIONS:\n%s\n", packageInteractionsResult)
	} else {
		fmt.Println("\n🔗 PACKAGE INTERACTIONS: No package interactions analysis generated")
	}

	if potentialIssuesResult != "" {
		fmt.Printf("\n⚠️  POTENTIAL ISSUES:\n%s\n", potentialIssuesResult)
	} else {
		fmt.Println("\n⚠️  POTENTIAL ISSUES: No potential issues identified")
	}

	if recommendationsResult != "" {
		fmt.Printf("\n✅ RECOMMENDATIONS:\n%s\n", recommendationsResult)
	} else {
		fmt.Println("\n✅ RECOMMENDATIONS: No recommendations provided")
	}

	// Save comprehensive report
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
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	currentDir := dir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in current directory (%s) or any parent directory. Please run this tool from within a Go module", currentDir)
		}
		dir = parent
	}
}

func saveReport(rawAnalyses string, finalResult *dsgo.Prediction) {
	filename := fmt.Sprintf("comprehensive_architecture_report_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
	fmt.Printf("\n💾 Saving comprehensive report to: %s\n", filename)

	file, err := os.Create(filename)
	if err != nil {
		log.Printf("❌ Failed to create report file '%s': %v\n\n💡 This error typically means:\n   • Insufficient permissions in current directory\n   • Disk space is full\n   • Invalid filename or path issues", filename, err)
		return
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("⚠️ Failed to close report file: %v", closeErr)
		} else {
			fmt.Printf("✅ Report file closed successfully\n")
		}
	}()

	// Write comprehensive report header
	if _, err := fmt.Fprintf(file, "COMPREHENSIVE ARCHITECTURE REPORT\n===============================\nGenerated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		log.Printf("❌ Failed to write header to report file: %v", err)
		return
	}

	// Write all the comprehensive sections
	if synthExecSummary := getStringOrZero(finalResult, FieldExecSummary); synthExecSummary != "" {
		if _, err := fmt.Fprintf(file, "EXECUTIVE SUMMARY\n===============\n%s\n\n", synthExecSummary); err != nil {
			log.Printf("❌ Failed to write executive summary to report file: %v", err)
			return
		}
	}

	if synthArchDiagram := getStringOrZero(finalResult, FieldArchDiagram); synthArchDiagram != "" {
		if _, err := fmt.Fprintf(file, "ARCHITECTURE DIAGRAM & RELATIONSHIPS\n==================================\n%s\n\n", synthArchDiagram); err != nil {
			log.Printf("❌ Failed to write architecture diagram to report file: %v", err)
			return
		}
	}

	if synthImprovement := getStringOrZero(finalResult, FieldImprovement); synthImprovement != "" {
		if _, err := fmt.Fprintf(file, "IMPROVEMENT SUGGESTIONS\n=====================\n%s\n\n", synthImprovement); err != nil {
			log.Printf("❌ Failed to write improvement suggestions to report file: %v", err)
			return
		}
	}

	if synthDetailedAnalysis := getStringOrZero(finalResult, "detailed_analysis"); synthDetailedAnalysis != "" {
		if _, err := fmt.Fprintf(file, "DETAILED ANALYSIS\n===============\n%s\n\n", synthDetailedAnalysis); err != nil {
			log.Printf("❌ Failed to write detailed analysis to report file: %v", err)
			return
		}
	}

	if synthPackageInteractions := getStringOrZero(finalResult, "package_interactions"); synthPackageInteractions != "" {
		if _, err := fmt.Fprintf(file, "PACKAGE INTERACTIONS\n==================\n%s\n\n", synthPackageInteractions); err != nil {
			log.Printf("❌ Failed to write package interactions to report file: %v", err)
			return
		}
	}

	if synthPotentialIssues := getStringOrZero(finalResult, "potential_issues"); synthPotentialIssues != "" {
		if _, err := fmt.Fprintf(file, "POTENTIAL ISSUES\n=============\n%s\n\n", synthPotentialIssues); err != nil {
			log.Printf("❌ Failed to write potential issues to report file: %v", err)
			return
		}
	}

	if synthRecommendations := getStringOrZero(finalResult, "recommendations"); synthRecommendations != "" {
		if _, err := fmt.Fprintf(file, "RECOMMENDATIONS\n=============\n%s\n\n", synthRecommendations); err != nil {
			log.Printf("❌ Failed to write recommendations to report file: %v", err)
			return
		}
	}

	// Add the raw analyses at the end
	if _, err := fmt.Fprintf(file, "===================\nAPPENDIX: RAW PACKAGE ANALYSES\n===================\n%s", rawAnalyses); err != nil {
		log.Printf("❌ Failed to write raw analyses to report file: %v", err)
		return
	}

	fmt.Printf("✅ Comprehensive report successfully saved to: %s\n", filename)
}

// Helper function for structural analysis
func performStructuralAnalysis(ctx context.Context, lm dsgo.LM, fsTools []dsgo.Tool, packages []string, strategy string) (string, error) {
	structureSig := dsgo.NewSignature("Perform structural analysis of packages").
		AddInput("packages", "array", "List of packages to analyze").
		AddInput("strategy", dsgo.FieldTypeString, "Analysis strategy to follow").
		AddOutput("package_structure", dsgo.FieldTypeString, "Directory and file structure of the analyzed packages").
		AddOutput("file_organization", dsgo.FieldTypeString, "How files are organized within packages").
		AddOutput("key_files", "array", "List of key files identified in the packages").
		AddOutput("structural_findings", dsgo.FieldTypeString, "Findings about the structural organization")

	react := dsgo.NewReAct(structureSig, lm, fsTools).WithVerbose(true).WithMaxIterations(256)

	result, err := react.Forward(ctx, map[string]any{
		"packages": packages,
		"strategy": strategy,
		"task":     fmt.Sprintf("Analyze the structural organization of the %s packages. List directory structure, identify key files, and document how the packages are organized.", strings.Join(packages, ", ")),
	})
	if err != nil {
		return "", err
	}

	keyFiles, hasKeyFiles := result.Get("key_files")
	if !hasKeyFiles {
		keyFiles = "No key files identified"
	}

	return fmt.Sprintf(`PACKAGE STRUCTURE ANALYSIS:
- Package Structure: %s
- File Organization: %s
- Key Files: %v
- Structural Findings: %s`,
		getStringOrZero(result, "package_structure"),
		getStringOrZero(result, "file_organization"),
		keyFiles,
		getStringOrZero(result, "structural_findings")), nil
}

// Helper function for detailed code analysis
func performDetailedAnalysis(ctx context.Context, lm dsgo.LM, fsTools []dsgo.Tool, packages []string, previousAnalysis string) (string, error) {
	detailedSig := dsgo.NewSignature("Perform detailed code analysis").
		AddInput("packages", "array", "List of packages to analyze").
		AddInput("previous_analysis", dsgo.FieldTypeString, "Previous structural analysis results").
		AddOutput("interfaces_and_types", dsgo.FieldTypeString, "Key interfaces and types defined in the packages").
		AddOutput("core_functions", dsgo.FieldTypeString, "Core functions and methods").
		AddOutput("api_design_patterns", dsgo.FieldTypeString, "API design patterns used").
		AddOutput("error_handling_patterns", dsgo.FieldTypeString, "Error handling strategies and patterns").
		AddOutput("code_quality_indicators", dsgo.FieldTypeString, "Indicators of code quality and maintainability")

	react := dsgo.NewReAct(detailedSig, lm, fsTools).WithVerbose(true).WithMaxIterations(256)

	result, err := react.Forward(ctx, map[string]any{
		"packages":         packages,
		"previous_analysis": previousAnalysis,
		"task":             fmt.Sprintf("Perform detailed code analysis of the %s packages. Focus on interfaces, types, core functions, API design patterns, error handling, and code quality.", strings.Join(packages, ", ")),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`DETAILED CODE ANALYSIS:
- Interfaces and Types: %s
- Core Functions: %s
- API Design Patterns: %s
- Error Handling Patterns: %s
- Code Quality Indicators: %s`,
		getStringOrZero(result, "interfaces_and_types"),
		getStringOrZero(result, "core_functions"),
		getStringOrZero(result, "api_design_patterns"),
		getStringOrZero(result, "error_handling_patterns"),
		getStringOrZero(result, "code_quality_indicators")), nil
}

// Helper function for relationship and dependency analysis
func performRelationshipAnalysis(ctx context.Context, lm dsgo.LM, fsTools []dsgo.Tool, packages []string, previousAnalysis string) (string, error) {
	relationshipSig := dsgo.NewSignature("Perform relationship and dependency analysis").
		AddInput("packages", "array", "List of packages to analyze").
		AddInput("previous_analysis", dsgo.FieldTypeString, "Previous analysis results").
		AddOutput("inter_package_dependencies", dsgo.FieldTypeString, "Dependencies between the analyzed packages").
		AddOutput("external_dependencies", dsgo.FieldTypeString, "External dependencies used by the packages").
		AddOutput("data_flow", dsgo.FieldTypeString, "How data flows between components").
		AddOutput("interaction_patterns", dsgo.FieldTypeString, "Patterns of interaction between components").
		AddOutput("potential_bottlenecks", dsgo.FieldTypeString, "Potential bottlenecks or issues in the architecture")

	react := dsgo.NewReAct(relationshipSig, lm, fsTools).WithVerbose(true).WithMaxIterations(256)

	result, err := react.Forward(ctx, map[string]any{
		"packages":          packages,
		"previous_analysis": previousAnalysis,
		"task":              fmt.Sprintf("Analyze relationships and dependencies among the %s packages. Focus on inter-package dependencies, external dependencies, data flow, interaction patterns, and potential bottlenecks.", strings.Join(packages, ", ")),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`RELATIONSHIP AND DEPENDENCY ANALYSIS:
- Inter-Package Dependencies: %s
- External Dependencies: %s
- Data Flow: %s
- Interaction Patterns: %s
- Potential Bottlenecks: %s`,
		getStringOrZero(result, "inter_package_dependencies"),
		getStringOrZero(result, "external_dependencies"),
		getStringOrZero(result, "data_flow"),
		getStringOrZero(result, "interaction_patterns"),
		getStringOrZero(result, "potential_bottlenecks")), nil
}
