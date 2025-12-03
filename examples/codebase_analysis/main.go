package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/shared/tools"
)

// Constants for model configuration
const (
	// Analysis configuration
	MaxRetries     = 3
	TimeoutSeconds = 300 // 5 minutes
)

// getModelName returns the model name from environment variable or default
func getModelName() string {
	if model := os.Getenv("EXAMPLES_DEFAULT_MODEL"); model != "" {
		return model
	}
	return "openrouter/amazon/nova-2-lite-v1" // default fallback
}

// ============================================================================
// TYPED SIGNATURE DEFINITIONS
// ============================================================================
// These structs define the input/output interfaces for our codebase analysis modules.
// Each struct uses dsgo struct tags to define fields for the language model.
//
// dsgo tag format: "direction[,optional][,desc=description][,enum=val1|val2|val3]"
// - direction: "input" or "output" (required)
// - optional: marks field as not required (optional)
// - desc: human-readable description (recommended)
// - enum: list of allowed values for classification fields (optional)
// ============================================================================

// AnalysisPlanInput defines the input for planning a comprehensive codebase analysis
type AnalysisPlanInput struct {
	// Task describes what needs to be analyzed and the scope of the analysis
	Task string `dsgo:"input,desc=The codebase analysis task to plan for, including scope and objectives"`
}

// AnalysisPlanOutput defines the structured output for analysis planning
type AnalysisPlanOutput struct {
	// Strategy provides the step-by-step approach for conducting the analysis
	Strategy string `dsgo:"output,desc=Step-by-step strategy for comprehensive analysis including methodology and approach"`

	// TargetDirectories specifies which directories should be the focus of analysis
	TargetDirectories string `dsgo:"output,desc=Comma-separated list of specific directories to focus on during analysis"`

	// AnalysisPhases breaks down the analysis into logical, sequential phases
	AnalysisPhases string `dsgo:"output,desc=Comma-separated list of analysis phases to execute in order, each with specific objectives"`
}

// CodebaseAnalysisInput defines the input for executing specific analysis phases
type CodebaseAnalysisInput struct {
	// Task specifies the particular analysis task to execute
	Task string `dsgo:"input,desc=Specific analysis task to execute with clear objectives"`

	// Strategy provides the methodology and approach to follow
	Strategy string `dsgo:"input,desc=Analysis strategy to follow, including methodology and specific techniques"`

	// Context provides additional information from previous analysis phases
	Context string `dsgo:"input,optional,desc=Additional context from previous analysis phases to build upon"`
}

// CodebaseAnalysisOutput defines the structured output for analysis execution
type CodebaseAnalysisOutput struct {
	// Findings contains the key discoveries and insights from the analysis
	Findings string `dsgo:"output,desc=Key findings from analysis, including important discoveries and insights"`

	// Summary provides a high-level overview of the analysis results
	Summary string `dsgo:"output,desc=Summary of analysis results, highlighting the most important aspects"`

	// NextSteps suggests follow-up actions and further analysis opportunities
	NextSteps string `dsgo:"output,desc=Suggested next steps for further analysis or actions based on current findings"`
}

// ============================================================================
// TYPED FEW-SHOT EXAMPLES
// ============================================================================
// These examples demonstrate the expected input/output patterns to guide the model.
// They significantly improve the quality and consistency of the model's responses.
// ============================================================================

// getAnalysisPlanExamples returns typed few-shot examples for analysis planning
func getAnalysisPlanExamples() ([]AnalysisPlanInput, []AnalysisPlanOutput) {
	inputs := []AnalysisPlanInput{
		{
			Task: "Analyze a microservices web application codebase for security vulnerabilities and performance bottlenecks",
		},
		{
			Task: "Review a machine learning pipeline codebase for data quality issues and model performance optimization opportunities",
		},
	}

	outputs := []AnalysisPlanOutput{
		{
			Strategy:          "Start with authentication and authorization mechanisms, then analyze API endpoints for injection vulnerabilities, followed by database query optimization, and finally review caching strategies and load balancing configurations",
			TargetDirectories: "auth/, api/, database/, cache/, loadbalancer/",
			AnalysisPhases:    "Security audit of authentication and authorization, API endpoint vulnerability scanning, Database performance analysis, Caching and load balancing review, Overall architecture assessment",
		},
		{
			Strategy:          "Begin with data ingestion and validation pipeline analysis, then examine feature engineering code, review model training and evaluation scripts, and finally assess deployment and monitoring infrastructure",
			TargetDirectories: "data_ingestion/, feature_engineering/, model_training/, deployment/, monitoring/",
			AnalysisPhases:    "Data quality and preprocessing analysis, Feature engineering review, Model training and evaluation assessment, Deployment pipeline analysis, Monitoring and observability review",
		},
	}

	return inputs, outputs
}

