package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/examples/snowflake_trainer/agents"
	"github.com/assagman/dsgo/examples/snowflake_trainer/trainer"
	"github.com/assagman/dsgo/examples/snowflake_trainer/types"
)

const (
	DefaultModel = "openrouter/google/gemini-2.5-flash"
	DefaultTopic = "Snowflake Data Cloud Platform"
	Timeout      = 15 * time.Minute
)

// SnowflakeDomains contains authoritative Snowflake domains for research filtering
var SnowflakeDomains = []string{
	"snowflake.com",
	"docs.snowflake.com",
	"community.snowflake.com",
	"learn.snowflake.com",
	"app.snowflake.com",
}

func main() {
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	// Initialize LM
	model := getEnvOrDefault("TRAINER_MODEL", DefaultModel)
	lm, err := dsgo.NewLM(ctx, model)
	if err != nil {
		log.Fatalf("Failed to create LM: %v", err)
	}

	// Initialize MCP tools
	mcpTools, err := initializeMCPTools(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize MCP tools: %v", err)
	}

	// Get user's learning request
	query := getQueryFromArgs()
	if query == "" {
		query = "Teach me everything about Snowflake performance tuning for production workloads"
	}

	fmt.Printf("🎓 Creating personalized trainer for: %s\n\n", query)

	// Execute research pipeline
	research, err := executeResearchPipeline(ctx, lm, mcpTools, query)
	if err != nil {
		log.Fatalf("Research failed: %v", err)
	}

	// Generate curriculum
	curriculum, err := generateCurriculum(ctx, lm, research)
	if err != nil {
		log.Fatalf("Curriculum generation failed: %v", err)
	}

	// Generate markdown research report
	generator := trainer.NewReportGenerator()

	// Prepare report configuration with enhanced fields
	reportConfig := types.ReportConfig{
		Title:           fmt.Sprintf("%s Research Report", curriculum.Topic),
		Topic:           curriculum.Topic,
		SkillLevel:      curriculum.SkillLevel,
		EstimatedTime:   curriculum.EstimatedTime,
		Modules:         curriculum.Modules,
		Quizzes:         curriculum.Quizzes,
		Exercises:       curriculum.Exercises,
		Challenges:      curriculum.Challenges,
		Glossary:        curriculum.Glossary,
		Resources:       curriculum.Resources,
		GeneratedAt:     curriculum.GeneratedAt,
		ResearchSources: curriculum.ResearchSources,
		ReportTitle:     fmt.Sprintf("%s Research Report", curriculum.Topic),
		ReportDate:      time.Now().Format("January 2, 2006"),
		Author:          "DSGo Research Report Generator",
		ExecutiveSummary: fmt.Sprintf("This comprehensive research report covers %s at a %s level. The report includes %d learning modules, %d quiz questions, and %d hands-on exercises designed to provide deep understanding and practical skills.",
			curriculum.Topic, curriculum.SkillLevel, len(curriculum.Modules), countQuestions(curriculum), len(curriculum.Exercises)),
		KeyTakeaways: []string{
			fmt.Sprintf("Master %s through structured learning modules", curriculum.Topic),
			"Apply knowledge with hands-on exercises and practical challenges",
			"Test understanding with comprehensive quiz assessments",
			"Access curated resources and glossary for continued learning",
		},
	}

	markdown, err := generator.Generate(reportConfig)
	if err != nil {
		log.Fatalf("Report generation failed: %v", err)
	}

	// Compute timestamp once
	ts := time.Now().Format("20060102_150405")

	// Save markdown report
	outputPath := filepath.Join("output", fmt.Sprintf("research_report_%s.md", ts))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		log.Fatalf("Failed to save report: %v", err)
	}

	// Generate and save HTML report
	htmlGen := trainer.NewHTMLReportGenerator()
	htmlContent, err := htmlGen.Generate(reportConfig)
	if err != nil {
		log.Fatalf("HTML report generation failed: %v", err)
	}
	htmlPath := filepath.Join("output", fmt.Sprintf("research_report_%s.html", ts))
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		log.Fatalf("Failed to save HTML report: %v", err)
	}

	// Report results
	fmt.Printf("\n✅ Research report generated! (Markdown + HTML)\n")
	fmt.Printf("📁 Markdown: %s\n", outputPath)
	fmt.Printf("🌐 HTML: %s (open in browser for interactive view)\n", htmlPath)
	fmt.Printf("📊 Modules: %d | ❓ Questions: %d | 🏋️ Exercises: %d\n", len(curriculum.Modules), countQuestions(curriculum), len(curriculum.Exercises))
	fmt.Printf("💰 Cost: $%.4f | ⏱️ Time: %v\n", research.TotalCost, time.Since(startTime))
	fmt.Printf("\n🚀 Start learning! 📖 Markdown | 🌐 HTML\n")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getQueryFromArgs() string {
	if len(os.Args) > 1 {
		return strings.Join(os.Args[1:], " ")
	}
	return ""
}

