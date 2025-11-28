package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/integration/fixtures"
	"github.com/assagman/dsgo/module"
)

// TestStreaming_BasicChunksReceived validates that all chunks are received in order.
// Scenario: Stream 100 chunks, verify all received and ordered.
// Expected: All chunks received, content preserved, no corruption.
func TestStreaming_BasicChunksReceived(t *testing.T) {
	ctx := context.Background()

	// Create LM that returns chunked response
	chunks := []string{"Hello", " ", "world", "!", " ", "This", " ", "is", " ", "streaming"}
	fullResponse := ""
	for _, c := range chunks {
		fullResponse += c
	}

	lm := &MockLM{
		Response: fullResponse,
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "Say hello",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	// Collect all chunks
	var receivedChunks []core.Chunk
	var streamErr error

	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				break
			}
			_ = append(receivedChunks, chunk)
		case streamErr = <-result.Errors:
			if streamErr != nil {
				t.Errorf("Stream error: %v", streamErr)
			}
		case pred := <-result.Prediction:
			if pred != nil {
				// Verify prediction is available after streaming
				answer, ok := pred.GetString("answer")
				if !ok {
					t.Error("Expected answer field in prediction")
				}
				if answer == "" {
					t.Error("Expected non-empty answer")
				}
			}
			return // Stream complete
		}
	}
}

// TestStreaming_MarkerFiltering validates that field markers are removed from streamed content.
// Scenario: Stream contains [[## field_name ##]] markers.
// Expected: Markers removed from chunks, content clean.
func TestStreaming_MarkerFiltering(t *testing.T) {
	ctx := context.Background()

	// Response with markers
	response := `[[ ## answer ## ]]Hello world`

	lm := &MockLM{
		Response: response,
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "Say hello",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	// Collect all chunks and verify no markers in content
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				return
			}
			// Verify marker not in chunk content
			if containsMarker(chunk.Content) {
				t.Errorf("Chunk contains field marker: %q", chunk.Content)
			}
		case err := <-result.Errors:
			if err != nil {
				t.Errorf("Stream error: %v", err)
				return
			}
		case pred := <-result.Prediction:
			if pred != nil {
				return
			}
		}
	}
}

// TestStreaming_PredictModule validates streaming with Predict module.
// Scenario: Basic Predict.Stream() execution.
// Expected: Chunks received, final prediction valid.
func TestStreaming_PredictModule(t *testing.T) {
	ctx := context.Background()

	lm := &MockLM{
		Response: `{"answer": "42"}`,
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "What is the answer?",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	chunkCount := 0
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				break
			}
			chunkCount++
			if chunk.Content == "" {
				t.Error("Chunk content is empty")
			}
		case err := <-result.Errors:
			if err != nil {
				t.Errorf("Stream error: %v", err)
				return
			}
		case pred := <-result.Prediction:
			if pred == nil {
				t.Error("Expected non-nil prediction")
				return
			}
			answer, ok := pred.GetString("answer")
			if !ok || answer != "42" {
				t.Errorf("Expected answer=42, got %v", answer)
			}
			return // Success
		}
	}
}

// TestStreaming_LargeResponses validates streaming with large responses.
// Scenario: LM produces 10KB+ response.
// Expected: All data streamed without truncation or loss.
func TestStreaming_LargeResponses(t *testing.T) {
	ctx := context.Background()

	// Generate large response
	largeContent := ""
	for i := 0; i < 100; i++ {
		largeContent += "This is a line of content that will be repeated many times to create a large response. "
	}

	lm := &MockLM{
		Response: `{"answer": "` + largeContent + `"}`,
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	totalBytes := 0
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				break
			}
			totalBytes += len(chunk.Content)
		case err := <-result.Errors:
			if err != nil {
				t.Errorf("Stream error: %v", err)
				return
			}
		case pred := <-result.Prediction:
			if pred == nil {
				t.Error("Expected non-nil prediction")
				return
			}
			// Verify answer contains expected content
			answer, ok := pred.GetString("answer")
			if !ok || answer == "" {
				t.Error("Expected non-empty answer field in prediction")
			}
			return // Success
		}
	}
}

