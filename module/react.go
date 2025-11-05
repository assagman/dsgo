package module

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/assagman/dsgo"
)

const (
	MaxReActIterations = 10
)

// ReAct implements the Reasoning and Acting pattern
type ReAct struct {
	Signature     *dsgo.Signature
	LM            dsgo.LM
	Tools         []dsgo.Tool
	Options       *dsgo.GenerateOptions
	Adapter       dsgo.Adapter
	History       *dsgo.History  // Optional conversation history
	Demos         []dsgo.Example // Optional few-shot examples
	MaxIterations int
	Verbose       bool
}

// NewReAct creates a new ReAct module
func NewReAct(signature *dsgo.Signature, lm dsgo.LM, tools []dsgo.Tool) *ReAct {
	r := &ReAct{
		Signature:     signature,
		LM:            lm,
		Tools:         tools,
		Options:       dsgo.DefaultGenerateOptions(),
		Adapter:       dsgo.NewFallbackAdapter(),
		MaxIterations: MaxReActIterations,
		Verbose:       false,
	}

	// AUTO-INJECT finish tool if not present
	if r.findTool("finish") == nil {
		finishTool := buildFinishTool(signature)
		r.Tools = append(r.Tools, *finishTool)
	}

	return r
}

// WithOptions sets custom generation options
func (r *ReAct) WithOptions(options *dsgo.GenerateOptions) *ReAct {
	r.Options = options
	return r
}

// WithAdapter sets a custom adapter
func (r *ReAct) WithAdapter(adapter dsgo.Adapter) *ReAct {
	r.Adapter = adapter
	return r
}

// WithHistory sets conversation history for multi-turn interactions
func (r *ReAct) WithHistory(history *dsgo.History) *ReAct {
	r.History = history
	return r
}

// WithDemos sets few-shot examples for in-context learning
func (r *ReAct) WithDemos(demos []dsgo.Example) *ReAct {
	r.Demos = demos
	return r
}

// WithMaxIterations sets the maximum number of ReAct iterations
func (r *ReAct) WithMaxIterations(max int) *ReAct {
	r.MaxIterations = max
	return r
}

// WithVerbose enables verbose logging
func (r *ReAct) WithVerbose(verbose bool) *ReAct {
	r.Verbose = verbose
	return r
}

// GetSignature returns the module's signature
func (r *ReAct) GetSignature() *dsgo.Signature {
	return r.Signature
}

