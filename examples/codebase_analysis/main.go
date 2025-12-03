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
	TimeoutSeconds = 60 * 10 // 10 minutes
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

// AnalysisPlanOutput defines the comprehensive flat checklist for analysis planning
type AnalysisPlanOutput struct {
	// AnalysisChecklist provides a comprehensive flat checklist of all analysis tasks
	AnalysisChecklist string `dsgo:"output,desc=Comprehensive flat checklist of all analysis tasks and items to examine, formatted as a detailed checklist with checkboxes and specific action items for thorough codebase analysis"`
}

// CodebaseAnalysisInput defines the input for executing specific analysis phases
type CodebaseAnalysisInput struct {
	// AnalysisChecklist provides the comprehensive flat checklist from planning phase
	AnalysisChecklist string `dsgo:"input,desc=Analysis checklist, 'plan', obtained in planning phase, must be provided to this field"`
}

// CodebaseAnalysisOutput defines the comprehensive structured output for detailed codebase analysis
type CodebaseAnalysisOutput struct {
	// ExecutiveSummary provides a high-level overview of the entire analysis
	ExecutiveSummary string `dsgo:"output,desc=Comprehensive executive summary covering the most critical findings, overall health assessment, and top-priority recommendations"`

	// Axis1_Readability covers code clarity and understandability (7 Axes of Code Quality)
	Axis1_Readability string `dsgo:"output,desc=Readability analysis including naming conventions, code structure, comment quality, function length, cyclomatic complexity (<10), cognitive complexity, and overall code clarity"`

	// Axis2_Maintainability covers ease of modification and enhancement (7 Axes of Code Quality)
	Axis2_Maintainability string `dsgo:"output,desc=Maintainability assessment including technical debt ratio, code duplication, modularity, coupling/cohesion, refactoring needs, and long-term sustainability metrics"`

	// Axis3_PerformanceScalability covers system performance and scaling capabilities (7 Axes of Code Quality)
	Axis3_PerformanceScalability string `dsgo:"output,desc=Performance & scalability analysis including p95/p99 latency, resource per transaction, GC pressure, memory allocation patterns, benchmarking results, and scalability bottlenecks"`

	// Axis4_Security covers security vulnerabilities and protections (7 Axes of Code Quality)
	Axis4_Security string `dsgo:"output,desc=Security analysis including gosec scan results, govulncheck findings, staticcheck issues, input validation, authentication/authorization, and security best practices compliance"`

	// Axis5_Reliability covers system reliability and error handling (7 Axes of Code Quality)
	Axis5_Reliability string `dsgo:"output,desc=Reliability assessment including error handling patterns, fault tolerance, recovery mechanisms, uptime metrics, and resilience testing results"`

	// Axis6_CodeCoverage covers testing completeness and quality (7 Axes of Code Quality)
	Axis6_CodeCoverage string `dsgo:"output,desc=Code coverage analysis including unit test coverage (>80%), integration test coverage, edge case testing, test quality assessment, and testing gap identification"`

	// Axis7_Documentation covers documentation completeness and freshness (7 Axes of Code Quality)
	Axis7_Documentation string `dsgo:"output,desc=Documentation assessment including API docs freshness, code comment coverage, README completeness, architectural diagrams, setup guides, and documentation maintenance"`

	// GoSpecificAnalysis covers Go-language specific analysis
	GoSpecificAnalysis string `dsgo:"output,desc=Go-specific analysis including idiomatic Go patterns, concurrency safety (goroutine leaks, race conditions), error handling patterns, package structure, interface design, memory management, and Go module dependency analysis"`

	// ArchitectureAnalysis covers the structural design and patterns
	ArchitectureAnalysis string `dsgo:"output,desc=Detailed architectural analysis including design patterns (Clean Architecture, Repository, Service patterns), layer separation, component relationships, modularity, coupling, cohesion, and architectural debt assessment"`

	// StaticAnalysisResults covers automated static analysis tool results
	StaticAnalysisResults string `dsgo:"output,desc=Comprehensive static analysis results including go vet findings, golangci-lint report, staticcheck issues, code quality metrics, and automated tool recommendations"`

	// PerformanceProfiling covers detailed performance profiling results
	PerformanceProfiling string `dsgo:"output,desc=Performance profiling analysis including CPU profiling with pprof, memory profiling, benchmarking with benchstat, hotspot identification, and optimization opportunities"`

	// SecurityScanResults covers detailed security scanning results
	SecurityScanResults string `dsgo:"output,desc=Security scanning results including gosec vulnerability scan, govulncheck dependency analysis, staticcheck security issues, OWASP compliance, and security risk assessment"`

	// TechnicalDebtAnalysis covers technical debt assessment and prioritization
	TechnicalDebtAnalysis string `dsgo:"output,desc=Technical debt analysis including debt quantification, prioritization matrix, remediation cost estimation, code churn analysis, and debt reduction strategies"`

	// DependencyVulnerabilityAnalysis covers third-party dependency security
	DependencyVulnerabilityAnalysis string `dsgo:"output,desc=Dependency vulnerability analysis including CVE scanning, license compliance, version management, update strategies, and supply chain security assessment"`

	// BenchmarkingResults covers performance benchmarking data
	BenchmarkingResults string `dsgo:"output,desc=Benchmarking results including performance baselines, regression testing, load testing results, scalability benchmarks, and performance trend analysis"`

	// CodeQualityMetrics provides quantitative code quality measurements
	CodeQualityMetrics string `dsgo:"output,desc=Code quality metrics including cyclomatic complexity distribution, maintainability index, code duplication percentage, test coverage metrics, and quality trend analysis"`

	// Recommendations provides actionable improvement suggestions
	Recommendations string `dsgo:"output,desc=Detailed, prioritized recommendations for improvements categorized by immediate, short-term, and long-term actions with specific implementation guidance and tool recommendations"`

	// ImplementationRoadmap provides a structured plan for improvements
	ImplementationRoadmap string `dsgo:"output,desc=Comprehensive implementation roadmap with phased approach, resource requirements, timelines, dependencies, success criteria, and tool implementation strategies"`

	// Conclusion provides final thoughts and overall assessment
	Conclusion string `dsgo:"output,desc=Final conclusion summarizing the overall codebase health, key strengths, critical issues, and strategic direction for future development with emphasis on Go best practices"`
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
			AnalysisChecklist: "☐ Review authentication and authorization mechanisms for security vulnerabilities\n☐ Analyze API endpoints for injection attacks (SQL, XSS, CSRF)\n☐ Examine database query patterns and optimization opportunities\n☐ Review caching strategies and load balancing configurations\n☐ Assess input validation and sanitization practices\n☐ Check for proper error handling and information disclosure\n☐ Verify session management and token security\n☐ Analyze rate limiting and DDoS protection mechanisms\n☐ Review logging and monitoring for security events\n☐ Examine dependency security and vulnerability scanning",
		},
		{
			AnalysisChecklist: "☐ Analyze data ingestion pipeline for quality issues and bottlenecks\n☐ Review feature engineering code for efficiency and correctness\n☐ Examine model training scripts for performance optimization\n☐ Assess model evaluation metrics and validation procedures\n☐ Review deployment pipeline for reliability and scalability\n☐ Analyze monitoring and observability infrastructure\n☐ Check data versioning and reproducibility mechanisms\n☐ Review model versioning and rollback procedures\n☐ Examine A/B testing and experiment tracking\n☐ Assess data privacy and compliance measures",
		},
	}

	return inputs, outputs
}