// getAnalysisExamples returns typed few-shot examples for codebase analysis execution
func getAnalysisExamples() ([]CodebaseAnalysisInput, []CodebaseAnalysisOutput) {
	inputs := []CodebaseAnalysisInput{
		{
			Task:     "Analyze the authentication module for security vulnerabilities",
			Strategy: "Review authentication flows, check for common security anti-patterns, analyze password handling, and verify session management",
			Context:  "Previous analysis identified the auth module as critical for overall system security",
		},
		{
			Task:     "Evaluate database query performance and identify optimization opportunities",
			Strategy: "Examine query patterns, analyze execution plans, identify missing indexes, and review data access patterns",
			Context:  "Application is experiencing slow response times under load",
		},
	}

	outputs := []CodebaseAnalysisOutput{
		{
			Findings:  "Authentication module uses secure password hashing with bcrypt, implements proper session management with secure cookies, but lacks multi-factor authentication and has insufficient rate limiting on login attempts",
			Summary:   "The authentication system is generally secure with proper password handling and session management, but requires additional security measures like MFA and improved rate limiting",
			NextSteps: "Implement multi-factor authentication, add comprehensive rate limiting, conduct penetration testing, and review authentication logs for suspicious patterns",
		},
		{
			Findings:  "Database queries show N+1 query problems in user profile loading, missing indexes on frequently queried columns, and inefficient JOIN operations in reporting queries",
			Summary:   "Performance issues stem from N+1 queries, missing indexes, and suboptimal JOIN operations, particularly affecting user profile loading and reporting features",
			NextSteps: "Add database indexes, optimize JOIN queries, implement query result caching, and consider database connection pooling improvements",
		},
	}

	return inputs, outputs
}

// ============================================================================
// MAIN APPLICATION
// ============================================================================

func main() {
	ctx := context.Background()

	// Get model name from environment or use default
	modelName := getModelName()

	// Initialize LM with advanced configuration
	lm, err := dsgo.NewLM(ctx, modelName)
	if err != nil {
		log.Fatalf("❌ Failed to create LM: %v\n\n💡 This error typically means:\n   • Missing or invalid API key in environment variables\n   • Model name '%s' is not available\n   • Network connectivity issues to the LLM provider\n   • Rate limiting from the provider\n\nPlease check:\n   • OPENROUTER_API_KEY (or other provider API key) environment variable\n   • Internet connectivity\n   • Provider status page for any outages", err, modelName)
	}
	fmt.Printf("✅ Successfully initialized LM: %s\n", modelName)

	// Execute comprehensive codebase analysis using typed signatures
	if err := executeTypedCodebaseAnalysis(ctx, lm); err != nil {
		log.Fatalf("❌ Codebase analysis failed: %v", err)
	}

	fmt.Println("🎉 Codebase analysis completed successfully!")
}

// executeTypedCodebaseAnalysis demonstrates the complete typed workflow
func executeTypedCodebaseAnalysis(ctx context.Context, lm dsgo.LM) error {
	// Phase 1: Create typed ChainOfThought planner with comprehensive error handling
	fmt.Println("\n📋 Phase 1: Creating Typed Analysis Planner...")

	planner, err := createTypedPlanner(lm)
	if err != nil {
		return fmt.Errorf("failed to create typed planner: %w", err)
	}

	// Phase 2: Generate comprehensive analysis plan
	fmt.Println("\n🎯 Phase 2: Generating Analysis Plan...")
	plan, err := generateAnalysisPlan(ctx, planner)
	if err != nil {
		return fmt.Errorf("failed to generate analysis plan: %w", err)
	}

	// Phase 3: Execute analysis with typed inputs/outputs
	fmt.Println("\n🔍 Phase 3: Executing Analysis...")
	analysisResult, err := executeAnalysis(ctx, lm, plan)
	if err != nil {
		return fmt.Errorf("failed to execute analysis: %w", err)
	}

	// Phase 4: Display comprehensive results
	fmt.Println("\n📊 Phase 4: Analysis Results...")
	displayResults(plan, analysisResult)

	return nil
}