// Forward executes the ReAct loop
func (r *ReAct) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	if err := r.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Use adapter to format messages with demos
	newMessages, err := r.Adapter.Format(r.Signature, inputs, r.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Build initial message list
	var messages []dsgo.Message

	// Add system prompt for ReAct pattern
	systemPrompt := r.buildSystemPrompt()
	if systemPrompt != "" {
		messages = append(messages, dsgo.Message{Role: "system", Content: systemPrompt})
	}

	// Prepend history if available
	if r.History != nil && !r.History.IsEmpty() {
		historyMessages := r.Adapter.FormatHistory(r.History)
		messages = append(messages, historyMessages...)
	}

	// Add new messages from adapter
	messages = append(messages, newMessages...)

	// Track observations for stagnation detection
	var lastObservation string
	var finalMode bool

	// ReAct loop: Thought -> Action -> Observation
	for i := 0; i < r.MaxIterations; i++ {
		if r.Verbose {
			fmt.Printf("\n=== ReAct Iteration %d ===\n", i+1)
		}

		// Activate final mode on last iteration
		if i == r.MaxIterations-1 {
			finalMode = true
			if r.Verbose {
				fmt.Println("⚠️  Final iteration - forcing final answer mode")
			}
		}

		// Copy options to avoid mutation
		options := r.Options.Copy()

		// In final mode, disable tools and inject instruction for final answer
		if finalMode {
			options.Tools = nil
			options.ToolChoice = "none"

			// Inject user message to prompt for final answer
			finalPrompt := r.buildFinalAnswerPrompt()
			messages = append(messages, dsgo.Message{
				Role:    "user",
				Content: finalPrompt,
			})

			if r.LM.SupportsJSON() {
				options.ResponseFormat = "json"
				// Auto-generate JSON schema from signature for structured outputs
				if options.ResponseSchema == nil {
					options.ResponseSchema = r.Signature.SignatureToJSONSchema()
				}
			}
		} else {
			// Normal mode: enable tools if available
			if r.LM.SupportsTools() && len(r.Tools) > 0 {
				options.Tools = r.Tools
				options.ToolChoice = "auto"
			}
		}

		// Enable JSON mode when tools are not used (for final answer)
		if r.LM.SupportsJSON() && len(options.Tools) == 0 {
			if _, isJSON := r.Adapter.(*dsgo.JSONAdapter); isJSON {
				options.ResponseFormat = "json"
				// Auto-generate JSON schema from signature for structured outputs
				if options.ResponseSchema == nil {
					options.ResponseSchema = r.Signature.SignatureToJSONSchema()
				}
			}
		}

		result, err := r.LM.Generate(ctx, messages, options)
		if err != nil {
			return nil, fmt.Errorf("LM generation failed at iteration %d: %w", i+1, err)
		}

		// If no tool calls, this should be the final answer
		if len(result.ToolCalls) == 0 {
			if r.Verbose {
				fmt.Printf("Thought: %s\n", result.Content)
				fmt.Println("Action: None (Final Answer)")
			}

			// Apply hardened parsing (P2)
			cleanedContent := stripToJSON(result.Content)

			// Use adapter to parse output
			outputs, err := r.Adapter.Parse(r.Signature, cleanedContent)
			if err != nil {
				// If in early iterations and parsing fails, guide model to use tools instead of accepting bad output
				if !finalMode && i < r.MaxIterations-2 {
					if r.Verbose {
						fmt.Println("⚠️  Parsing failed and tools available - requesting tool use")
					}
					messages = append(messages, dsgo.Message{
						Role:    "assistant",
						Content: result.Content,
					})
					messages = append(messages, dsgo.Message{
						Role:    "user",
						Content: "Please use the available tools to gather the information needed, then provide a complete answer in the requested format. Do not include any meta-commentary or explanations - just the answer.",
					})
					continue
				}

				// If in final mode and parsing fails, run extraction (P1)
				if finalMode {
					if r.Verbose {
						fmt.Println("⚠️  Final answer parsing failed - running extraction")
					}
					return r.runExtract(ctx, messages, inputs)
				}

				// FALLBACK: If structured parsing fails, attempt text extraction for string fields
				// This makes ReAct resilient to less capable models that don't follow structured formats
				extractedOutputs := r.extractTextOutputs(cleanedContent, messages)
				if len(extractedOutputs) > 0 {
					if r.Verbose {
						fmt.Println("⚠️  Structured parsing failed - falling back to raw text extraction")
					}
					outputs = extractedOutputs
				} else {
					// Last resort: run extraction
					if r.Verbose {
						fmt.Println("⚠️  All parsing failed - running extraction")
					}
					return r.runExtract(ctx, messages, inputs)
				}
			}

			// Apply type coercion (P2)
			outputs = coerceBasicTypes(r.Signature, outputs)

			// Apply output normalization
			outputs = dsgo.NormalizeOutputKeys(r.Signature, outputs)

			// Use partial validation to allow missing optional fields
			if err := r.Signature.ValidateOutputs(outputs); err != nil {
				// Validation failed - try extraction as fallback
				if r.Verbose {
					fmt.Printf("⚠️  Output validation failed: %v - running extraction\n", err)
				}
				return r.runExtract(ctx, messages, inputs)
			}

			// Extract adapter metadata
			adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

			// Extract rationale if present
			rationale := ""
			if reasoning, exists := outputs["reasoning"]; exists {
				rationale = fmt.Sprintf("%v", reasoning)
				// Remove reasoning from outputs if not part of signature
				if r.Signature.GetOutputField("reasoning") == nil {
					delete(outputs, "reasoning")
				}
			}

			// Update history if present
			if r.History != nil {
				// Add only the new user message(s) (not from history)
				for _, msg := range newMessages {
					if msg.Role == "user" {
						r.History.Add(msg)
					}
				}

				// Add assistant response
				r.History.Add(dsgo.Message{
					Role:    "assistant",
					Content: result.Content,
				})
			}

			// Build Prediction object
			prediction := dsgo.NewPrediction(outputs).
				WithRationale(rationale).
				WithUsage(result.Usage).
				WithModuleName("ReAct").
				WithInputs(inputs)

			// Add adapter metrics if available
			if adapterUsed != "" {
				prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
			}

			return prediction, nil
		}

		// Add assistant's response with tool calls
		messages = append(messages, dsgo.Message{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		})

		if r.Verbose {
			fmt.Printf("Thought: %s\n", result.Content)
		}

		// Execute tool calls and add observations
		var currentObservation string
		for _, toolCall := range result.ToolCalls {
			if r.Verbose {
				fmt.Printf("Action: %s(%v)\n", toolCall.Name, toolCall.Arguments)
			}

			// Check if this is a "finish" tool call - treat as final answer
			if strings.ToLower(toolCall.Name) == "finish" {
				if r.Verbose {
					fmt.Println("Finish tool called - extracting final answer")
				}

				// Extract outputs from finish tool arguments
				outputs := make(map[string]any)
				for k, v := range toolCall.Arguments {
					outputs[k] = v
				}

				// Validate outputs match signature
				if err := r.Signature.ValidateOutputs(outputs); err != nil {
					// If finish tool args don't match signature, continue and let model try again
					observation := fmt.Sprintf("Error: finish tool arguments don't match required outputs: %v", err)
					messages = append(messages, dsgo.Message{
						Role:    "tool",
						Content: observation,
						ToolID:  toolCall.ID,
					})
					if r.Verbose {
						fmt.Printf("Observation: %s\n", observation)
					}
					currentObservation = observation
					continue
				}

				// Build prediction and return
				prediction := dsgo.NewPrediction(outputs).
					WithUsage(result.Usage).
					WithModuleName("ReAct").
					WithInputs(inputs)

				return prediction, nil
			}

			tool := r.findTool(toolCall.Name)
			if tool == nil {
				observation := fmt.Sprintf("Error: Tool '%s' not found", toolCall.Name)
				messages = append(messages, dsgo.Message{
					Role:    "tool",
					Content: observation,
					ToolID:  toolCall.ID,
				})
				if r.Verbose {
					fmt.Printf("Observation: %s\n", observation)
				}
				currentObservation = observation
				continue
			}

			result, err := tool.Execute(ctx, toolCall.Arguments)
			if err != nil {
				observation := fmt.Sprintf("Error executing tool: %v", err)
				messages = append(messages, dsgo.Message{
					Role:    "tool",
					Content: observation,
					ToolID:  toolCall.ID,
				})
				if r.Verbose {
					fmt.Printf("Observation: %s\n", observation)
				}
				currentObservation = observation
				continue
			}

			observation := fmt.Sprintf("%v", result)
			messages = append(messages, dsgo.Message{
				Role:    "tool",
				Content: observation,
				ToolID:  toolCall.ID,
			})
			if r.Verbose {
				fmt.Printf("Observation: %s\n", observation)
			}
			currentObservation = observation
		}

		// Detect stagnation: if same observation appears twice in a row, force final answer
		if currentObservation != "" && currentObservation == lastObservation {
			if r.Verbose {
				fmt.Println("\n⚠️  Stagnation detected - activating final mode")
			}
			finalMode = true
			messages = append(messages, dsgo.Message{
				Role:    "user",
				Content: "You've received the same observation twice. Please provide your final answer now as a JSON object with all required fields. Do not call any more tools.",
			})
		}
		lastObservation = currentObservation
	}

	// Max iterations exceeded - run extraction to salvage an answer (P1)
	if r.Verbose {
		fmt.Printf("\n⚠️  Exceeded maximum iterations (%d) - running extraction\n", r.MaxIterations)
	}
	return r.runExtract(ctx, messages, inputs)
}

