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
	return &ReAct{
		Signature:     signature,
		LM:            lm,
		Tools:         tools,
		Options:       dsgo.DefaultGenerateOptions(),
		Adapter:       dsgo.NewFallbackAdapter().WithReasoning(true),
		MaxIterations: MaxReActIterations,
		Verbose:       false,
	}
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

	// ReAct loop: Thought -> Action -> Observation
	for i := 0; i < r.MaxIterations; i++ {
		if r.Verbose {
			fmt.Printf("\n=== ReAct Iteration %d ===\n", i+1)
		}

		// Copy options to avoid mutation
		options := r.Options.Copy()
		if r.LM.SupportsTools() && len(r.Tools) > 0 {
			options.Tools = r.Tools
			options.ToolChoice = "auto"
		}

		// Enable JSON mode when tools are not used (for final answer)
		if r.LM.SupportsJSON() && len(options.Tools) == 0 {
			if _, isJSON := r.Adapter.(*dsgo.JSONAdapter); isJSON {
				options.ResponseFormat = "json"
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
				return nil, fmt.Errorf("failed to parse final answer: %w", err)
			}

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

func (r *ReAct) findTool(name string) *dsgo.Tool {
	for i := range r.Tools {
		if r.Tools[i].Name == name {
			return &r.Tools[i]
		}
	}
	return nil
}