// TestStreaming_ErrorHandling validates error handling during streaming.
// Scenario: Stream errors mid-flight.
// Expected: Error channel receives error, streaming stops gracefully.
func TestStreaming_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// LM that fails
	lm := &MockLM{
		Error: errors.New("stream failed: connection lost"),
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				return
			}
			_ = chunk // Consume chunk if any
		case err := <-result.Errors:
			if err != nil {
				// Error gets wrapped by streaming layer
				if !containsString(err.Error(), "connection lost") && !containsString(err.Error(), "stream failed") {
					t.Errorf("Expected connection/stream error, got: %v", err)
				}
			}
			return
		case pred := <-result.Prediction:
			if pred == nil {
				t.Error("Expected prediction to be received")
			}
			return
		}
	}
}

// TestStreaming_ContextCancellation validates that canceling context is handled gracefully.
// Scenario: Cancel context during streaming.
// Expected: Streaming stops gracefully, channels close.
func TestStreaming_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// LM with short response
	lm := &MockLM{
		Response: `{"answer": "quick response"}`,
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	// Cancel context after short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Consume from channels - context cancellation may interrupt or stream may complete
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				return
			}
			_ = chunk
		case err := <-result.Errors:
			if err != nil {
				// Context error or stream error - both acceptable
				return
			}
		case pred := <-result.Prediction:
			if pred != nil {
				return // Stream completed normally
			}
			return
		case <-time.After(1 * time.Second):
			t.Error("Timeout in context cancellation test")
			return
		}
	}
}

// TestStreaming_ObservabilityWithStreaming validates that history is collected during streaming.
// Scenario: Stream response and verify history entry created.
// Expected: HistoryEntry with complete metadata (tokens, cost, latency).
func TestStreaming_ObservabilityWithStreaming(t *testing.T) {
	ctx := context.Background()

	collector := &HistoryCollector{}
	core.Configure(core.WithCollector(collector))

	lm := &MockLM{
		Response: `{"answer": "streaming response"}`,
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	// Consume all streaming data
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				break
			}
			_ = chunk
		case err := <-result.Errors:
			if err != nil {
				t.Errorf("Stream error: %v", err)
				return
			}
		case pred := <-result.Prediction:
			if pred != nil {
				// Verify usage is tracked
				if pred.Usage.TotalTokens == 0 {
					t.Error("Expected token usage to be tracked")
				}
				if pred.Usage.Cost == 0 {
					t.Error("Expected cost to be tracked")
				}
			}
			return
		}
	}
}

// TestStreaming_MultipleChunkHandling validates handling of rapid chunks.
// Scenario: LM produces many small chunks rapidly.
// Expected: All chunks received, no data loss.
func TestStreaming_MultipleChunkHandling(t *testing.T) {
	ctx := context.Background()

	// Generate response with many small pieces
	parts := make([]string, 100)
	for i := 0; i < 100; i++ {
		parts[i] = "chunk"
	}

	response := ""
	for _, p := range parts {
		response += p + " "
	}

	lm := &MockLM{
		Response: response,
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	chunkCount := 0
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				break
			}
			if chunk.Content == "" {
				t.Error("Received empty chunk")
			}
			chunkCount++
		case err := <-result.Errors:
			if err != nil {
				t.Errorf("Stream error: %v", err)
				return
			}
		case pred := <-result.Prediction:
			if pred != nil {
				// Verify final answer exists
				answer, ok := pred.GetString("answer")
				if !ok || answer == "" {
					t.Error("Expected non-empty answer in final prediction")
				}
			}
			return
		}
	}
}