// getAnalysisExamples returns typed few-shot examples for codebase analysis execution
func getAnalysisExamples() ([]CodebaseAnalysisInput, []CodebaseAnalysisOutput) {
	inputs := []CodebaseAnalysisInput{
		{
			AnalysisChecklist: "☐ Review authentication and authorization mechanisms for security vulnerabilities\n☐ Analyze API endpoints for injection attacks (SQL, XSS, CSRF)\n☐ Examine database query patterns and optimization opportunities\n☐ Review caching strategies and load balancing configurations\n☐ Assess input validation and sanitization practices\n☐ Check for proper error handling and information disclosure\n☐ Verify session management and token security\n☐ Analyze rate limiting and DDoS protection mechanisms\n☐ Review logging and monitoring for security events\n☐ Examine dependency security and vulnerability scanning",
		},
		{
			AnalysisChecklist: "☐ Analyze database query patterns and execution plans\n☐ Identify missing indexes and optimization opportunities\n☐ Review connection pooling and resource management\n☐ Examine N+1 query problems and data access patterns\n☐ Assess caching strategies and implementation\n☐ Evaluate database schema design and normalization\n☐ Check for proper transaction management and isolation\n☐ Review backup and recovery procedures\n☐ Analyze database security and access controls\n☐ Examine performance monitoring and alerting",
		},
	}

	outputs := []CodebaseAnalysisOutput{
		{
			ExecutiveSummary:                "Authentication system demonstrates strong foundational security practices but requires enhancements in multi-factor authentication, rate limiting, and comprehensive monitoring to meet enterprise security standards.",
			Axis1_Readability:               "Code readability is excellent with clear naming conventions, well-structured functions averaging 25 lines, and comprehensive comments. Cyclomatic complexity averages 6.8/10 with no functions exceeding 15. Cognitive complexity is low with minimal nesting.",
			Axis2_Maintainability:           "High maintainability score of 8.5/10 with technical debt ratio below 5%. Code duplication is minimal at 3%. Modular design with clear separation of concerns enables easy modification. Refactoring needs are minimal.",
			Axis3_PerformanceScalability:    "Authentication latency averages 150ms with p95 at 280ms. Memory usage is efficient with minimal allocations. Current architecture supports horizontal scaling. GC pressure is low. Session storage could become bottleneck at scale.",
			Axis4_Security:                  "Strong security implementation with bcrypt password hashing (cost 12), secure cookie configuration, and proper session management. gosec scan shows no critical vulnerabilities. Missing MFA and enhanced rate limiting considered medium risk.",
			Axis5_Reliability:               "High reliability with comprehensive error handling and graceful degradation. Fault tolerance implemented through retry mechanisms. Recovery procedures are documented. Uptime metrics show 99.9% availability. Error rates are below 0.1%.",
			Axis6_CodeCoverage:              "Unit test coverage at 85% for core authentication flows. Integration tests cover session management. Edge case testing needs improvement. Test quality is high with meaningful assertions. Testing gaps identified in security scenarios.",
			Axis7_Documentation:             "API documentation is comprehensive with clear examples. Code comments cover 90% of functions. README is detailed but lacks advanced security configuration guides. Documentation freshness score is 8/10 with recent updates.",
			GoSpecificAnalysis:              "Idiomatic Go patterns followed consistently. Proper error handling with wrapped errors. Interface-based design enables testability. Concurrency safety verified with race detector. Memory management is efficient. Go modules are well-structured.",
			ArchitectureAnalysis:            "Authentication module follows well-structured layered architecture with clear separation between authentication logic, session management, and user data access. Implements proper dependency injection and interface-based design following Clean Architecture principles.",
			StaticAnalysisResults:           "go vet: no issues found. golangci-lint: 3 minor style warnings. staticcheck: 2 performance suggestions. Overall code quality score: 9.2/10. No critical issues detected. All recommendations are implementable.",
			PerformanceProfiling:            "CPU profiling shows authentication logic consumes 45% of request time. Memory profiling indicates efficient allocation patterns. Benchmarking shows 1000 auths/sec capability. Hotspots identified in password hashing (expected).",
			SecurityScanResults:             "gosec: 0 critical, 2 medium warnings. govulncheck: no known vulnerabilities. staticcheck: 1 security-related suggestion. OWASP compliance score: 7.5/10. Authentication mechanisms are robust but need MFA enhancement.",
			TechnicalDebtAnalysis:           "Technical debt ratio: 4.2% (below industry average of 8%). Code churn: 6% (stable). Debt prioritization shows 2 high-priority items (MFA, rate limiting). Remediation cost estimated at 40 developer hours. Debt reduction strategy defined.",
			DependencyVulnerabilityAnalysis: "All dependencies up-to-date with no CVEs. License compliance verified. Supply chain security assessment passed. Version management follows semantic versioning. Dependency update strategy automated.",
			BenchmarkingResults:             "Authentication baseline: 1000 ops/sec. Load testing: 500 concurrent users handled successfully. Regression testing: no performance degradation detected. Scalability benchmarks show linear scaling to 10,000 requests/sec.",
			CodeQualityMetrics:              "Cyclomatic complexity: avg 6.8, max 12. Maintainability index: 85/100. Code duplication: 3%. Test coverage: 85%. Quality trend: improving over last 6 months. All metrics above industry benchmarks.",
			Recommendations:                 "Immediate: Implement account lockout and enhanced rate limiting. Short-term: Add multi-factor authentication and security monitoring. Long-term: Implement zero-trust architecture and advanced threat detection. Tool recommendations: gosec, govulncheck.",
			ImplementationRoadmap:           "Phase 1 (2 weeks): Enhanced rate limiting and account lockout implementation. Phase 2 (4 weeks): MFA implementation with TOTP support. Phase 3 (8 weeks): Advanced security monitoring and threat detection integration. Tool deployment in parallel.",
			Conclusion:                      "The authentication system provides a solid foundation for secure access control with excellent Go practices and high code quality. Strategic enhancements in MFA and monitoring will elevate it to enterprise security standards while maintaining strong technical foundations.",
		},
		{
			ExecutiveSummary:                "Database layer demonstrates significant performance optimization opportunities through query restructuring, indexing improvements, and caching implementation, with potential for 60-80% performance gains.",
			Axis1_Readability:               "Database code readability varies by module. Legacy code shows higher complexity with some functions exceeding 50 lines. Naming conventions are consistent. Comments are adequate but could be more comprehensive. Cyclomatic complexity averages 8.5 with some functions at 18+.",
			Axis2_Maintainability:           "Moderate maintainability with technical debt ratio at 12% (above average). Code duplication detected at 8% in query patterns. Business logic creep observed in data access layer. Refactoring needs identified in reporting queries. Migration process is well-structured.",
			Axis3_PerformanceScalability:    "Critical performance issues identified. Query performance averages 2.3s (target <500ms). N+1 query patterns detected. Missing indexes on frequently filtered columns. Connection pool efficiency at 40%. Current architecture limits horizontal scaling opportunities.",
			Axis4_Security:                  "Database security is adequate with proper connection encryption and parameterized queries. gosec shows no SQL injection risks. Row-level security not implemented where needed. Audit logging is minimal. govulncheck shows no dependency vulnerabilities.",
			Axis5_Reliability:               "Database reliability is good with proper error handling. Connection retry mechanisms implemented. Fault tolerance through connection pooling. Recovery procedures documented but need testing. Error rates at 2% (higher than desired). Backup strategies comprehensive.",
			Axis6_CodeCoverage:              "Database test coverage is low at 45% (below 80% target). Integration tests use in-memory databases not reflecting production characteristics. Load testing absent. Edge case testing minimal. Test quality needs improvement in error scenarios.",
			Axis7_Documentation:             "Database schema documentation is comprehensive. Query optimization guidelines missing. Performance tuning documentation outdated. Migration scripts well-documented. API documentation adequate. Documentation freshness score: 6/10.",
			GoSpecificAnalysis:              "Go database patterns generally well-implemented. Proper use of context for cancellation. Connection handling follows best practices. Error handling needs improvement for database-specific scenarios. Memory management in query results could be optimized.",
			ArchitectureAnalysis:            "Database access layer follows repository pattern with proper abstraction. Connection pooling implemented but not optimally configured. Query builder usage inconsistent across modules. Some violation of Clean Architecture principles in data access.",
			StaticAnalysisResults:           "go vet: 4 warnings about unused variables. golangci-lint: 12 issues (8 style, 4 performance). staticcheck: 6 suggestions including inefficient SQL patterns. Overall code quality score: 6.8/10. Performance-related issues prioritized.",
			PerformanceProfiling:            "CPU profiling shows 70% time in database queries. Memory profiling indicates high allocation in result processing. Benchmarking reveals 100 queries/sec (target: 500+). Hotspots in user profile loading (N+1 queries). Index usage suboptimal at 65%.",
			SecurityScanResults:             "gosec: 1 medium risk (insufficient input validation). govulncheck: no vulnerabilities. staticcheck: 2 security suggestions. OWASP compliance: 6.5/10. Parameterized queries properly implemented. Row-level security gaps identified.",
			TechnicalDebtAnalysis:           "Technical debt ratio: 12% (high). Code churn: 15% (active development). Debt prioritization shows 5 high-priority performance items. Remediation cost estimated at 120 developer hours. Debt concentrated in reporting and query optimization.",
			DependencyVulnerabilityAnalysis: "Database driver dependencies current. ORM usage creates performance overhead. One dependency has minor CVE (CVSS 3.1). License compliance verified. Supply chain security adequate. Version management needs optimization.",
			BenchmarkingResults:             "Current baseline: 100 queries/sec (target: 500+). Load testing: fails at 200 concurrent users. Regression testing: 15% performance degradation over 3 months. Scalability: poor due to architectural limitations. Optimization potential: 60-80% improvement.",
			CodeQualityMetrics:              "Cyclomatic complexity: avg 8.5, max 22. Maintainability index: 65/100. Code duplication: 8%. Test coverage: 45%. Quality trend: declining due to accumulated technical debt. Several metrics below industry benchmarks.",
			Recommendations:                 "Immediate: Add missing indexes and fix N+1 queries. Short-term: Implement query result caching and optimize JOIN operations. Long-term: Implement read replicas and consider database sharding. Tools: pprof, benchstat, gosec, query optimization tools.",
			ImplementationRoadmap:           "Phase 1 (1 week): Critical indexing and N+1 query fixes. Phase 2 (2 weeks): Query optimization and caching implementation. Phase 3 (6 weeks): Read replica deployment and connection pool optimization. Tool deployment throughout phases.",
			Conclusion:                      "Database layer requires immediate attention for performance optimization and technical debt reduction. Current implementation shows good security practices but suffers from performance issues and accumulated technical debt. With proper optimization, significant improvements are achievable.",
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

	// Phase 3: Execute comprehensive analysis (with shorter timeout for demo)
	fmt.Println("\n🔍 Phase 3: Executing Analysis...")
	fmt.Println("   (Note: This phase may take several minutes for comprehensive analysis)")

	analysisCtx, cancel := context.WithTimeout(ctx, time.Duration(TimeoutSeconds)*time.Second)
	defer cancel()

	analysisResult, err := executeAnalysis(analysisCtx, lm, plan)
	if err != nil {
		fmt.Printf("⚠️  Analysis phase incomplete: %v\n", err)
		fmt.Println("   The comprehensive analysis plan above is ready for use.")
		fmt.Println("   You can execute the analysis manually using the checklist.")
		analysisResult = nil // Set to nil to avoid display issues
	}

	// Display comprehensive results
	if analysisResult != nil {
		displayResults(plan, analysisResult)
	} else {
		fmt.Println("\n🎯 ANALYSIS COMPLETE - PLAN READY")
		fmt.Println("=====================================")
		fmt.Println("✅ Comprehensive analysis plan generated successfully!")
		fmt.Println("📋 Use the checklist above to conduct detailed analysis")
		fmt.Println("🔧 Each checklist item can be executed independently")
	}

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
	temperature := 0.7
	maxTokens := 1024 * 100
	planner = planner.WithOptions(&dsgo.GenerateOptions{
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})

	// Use JSON adapter for structured output
	planner = planner.WithAdapter(dsgo.NewJSONAdapter())

	fmt.Println("✅ Typed planner created with advanced configuration")
	fmt.Printf("   • Few-shot examples: %d\n", len(exampleInputs))
	fmt.Printf("   • Temperature: %f\n", temperature)
	fmt.Printf("   • Max tokens: %d\n", maxTokens)
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

	// Debug: Print plan content
	fmt.Printf("   • Plan Analysis Checklist length: %d chars\n", len(output.AnalysisChecklist))
	if output.AnalysisChecklist == "" {
		fmt.Printf("   ⚠️  WARNING: AnalysisChecklist is empty!\n")
	}

	// Display the plan immediately
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📋 COMPREHENSIVE ANALYSIS PLAN")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\n%s\n", output.AnalysisChecklist)
	fmt.Println(strings.Repeat("=", 80))

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
		Temperature: 0.3,  // Lower for more structured output
		MaxTokens:   4000, // Allow for comprehensive analysis
	})

	analyzer = analyzer.WithMaxIterations(64) // high number of iterations for thorough analysis

	// Create analysis input based on the generated plan
	analysisInput := CodebaseAnalysisInput{
		AnalysisChecklist: plan.AnalysisChecklist,
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

	// Debug: Check if plan is nil or empty
	if plan == nil {
		fmt.Println("❌ ERROR: Plan is nil!")
		return
	}

	// Analysis Checklist
	fmt.Println("\n📋 COMPREHENSIVE ANALYSIS CHECKLIST")
	fmt.Println(strings.Repeat("-", 40))
	if plan.AnalysisChecklist == "" {
		fmt.Println("⚠️  WARNING: AnalysisChecklist is empty!")
	} else {
		fmt.Printf("%s\n", plan.AnalysisChecklist)
	}

	// Display Analysis Results
	fmt.Println("\n🔍 COMPREHENSIVE ANALYSIS RESULTS")
	fmt.Println(strings.Repeat("-", 60))

	// Executive Summary
	fmt.Println("\n📋 EXECUTIVE SUMMARY")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.ExecutiveSummary)

	// 7 Axes of Code Quality
	fmt.Println("\n🎯 7 AXES OF CODE QUALITY")
	fmt.Println(strings.Repeat("-", 40))

	// Axis 1: Readability
	fmt.Println("\n📖 AXIS 1: READABILITY")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.Axis1_Readability)

	// Axis 2: Maintainability
	fmt.Println("\n🔧 AXIS 2: MAINTAINABILITY")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.Axis2_Maintainability)

	// Axis 3: Performance & Scalability
	fmt.Println("\n⚡ AXIS 3: PERFORMANCE & SCALABILITY")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("%s\n", analysis.Axis3_PerformanceScalability)

	// Axis 4: Security
	fmt.Println("\n🔒 AXIS 4: SECURITY")
	fmt.Println(strings.Repeat("-", 25))
	fmt.Printf("%s\n", analysis.Axis4_Security)

	// Axis 5: Reliability
	fmt.Println("\n🛡️  AXIS 5: RELIABILITY")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.Axis5_Reliability)

	// Axis 6: Code Coverage
	fmt.Println("\n🧪 AXIS 6: CODE COVERAGE")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.Axis6_CodeCoverage)

	// Axis 7: Documentation
	fmt.Println("\n📚 AXIS 7: DOCUMENTATION")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.Axis7_Documentation)

	// Go-Specific Analysis
	fmt.Println("\n🐹 GO-SPECIFIC ANALYSIS")
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("%s\n", analysis.GoSpecificAnalysis)

	// Architecture Analysis
	fmt.Println("\n🏗️  ARCHITECTURE ANALYSIS")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.ArchitectureAnalysis)

	// Static Analysis Results
	fmt.Println("\n🔍 STATIC ANALYSIS RESULTS")
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("%s\n", analysis.StaticAnalysisResults)

	// Performance Profiling
	fmt.Println("\n📊 PERFORMANCE PROFILING")
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("%s\n", analysis.PerformanceProfiling)

	// Security Scan Results
	fmt.Println("\n🛡️ SECURITY SCAN RESULTS")
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("%s\n", analysis.SecurityScanResults)

	// Technical Debt Analysis
	fmt.Println("\n⚠️  TECHNICAL DEBT ANALYSIS")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("%s\n", analysis.TechnicalDebtAnalysis)

	// Dependency Vulnerability Analysis
	fmt.Println("\n🔗 DEPENDENCY VULNERABILITY ANALYSIS")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("%s\n", analysis.DependencyVulnerabilityAnalysis)

	// Benchmarking Results
	fmt.Println("\n📈 BENCHMARKING RESULTS")
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("%s\n", analysis.BenchmarkingResults)

	// Code Quality Metrics
	fmt.Println("\n📋 CODE QUALITY METRICS")
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("%s\n", analysis.CodeQualityMetrics)

	// Recommendations
	fmt.Println("\n💡 RECOMMENDATIONS")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.Recommendations)

	// Implementation Roadmap
	fmt.Println("\n🗺️  IMPLEMENTATION ROADMAP")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.ImplementationRoadmap)

	// Conclusion
	fmt.Println("\n🎯 CONCLUSION")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("%s\n", analysis.Conclusion)

	fmt.Println("\n" + separator)
	fmt.Println("🎉 ANALYSIS COMPLETE")
	fmt.Println(separator)
}
