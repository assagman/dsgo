package dsgo

import (
	"context"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/cost"
	"github.com/assagman/dsgo/internal/ids"
)

// LMWrapper wraps an LM to add observability (cost, latency, history collection)
type LMWrapper struct {
	lm         LM
	collector  Collector
	calculator *cost.Calculator
	sessionID  string
}

// NewLMWrapper creates a new LM wrapper with observability features
func NewLMWrapper(lm LM, collector Collector) LM {
	return &LMWrapper{
		lm:         lm,
		collector:  collector,
		calculator: cost.NewCalculator(),
		sessionID:  ids.NewUUID(),
	}
}

// NewLMWrapperWithSession creates a new LM wrapper with a custom session ID
func NewLMWrapperWithSession(lm LM, collector Collector, sessionID string) LM {
	return &LMWrapper{
		lm:         lm,
		collector:  collector,
		calculator: cost.NewCalculator(),
		sessionID:  sessionID,
	}
}

// Generate wraps the underlying LM's Generate with observability
func (w *LMWrapper) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	startTime := time.Now()
	entryID := ids.NewUUID()

	// Call underlying LM
	result, err := w.lm.Generate(ctx, messages, options)

	// Calculate latency
	latency := time.Since(startTime).Milliseconds()

	// Build history entry
	entry := w.buildHistoryEntry(entryID, startTime, messages, options, result, latency, err)

	// Collect history (best effort - don't fail the call if collection fails)
	if w.collector != nil {
		_ = w.collector.Collect(entry)
	}

	// Update result with cost and latency if successful
	if err == nil && result != nil {
		result.Usage.Cost = entry.Usage.Cost
		result.Usage.Latency = latency
	}

	return result, err
}

// Stream wraps the underlying LM's Stream with observability
func (w *LMWrapper) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	startTime := time.Now()
	entryID := ids.NewUUID()

	// Create output channels
	outChunkChan := make(chan Chunk)
	outErrChan := make(chan error, 1)

	// Get underlying stream channels
	inChunkChan, inErrChan := w.lm.Stream(ctx, messages, options)

	// Start goroutine to wrap and observe streaming
	go func() {
		defer close(outChunkChan)
		defer close(outErrChan)

		var (
			accumulatedContent string
			accumulatedCalls   []ToolCall
			finalUsage         Usage
			finishReason       string
			streamErr          error
		)

		// Forward chunks and accumulate data
		for {
			select {
			case chunk, ok := <-inChunkChan:
				if !ok {
					// Channel closed, stream complete
					goto StreamComplete
				}

				// Accumulate data
				accumulatedContent += chunk.Content
				if len(chunk.ToolCalls) > 0 {
					accumulatedCalls = append(accumulatedCalls, chunk.ToolCalls...)
				}
				if chunk.FinishReason != "" {
					finishReason = chunk.FinishReason
				}
				// Update usage (final chunk typically has complete usage)
				if chunk.Usage.TotalTokens > 0 {
					finalUsage = chunk.Usage
				}

				// Forward to caller
				outChunkChan <- chunk

			case err := <-inErrChan:
				if err != nil {
					streamErr = err
					outErrChan <- err
					goto StreamComplete
				}
			}
		}

	StreamComplete:
		// Calculate latency
		latency := time.Since(startTime).Milliseconds()

		// Build synthetic result for history entry
		var result *GenerateResult
		if streamErr == nil {
			result = &GenerateResult{
				Content:      accumulatedContent,
				ToolCalls:    accumulatedCalls,
				FinishReason: finishReason,
				Usage:        finalUsage,
			}
		}

		// Build and collect history entry
		entry := w.buildHistoryEntry(entryID, startTime, messages, options, result, latency, streamErr)

		// Update cost in entry if we have usage data
		if result != nil && result.Usage.TotalTokens > 0 {
			modelName := w.lm.Name()
			calculatedCost := w.calculator.Calculate(
				modelName,
				result.Usage.PromptTokens,
				result.Usage.CompletionTokens,
			)
			entry.Usage.Cost = calculatedCost
		}

		// Collect history (best effort)
		if w.collector != nil {
			_ = w.collector.Collect(entry)
		}
	}()

	return outChunkChan, outErrChan
}

