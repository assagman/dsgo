package module

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/logging"
)

// Predict is the basic prediction module
type Predict struct {
	Signature *dsgo.Signature
	LM        dsgo.LM
	Options   *dsgo.GenerateOptions
	Adapter   dsgo.Adapter
	History   *dsgo.History  // Optional conversation history
	Demos     []dsgo.Example // Optional few-shot examples
}

// NewPredict creates a new Predict module
func NewPredict(signature *dsgo.Signature, lm dsgo.LM) *Predict {
	return &Predict{
		Signature: signature,
		LM:        lm,
		Options:   dsgo.DefaultGenerateOptions(),
		Adapter:   dsgo.NewFallbackAdapter(), // Use fallback adapter for robustness
	}
}

// WithOptions sets custom generation options
func (p *Predict) WithOptions(options *dsgo.GenerateOptions) *Predict {
	p.Options = options
	return p
}

// WithAdapter sets a custom adapter
func (p *Predict) WithAdapter(adapter dsgo.Adapter) *Predict {
	p.Adapter = adapter
	return p
}

// WithHistory sets conversation history for multi-turn interactions
func (p *Predict) WithHistory(history *dsgo.History) *Predict {
	p.History = history
	return p
}

// WithDemos sets few-shot examples for in-context learning
func (p *Predict) WithDemos(demos []dsgo.Example) *Predict {
	p.Demos = demos
	return p
}

// GetSignature returns the module's signature
func (p *Predict) GetSignature() *dsgo.Signature {
	return p.Signature
}

// Forward executes the prediction
func (p *Predict) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	// Ensure context has a request ID
	ctx = logging.EnsureRequestID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, "Predict", p.Signature.Description)

	var predErr error
	defer func() {
		logging.LogPredictionEnd(ctx, "Predict", time.Since(startTime), predErr)
	}()

	if err := p.Signature.ValidateInputs(inputs); err != nil {
		predErr = fmt.Errorf("input validation failed: %w", err)
		return nil, predErr
	}

	// Use adapter to format messages with demos
	newMessages, err := p.Adapter.Format(p.Signature, inputs, p.Demos)
	if err != nil {
		predErr = fmt.Errorf("failed to format messages: %w", err)
		return nil, predErr
	}

	// Build final message list
	var messages []dsgo.Message

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
		if _, isJSON := p.Adapter.(*dsgo.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				options.ResponseSchema = p.Signature.SignatureToJSONSchema()
			}
		}
	}

	result, err := p.LM.Generate(ctx, messages, options)
	if err != nil {
		predErr = fmt.Errorf("LM generation failed: %w", err)
		return nil, predErr
	}

	// Use adapter to parse output
	outputs, err := p.Adapter.Parse(p.Signature, result.Content)
	if err != nil {
		predErr = fmt.Errorf("failed to parse output: %w", err)
		return nil, predErr
	}

	if err := p.Signature.ValidateOutputs(outputs); err != nil {
		predErr = fmt.Errorf("output validation failed: %w", err)
		return nil, predErr
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
		p.History.Add(dsgo.Message{
			Role:    "assistant",
			Content: result.Content,
		})
	}

	// Extract adapter metadata
	adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

	// Build Prediction object
	prediction := dsgo.NewPrediction(outputs).
		WithUsage(result.Usage).
		WithModuleName("Predict").
		WithInputs(inputs)

	// Add adapter metrics if available
	if adapterUsed != "" {
		prediction.WithAdapterMetrics(adapterUsed, parseAttempts, fallbackUsed)
	}

	return prediction, nil
}

// StreamResult represents the result of a streaming prediction
type StreamResult struct {
	Chunks     <-chan dsgo.Chunk       // Channel for receiving streaming chunks
	Prediction <-chan *dsgo.Prediction // Channel for receiving final prediction (sent after stream completes)
	Errors     <-chan error            // Channel for receiving errors
}

// Stream executes the prediction with streaming output
// Returns channels for chunks, final prediction, and errors
// The chunks channel emits incremental content in real-time
// The prediction channel emits the final parsed prediction after the stream completes
// The errors channel emits any errors that occur during streaming or parsing
func (p *Predict) Stream(ctx context.Context, inputs map[string]any) (*StreamResult, error) {
	// Ensure context has a request ID
	ctx = logging.EnsureRequestID(ctx)

	startTime := time.Now()
	logging.LogPredictionStart(ctx, "Predict.Stream", p.Signature.Description)

	if err := p.Signature.ValidateInputs(inputs); err != nil {
		logging.LogPredictionEnd(ctx, "Predict.Stream", time.Since(startTime), err)
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Use adapter to format messages with demos
	newMessages, err := p.Adapter.Format(p.Signature, inputs, p.Demos)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Build final message list
	var messages []dsgo.Message

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
		if _, isJSON := p.Adapter.(*dsgo.JSONAdapter); isJSON {
			options.ResponseFormat = "json"
			// Auto-generate JSON schema from signature for structured outputs
			if options.ResponseSchema == nil {
				options.ResponseSchema = p.Signature.SignatureToJSONSchema()
			}
		}
	}

	// Call LM Stream
	chunkChan, errChan := p.LM.Stream(ctx, messages, options)

	// Create result channels
	outputChunks := make(chan dsgo.Chunk)
	predictionChan := make(chan *dsgo.Prediction, 1)
	errorChan := make(chan error, 1)

	// Start goroutine to handle streaming and final parsing
	go func() {
		defer close(outputChunks)
		defer close(predictionChan)
		defer close(errorChan)

		var streamErr error
		defer func() {
			logging.LogPredictionEnd(ctx, "Predict.Stream", time.Since(startTime), streamErr)
		}()

		var fullContent strings.Builder
		var finalUsage dsgo.Usage

		// Forward chunks and accumulate content
		for chunk := range chunkChan {
			// Forward chunk to caller
			outputChunks <- chunk

			// Call user callback if provided
			if options.StreamCallback != nil {
				options.StreamCallback(chunk)
			}

			// Accumulate content
			fullContent.WriteString(chunk.Content)

			// Capture final metadata
			if chunk.Usage.TotalTokens > 0 {
				finalUsage = chunk.Usage
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

		// Parse accumulated content
		content := fullContent.String()
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
			p.History.Add(dsgo.Message{
				Role:    "assistant",
				Content: content,
			})
		}

		// Extract adapter metadata
		adapterUsed, parseAttempts, fallbackUsed := dsgo.ExtractAdapterMetadata(outputs)

		// Build Prediction object
		prediction := dsgo.NewPrediction(outputs).
			WithUsage(finalUsage).
			WithModuleName("Predict").
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