func (r *ReAct) buildSystemPrompt() string {
	// Don't build system prompt if only the finish tool exists (no real tools)
	if len(r.Tools) == 0 || (len(r.Tools) == 1 && r.Tools[0].Name == "finish") {
		return ""
	}

	var prompt strings.Builder
	prompt.WriteString("You are a helpful AI assistant that uses tools to answer questions.\n\n")
	prompt.WriteString("Follow these steps:\n")
	prompt.WriteString("1. Use the available tools to gather the information you need\n")
	prompt.WriteString("2. Once you have enough information, call the 'finish' tool with your complete answer\n")
	prompt.WriteString("3. If you already have the answer, call 'finish' immediately\n\n")

	prompt.WriteString("IMPORTANT:\n")
	prompt.WriteString("- Use the native tool calling mechanism\n")
	prompt.WriteString("- Do NOT write textual representations like 'Action: search(...)' or 'Thought:'\n")
	prompt.WriteString("- When calling 'finish', provide ALL required fields in the tool arguments\n")
	prompt.WriteString("- Do not include explanations or meta-commentary\n")

	return prompt.String()
}

func (r *ReAct) buildFinalAnswerPrompt() string {
	var prompt strings.Builder
	prompt.WriteString("Based on all the information gathered above, please provide your final answer now.\n\n")

	// Add output format specification (P3: clearer JSON instructions)
	prompt.WriteString("Respond with a valid JSON object containing these fields:\n")
	for _, field := range r.Signature.OutputFields {
		optional := ""
		if field.Optional {
			optional = " (optional)"
		}
		classInfo := ""
		if field.Type == dsgo.FieldTypeClass && len(field.Classes) > 0 {
			classInfo = fmt.Sprintf(" [one of: %s]", strings.Join(field.Classes, ", "))
		}
		if field.Description != "" {
			prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
		} else {
			prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
		}
	}

	prompt.WriteString("\nCRITICAL REQUIREMENTS:\n")
	prompt.WriteString("- Return ONLY a valid JSON object (no code fences, no explanations)\n")
	prompt.WriteString("- Include all required fields with appropriate values\n")
	prompt.WriteString("- Use the exact field names specified above\n")
	prompt.WriteString("- Provide a complete answer based on all observations you've gathered\n")

	return prompt.String()
}