// Name returns the underlying LM's name
func (w *LMWrapper) Name() string {
	return w.lm.Name()
}

// SupportsJSON returns whether the underlying LM supports JSON
func (w *LMWrapper) SupportsJSON() bool {
	return w.lm.SupportsJSON()
}

// SupportsTools returns whether the underlying LM supports tools
func (w *LMWrapper) SupportsTools() bool {
	return w.lm.SupportsTools()
}

// buildHistoryEntry constructs a complete HistoryEntry
func (w *LMWrapper) buildHistoryEntry(
	entryID string,
	startTime time.Time,
	messages []Message,
	options *GenerateOptions,
	result *GenerateResult,
	latency int64,
	err error,
) *HistoryEntry {
	entry := &HistoryEntry{
		ID:        entryID,
		Timestamp: startTime,
		SessionID: w.sessionID,
		Provider:  w.getProvider(),
		Model:     w.lm.Name(),
		Request:   w.buildRequestMeta(messages, options),
		Cache:     CacheMeta{Hit: false}, // Default, will be updated from metadata
	}

	// Populate response metadata
	if result != nil {
		entry.Response = ResponseMeta{
			Content:        result.Content,
			ToolCalls:      result.ToolCalls,
			FinishReason:   result.FinishReason,
			ResponseLength: len(result.Content),
			ToolCallCount:  len(result.ToolCalls),
		}

		// Populate usage metadata
		entry.Usage = result.Usage
		entry.Usage.Latency = latency

		// Calculate cost
		modelName := w.lm.Name()
		calculatedCost := w.calculator.Calculate(
			modelName,
			result.Usage.PromptTokens,
			result.Usage.CompletionTokens,
		)
		entry.Usage.Cost = calculatedCost

		// Wire provider-specific metadata
		if result.Metadata != nil {
			entry.ProviderMeta = result.Metadata

			// Extract cache hit status from metadata
			if cacheStatus, ok := result.Metadata["cache_status"].(string); ok {
				entry.Cache.Hit = (cacheStatus == "hit")
				entry.Cache.Source = "provider"
			} else if cacheHit, ok := result.Metadata["cache_hit"].(bool); ok {
				entry.Cache.Hit = cacheHit
				entry.Cache.Source = "provider"
			}
		}
	}

	// Populate error metadata if failed
	if err != nil {
		entry.Error = &ErrorMeta{
			Message: err.Error(),
			Type:    "generation_error",
		}
	}

	return entry
}

// buildRequestMeta constructs request metadata
func (w *LMWrapper) buildRequestMeta(messages []Message, options *GenerateOptions) RequestMeta {
	promptLength := 0
	for _, msg := range messages {
		promptLength += len(msg.Content)
	}

	meta := RequestMeta{
		Messages:       messages,
		Options:        options,
		PromptLength:   promptLength,
		MessageCount:   len(messages),
		ResponseFormat: "text",
	}

	if options != nil {
		meta.HasTools = len(options.Tools) > 0
		meta.ToolCount = len(options.Tools)
		meta.ResponseFormat = options.ResponseFormat
	}

	return meta
}

// getProvider returns the provider name, preferring settings.DefaultProvider
func (w *LMWrapper) getProvider() string {
	// Use global settings if available
	settings := GetSettings()
	if settings.DefaultProvider != "" {
		return settings.DefaultProvider
	}

	// Fall back to extracting from model name
	return w.extractProviderFromModel()
}

// extractProviderFromModel attempts to extract provider name from LM name
func (w *LMWrapper) extractProviderFromModel() string {
	name := strings.ToLower(w.lm.Name())

	// Common provider patterns
	if strings.Contains(name, "gpt") || strings.Contains(name, "openai") {
		return "openai"
	}
	if strings.Contains(name, "claude") || strings.Contains(name, "anthropic") {
		return "anthropic"
	}
	if strings.Contains(name, "gemini") || strings.Contains(name, "google") {
		return "google"
	}
	if strings.Contains(name, "llama") || strings.Contains(name, "meta") {
		return "meta"
	}

	// Default to unknown
	return "unknown"
}
