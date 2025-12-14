package module

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
)

// Predict is the basic prediction module
type Predict struct {
	Signature *core.Signature
	LM        core.LM
	Options   *core.GenerateOptions
	Adapter   core.Adapter
	History   *core.History  // Optional conversation history
	Demos     []core.Example // Optional few-shot examples
}

// NewPredict creates a new Predict module
func NewPredict(signature *core.Signature, lm core.LM) *Predict {
	return &Predict{
		Signature: signature,
		LM:        lm,
		Options:   core.DefaultGenerateOptions(),
		Adapter:   core.NewFallbackAdapter(), // Use fallback adapter for robustness
	}
}

// WithOptions sets custom generation options
func (p *Predict) WithOptions(options *core.GenerateOptions) *Predict {
	p.Options = options
	return p
}

// WithAdapter sets a custom adapter
func (p *Predict) WithAdapter(adapter core.Adapter) *Predict {
	p.Adapter = adapter
	return p
}

// WithHistory sets conversation history for multi-turn interactions
func (p *Predict) WithHistory(history *core.History) *Predict {
	p.History = history
	return p
}

// WithDemos sets few-shot examples for in-context learning
func (p *Predict) WithDemos(demos []core.Example) *Predict {
	p.Demos = demos
	return p
}

// GetSignature returns the module's signature
func (p *Predict) GetSignature() *core.Signature {
	return p.Signature
}

// Forward executes the prediction
func (p *Predict) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	// Ensure context has IDs
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModulePredict, p.Signature.Description)

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, logging.ModulePredict, time.Since(startTime), predErr)
	}()

	if err := p.Signature.ValidateInputs(inputs); err != nil {
		predErr = fmt.Errorf("input validation failed: %w", err)
		return nil, predErr
	}

	// Check if structured outputs are enabled
	settings := core.GetSettings()
	useStructuredMode := settings.StructuredOutput.Enabled

	// If structured mode is enabled, use the structured output enforcement loop
	if useStructuredMode {
		return p.forwardStructured(ctx, inputs)
	}

	// Otherwise, use the legacy path
	return p.forwardLegacy(ctx, inputs)
}