func (r *ReAct) findTool(name string) *dsgo.Tool {
	for i := range r.Tools {
		if r.Tools[i].Name == name {
			return &r.Tools[i]
		}
	}
	return nil
}

// extractTextOutputs attempts to extract output fields from raw text when structured parsing fails
// This is a last-resort fallback for less capable models that don't follow JSON/Chat formats
func (r *ReAct) extractTextOutputs(content string, messages []dsgo.Message) map[string]any {
	outputs := make(map[string]any)
	content = strings.TrimSpace(content)

	// If content is empty or very short, try to synthesize from message history
	if len(content) < 10 {
		if r.Verbose {
			fmt.Println("⚠️  Content too short, synthesizing from observations")
		}
		content = r.synthesizeAnswerFromHistory(messages)
	}

	// Only attempt extraction for string fields
	var stringFields []dsgo.Field
	for _, field := range r.Signature.OutputFields {
		if field.Type == dsgo.FieldTypeString {
			stringFields = append(stringFields, field)
		}
	}

	if len(stringFields) == 0 {
		return nil
	}

	// Strategy 1: If only one string field and it's "answer", use entire content
	if len(stringFields) == 1 && stringFields[0].Name == "answer" {
		outputs["answer"] = content
		return outputs
	}

	// Strategy 2: For multiple fields, try simple heuristics
	// - If all required fields are strings, split content or use entire content for primary field
	var primaryFieldName string
	if answerField := r.Signature.GetOutputField("answer"); answerField != nil {
		primaryFieldName = "answer"
	} else if len(stringFields) > 0 {
		primaryFieldName = stringFields[0].Name
	}

	if primaryFieldName != "" {
		// Use entire content for primary field (answer)
		outputs[primaryFieldName] = content

		// For other string fields, try to provide reasonable defaults
		for _, field := range stringFields {
			if field.Name != primaryFieldName {
				if field.Name == "sources" {
					// Extract mentions that look like sources
					outputs["sources"] = "Based on search results and tool observations"
				} else if !field.Optional {
					// Provide placeholder for required fields
					outputs[field.Name] = content
				}
			}
		}
	}

	return outputs
}

// synthesizeAnswerFromHistory extracts and summarizes observations from the message history
// Used as a fallback when the model produces empty content in final mode
func (r *ReAct) synthesizeAnswerFromHistory(messages []dsgo.Message) string {
	var observations []string

	// Extract tool observations from message history
	for _, msg := range messages {
		if msg.Role == "tool" && msg.Content != "" {
			// Skip error messages
			if !strings.HasPrefix(msg.Content, "Error:") {
				observations = append(observations, msg.Content)
			}
		}
	}

	if len(observations) == 0 {
		return "No information available from tools"
	}

	// Use the most recent relevant observation
	// Take the last non-duplicate observation
	seen := make(map[string]bool)
	var uniqueObs []string
	for i := len(observations) - 1; i >= 0 && len(uniqueObs) < 3; i-- {
		obs := observations[i]
		if !seen[obs] && len(obs) > 20 {
			uniqueObs = append([]string{obs}, uniqueObs...)
			seen[obs] = true
		}
	}

	if len(uniqueObs) > 0 {
		return strings.Join(uniqueObs, " ")
	}

	return observations[len(observations)-1]
}