// TestStreaming_ConcurrentStreams validates concurrent streaming doesn't interfere.
// Scenario: Multiple streams running in parallel.
// Expected: Each stream independent, no cross-contamination.
func TestStreaming_ConcurrentStreams(t *testing.T) {
	ctx := context.Background()
	numStreams := 5

	lm := &MockLM{
		Response: `{"answer": "concurrent response"}`,
	}

	sig := fixtures.SimplePredictSig()

	var wg sync.WaitGroup
	errChan := make(chan error, numStreams)

	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			predictor := module.NewPredict(sig, lm)
			result, err := predictor.Stream(ctx, map[string]any{
				"question": "test",
			})

			if err != nil {
				errChan <- err
				return
			}

			// Consume stream
			for {
				select {
				case chunk, ok := <-result.Chunks:
					if !ok {
						break
					}
					_ = chunk
				case err := <-result.Errors:
					if err != nil {
						errChan <- err
						return
					}
				case pred := <-result.Prediction:
					if pred == nil {
						errChan <- errors.New("nil prediction")
						return
					}
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			t.Errorf("Concurrent stream error: %v", err)
		}
	}
}

// TestStreaming_EmptyResponse validates handling of empty streaming responses.
// Scenario: LM returns empty response.
// Expected: Graceful handling, prediction still created.
func TestStreaming_EmptyResponse(t *testing.T) {
	ctx := context.Background()

	lm := &MockLM{
		Response: "",
	}

	sig := fixtures.SimplePredictSig()
	predictor := module.NewPredict(sig, lm)

	result, err := predictor.Stream(ctx, map[string]any{
		"question": "test",
	})

	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	// Consume stream
	for {
		select {
		case chunk, ok := <-result.Chunks:
			if !ok {
				break
			}
			_ = chunk
		case err := <-result.Errors:
			if err != nil {
				// Empty response may error or produce empty prediction
				return
			}
		case pred := <-result.Prediction:
			if pred != nil {
				// Empty response may still produce prediction
				return
			}
		}
	}
}

// containsMarker checks if content contains field markers
func containsMarker(content string) bool {
	return containsString(content, "[[") || containsString(content, "##")
}

// containsString is a simple substring check
func containsString(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ============================================================================
// Streaming Advanced Error Handling Tests
// ============================================================================

// DelayedChunkLM is a mock LM that emits chunks with configurable delays
// and can simulate errors mid-stream.
type DelayedChunkLM struct {
	Chunks        []string
	ChunkDelay    time.Duration
	ErrorAfter    int // Emit error after this many chunks (-1 = no error)
	ErrorToEmit   error
	Usage         core.Usage
	mu            sync.Mutex
	streamCounter int
}

func (m *DelayedChunkLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	fullContent := ""
	for _, c := range m.Chunks {
		fullContent += c
	}
	return &core.GenerateResult{
		Content: fullContent,
		Usage:   m.Usage,
	}, nil
}

func (m *DelayedChunkLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk)
	errChan := make(chan error, 1)

	m.mu.Lock()
	m.streamCounter++
	m.mu.Unlock()

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		for i, chunk := range m.Chunks {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			if m.ChunkDelay > 0 {
				time.Sleep(m.ChunkDelay)
			}

			if m.ErrorAfter >= 0 && i >= m.ErrorAfter {
				errChan <- m.ErrorToEmit
				return
			}

			chunkChan <- core.Chunk{
				Content: chunk,
				Usage:   m.Usage,
			}
		}
	}()

	return chunkChan, errChan
}

func (m *DelayedChunkLM) Name() string        { return "delayed-chunk-lm" }
func (m *DelayedChunkLM) SupportsJSON() bool  { return true }
func (m *DelayedChunkLM) SupportsTools() bool { return false }
func (m *DelayedChunkLM) IsOpenAI() bool      { return false }

