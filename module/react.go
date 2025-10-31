package module

import (
	"context"
	"fmt"
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
		Adapter:       dsgo.NewFallbackAdapter().WithReasoning(true),
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

			// Use adapter to parse output
			outputs, err := r.Adapter.Parse(r.Signature, result.Content)
			if err != nil {
				// FALLBACK: If structured parsing fails, attempt text extraction for string fields
				// This makes ReAct resilient to less capable models that don't follow structured formats
				extractedOutputs := r.extractTextOutputs(result.Content, messages)
				if len(extractedOutputs) > 0 {
					if r.Verbose {
						fmt.Println("⚠️  Structured parsing failed - falling back to raw text extraction")
					}
					outputs = extractedOutputs
				} else {
					return nil, fmt.Errorf("failed to parse final answer: %w", err)
				}
			}

			// Apply output normalization and coercion before validation
			outputs = dsgo.NormalizeOutputKeys(r.Signature, outputs)

			if err := r.Signature.ValidateOutputs(outputs); err != nil {
				return nil, fmt.Errorf("output validation failed: %w", err)
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

	return nil, fmt.Errorf("exceeded maximum iterations (%d) without reaching final answer", r.MaxIterations)
}

func (r *ReAct) buildSystemPrompt() string {
	// Don't build system prompt if only the finish tool exists (no real tools)
	if len(r.Tools) == 0 || (len(r.Tools) == 1 && r.Tools[0].Name == "finish") {
		return ""
	}

	var prompt strings.Builder
	prompt.WriteString("You are a helpful AI assistant that can use tools to answer questions.\n\n")
	prompt.WriteString("Follow the ReAct (Reasoning and Acting) pattern:\n")
	prompt.WriteString("1. Think: Reason about the problem and what information you need\n")
	prompt.WriteString("2. Act: Use available tools to gather information\n")
	prompt.WriteString("3. Observe: Analyze the tool results\n")
	prompt.WriteString("4. Repeat until you have enough information to provide a final answer\n\n")
	prompt.WriteString("When you have gathered sufficient information, call the 'finish' tool with your complete answer. ")
	prompt.WriteString("The finish tool takes all the required output fields as parameters and cleanly concludes your research.\n")

	return prompt.String()
}

func (r *ReAct) buildFinalAnswerPrompt() string {
	var prompt strings.Builder
	prompt.WriteString("Based on all the information gathered above, please provide your final answer now.\n\n")

	// Add output format specification
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

	prompt.WriteString("\nIMPORTANT: Provide a complete answer based on the observations you've gathered. ")
	prompt.WriteString("Return ONLY valid JSON with all required fields.\n")

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