// buildFinishTool creates a synthetic "finish" tool that allows models to explicitly
// conclude the ReAct loop by providing final outputs matching the signature
func buildFinishTool(signature *dsgo.Signature) *dsgo.Tool {
	tool := dsgo.NewTool(
		"finish",
		"Call this tool when you have gathered enough information and are ready to provide the final answer. Use the tool arguments to provide your complete answer.",
		func(ctx context.Context, args map[string]any) (any, error) {
			// This tool is intercepted in Forward() before execution
			return "Final answer provided", nil
		},
	)

	// Add parameters matching the output signature
	for _, field := range signature.OutputFields {
		description := field.Description
		if description == "" {
			description = fmt.Sprintf("The %s field of the final answer", field.Name)
		}

		// Determine parameter type
		paramType := "string"
		switch field.Type {
		case dsgo.FieldTypeInt:
			paramType = "number"
		case dsgo.FieldTypeBool:
			paramType = "boolean"
		}

		// Add class information to description
		if field.Type == dsgo.FieldTypeClass && len(field.Classes) > 0 {
			description = fmt.Sprintf("%s (one of: %s)", description, strings.Join(field.Classes, ", "))
		}

		tool.AddParameter(field.Name, paramType, description, !field.Optional)
	}

	return tool
}

// stripToJSON removes common LLM artifacts from JSON output
// Handles: code fences, trailing commentary, leading/trailing text
func stripToJSON(content string) string {
	content = strings.TrimSpace(content)

	// Remove markdown code fences
	re := regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")
	if matches := re.FindStringSubmatch(content); len(matches) > 1 {
		content = strings.TrimSpace(matches[1])
	}

	// Find JSON object boundaries
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start != -1 && end != -1 && end > start {
		content = content[start : end+1]
	}

	return strings.TrimSpace(content)
}

// coerceBasicTypes handles type mismatches in parsed outputs
// Converts: string numbers to ints, string bools to bools, etc.
func coerceBasicTypes(signature *dsgo.Signature, outputs map[string]any) map[string]any {
	coerced := make(map[string]any)

	for key, value := range outputs {
		field := signature.GetOutputField(key)
		if field == nil {
			coerced[key] = value
			continue
		}

		switch field.Type {
		case dsgo.FieldTypeInt:
			// Try to convert string to int
			if strVal, ok := value.(string); ok {
				// Extract first number from string (e.g., "5 years" -> 5)
				re := regexp.MustCompile(`-?\d+`)
				if match := re.FindString(strVal); match != "" {
					if intVal, err := strconv.Atoi(match); err == nil {
						coerced[key] = intVal
						continue
					}
				}
			}
			// Try float64 (from JSON unmarshaling) to int
			if floatVal, ok := value.(float64); ok {
				coerced[key] = int(floatVal)
				continue
			}
			coerced[key] = value

		case dsgo.FieldTypeBool:
			// Try to convert string to bool
			if strVal, ok := value.(string); ok {
				strVal = strings.ToLower(strings.TrimSpace(strVal))
				if strVal == "true" || strVal == "yes" || strVal == "1" {
					coerced[key] = true
					continue
				}
				if strVal == "false" || strVal == "no" || strVal == "0" {
					coerced[key] = false
					continue
				}
			}
			coerced[key] = value

		case dsgo.FieldTypeString:
			// Convert any type to string if needed
			if value != nil {
				coerced[key] = fmt.Sprintf("%v", value)
			} else {
				coerced[key] = value
			}

		default:
			coerced[key] = value
		}
	}

	return coerced
}