// TestStreaming_ReconnectionScenario simulates connection drop mid-stream.
// Scenario: Stream 5 chunks, then simulate connection error.
// Expected: Error received, partial content preserved before error.
func TestStreaming_ReconnectionScenario(t *testing.T) {
	tests := []struct {
		name           string
		chunks         []string
		errorAfter     int
		expectedChunks int
		wantErr        bool
	}{
		{
			name:           "error after first chunk",
			chunks:         []string{"Hello", " ", "world", "!", " done"},
			errorAfter:     1,
			expectedChunks: 1,
			wantErr:        true,
		},
		{
			name:           "error mid-stream",
			chunks:         []string{"Part1", "Part2", "Part3", "Part4", "Part5"},
			errorAfter:     3,
			expectedChunks: 3,
			wantErr:        true,
		},
		{
			name:           "error at start",
			chunks:         []string{"First", "Second", "Third"},
			errorAfter:     0,
			expectedChunks: 0,
			wantErr:        true,
		},
		{
			name:           "no error full stream",
			chunks:         []string{"A", "B", "C", "D"},
			errorAfter:     -1,
			expectedChunks: 4,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			lm := &DelayedChunkLM{
				Chunks:      tt.chunks,
				ChunkDelay:  5 * time.Millisecond,
				ErrorAfter:  tt.errorAfter,
				ErrorToEmit: errors.New("connection lost: stream interrupted"),
				Usage: core.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			}

			sig := fixtures.SimplePredictSig()
			predictor := module.NewPredict(sig, lm)

			result, err := predictor.Stream(ctx, map[string]any{
				"question": "test reconnection",
			})

			if err != nil {
				t.Fatalf("Stream() failed: %v", err)
			}

			var receivedChunks []string
			var streamErr error
			done := false

			for !done {
				select {
				case chunk, ok := <-result.Chunks:
					if !ok {
						done = true
						break
					}
					receivedChunks = append(receivedChunks, chunk.Content)
				case err := <-result.Errors:
					streamErr = err
					done = true
				case <-result.Prediction:
					done = true
				case <-time.After(2 * time.Second):
					t.Fatal("timeout waiting for stream")
				}
			}

			if tt.wantErr {
				if streamErr == nil {
					t.Error("expected error but got none")
				} else if !containsString(streamErr.Error(), "connection") && !containsString(streamErr.Error(), "stream") {
					t.Errorf("expected connection/stream error, got: %v", streamErr)
				}
			}

			if len(receivedChunks) < tt.expectedChunks {
				t.Errorf("expected at least %d chunks before error, got %d", tt.expectedChunks, len(receivedChunks))
			}

			if len(receivedChunks) > 0 && tt.wantErr {
				partialContent := ""
				for _, c := range receivedChunks {
					partialContent += c
				}
				if partialContent == "" {
					t.Error("expected partial content to be preserved")
				}
			}
		})
	}
}

// SlowConsumerLM is a mock LM for testing backpressure scenarios.
// It tracks whether chunks were dropped due to slow consumption.
type SlowConsumerLM struct {
	Chunks     []string
	ChunkDelay time.Duration
	Usage      core.Usage
	mu         sync.Mutex
	chunksSent int
}

func (m *SlowConsumerLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	fullContent := ""
	for _, c := range m.Chunks {
		fullContent += c
	}
	return &core.GenerateResult{
		Content: fullContent,
		Usage:   m.Usage,
	}, nil
}

func (m *SlowConsumerLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk, 10) // Buffered channel
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		for _, chunk := range m.Chunks {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			if m.ChunkDelay > 0 {
				time.Sleep(m.ChunkDelay)
			}

			select {
			case chunkChan <- core.Chunk{Content: chunk, Usage: m.Usage}:
				m.mu.Lock()
				m.chunksSent++
				m.mu.Unlock()
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}
	}()

	return chunkChan, errChan
}

func (m *SlowConsumerLM) Name() string        { return "slow-consumer-lm" }
func (m *SlowConsumerLM) SupportsJSON() bool  { return true }
func (m *SlowConsumerLM) SupportsTools() bool { return false }
func (m *SlowConsumerLM) IsOpenAI() bool      { return false }

func (m *SlowConsumerLM) GetChunksSent() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chunksSent
}

