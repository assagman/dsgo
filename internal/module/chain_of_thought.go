package module

import (
	"context"
	"fmt"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
)

// ChainOfThought module encourages step-by-step reasoning
type ChainOfThought struct {
	Signature *core.Signature
	LM        core.LM
	Options   *core.GenerateOptions
	Adapter   core.Adapter
	History   *core.History  // Optional conversation history
	Demos     []core.Example // Optional few-shot examples
}

// NewChainOfThought creates a new ChainOfThought module
func NewChainOfThought(signature *core.Signature, lm core.LM) *ChainOfThought {
	return &ChainOfThought{
		Signature: signature,
		LM:        lm,
		Options:   core.DefaultGenerateOptions(),
		Adapter:   core.NewFallbackAdapter().WithReasoning(true),
	}
}

// WithOptions sets custom generation options
func (cot *ChainOfThought) WithOptions(options *core.GenerateOptions) *ChainOfThought {
	cot.Options = options
	return cot
}

// WithAdapter sets a custom adapter
func (cot *ChainOfThought) WithAdapter(adapter core.Adapter) *ChainOfThought {
	cot.Adapter = adapter
	return cot
}

// WithHistory sets conversation history for multi-turn interactions
func (cot *ChainOfThought) WithHistory(history *core.History) *ChainOfThought {
	cot.History = history
	return cot
}

// WithDemos sets few-shot examples for in-context learning
func (cot *ChainOfThought) WithDemos(demos []core.Example) *ChainOfThought {
	cot.Demos = demos
	return cot
}

// GetSignature returns the module's signature
func (cot *ChainOfThought) GetSignature() *core.Signature {
	return cot.Signature
}

// Forward executes the chain of thought reasoning
func (cot *ChainOfThought) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	// Ensure context has IDs
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleChainOfThought, cot.Signature.Description)

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModuleChainOfThought, time.Since(startTime), predErr)
	}()

	if err := cot.Signature.ValidateInputs(inputs); err != nil {
		predErr = fmt.Errorf("input validation failed: %w", err)
		return nil, predErr
	}

	// Check if structured outputs are enabled
	settings := core.GetSettings()
	useStructuredMode := settings.StructuredOutput.Enabled

	// If structured mode is enabled, use the structured output enforcement loop
	if useStructuredMode {
		return cot.forwardStructured(ctx, inputs)
	}

	// Otherwise, use the legacy path
	return cot.forwardLegacy(ctx, inputs)
}