// runExtract performs post-loop extraction to synthesize a final answer
// from the accumulated message history (trajectory). This is the critical
// fallback that ensures ReAct always returns something, even if the main
// loop fails or produces unparseable output.
//
// This phase uses a temporary adapter WITH reasoning enabled, mimicking
// ChainOfThought behavior during extraction.
func (r *ReAct) runExtract(ctx context.Context, messages []dsgo.Message, inputs map[string]any) (*dsgo.Prediction, error) {
	if r.Verbose {
		fmt.Println("\n=== Running Post-Loop Extraction (with reasoning) ===")
	}

	// Build extraction prompt
	extractPrompt := r.buildExtractionPrompt()

	// Append extraction request to message history
	extractMessages := make([]dsgo.Message, len(messages))
	copy(extractMessages, messages)
	extractMessages = append(extractMessages, dsgo.Message{
		Role:    "user",
		Content: extractPrompt,
	})

	// Copy options and force JSON mode
	options := r.Options.Copy()
	options.Tools = nil
	options.ToolChoice = "none"

	if r.LM.SupportsJSON() {
		options.ResponseFormat = "json"
		if options.ResponseSchema == nil {
			options.ResponseSchema = r.Signature.SignatureToJSONSchema()
		}
	}

	// Generate extraction
	result, err := r.LM.Generate(ctx, extractMessages, options)
	if err != nil {
		return nil, fmt.Errorf("extraction generation failed: %w", err)
	}

	if r.Verbose {
		fmt.Printf("Extraction response: %s\n", result.Content)
	}

	// Apply hardened parsing
	cleanedContent := stripToJSON(result.Content)

	// Create temporary adapter WITH reasoning for extraction phase
	extractAdapter := dsgo.NewFallbackAdapter().WithReasoning(true)

	// Try adapter parsing first (with reasoning)
	outputs, err := extractAdapter.Parse(r.Signature, cleanedContent)
	if err != nil {
		// Fallback: try direct JSON parsing
		outputs = make(map[string]any)
		if jsonErr := json.Unmarshal([]byte(cleanedContent), &outputs); jsonErr != nil {
			// Last resort: extract text outputs
			outputs = r.extractTextOutputs(cleanedContent, extractMessages)
			if len(outputs) == 0 {
				return nil, fmt.Errorf("extraction failed to parse output: %w (JSON error: %v)", err, jsonErr)
			}
		}
	}

	// Extract and remove rationale/reasoning field from outputs
	var rationale string
	if val, ok := outputs["rationale"]; ok {
		if str, ok := val.(string); ok {
			rationale = str
			delete(outputs, "rationale")
		}
	}
	if rationale == "" {
		if val, ok := outputs["reasoning"]; ok {
			if str, ok := val.(string); ok {
				rationale = str
				delete(outputs, "reasoning")
			}
		}
	}

	// Apply type coercion
	outputs = coerceBasicTypes(r.Signature, outputs)

	// Apply output normalization
	outputs = dsgo.NormalizeOutputKeys(r.Signature, outputs)

	// Use partial validation (allow missing optional fields)
	diagnostics := r.Signature.ValidateOutputsPartial(outputs)

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

	// Build prediction with diagnostics and rationale
	pred := &dsgo.Prediction{
		Outputs:          outputs,
		Usage:            result.Usage,
		AdapterUsed:      adapterUsed,
		ParseAttempts:    parseAttempts,
		FallbackUsed:     fallbackUsed,
		ParseDiagnostics: diagnostics,
	}

	// Attach rationale if found
	if rationale != "" {
		pred = pred.WithRationale(rationale)
		if r.Verbose {
			fmt.Printf("Extracted rationale: %s\n", rationale)
		}
	}

	if r.Verbose {
		fmt.Printf("Extracted outputs: %+v\n", outputs)
		if diagnostics != nil && diagnostics.HasErrors() {
			fmt.Printf("⚠️  Extraction diagnostics: %v\n", diagnostics)
		}
	}

	return pred, nil
}

// buildExtractionPrompt creates a prompt for post-loop extraction
func (r *ReAct) buildExtractionPrompt() string {
	var prompt strings.Builder
	prompt.WriteString("Based on the conversation above, including all tool observations and reasoning, ")
	prompt.WriteString("please synthesize a final answer now.\n\n")

	prompt.WriteString("Respond with a JSON object containing:\n")
	for _, field := range r.Signature.OutputFields {
		optional := ""
		if field.Optional {
			optional = " (optional)"
		}
		classInfo := ""
		if field.Type == dsgo.FieldTypeClass && len(field.Classes) > 0 {
			classInfo = fmt.Sprintf(" [one of: %s]", strings.Join(field.Classes, ", "))
		}
		if field.Description != "" {
			prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
		} else {
			prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
		}
	}

	prompt.WriteString("\nIMPORTANT:\n")
	prompt.WriteString("- Use all information from the tool observations above\n")
	prompt.WriteString("- Provide your best answer even if some information is missing\n")
	prompt.WriteString("- Return ONLY valid JSON with the required fields\n")
	prompt.WriteString("- Do not include any explanations or commentary\n")

	return prompt.String()
}