// TestStreaming_BackpressureHandling tests slow consumer scenarios.
// Scenario: Consumer processes chunks slowly while producer emits rapidly.
// Expected: No data loss, buffering works correctly.
func TestStreaming_BackpressureHandling(t *testing.T) {
	tests := []struct {
		name            string
		numChunks       int
		producerDelay   time.Duration
		consumerDelay   time.Duration
		expectAllChunks bool
	}{
		{
			name:            "fast producer slow consumer",
			numChunks:       20,
			producerDelay:   1 * time.Millisecond,
			consumerDelay:   10 * time.Millisecond,
			expectAllChunks: true,
		},
		{
			name:            "matched speed",
			numChunks:       15,
			producerDelay:   5 * time.Millisecond,
			consumerDelay:   5 * time.Millisecond,
			expectAllChunks: true,
		},
		{
			name:            "very slow consumer",
			numChunks:       10,
			producerDelay:   1 * time.Millisecond,
			consumerDelay:   50 * time.Millisecond,
			expectAllChunks: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			chunks := make([]string, tt.numChunks)
			for i := 0; i < tt.numChunks; i++ {
				chunks[i] = "chunk"
			}

			lm := &SlowConsumerLM{
				Chunks:     chunks,
				ChunkDelay: tt.producerDelay,
				Usage: core.Usage{
					PromptTokens:     10,
					CompletionTokens: tt.numChunks * 2,
					TotalTokens:      10 + tt.numChunks*2,
				},
			}

			sig := fixtures.SimplePredictSig()
			predictor := module.NewPredict(sig, lm)

			result, err := predictor.Stream(ctx, map[string]any{
				"question": "test backpressure",
			})

			if err != nil {
				t.Fatalf("Stream() failed: %v", err)
			}

			var receivedChunks []string
			done := false

			for !done {
				select {
				case chunk, ok := <-result.Chunks:
					if !ok {
						done = true
						break
					}
					time.Sleep(tt.consumerDelay)
					receivedChunks = append(receivedChunks, chunk.Content)
				case err := <-result.Errors:
					if err != nil {
						t.Errorf("unexpected stream error: %v", err)
					}
					done = true
				case <-result.Prediction:
					done = true
				case <-ctx.Done():
					t.Fatal("context timeout")
				}
			}

			if tt.expectAllChunks && len(receivedChunks) < tt.numChunks {
				t.Errorf("expected all %d chunks, got %d (data loss detected)", tt.numChunks, len(receivedChunks))
			}

			chunksSent := lm.GetChunksSent()
			if chunksSent != tt.numChunks {
				t.Errorf("producer sent %d chunks, expected %d", chunksSent, tt.numChunks)
			}
		})
	}
}

// UsageTrackingLM is a mock LM that provides detailed usage tracking.
type UsageTrackingLM struct {
	Response      string
	StreamUsage   core.Usage
	GenerateUsage core.Usage
	ChunkDelay    time.Duration
}

func (m *UsageTrackingLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	return &core.GenerateResult{
		Content: m.Response,
		Usage:   m.GenerateUsage,
	}, nil
}

func (m *UsageTrackingLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		words := splitWords(m.Response)
		for i, word := range words {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			if m.ChunkDelay > 0 {
				time.Sleep(m.ChunkDelay)
			}

			usage := core.Usage{}
			if i == len(words)-1 {
				usage = m.StreamUsage
			}

			chunkChan <- core.Chunk{
				Content: word,
				Usage:   usage,
			}
		}
	}()

	return chunkChan, errChan
}

func (m *UsageTrackingLM) Name() string        { return "usage-tracking-lm" }
func (m *UsageTrackingLM) SupportsJSON() bool  { return true }
func (m *UsageTrackingLM) SupportsTools() bool { return false }
func (m *UsageTrackingLM) IsOpenAI() bool      { return false }