// wrapToolWithDomainFilter creates a tool wrapper that enhances search queries with domain filters
func wrapToolWithDomainFilter(tool dsgo.Tool, domains []string) dsgo.Tool {
	// For Exa search tools, we enhance the query parameter
	if tool.Name == "search" || tool.Name == "exa_search" {
		originalFunc := tool.Function
		tool.Function = func(ctx context.Context, args map[string]any) (any, error) {
			// Enhance query with domain filters
			if query, ok := args["query"].(string); ok {
				domainFilter := ""
				for i, domain := range domains {
					if i == 0 {
						domainFilter = fmt.Sprintf("site:%s", domain)
					} else {
						domainFilter = fmt.Sprintf("%s OR site:%s", domainFilter, domain)
					}
				}
				args["query"] = fmt.Sprintf("%s %s", query, domainFilter)
				log.Printf("Enhanced search query: %s", args["query"])
			}
			return originalFunc(ctx, args)
		}
	}
	return tool
}

func initializeMCPTools(ctx context.Context) ([]dsgo.Tool, error) {
	var allTools []dsgo.Tool

	// Initialize Exa client if API key is available
	if exaKey := os.Getenv("EXA_API_KEY"); exaKey != "" {
		exaClient, err := dsgo.NewMCPExaClient(exaKey)
		if err != nil {
			log.Printf("Warning: Failed to initialize Exa client: %v", err)
		} else {
			if err := exaClient.Initialize(ctx); err != nil {
				log.Printf("Warning: Failed to initialize Exa connection: %v", err)
			} else {
				tools := exaClient.GetTools()
				// Wrap tools with domain filtering for Snowflake
				for _, tool := range tools {
					allTools = append(allTools, wrapToolWithDomainFilter(tool, SnowflakeDomains))
				}
			}
		}
	}

	// Initialize Tavily client if API key is available
	if tavilyKey := os.Getenv("TAVILY_API_KEY"); tavilyKey != "" {
		tavilyClient, err := dsgo.NewMCPTavilyClient(tavilyKey)
		if err != nil {
			log.Printf("Warning: Failed to initialize Tavily client: %v", err)
		} else {
			if err := tavilyClient.Initialize(ctx); err != nil {
				log.Printf("Warning: Failed to initialize Tavily connection: %v", err)
			} else {
				// Tavily provides search and extract tools
				allTools = append(allTools, tavilyClient.GetTools()...)
			}
		}
	}

	if len(allTools) == 0 {
		log.Printf("Warning: No MCP tools available. Set EXA_API_KEY and/or TAVILY_API_KEY for better research results.")
	}

	return allTools, nil
}

