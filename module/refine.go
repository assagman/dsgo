package module

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/logging"
)

// Refine is a module that iteratively improves outputs through refinement.
//
// It supports:
//   - Conversation history for multi-turn context (WithHistory)
//   - Automatic history tracking (WithHistoryTracking)
//   - Few-shot examples for in-context learning (WithDemos)
//   - Configurable refinement iterations (WithMaxIterations)
//   - Custom feedback field names (WithRefinementField)
//
// Thread Safety: Refine is safe for concurrent use when History is shared,
// as core.History uses internal synchronization. However, if TrackHistory is enabled,
// concurrent calls will interleave history entries.
//
// By default, History is read-only (TrackHistory=false) to avoid conflicts with
// caller-managed history.
//
// Refine does not support tool execution loops. If the model requests tool execution,
// use the ReAct module instead.
type Refine struct {
	Signature       *core.Signature
	LM              core.LM
	Options         *core.GenerateOptions
	Adapter         core.Adapter
	MaxIterations   int
	RefinementField string         // Field name to use for refinement feedback
	History         *core.History  // Optional conversation history
	TrackHistory    bool           // Opt-in: automatically append to History during Forward
	Demos           []core.Example // Optional few-shot examples
}

// NewRefine creates a new Refine module
func NewRefine(signature *core.Signature, lm core.LM) *Refine {
	return &Refine{
		Signature:       signature,
		LM:              lm,
		Options:         core.DefaultGenerateOptions(),
		Adapter:         core.NewFallbackAdapter(),
		MaxIterations:   3,
		RefinementField: "feedback",
	}
}

// WithOptions sets custom generation options
func (r *Refine) WithOptions(options *core.GenerateOptions) *Refine {
	r.Options = options
	return r
}

// WithAdapter sets a custom adapter
func (r *Refine) WithAdapter(adapter core.Adapter) *Refine {
	r.Adapter = adapter
	return r
}

// WithMaxIterations sets the maximum number of refinement iterations
func (r *Refine) WithMaxIterations(max int) *Refine {
	r.MaxIterations = max
	return r
}

// WithRefinementField sets the field name for refinement feedback
func (r *Refine) WithRefinementField(field string) *Refine {
	r.RefinementField = field
	return r
}

// WithHistory sets conversation history for multi-turn interactions
func (r *Refine) WithHistory(history *core.History) *Refine {
	r.History = history
	return r
}

// WithDemos sets few-shot examples for in-context learning
func (r *Refine) WithDemos(demos []core.Example) *Refine {
	r.Demos = demos
	return r
}

// WithHistoryTracking enables automatic history updates during Forward.
//
// When enabled, Refine will append user prompts and assistant responses to History.
// Default is false (read-only) to avoid conflicts with caller-managed history.
func (r *Refine) WithHistoryTracking(enabled bool) *Refine {
	r.TrackHistory = enabled
	return r
}

// GetSignature returns the module's signature
func (r *Refine) GetSignature() *core.Signature {
	return r.Signature
}

// Forward executes the refinement loop
func (r *Refine) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	// Ensure context has IDs
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModuleRefine, r.Signature.Description)

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModuleRefine, time.Since(startTime), predErr)
	}()

	if err := r.Signature.ValidateInputs(inputs); err != nil {
		predErr = fmt.Errorf("input validation failed: %w", err)
		return nil, predErr
	}

	// Generate initial prediction
	prediction, err := r.generatePrediction(ctx, inputs, nil)
	if err != nil {
		predErr = fmt.Errorf("initial prediction failed: %w", err)
		return nil, predErr
	}

	// Check if feedback is provided for refinement
	feedback, hasFeedback := inputs[r.RefinementField]
	if !hasFeedback || r.MaxIterations <= 1 {
		return prediction, nil
	}

	// Refinement loop
	for i := 0; i < r.MaxIterations-1; i++ {
		logging.GetLogger().Debug(ctx, "Refine iteration", map[string]any{
			"iteration": i + 1,
			"feedback":  feedback,
		})

		// Generate refinement prompt
		refined, err := r.generateRefinement(ctx, inputs, prediction.Outputs, fmt.Sprintf("%v", feedback))
		if err != nil {
			// If refinement fails, return the last valid prediction
			logging.GetLogger().Warn(ctx, "Refinement step failed", map[string]any{
				"error": err.Error(),
			})
			return prediction, nil
		}

		prediction = refined
	}

	return prediction, nil
}