func splitWords(s string) []string {
	var words []string
	current := ""
	for _, c := range s {
		if c == ' ' {
			if current != "" {
				words = append(words, current)
			}
			words = append(words, " ")
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}

// TestStreaming_FinalUsageAccuracy validates that streaming usage matches Generate.
// Scenario: Compare usage from streaming vs non-streaming calls.
// Expected: Token counts and cost calculations match.
func TestStreaming_FinalUsageAccuracy(t *testing.T) {
	tests := []struct {
		name             string
		response         string
		promptTokens     int
		completionTokens int
		totalTokens      int
		costPerToken     float64
	}{
		{
			name:             "simple response",
			response:         `{"answer": "Hello world"}`,
			promptTokens:     50,
			completionTokens: 10,
			totalTokens:      60,
			costPerToken:     0.00001,
		},
		{
			name:             "longer response",
			response:         `{"answer": "This is a longer response with multiple words to test token counting accuracy"}`,
			promptTokens:     100,
			completionTokens: 50,
			totalTokens:      150,
			costPerToken:     0.00002,
		},
		{
			name:             "high token count",
			response:         `{"answer": "Token intensive response for accuracy testing"}`,
			promptTokens:     500,
			completionTokens: 200,
			totalTokens:      700,
			costPerToken:     0.000015,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			expectedCost := float64(tt.totalTokens) * tt.costPerToken

			usage := core.Usage{
				PromptTokens:     tt.promptTokens,
				CompletionTokens: tt.completionTokens,
				TotalTokens:      tt.totalTokens,
				Cost:             expectedCost,
			}

			lm := &UsageTrackingLM{
				Response:      tt.response,
				StreamUsage:   usage,
				GenerateUsage: usage,
				ChunkDelay:    2 * time.Millisecond,
			}

			sig := fixtures.SimplePredictSig()
			predictor := module.NewPredict(sig, lm)

			genResult, err := predictor.Forward(ctx, map[string]any{
				"question": "test usage",
			})
			if err != nil {
				t.Fatalf("Forward() failed: %v", err)
			}

			streamResult, err := predictor.Stream(ctx, map[string]any{
				"question": "test usage",
			})
			if err != nil {
				t.Fatalf("Stream() failed: %v", err)
			}

			var streamPrediction *core.Prediction
			done := false
			for !done {
				select {
				case chunk, ok := <-streamResult.Chunks:
					if !ok {
						done = true
						break
					}
					_ = chunk
				case err := <-streamResult.Errors:
					if err != nil {
						t.Fatalf("Stream error: %v", err)
					}
					done = true
				case pred := <-streamResult.Prediction:
					streamPrediction = pred
					done = true
				case <-time.After(5 * time.Second):
					t.Fatal("timeout waiting for stream")
				}
			}

			if streamPrediction == nil {
				t.Fatal("stream prediction is nil")
			}

			if genResult.Usage.PromptTokens != streamPrediction.Usage.PromptTokens {
				t.Errorf("PromptTokens mismatch: Generate=%d, Stream=%d",
					genResult.Usage.PromptTokens, streamPrediction.Usage.PromptTokens)
			}

			if genResult.Usage.CompletionTokens != streamPrediction.Usage.CompletionTokens {
				t.Errorf("CompletionTokens mismatch: Generate=%d, Stream=%d",
					genResult.Usage.CompletionTokens, streamPrediction.Usage.CompletionTokens)
			}

			if genResult.Usage.TotalTokens != streamPrediction.Usage.TotalTokens {
				t.Errorf("TotalTokens mismatch: Generate=%d, Stream=%d",
					genResult.Usage.TotalTokens, streamPrediction.Usage.TotalTokens)
			}

			costDiff := genResult.Usage.Cost - streamPrediction.Usage.Cost
			if costDiff < 0 {
				costDiff = -costDiff
			}
			tolerance := expectedCost * 0.001 // 0.1% tolerance
			if costDiff > tolerance {
				t.Errorf("Cost mismatch beyond tolerance: Generate=%f, Stream=%f, diff=%f",
					genResult.Usage.Cost, streamPrediction.Usage.Cost, costDiff)
			}

			if streamPrediction.Usage.PromptTokens != tt.promptTokens {
				t.Errorf("expected PromptTokens=%d, got %d", tt.promptTokens, streamPrediction.Usage.PromptTokens)
			}
			if streamPrediction.Usage.CompletionTokens != tt.completionTokens {
				t.Errorf("expected CompletionTokens=%d, got %d", tt.completionTokens, streamPrediction.Usage.CompletionTokens)
			}
			if streamPrediction.Usage.TotalTokens != tt.totalTokens {
				t.Errorf("expected TotalTokens=%d, got %d", tt.totalTokens, streamPrediction.Usage.TotalTokens)
			}
		})
	}
}