// createTypedPlanner creates a typed ChainOfThought module with advanced configuration
func createTypedPlanner(lm dsgo.LM) (*dsgo.Func[AnalysisPlanInput, AnalysisPlanOutput], error) {
	// Create typed ChainOfThought module
	planner, err := dsgo.NewTypedCoT[AnalysisPlanInput, AnalysisPlanOutput](lm)
	if err != nil {
		return nil, fmt.Errorf("failed to create typed ChainOfThought: %w", err)
	}

	// Add typed few-shot examples for better guidance
	exampleInputs, exampleOutputs := getAnalysisPlanExamples()
	planner, err = planner.WithDemosTyped(exampleInputs, exampleOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to add typed examples: %w", err)
	}

	// Configure custom generation options for better analysis quality
	planner = planner.WithOptions(&dsgo.GenerateOptions{
		Temperature: 0.3,  // Lower temperature for more consistent analysis
		MaxTokens:   2000, // Allow for detailed responses
	})

	// Use JSON adapter for structured output
	planner = planner.WithAdapter(dsgo.NewJSONAdapter())

	fmt.Println("✅ Typed planner created with advanced configuration")
	fmt.Printf("   • Few-shot examples: %d\n", len(exampleInputs))
	fmt.Printf("   • Temperature: 0.3\n")
	fmt.Printf("   • Max tokens: 2000\n")
	fmt.Printf("   • Adapter: JSON\n")

	return planner, nil
}

// generateAnalysisPlan generates a comprehensive analysis plan using typed inputs
func generateAnalysisPlan(ctx context.Context, planner *dsgo.Func[AnalysisPlanInput, AnalysisPlanOutput]) (*AnalysisPlanOutput, error) {
	// Define the analysis task with clear scope and objectives
	task := `Generate a comprehensive analysis plan for the DSGo codebase that includes:
1) Core architecture and design pattern analysis
2) Module dependency and interaction mapping  
3) Code quality and maintainability assessment
4) Performance and scalability evaluation
5) Security vulnerability identification
6) Documentation and testing coverage review

Focus on understanding the three-layer architecture (Core, Modules, Providers) and how they work together to implement the DSPy framework in Go.`

	// Create typed input
	input := AnalysisPlanInput{
		Task: task,
	}

	// Execute with timeout and comprehensive error handling
	ctx, cancel := context.WithTimeout(ctx, time.Duration(TimeoutSeconds)*time.Second)
	defer cancel()

	fmt.Printf("📝 Generating analysis plan (timeout: %ds)...\n", TimeoutSeconds)

	// Execute the typed module - RunWithPrediction returns (output, prediction, error)
	output, prediction, err := planner.RunWithPrediction(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("typed planner execution failed: %w", err)
	}

	// Log execution metadata for observability
	fmt.Printf("✅ Analysis plan generated successfully\n")
	fmt.Printf("   • Tokens used: %d\n", prediction.Usage.TotalTokens)
	fmt.Printf("   • Cost: $%.6f\n", prediction.Usage.Cost)
	fmt.Printf("   • Latency: %.2fms\n", float64(prediction.Usage.Latency))
	fmt.Printf("   • Adapter used: %s\n", prediction.AdapterUsed)

	return &output, nil
}

