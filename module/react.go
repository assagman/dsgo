package module

import (
	"context"
	"encoding/json"
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
	MaxIterations int
	Verbose       bool
}

// NewReAct creates a new ReAct module
func NewReAct(signature *dsgo.Signature, lm dsgo.LM, tools []dsgo.Tool) *ReAct {
	return &ReAct{
		Signature:     signature,
		LM:            lm,
		Tools:         tools,
		Options:       dsgo.DefaultGenerateOptions(),
		MaxIterations: MaxReActIterations,
		Verbose:       false,
	}
}

// WithOptions sets custom generation options
func (r *ReAct) WithOptions(options *dsgo.GenerateOptions) *ReAct {
	r.Options = options
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
func (r *ReAct) Forward(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	if err := r.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	messages := []dsgo.Message{}
	systemPrompt := r.buildSystemPrompt()
	if systemPrompt != "" {
		messages = append(messages, dsgo.Message{Role: "system", Content: systemPrompt})
	}

	userPrompt, err := r.buildInitialPrompt(inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to build initial prompt: %w", err)
	}
	messages = append(messages, dsgo.Message{Role: "user", Content: userPrompt})

	// ReAct loop: Thought -> Action -> Observation
	for i := 0; i < r.MaxIterations; i++ {
		if r.Verbose {
			fmt.Printf("\n=== ReAct Iteration %d ===\n", i+1)
		}

		options := r.Options
		if r.LM.SupportsTools() && len(r.Tools) > 0 {
			options.Tools = r.Tools
			options.ToolChoice = "auto"
		}

		result, err := r.LM.Generate(ctx, messages, options)
		if err != nil {
			return nil, fmt.Errorf("LM generation failed at iteration %d: %w", i, err)
		}

		// If no tool calls, this should be the final answer
		if len(result.ToolCalls) == 0 {
			if r.Verbose {
				fmt.Printf("Thought: %s\n", result.Content)
				fmt.Println("Action: None (Final Answer)")
			}

			outputs, err := r.parseFinalAnswer(result.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to parse final answer: %w", err)
			}

			if err := r.Signature.ValidateOutputs(outputs); err != nil {
				return nil, fmt.Errorf("output validation failed: %w", err)
			}

			return outputs, nil
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
		for _, toolCall := range result.ToolCalls {
			if r.Verbose {
				fmt.Printf("Action: %s(%v)\n", toolCall.Name, toolCall.Arguments)
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
		}
	}

	return nil, fmt.Errorf("exceeded maximum iterations (%d) without reaching final answer", r.MaxIterations)
}

func (r *ReAct) buildSystemPrompt() string {
	if len(r.Tools) == 0 {
		return ""
	}

	var prompt strings.Builder
	prompt.WriteString("You are a helpful AI assistant that can use tools to answer questions.\n\n")
	prompt.WriteString("Follow the ReAct (Reasoning and Acting) pattern:\n")
	prompt.WriteString("1. Think: Reason about the problem and what information you need\n")
	prompt.WriteString("2. Act: Use available tools to gather information\n")
	prompt.WriteString("3. Observe: Analyze the tool results\n")
	prompt.WriteString("4. Repeat until you have enough information to provide a final answer\n\n")
	prompt.WriteString("When you have gathered sufficient information, provide your final answer in the required JSON format without calling any more tools.\n")

	return prompt.String()
}

func (r *ReAct) buildInitialPrompt(inputs map[string]any) (string, error) {
	var prompt strings.Builder

	if r.Signature.Description != "" {
		prompt.WriteString(r.Signature.Description)
		prompt.WriteString("\n\n")
	}

	// Add input fields
	if len(r.Signature.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range r.Signature.InputFields {
			value, exists := inputs[field.Name]
			if !exists {
				return "", fmt.Errorf("missing required input field: %s", field.Name)
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("%s (%s): %v\n", field.Name, field.Description, value))
			} else {
				prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
			}
		}
		prompt.WriteString("\n")
	}

	// Add output format specification
	if len(r.Signature.OutputFields) > 0 {
		prompt.WriteString("--- Final Answer Format ---\n")
		prompt.WriteString("When you have enough information, respond with a JSON object containing:\n")
		prompt.WriteString("- reasoning (string): Your step-by-step thought process\n")
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
	}

	return prompt.String(), nil
}

func (r *ReAct) findTool(name string) *dsgo.Tool {
	for i := range r.Tools {
		if r.Tools[i].Name == name {
			return &r.Tools[i]
		}
	}
	return nil
}

func (r *ReAct) parseFinalAnswer(content string) (map[string]any, error) {
	content = strings.TrimSpace(content)

	// Try to extract JSON from markdown code blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + 3
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	} else {
		// Try to find JSON object in the content
		start := strings.Index(content, "{")
		if start >= 0 {
			// Find matching closing brace, accounting for strings
			depth := 0
			inString := false
			escape := false
			for i := start; i < len(content); i++ {
				if escape {
					escape = false
					continue
				}
				if content[i] == '\\' {
					escape = true
					continue
				}
				if content[i] == '"' {
					inString = !inString
				}
				if !inString {
					if content[i] == '{' {
						depth++
					} else if content[i] == '}' {
						depth--
						if depth == 0 {
							content = content[start : i+1]
							break
						}
					}
				}
			}
		}
	}

	content = strings.TrimSpace(content)

	// Fix common JSON issues: unescaped newlines in strings
	content = fixJSONNewlines(content)

	var outputs map[string]any
	if err := json.Unmarshal([]byte(content), &outputs); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w (content: %s)", err, content)
	}

	return outputs, nil
}

// fixJSONNewlines attempts to fix unescaped newlines in JSON strings
func fixJSONNewlines(jsonStr string) string {
	var result strings.Builder
	inString := false
	escape := false

	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]

		if escape {
			result.WriteByte(ch)
			escape = false
			continue
		}

		if ch == '\\' {
			result.WriteByte(ch)
			escape = true
			continue
		}

		if ch == '"' {
			inString = !inString
			result.WriteByte(ch)
			continue
		}

		// If we're inside a string and encounter a newline, escape it
		if inString && (ch == '\n' || ch == '\r') {
			if ch == '\n' {
				result.WriteString("\\n")
			}
			// Skip \r as it's often paired with \n
			continue
		}

		result.WriteByte(ch)
	}

	return result.String()
}
