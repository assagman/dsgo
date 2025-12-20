package core

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/assagman/dsgo/cost"
	"github.com/assagman/dsgo/internal/ids"
	"github.com/assagman/dsgo/logging"
)

// lmWrapper wraps an LM to add observability (cost, latency, history collection)
type lmWrapper struct {
	lm         LM
	collector  Collector
	calculator *cost.Calculator
	sessionID  string
	provider   string // Store provider name to avoid guessing
}

var missingPricingWarned sync.Map

// newLMWrapper creates a new LM wrapper with observability features
func newLMWrapper(lm LM, collector Collector) LM {
	return &lmWrapper{
		lm:         lm,
		collector:  collector,
		calculator: cost.NewCalculator(),
		sessionID:  ids.NewUUID(),
	}
}

// newLMWrapperWithProvider creates a new LM wrapper with observability features and explicit provider
func newLMWrapperWithProvider(lm LM, collector Collector, provider string) LM {
	return &lmWrapper{
		lm:         lm,
		collector:  collector,
		calculator: cost.NewCalculator(),
		sessionID:  ids.NewUUID(),
		provider:   provider,
	}
}

// newLMWrapperWithSession creates a new LM wrapper with a custom session ID
func newLMWrapperWithSession(lm LM, collector Collector, sessionID string) LM {
	return &lmWrapper{
		lm:         lm,
		collector:  collector,
		calculator: cost.NewCalculator(),
		sessionID:  sessionID,
	}
}

// Generate wraps the underlying LM's Generate with observability
func (w *lmWrapper) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	startTime := time.Now()
	entryID := ids.NewUUID()

	// Call underlying LM
	result, err := w.lm.Generate(ctx, messages, options)

	// Calculate latency
	latency := time.Since(startTime).Milliseconds()

	// Build history entry
	entry := w.buildHistoryEntry(ctx, entryID, startTime, messages, options, result, latency, err)

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
func (w *lmWrapper) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
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
			chunkClosed        bool
			errClosed          bool
		)

		// Forward chunks and accumulate data
		for {
			select {
			case chunk, ok := <-inChunkChan:
				if !ok {
					chunkClosed = true
					// Check if error channel also closed
					if errClosed {
						goto StreamComplete
					}
					// Continue to drain error channel
					continue
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

			case err, ok := <-inErrChan:
				if !ok {
					errClosed = true
					// Check if chunk channel also closed
					if chunkClosed {
						goto StreamComplete
					}
					// Continue to drain chunk channel
					continue
				}
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
		entry := w.buildHistoryEntry(ctx, entryID, startTime, messages, options, result, latency, streamErr)

		// Collect history (best effort)
		if w.collector != nil {
			_ = w.collector.Collect(entry)
		}
	}()

	return outChunkChan, outErrChan
}

func (w *lmWrapper) calculateCost(ctx context.Context, canonicalModel string, promptTokens, completionTokens int) float64 {
	if promptTokens == 0 && completionTokens == 0 {
		return 0
	}

	// Check if we have pricing for this model
	if w.calculator.HasPricing(canonicalModel) {
		return w.calculator.Calculate(canonicalModel, promptTokens, completionTokens)
	}

	// Use canonical model as the warning key to avoid duplicate warnings
	if _, loaded := missingPricingWarned.LoadOrStore(canonicalModel, struct{}{}); !loaded {
		logging.GetLogger().Warn(ctx, "No pricing information for model", map[string]any{
			"module":            "cost",
			"model":             canonicalModel,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
		})
	}

	return 0
}

// Name returns the underlying LM's name
func (w *lmWrapper) Name() string {
	return w.lm.Name()
}

// SupportsJSON returns whether the underlying LM supports JSON
func (w *lmWrapper) SupportsJSON() bool {
	return w.lm.SupportsJSON()
}

// SupportsTools returns whether the underlying LM supports tools
func (w *lmWrapper) SupportsTools() bool {
	return w.lm.SupportsTools()
}

// IsOpenAI returns whether the underlying LM is OpenAI
func (w *lmWrapper) IsOpenAI() bool {
	return w.lm.IsOpenAI()
}

func (w *lmWrapper) Unwrap() LM {
	return w.lm
}

// buildHistoryEntry constructs a complete HistoryEntry
func (w *lmWrapper) buildHistoryEntry(
	ctx context.Context,
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

		// Calculate cost (best-effort)
		provider := w.getProvider()
		canonicalModel := canonicalModelID(provider, w.lm.Name())
		entry.Usage.Cost = w.calculateCost(ctx, canonicalModel, result.Usage.PromptTokens, result.Usage.CompletionTokens)

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
func (w *lmWrapper) buildRequestMeta(messages []Message, options *GenerateOptions) RequestMeta {
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

// getProvider returns the provider name, preferring the explicitly set provider.
// This is critical for accurate cost calculation, as provider/model matching requires
// the correct provider prefix. In normal usage, core.NewLM() always sets w.provider
// when creating the wrapper, so extractProviderFromModel() is a fallback for:
// - Test mocks that use newLMWrapper() without explicit provider
// - Custom LM implementations that don't go through core.NewLM()
func (w *lmWrapper) getProvider() string {
	// Use explicitly set provider if available (set by core.NewLM)
	if w.provider != "" {
		return w.provider
	}

	// Use global settings if available
	settings := GetSettings()
	if settings.DefaultProvider != "" {
		return settings.DefaultProvider
	}

	// Fall back to extracting from model name
	// WARNING: This is a best-effort heuristic and may fail for custom LMs
	return w.extractProviderFromModel()
}

// extractProviderFromModel attempts to extract provider name from LM name
func (w *lmWrapper) extractProviderFromModel() string {
	name := strings.ToLower(w.lm.Name())

	// Common provider patterns
	if strings.Contains(name, "gpt") || strings.Contains(name, "openai") {
		return "openai"
	}
	if strings.Contains(name, "llama") || strings.Contains(name, "meta") {
		return "meta"
	}

	// Default to unknown
	return "unknown"
}

func canonicalModelID(provider, name string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	name = strings.ToLower(strings.TrimSpace(name))
	if provider == "" {
		return name
	}
	if strings.HasPrefix(name, provider+"/") {
		return name
	}
	return provider + "/" + name
}