// forwardStructured executes chain of thought with structured output enforcement
func (cot *ChainOfThought) forwardStructured(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	settings := core.GetSettings()

	// Select adapter based on LM capabilities
	adapter := cot.Adapter
	if cot.LM.SupportsJSON() && settings.StructuredOutput.Enabled {
		// Use schema-first adapter for LMs that support JSON mode
		adapter = core.NewSchemaFirstAdapter(true).WithReasoning(true)
	}

	// Build the new messages (without history) for History tracking
	newMessages, err := cot.Adapter.Format(cot.Signature, inputs, cot.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Create a custom adapter wrapper that includes history
	wrappedAdapter := &cotAdapter{
		base:    adapter,
		history: cot.History,
	}

	// Call structured output enforcement loop
	result, err := core.GenerateStructured(
		ctx,
		cot.LM,
		cot.Signature,
		inputs,
		cot.Demos,
		core.GenerateStructuredOptions{
			Adapter:        wrappedAdapter,
			BaseOptions:    cot.Options,
			MaxAttempts:    settings.StructuredOutput.MaxAttempts,
			Temperature:    settings.StructuredOutput.Temperature,
			UseJSONFormat:  cot.LM.SupportsJSON(),
			StreamCallback: cot.Options.StreamCallback,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("structured output generation failed: %w", err)
	}

	// Extract rationale from outputs
	rationale := ""
	outputs := result.Outputs
	if reasoning, exists := outputs["reasoning"]; exists {
		rationale = fmt.Sprintf("%v", reasoning)
		// Remove reasoning from outputs if not part of signature
		if cot.Signature.GetOutputField("reasoning") == nil {
			delete(outputs, "reasoning")
		}
	}

	// Build prediction from result
	pred := core.NewPrediction(outputs).
		WithRationale(rationale).
		WithUsage(result.Usage).
		WithModuleName(logging.ModuleChainOfThought).
		WithInputs(inputs)

	// Add adapter metrics if available
	if result.AdapterUsed != "" {
		pred.WithAdapterMetrics(result.AdapterUsed, result.ParseAttempts, result.FallbackUsed)
	}

	// Update history if present (match legacy behavior)
	if cot.History != nil {
		for _, msg := range newMessages {
			if msg.Role == "user" {
				cot.History.Add(msg)
			}
		}
		cot.History.Add(core.Message{Role: "assistant", Content: result.Content})
	}

	// Add parse diagnostics if present
	if result.Diagnostics != nil {
		pred.WithParseDiagnostics(result.Diagnostics)
	}

	// If output didn't converge but we have parseable output, return with diagnostics
	if !result.Converged && result.Diagnostics != nil {
		// This is lenient completion - we have partial output with diagnostics
		return pred, nil
	}

	// If output converged, return successfully
	if result.Converged {
		return pred, nil
	}

	// If no convergence and no diagnostics, return error
	return nil, fmt.Errorf("structured output failed to converge and no parseable output available")
}

// cotAdapter wraps an adapter to inject history
type cotAdapter struct {
	base    core.Adapter
	history *core.History
}

func (ca *cotAdapter) Format(sig *core.Signature, inputs map[string]any, demos []core.Example) ([]core.Message, error) {
	messages, err := ca.base.Format(sig, inputs, demos)
	if err != nil {
		return nil, err
	}

	// Prepend history if available
	if ca.history != nil && !ca.history.IsEmpty() {
		historyMessages := ca.base.FormatHistory(ca.history)
		messages = append(historyMessages, messages...)
	}

	return messages, nil
}

func (ca *cotAdapter) Parse(sig *core.Signature, content string) (map[string]any, error) {
	return ca.base.Parse(sig, content)
}

func (ca *cotAdapter) FormatHistory(history *core.History) []core.Message {
	return ca.base.FormatHistory(history)
}

// forwardLegacy executes chain of thought using the legacy path (without structured output enforcement)
func (cot *ChainOfThought) forwardLegacy(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	// Use adapter to format messages with demos
	newMessages, err := cot.Adapter.Format(cot.Signature, inputs, cot.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Build final message list
	var messages []core.Message

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
		if _, isJSON := cot.Adapter.(*core.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				// Use OpenAI-compliant schema for OpenAI providers to avoid strict mode errors
				if cot.LM.IsOpenAI() {
					options.ResponseSchema = cot.Signature.SignatureToOpenAIJSONSchema()
				} else {
					options.ResponseSchema = cot.Signature.SignatureToJSONSchema()
				}
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
	adapterUsed, parseAttempts, fallbackUsed := core.ExtractAdapterMetadata(outputs)

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
		cot.History.Add(core.Message{
			Role:    "assistant",
			Content: result.Content,
		})
	}

	// Build Prediction object with rationale
	prediction := core.NewPrediction(outputs).
		WithRationale(rationale).
		WithUsage(result.Usage).
		WithModuleName(logging.ModuleChainOfThought).
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}

// Clone creates an independent copy of ChainOfThought module
func (cot *ChainOfThought) Clone() core.Module {
	cloned := &ChainOfThought{
		Signature: cot.Signature,
		LM:        cot.LM,
		Options:   cot.Options,
		Adapter:   cot.Adapter,
		History:   nil,
		Demos:     make([]core.Example, len(cot.Demos)),
	}

	copy(cloned.Demos, cot.Demos)

	if cot.History != nil {
		cloned.History = cot.History.Clone()
	}

	return cloned
}