// forwardStructured executes prediction with structured output enforcement
func (p *Predict) forwardStructured(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	settings := core.GetSettings()

	// Select adapter based on LM capabilities
	adapter := p.Adapter
	if p.LM.SupportsJSON() && settings.StructuredOutput.Enabled {
		// Use schema-first adapter for LMs that support JSON mode
		adapter = core.NewSchemaFirstAdapter(true).WithReasoning(false)
	}

	// Build the new messages (without history) for History tracking
	newMessages, err := p.Adapter.Format(p.Signature, inputs, p.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Create a custom adapter wrapper that includes history
	wrappedAdapter := &predictAdapter{
		base:    adapter,
		history: p.History,
	}

	// Call structured output enforcement loop
	result, err := core.GenerateStructured(
		ctx,
		p.LM,
		p.Signature,
		inputs,
		p.Demos,
		core.GenerateStructuredOptions{
			Adapter:        wrappedAdapter,
			BaseOptions:    p.Options,
			MaxAttempts:    settings.StructuredOutput.MaxAttempts,
			Temperature:    settings.StructuredOutput.Temperature,
			UseJSONFormat:  p.LM.SupportsJSON(),
			StreamCallback: p.Options.StreamCallback,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("structured output generation failed: %w", err)
	}

	// Build prediction from result
	pred := core.NewPrediction(result.Outputs)
	pred.WithUsage(result.Usage)
	pred.WithModuleName(logging.ModulePredict)
	pred.WithInputs(inputs)

	// Add adapter metrics if available
	if result.AdapterUsed != "" {
		pred.WithAdapterMetrics(result.AdapterUsed, result.ParseAttempts, result.FallbackUsed)
	}

	// Update history if present (match legacy behavior)
	if p.History != nil {
		for _, msg := range newMessages {
			if msg.Role == "user" {
				p.History.Add(msg)
			}
		}
		p.History.Add(core.Message{Role: "assistant", Content: result.Content})
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

// predictAdapter wraps an adapter to inject history
type predictAdapter struct {
	base    core.Adapter
	history *core.History
}

func (pa *predictAdapter) Format(sig *core.Signature, inputs map[string]any, demos []core.Example) ([]core.Message, error) {
	messages, err := pa.base.Format(sig, inputs, demos)
	if err != nil {
		return nil, err
	}

	// Prepend history if available
	if pa.history != nil && !pa.history.IsEmpty() {
		historyMessages := pa.base.FormatHistory(pa.history)
		messages = append(historyMessages, messages...)
	}

	return messages, nil
}

func (pa *predictAdapter) Parse(sig *core.Signature, content string) (map[string]any, error) {
	return pa.base.Parse(sig, content)
}

func (pa *predictAdapter) FormatHistory(history *core.History) []core.Message {
	return pa.base.FormatHistory(history)
}

// forwardLegacy executes the prediction using the legacy path (without structured output enforcement)
func (p *Predict) forwardLegacy(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	// Use adapter to format messages with demos
	newMessages, err := p.Adapter.Format(p.Signature, inputs, p.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Build final message list
	var messages []core.Message

	// Prepend history if available
	if p.History != nil && !p.History.IsEmpty() {
		historyMessages := p.Adapter.FormatHistory(p.History)
		messages = append(messages, historyMessages...)
	}

	// Add new messages
	messages = append(messages, newMessages...)

	// Copy options to avoid mutation
	options := p.Options.Copy()
	// Only force JSON mode for JSONAdapter (not ChatAdapter or FallbackAdapter)
	if p.LM.SupportsJSON() {
		if _, isJSON := p.Adapter.(*core.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				// Use OpenAI-compliant schema for OpenAI providers to avoid strict mode errors
				if p.LM.IsOpenAI() {
					options.ResponseSchema = p.Signature.SignatureToOpenAIJSONSchema()
				} else {
					options.ResponseSchema = p.Signature.SignatureToJSONSchema()
				}
			}
		}
	}

	result, err := p.LM.Generate(ctx, messages, options)
	if err != nil {
		return nil, fmt.Errorf("LM generation failed: %w", err)
	}

	// Handle finish_reason: Predict doesn't support tool execution loops
	if result.FinishReason == "tool_calls" {
		return nil, fmt.Errorf("model requested tool execution (finish_reason=tool_calls) but Predict module doesn't support tool loops - use React module instead")
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
	outputs, err := p.Adapter.Parse(p.Signature, result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	if err := p.Signature.ValidateOutputs(outputs); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}

	// Update history if present
	if p.History != nil {
		// Add only the new user message(s) (not from history)
		for _, msg := range newMessages {
			if msg.Role == "user" {
				p.History.Add(msg)
			}
		}

		// Add assistant response
		p.History.Add(core.Message{
			Role:    "assistant",
			Content: result.Content,
		})
	}

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := core.ExtractAdapterMetadata(outputs)

	// Build Prediction object
	prediction := core.NewPrediction(outputs).
		WithUsage(result.Usage).
		WithModuleName(logging.ModulePredict).
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}

// StreamResult represents the result of a streaming prediction
type StreamResult struct {
	Chunks     <-chan core.Chunk       // Channel for receiving streaming chunks
	Prediction <-chan *core.Prediction // Channel for receiving final prediction (sent after stream completes)
	Errors     <-chan error            // Channel for receiving errors
}

// Stream executes the prediction with streaming output
// Returns channels for chunks, final prediction, and errors
// The chunks channel emits incremental content in real-time
// The prediction channel emits the final parsed prediction after the stream completes
// The errors channel emits any errors that occur during streaming or parsing
func (p *Predict) Stream(ctx context.Context, inputs map[string]any) (*StreamResult, error) {
	// Ensure context has IDs
	ctx = logging.EnsureRequestID(ctx)
	ctx = logging.EnsureCorrelationID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, logging.ModulePredict+".Stream", p.Signature.Description)

	if err := p.Signature.ValidateInputs(inputs); err != nil {
		logging.LogPredictionEnd(ctx, logging.ModulePredict+".Stream", time.Since(startTime), err)
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Use adapter to format messages with demos
	newMessages, err := p.Adapter.Format(p.Signature, inputs, p.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Build final message list
	var messages []core.Message

	// Prepend history if available
	if p.History != nil && !p.History.IsEmpty() {
		historyMessages := p.Adapter.FormatHistory(p.History)
		messages = append(messages, historyMessages...)
	}

	// Add new messages
	messages = append(messages, newMessages...)

	// Copy options to avoid mutation
	options := p.Options.Copy()
	// Only force JSON mode for JSONAdapter (not ChatAdapter or FallbackAdapter)
	if p.LM.SupportsJSON() {
		if _, isJSON := p.Adapter.(*core.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				// Use OpenAI-compliant schema for OpenAI providers to avoid strict mode errors
				if p.LM.IsOpenAI() {
					options.ResponseSchema = p.Signature.SignatureToOpenAIJSONSchema()
				} else {
					options.ResponseSchema = p.Signature.SignatureToJSONSchema()
				}
			}
		}
	}

	// Call LM Stream
	chunkChan, errChan := p.LM.Stream(ctx, messages, options)

	// Create result channels
	outputChunks := make(chan core.Chunk)
	predictionChan := make(chan *core.Prediction, 1)
	errorChan := make(chan error, 1)

	// Start goroutine to handle streaming and final parsing
	go func() {
		defer close(outputChunks)
		defer close(predictionChan)
		defer close(errorChan)

		var streamErr error
		defer func() {
			logging.LogPredictionEnd(ctx, logging.ModulePredict+".Stream", time.Since(startTime), streamErr)
		}()

		// Use StreamingBuffer for automatic recovery
		streamBuffer := core.NewStreamingBuffer()
		markerFilter := core.NewStreamingMarkerFilter()
		var finalUsage core.Usage

		// Forward chunks and accumulate content
		for chunk := range chunkChan {
			// Strip field markers from chunk content for clean user-facing output
			// Markers are internal DSGo artifacts and should not leak through public API
			// Set DSGO_DEBUG_MARKERS=1 to see raw output with markers (for debugging)
			cleanChunk := chunk
			if os.Getenv("DSGO_DEBUG_MARKERS") != "1" {
				cleanChunk.Content = markerFilter.ProcessChunk(chunk.Content)
			}

			// Forward clean chunk to caller
			outputChunks <- cleanChunk

			// Call user callback if provided (with clean chunk)
			if options.StreamCallback != nil {
				options.StreamCallback(cleanChunk)
			}

			// Accumulate original content with streaming buffer (for parsing)
			streamBuffer.Write(chunk.Content)

			// Capture final metadata
			if chunk.Usage.TotalTokens > 0 {
				finalUsage = chunk.Usage
			}
		}

		// Flush any remaining marker filter buffer
		if os.Getenv("DSGO_DEBUG_MARKERS") != "1" {
			remaining := markerFilter.Flush()
			if remaining != "" {
				flushChunk := core.Chunk{Content: remaining}
				outputChunks <- flushChunk
				if options.StreamCallback != nil {
					options.StreamCallback(flushChunk)
				}
			}
		}

		// Check for streaming errors
		select {
		case err := <-errChan:
			if err != nil {
				streamErr = fmt.Errorf("LM streaming failed: %w", err)
				errorChan <- streamErr
				return
			}
		default:
		}

		// Finalize streaming buffer (applies recovery fixes)
		content := streamBuffer.Finalize()
		outputs, err := p.Adapter.Parse(p.Signature, content)
		if err != nil {
			streamErr = fmt.Errorf("failed to parse output: %w", err)
			errorChan <- streamErr
			return
		}

		// Use partial validation for robustness
		diag := p.Signature.ValidateOutputsPartial(outputs)

		// Check for critical errors (type errors that cannot be recovered)
		if len(diag.TypeErrors) > 0 {
			// Type errors are critical - send error
			var errMsgs []string
			for field, err := range diag.TypeErrors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", field, err))
			}
			streamErr = fmt.Errorf("output validation failed with type errors: %v", strings.Join(errMsgs, "; "))
			errorChan <- streamErr
			return
		}

		// Update history if present
		if p.History != nil {
			// Add only the new user message(s) (not from history)
			for _, msg := range newMessages {
				if msg.Role == "user" {
					p.History.Add(msg)
				}
			}

			// Add assistant response
			p.History.Add(core.Message{
				Role:    "assistant",
				Content: content,
			})
		}

		// Extract adapter metadata
		adapterUsed, parseAttempts, fallbackUsed := core.ExtractAdapterMetadata(outputs)

		// Build Prediction object
		prediction := core.NewPrediction(outputs).
			WithUsage(finalUsage).
			WithModuleName(logging.ModulePredict).
			WithInputs(inputs)

		// Add adapter metrics if available
		if adapterUsed != "" {
			prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
		}

		// Attach diagnostics if there were any issues (missing fields or class errors)
		if diag.HasErrors() {
			prediction.WithParseDiagnostics(diag)
		}

		// Send final prediction
		predictionChan <- prediction
	}()

	return &StreamResult{
		Chunks:     outputChunks,
		Prediction: predictionChan,
		Errors:     errorChan,
	}, nil
}

// Clone creates an independent copy of Predict module
func (p *Predict) Clone() core.Module {
	cloned := &Predict{
		Signature: p.Signature,
		LM:        p.LM,
		Options:   p.Options,
		Adapter:   p.Adapter,
		History:   nil, // Start with fresh history
		Demos:     make([]core.Example, len(p.Demos)),
	}

	// Copy demos
	copy(cloned.Demos, p.Demos)

	// Clone history if it exists
	if p.History != nil {
		cloned.History = p.History.Clone()
	}

	return cloned
}