func executeResearchPipeline(ctx context.Context, lm dsgo.LM, tools []dsgo.Tool, query string) (*types.ResearchFindings, error) {
	// Step 1: Parse learning request with ChatAgent
	chatAgent := agents.NewChatAgent(lm)
	chatResult, err := chatAgent.Forward(ctx, map[string]any{
		"query": query,
	})
	if err != nil {
		return nil, fmt.Errorf("chat agent failed: %w", err)
	}

	learningObjectives := mustGetString(chatResult, "learningObjectives")
	skillLevel := mustGetString(chatResult, "skillLevel")

	// Step 2: Plan research with SupervisorAgent
	supervisorAgent := agents.NewSupervisorAgent(lm)
	supervisorResult, err := supervisorAgent.Forward(ctx, map[string]any{
		"query":              query,
		"learningObjectives": learningObjectives,
		"skillLevel":         skillLevel,
	})
	if err != nil {
		return nil, fmt.Errorf("supervisor agent failed: %w", err)
	}

	// Parse sub-topics
	subTopics, err := getSubTopics(supervisorResult, "subTopics")
	if err != nil {
		// Log full response for debugging
		log.Printf("Warning: Failed to parse subTopics. Full response: %+v", supervisorResult)
		return nil, fmt.Errorf("failed to parse sub-topics: %w", err)
	}

	// Step 3: Parallel research with WebResearchAgent using domain-filtered tools
	filteredTools := agents.NewSnowflakeDomainFilter(tools).GetFilteredTools()
	researchAgent := agents.NewWebResearchAgent(lm, filteredTools)
	maxWorkers, _ := strconv.Atoi(getEnvOrDefault("TRAINER_MAX_WORKERS", "16"))
	parallel := dsgo.NewParallel(researchAgent).
		WithMaxWorkers(maxWorkers).
		WithReturnAll(true).
		WithVerbose(true)

	// Prepare research inputs with Snowflake context
	researchInputs := agents.AddSnowflakeContext(subTopics, skillLevel)

	// Execute parallel research
	batchInputs := map[string]any{
		"_batch": researchInputs,
	}

	parallelResult, err := parallel.Forward(ctx, batchInputs)
	if err != nil {
		return nil, fmt.Errorf("parallel research failed: %w", err)
	}

	// Collect research findings
	var findings []types.ResearchOutput
	var allSources []string

	for _, comp := range parallelResult.Completions {
		pred := dsgo.NewPrediction(comp)

		findings = append(findings, types.ResearchOutput{
			CoreConcepts:   mustGetString(pred, "coreConcepts"),
			Explanations:   mustGetString(pred, "explanations"),
			Examples:       mustGetString(pred, "examples"),
			CodeSnippets:   mustGetString(pred, "codeSnippets"),
			CommonMistakes: mustGetString(pred, "commonMistakes"),
			Sources:        mustGetString(pred, "sources"),
			QuizMaterial:   mustGetString(pred, "quizMaterial"),
		})
		allSources = append(allSources, mustGetString(pred, "sources"))
	}

	// Filter out any research findings that are not Snowflake-specific
	var filteredFindings []types.ResearchOutput
	var filteredSources []string
	for i, f := range findings {
		fields := []string{f.CoreConcepts, f.Explanations, f.Examples, f.CodeSnippets, f.CommonMistakes, f.QuizMaterial}
		hasSnowflake := false
		for _, field := range fields {
			if agents.ValidateSnowflakeContent(field) {
				hasSnowflake = true
				break
			}
		}

		source := allSources[i]
		if strings.Contains(strings.ToLower(source), "snowflake") {
			hasSnowflake = true
		}

		if hasSnowflake {
			filteredFindings = append(filteredFindings, f)
			filteredSources = append(filteredSources, source)
		} else {
			log.Printf("Warning: filtered non-Snowflake research result: %v", f.Sources)
		}
	}

	// Use filtered results if available
	if len(filteredFindings) > 0 {
		findings = filteredFindings
		allSources = filteredSources
	}

	// Step 4: Synthesize findings with CombinerAgent
	combinerAgent := agents.NewCombinerAgent(lm)
	findingsJSON, _ := json.Marshal(findings)
	combinerResult, err := combinerAgent.Forward(ctx, map[string]any{
		"originalQuery": query,
		"findings":      string(findingsJSON),
		"skillLevel":    skillLevel,
	})
	if err != nil {
		return nil, fmt.Errorf("combiner agent failed: %w", err)
	}

	return &types.ResearchFindings{
		SubTopics: findings,
		TotalCost: parallelResult.Usage.Cost,
		TotalTime: time.Since(time.Now()), // Would track actual time
		Sources:   allSources,
		Unified: types.CombinerOutput{
			UnifiedKnowledge:   mustGetString(combinerResult, "unifiedKnowledge"),
			KeyTakeaways:       mustGetString(combinerResult, "keyTakeaways"),
			LearningPath:       mustGetString(combinerResult, "learningPath"),
			DifficultyMapping:  mustGetString(combinerResult, "difficultyMapping"),
			PracticalExercises: mustGetString(combinerResult, "practicalExercises"),
		},
	}, nil
}