func (r *Refine) generatePrediction(ctx context.Context, inputs map[string]any, previousOutput map[string]any) (*core.Prediction, error) {
	// Build custom prompt for refinement context
	var messages []core.Message

	if previousOutput != nil {
		// If we have previous output, build custom refinement prompt
		var prompt strings.Builder

		prompt.WriteString("Refine the previous output based on the context:\n\n")

		// Add previous output
		prompt.WriteString("--- Previous Output ---\n")
		for k, v := range previousOutput {
			prompt.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
		prompt.WriteString("\n")

		// Add inputs for context
		prompt.WriteString("--- Context ---\n")
		for _, field := range r.Signature.InputFields {
			if field.Name == r.RefinementField {
				continue
			}
			value, exists := inputs[field.Name]
			if !exists {
				continue
			}
			prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
		}
		prompt.WriteString("\n")

		// Add output format
		prompt.WriteString("--- Required Output Format ---\n")
		prompt.WriteString("Respond with a JSON object containing:\n")
		for _, field := range r.Signature.OutputFields {
			optional := ""
			if field.Optional {
				optional = " (optional)"
			}
			classInfo := ""
			if field.Type == core.FieldTypeClass && len(field.Classes) > 0 {
				classInfo = fmt.Sprintf(" [one of: %s]", strings.Join(field.Classes, ", "))
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
			} else {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
			}
		}

		messages = []core.Message{{Role: "user", Content: prompt.String()}}
	} else {
		// Initial prediction, use adapter
		var err error
		messages, err = r.Adapter.Format(r.Signature, inputs, r.Demos)
		if err != nil {
			return nil, fmt.Errorf("failed to format messages: %w", err)
		}

		// Prepend history if available
		if r.History != nil && !r.History.IsEmpty() {
			historyMessages := r.Adapter.FormatHistory(r.History)
			messages = append(historyMessages, messages...)
		}
	}

	// Copy options to avoid mutation
	options := r.Options.Copy()
	if r.LM.SupportsJSON() {
		if _, isJSON := r.Adapter.(*core.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				// Use OpenAI-compliant schema for OpenAI providers to avoid strict mode errors
				if r.LM.IsOpenAI() {
					options.ResponseSchema = r.Signature.SignatureToOpenAIJSONSchema()
				} else {
					options.ResponseSchema = r.Signature.SignatureToJSONSchema()
				}
			}
		}
	}

	result, err := r.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	// Handle finish_reason: Refine doesn't support tool execution loops
	if result.FinishReason == "tool_calls" {
		return nil, fmt.Errorf("model requested tool execution (finish_reason=tool_calls) but Refine module doesn't support tool loops - use React module instead")
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
	outputs, err := r.Adapter.Parse(r.Signature, result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if err := r.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := core.ExtractAdapterMetadata(outputs)

	// Build Prediction object
	prediction := core.NewPrediction(outputs).
		WithUsage(result.Usage).
		WithModuleName(logging.ModuleRefine).
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	// Update history if present (only for initial prediction, not refinements)
	if r.History != nil && previousOutput == nil && r.TrackHistory {
		// Add the new messages to history
		for _, msg := range messages {
			if msg.Role == "user" {
				r.History.Add(msg)
			}
		}
		r.History.Add(core.Message{Role: "assistant", Content: result.Content})
	}

	return prediction, nil
}

func (r *Refine) generateRefinement(ctx context.Context, inputs map[string]any, previousOutput map[string]any, feedback string) (*core.Prediction, error) {
	var prompt strings.Builder

	prompt.WriteString("Refine the previous output based on the following feedback:\n\n")
	prompt.WriteString(fmt.Sprintf("Feedback: %s\n\n", feedback))

	// Add previous output
	prompt.WriteString("--- Previous Output ---\n")
	for k, v := range previousOutput {
		prompt.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}
	prompt.WriteString("\n")

	// Add original inputs for context
	prompt.WriteString("--- Original Inputs ---\n")
	for _, field := range r.Signature.InputFields {
		if field.Name == r.RefinementField {
			continue
		}
		value, exists := inputs[field.Name]
		if !exists {
			continue
		}
		prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
	}
	prompt.WriteString("\n")

	// Add output format
	prompt.WriteString("--- Improved Output Format ---\n")
	prompt.WriteString("Respond with a JSON object containing the refined version:\n")
	for _, field := range r.Signature.OutputFields {
		optional := ""
		if field.Optional {
			optional = " (optional)"
		}
		classInfo := ""
		if field.Type == core.FieldTypeClass && len(field.Classes) > 0 {
			classInfo = fmt.Sprintf(" [one of: %s]", strings.Join(field.Classes, ", "))
		}
		if field.Description != "" {
			prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
		} else {
			prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
		}
	}

	messages := []core.Message{
		{Role: "user", Content: prompt.String()},
	}

	// Copy options to avoid mutation
	options := r.Options.Copy()
	if r.LM.SupportsJSON() {
		if _, isJSON := r.Adapter.(*core.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				// Use OpenAI-compliant schema for OpenAI providers to avoid strict mode errors
				if r.LM.IsOpenAI() {
					options.ResponseSchema = r.Signature.SignatureToOpenAIJSONSchema()
				} else {
					options.ResponseSchema = r.Signature.SignatureToJSONSchema()
				}
			}
		}
	}

	result, err := r.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, err
	}

	// Handle finish_reason: Refine doesn't support tool execution loops
	if result.FinishReason == "tool_calls" {
		return nil, fmt.Errorf("model requested tool execution (finish_reason=tool_calls) but Refine module doesn't support tool loops - use React module instead")
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
	outputs, err := r.Adapter.Parse(r.Signature, result.Content)
	if err != nil {
		return nil, err
	}

	if err := r.Signature.ValidateOutputs(outputs); err != nil {
		return nil, err
	}

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := core.ExtractAdapterMetadata(outputs)

	// Build Prediction object
	prediction := core.NewPrediction(outputs).
		WithUsage(result.Usage).
		WithModuleName(logging.ModuleRefine).
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}

// Clone creates a new Refine module sharing the same LM, Signature,
// Options, Adapter, History, and Demos.
//
// The clone can be configured independently but shares underlying resources with
// the original. This is suitable for parallel execution where each clone needs
// its own state but can share read-only configuration.
func (r *Refine) Clone() core.Module {
	cloned := &Refine{
		Signature:       r.Signature,
		LM:              r.LM,
		Options:         r.Options,
		Adapter:         r.Adapter,
		MaxIterations:   r.MaxIterations,
		RefinementField: r.RefinementField,
		History:         r.History,
		TrackHistory:    r.TrackHistory,
		Demos:           r.Demos,
	}
	return cloned
}
