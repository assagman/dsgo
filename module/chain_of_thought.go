package module

import (
	"context"
	"fmt"

	"github.com/assagman/dsgo"
)

// ChainOfThought module encourages step-by-step reasoning
type ChainOfThought struct {
	Signature *dsgo.Signature
	LM        dsgo.LM
	Options   *dsgo.GenerateOptions
	Adapter   dsgo.Adapter
	History   *dsgo.History  // Optional conversation history
	Demos     []dsgo.Example // Optional few-shot examples
}

// NewChainOfThought creates a new ChainOfThought module
func NewChainOfThought(signature *dsgo.Signature, lm dsgo.LM) *ChainOfThought {
	return &ChainOfThought{
		Signature: signature,
		LM:        lm,
		Options:   dsgo.DefaultGenerateOptions(),
		Adapter:   dsgo.NewFallbackAdapter().WithReasoning(true),
	}
}

// WithOptions sets custom generation options
func (cot *ChainOfThought) WithOptions(options *dsgo.GenerateOptions) *ChainOfThought {
	cot.Options = options
	return cot
}

// WithAdapter sets a custom adapter
func (cot *ChainOfThought) WithAdapter(adapter dsgo.Adapter) *ChainOfThought {
	cot.Adapter = adapter
	return cot
}

// WithHistory sets conversation history for multi-turn interactions
func (cot *ChainOfThought) WithHistory(history *dsgo.History) *ChainOfThought {
	cot.History = history
	return cot
}

// WithDemos sets few-shot examples for in-context learning
func (cot *ChainOfThought) WithDemos(demos []dsgo.Example) *ChainOfThought {
	cot.Demos = demos
	return cot
}

// GetSignature returns the module's signature
func (cot *ChainOfThought) GetSignature() *dsgo.Signature {
	return cot.Signature
}

// Forward executes the chain of thought reasoning
func (cot *ChainOfThought) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	if err := cot.Signature.ValidateInputs(inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Use adapter to format messages with demos
	newMessages, err := cot.Adapter.Format(cot.Signature, inputs, cot.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Build final message list
	var messages []dsgo.Message

	// Prepend history if available
	if cot.History != nil && !cot.History.IsEmpty() {
		historyMessages := cot.Adapter.FormatHistory(cot.History)
		messages = append(messages, historyMessages...)
	}

	// Add new messages
	messages = append(messages, newMessages...)

	// Copy options to avoid mutation
	options := cot.Options.Copy()
	if cot.LM.SupportsJSON() {
		if _, isJSON := cot.Adapter.(*dsgo.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				options.ResponseSchema = cot.Signature.SignatureToJSONSchema()
			}
		}
	}

	result, err := cot.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	// Handle finish_reason: ChainOfThought doesn't support tool execution loops
	if result.FinishReason == "tool_calls" {
		return nil, fmt.Errorf("model requested tool execution (finish_reason=tool_calls) but ChainOfThought module doesn't support tool loops - use React module instead")
	}

	// Handle finish_reason=length: Model hit max_tokens, output truncated/incomplete
	if result.FinishReason == "length" {
		return nil, fmt.Errorf("model hit max_tokens limit (finish_reason=length) - output truncated - increase MaxTokens in options")
	}

	// Check for empty content with finish_reason=stop (actual error)
	if result.Content == "" && result.FinishReason == "stop" {
		return nil, fmt.Errorf("model returned empty content despite finish_reason=stop (model error)")
	}

	// Use adapter to parse output
	outputs, err := cot.Adapter.Parse(cot.Signature, result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if err := cot.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

	// Extract rationale from outputs
	rationale := ""
	if reasoning, exists := outputs["reasoning"]; exists {
		rationale = fmt.Sprintf("%v", reasoning)
		// Remove reasoning from outputs if not part of signature
		if cot.Signature.GetOutputField("reasoning") == nil {
			delete(outputs, "reasoning")
		}
	}

	// Update history if present
	if cot.History != nil {
		// Add only the new user message(s) (not from history)
		for _, msg := range newMessages {
			if msg.Role == "user" {
				cot.History.Add(msg)
			}
		}

		// Add assistant response
		cot.History.Add(dsgo.Message{
			Role:    "assistant",
			Content: result.Content,
		})
	}

	// Build Prediction object with rationale
	prediction := dsgo.NewPrediction(outputs).
		WithRationale(rationale).
		WithUsage(result.Usage).
		WithModuleName("ChainOfThought").
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}