func mustGetString(p *dsgo.Prediction, key string) string {
	s, _ := p.GetString(key)
	return s
}

func getSubTopics(p *dsgo.Prediction, key string) ([]string, error) {
	val, ok := p.Get(key)
	if !ok {
		return nil, fmt.Errorf("field %s not found", key)
	}

	// Case 1: Already a slice of strings (direct parse)
	if slice, ok := val.([]string); ok {
		return slice, nil
	}

	// Case 2: Slice of interfaces (from generic JSON parse)
	if slice, ok := val.([]interface{}); ok {
		var result []string
		for _, item := range slice {
			// Handle string items
			if s, ok := item.(string); ok {
				result = append(result, s)
				continue
			}
			// Handle object items with nested topics array (e.g., {category: "...", topics: [...]})
			if obj, ok := item.(map[string]interface{}); ok {
				// First check for nested "topics" array (common LLM pattern)
				if topics, ok := obj["topics"].([]interface{}); ok {
					for _, t := range topics {
						if s, ok := t.(string); ok {
							result = append(result, s)
						}
					}
					continue
				}
				// Fallback: try common keys for single topic name
				for _, k := range []string{"topic", "name", "title", "subTopic", "sub_topic"} {
					if sVal, ok := obj[k].(string); ok {
						result = append(result, sVal)
						break
					}
				}
			}
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	// Case 3: Map (nested JSON object) - try to find a list inside
	if m, ok := val.(map[string]interface{}); ok {
		// Look for any key that holds a list
		for _, v := range m {
			if list, ok := v.([]interface{}); ok {
				var result []string
				for _, item := range list {
					// Handle string items
					if s, ok := item.(string); ok {
						result = append(result, s)
						continue
					}
					// Handle object items (extract name/topic)
					if obj, ok := item.(map[string]interface{}); ok {
						// Try common keys
						for _, k := range []string{"name", "topic", "title", "subTopic", "sub_topic"} {
							if sVal, ok := obj[k].(string); ok {
								result = append(result, sVal)
								break
							}
						}
					}
				}
				if len(result) > 0 {
					return result, nil
				}
			}
		}
	}

	// Case 4: String (needs unmarshaling)
	if s, ok := val.(string); ok {
		// Sanitize string
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)

		var list []string
		if err := json.Unmarshal([]byte(s), &list); err == nil {
			return list, nil
		}

		// Try unmarshaling as object with list
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(s), &obj); err == nil {
			for _, v := range obj {
				if l, ok := v.([]interface{}); ok {
					var result []string
					for _, item := range l {
						if sVal, ok := item.(string); ok {
							result = append(result, sVal)
						}
					}
					if len(result) > 0 {
						return result, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("could not extract list of strings from value type %T: %v", val, val)
}

func parseJSONField(p *dsgo.Prediction, key string, target interface{}) error {
	val, ok := p.Get(key)
	if !ok {
		return fmt.Errorf("field %s not found", key)
	}

	var jsonBytes []byte

	// 1. Get raw JSON bytes
	if s, ok := val.(string); ok {
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
		jsonBytes = []byte(s)
	} else {
		// If it's already a map/slice, marshal it back to bytes to handle generic unmarshal
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("failed to marshal intermediate value: %w", err)
		}
		jsonBytes = b
	}

	// 2. Try direct unmarshal
	if err := json.Unmarshal(jsonBytes, target); err == nil {
		return nil
	}

	// 3. If target is a pointer to slice, and JSON is object, try to find array in object
	// We use reflection to check if target is pointer to slice
	// Or simpler: try to unmarshal into map[string]raw, find array, unmarshal that

	// Only attempt unwrapping if we are expecting a slice (target)
	// We can try blindly unwrapping if direct failed
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(jsonBytes, &wrapper); err == nil {
		// Look for any field that is an array
		for _, raw := range wrapper {
			if len(raw) > 0 && raw[0] == '[' {
				if err := json.Unmarshal(raw, target); err == nil {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to parse JSON into target type")
}

func generateCurriculum(ctx context.Context, lm dsgo.LM, research *types.ResearchFindings) (*types.Curriculum, error) {
	curriculumAgent := agents.NewCurriculumAgent(lm)

	// Convert research findings to JSON for the curriculum agent
	researchJSON, _ := json.Marshal(research.SubTopics)

	result, err := curriculumAgent.Forward(ctx, map[string]any{
		"unifiedKnowledge":   research.Unified.UnifiedKnowledge,
		"researchFindings":   string(researchJSON),
		"keyTakeaways":       research.Unified.KeyTakeaways,
		"learningPath":       research.Unified.LearningPath,
		"practicalExercises": research.Unified.PracticalExercises,
		"learningObjectives": "Master " + getEnvOrDefault("TRAINER_TOPIC", DefaultTopic),
		"skillLevel":         getEnvOrDefault("TRAINER_SKILL_LEVEL", "intermediate"),
		"estimatedDuration":  "2-3 hours",
	})
	if err != nil {
		return nil, fmt.Errorf("curriculum agent failed: %w", err)
	}

	// Parse JSON outputs
	var modules []types.Module
	var quizzes []types.Quiz
	var exercises []types.PracticalExercise
	var challenges []types.Challenge
	var glossary []types.GlossaryEntry
	var resources []types.Resource

	if err := parseJSONField(result, "modules", &modules); err != nil {
		log.Printf("Warning: Failed to parse modules: %v. Raw: %v", err, result.Outputs["modules"])
	}
	if err := parseJSONField(result, "quizzes", &quizzes); err != nil {
		log.Printf("Warning: Failed to parse quizzes: %v. Raw: %v", err, result.Outputs["quizzes"])
	}
	if err := parseJSONField(result, "exercises", &exercises); err != nil {
		log.Printf("Warning: Failed to parse exercises: %v", err)
	}
	if err := parseJSONField(result, "challenges", &challenges); err != nil {
		log.Printf("Warning: Failed to parse challenges: %v", err)
	}
	if err := parseJSONField(result, "glossary", &glossary); err != nil {
		log.Printf("Warning: Failed to parse glossary: %v", err)
		// Fallback: try map[string]string
		var glossaryMap map[string]string
		if err2 := parseJSONField(result, "glossary", &glossaryMap); err2 == nil {
			for term, def := range glossaryMap {
				glossary = append(glossary, types.GlossaryEntry{
					Term:       term,
					Definition: def,
					Category:   "General",
				})
			}
		}
	}
	if err := parseJSONField(result, "resources", &resources); err != nil {
		log.Printf("Warning: Failed to parse resources: %v", err)
		// Fallback: try []string
		var urls []string
		if err2 := parseJSONField(result, "resources", &urls); err2 == nil {
			for _, u := range urls {
				resources = append(resources, types.Resource{
					Title:       "Resource",
					URL:         u,
					Type:        "Link",
					Description: "Reference link",
				})
			}
		}
	}

	return &types.Curriculum{
		Modules:         modules,
		Quizzes:         quizzes,
		Exercises:       exercises,
		Challenges:      challenges,
		Glossary:        glossary,
		Resources:       resources,
		GeneratedAt:     time.Now(),
		ResearchSources: research.Sources,
		Topic:           getEnvOrDefault("TRAINER_TOPIC", DefaultTopic),
		SkillLevel:      getEnvOrDefault("TRAINER_SKILL_LEVEL", "intermediate"),
		EstimatedTime:   "2-3 hours",
	}, nil
}

func countQuestions(curriculum *types.Curriculum) int {
	total := 0
	for _, quiz := range curriculum.Quizzes {
		total += len(quiz.Questions)
	}
	return total
}