// executeAnalysis executes the analysis plan using typed inputs with filesystem tools
func executeAnalysis(ctx context.Context, lm dsgo.LM, plan *AnalysisPlanOutput) (*CodebaseAnalysisOutput, error) {
	// Create filesystem tools for actual codebase analysis
	filesystemTools := tools.GetAllFilesystemTools()

	// Create a new typed ReAct module for analysis execution with filesystem tools
	analyzer, err := dsgo.NewTypedReAct[CodebaseAnalysisInput, CodebaseAnalysisOutput](lm, filesystemTools)
	if err != nil {
		return nil, fmt.Errorf("failed to create typed ReAct analyzer: %w", err)
	}

	// Add analysis-specific examples
	exampleInputs, exampleOutputs := getAnalysisExamples()
	analyzer, err = analyzer.WithDemosTyped(exampleInputs, exampleOutputs)
	if err != nil {
		return nil, fmt.Errorf("failed to add analysis examples: %w", err)
	}

	// Configure for detailed analysis with filesystem interaction
	analyzer = analyzer.WithOptions(&dsgo.GenerateOptions{
		Temperature: 0.4,  // Slightly higher for creative insights
		MaxTokens:   3000, // Allow for comprehensive analysis
	})

	analyzer = analyzer.WithMaxIterations(64) // high number of iterations for thorough analysis

	// Create analysis input based on the generated plan, emphasizing filesystem usage
	analysisInput := CodebaseAnalysisInput{
		Task: fmt.Sprintf(`Execute comprehensive codebase analysis using filesystem tools to actually examine the DSGo codebase.

Strategy to follow: %s

Target directories to examine: %s
Analysis phases to complete: %s

IMPORTANT: Use the available filesystem tools (list_files, read_file, search_files) to:
1. Read actual source code files to understand implementation
2. Examine the project structure and directory organization
3. Analyze Go modules and dependencies
4. Review test files and documentation
5. Look at configuration files and build scripts

Base your analysis on actual code examination, not assumptions.`, plan.Strategy, plan.TargetDirectories, plan.AnalysisPhases),
		Strategy: plan.Strategy,
		Context:  fmt.Sprintf("Target directories: %s, Analysis phases: %s. Use filesystem tools to examine actual code.", plan.TargetDirectories, plan.AnalysisPhases),
	}

	// Execute analysis with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(TimeoutSeconds)*time.Second)
	defer cancel()

	fmt.Printf("🔍 Executing filesystem-based analysis (timeout: %ds)...\n", TimeoutSeconds)
	fmt.Printf("   • Available tools: %d (list_files, read_file, search_files)\n", len(filesystemTools))

	// Execute the analysis - RunWithPrediction returns (output, prediction, error)
	output, prediction, err := analyzer.RunWithPrediction(ctx, analysisInput)
	if err != nil {
		return nil, fmt.Errorf("typed ReAct analyzer execution failed: %w", err)
	}

	// Log execution metadata
	fmt.Printf("✅ Filesystem-based analysis executed successfully\n")
	fmt.Printf("   • Tokens used: %d\n", prediction.Usage.TotalTokens)
	fmt.Printf("   • Cost: $%.6f\n", prediction.Usage.Cost)
	fmt.Printf("   • Latency: %.2fms\n", float64(prediction.Usage.Latency))

	return &output, nil
}

// displayResults displays comprehensive analysis results
func displayResults(plan *AnalysisPlanOutput, analysis *CodebaseAnalysisOutput) {
	separator := strings.Repeat("=", 80)

	fmt.Println("\n" + separator)
	fmt.Println("📊 COMPREHENSIVE CODEBASE ANALYSIS RESULTS")
	fmt.Println(separator)

	// Display Analysis Plan
	fmt.Println("\n🎯 ANALYSIS PLAN")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Strategy:\n%s\n\n", plan.Strategy)
	fmt.Printf("Target Directories:\n")
	targetDirs := strings.Split(plan.TargetDirectories, ",")
	for i, dir := range targetDirs {
		fmt.Printf("  %d. %s\n", i+1, strings.TrimSpace(dir))
	}
	fmt.Printf("\nAnalysis Phases:\n")
	phases := strings.Split(plan.AnalysisPhases, ",")
	for i, phase := range phases {
		fmt.Printf("  %d. %s\n", i+1, strings.TrimSpace(phase))
	}

	// Display Analysis Results
	fmt.Println("\n🔍 ANALYSIS RESULTS")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Key Findings:\n%s\n\n", analysis.Findings)
	fmt.Printf("Summary:\n%s\n\n", analysis.Summary)
	fmt.Printf("Next Steps:\n%s\n", analysis.NextSteps)

	fmt.Println("\n" + separator)
	fmt.Println("🎉 ANALYSIS COMPLETE")
	fmt.Println(separator)
}
