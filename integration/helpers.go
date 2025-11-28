package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/assagman/dsgo"
)

// ContextWithTimeout creates a context with a timeout for tests
func ContextWithTimeout(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

// NewMockLMWithResponse creates a mock LM with a single response
func NewMockLMWithResponse(response string) dsgo.LM {
	return &MockLM{
		Response: response,
	}
}

// NewMockLMWithResponses creates a mock LM that cycles through responses
func NewMockLMWithResponses(responses []string) dsgo.LM {
	return &MockLM{
		Responses: responses,
		Index:     0,
	}
}

// NewFailThenSucceedLM creates a mock LM that fails N times then succeeds
func NewFailThenSucceedLM(failAttempts int, successResponse string) dsgo.LM {
	responses := make([]string, failAttempts+1)
	// Fill with empty responses (failures)
	for i := 0; i < failAttempts; i++ {
		responses[i] = ""
	}
	// Last response succeeds
	responses[failAttempts] = successResponse
	return &MockLM{
		Responses: responses,
		Index:     0,
	}
}

// NewAlwaysFailLM creates a mock LM that always fails
func NewAlwaysFailLM(errorType string) dsgo.LM {
	return &MockLM{
		Error: errors.New("LM generation failed: " + errorType),
	}
}

// NewTimeoutThenSucceedLM creates a mock LM that times out on specified attempt then succeeds
func NewTimeoutThenSucceedLM(timeoutAttempt int, duration time.Duration, successResponse string) dsgo.LM {
	m := &MockLM{
		SupportsJSONValue:  false,
		SupportsToolsValue: false,
	}
	// Store attempt info
	m.Responses = []string{"timeout", successResponse}
	m.Latency = duration
	m.Index = timeoutAttempt
	return m
}

// NewLatencyLM creates a mock LM with specified latency
func NewLatencyLM(latency time.Duration, response string) dsgo.LM {
	return &MockLM{
		Response: response,
		Latency:  latency,
	}
}

// NewConstantResponseLM creates a mock LM with a constant response
func NewConstantResponseLM(response string) dsgo.LM {
	return &MockLM{
		Response: response,
	}
}

// MockLM is a test double for dsgo.LM interface - exported for direct use in tests
type MockLM struct {
	mu                 sync.Mutex
	Response           string
	Responses          []string
	Index              int
	SupportsJSONValue  bool
	SupportsToolsValue bool
	Error              error
	Latency            time.Duration
}

// Generate implements dsgo.LM
func (m *MockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	// Handle latency with context cancellation support
	if m.Latency > 0 {
		select {
		case <-time.After(m.Latency):
			// Latency elapsed, continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.Error != nil {
		return nil, m.Error
	}

	// Lock for thread-safe response cycling
	m.mu.Lock()
	response := m.Response
	if len(m.Responses) > 0 {
		if m.Index >= len(m.Responses) {
			m.Index = 0
		}
		response = m.Responses[m.Index]
		m.Index++
	}
	m.mu.Unlock()

	return &dsgo.GenerateResult{
		Content: response,
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             0.001, // Simulate ~$0.001 per call (typical mock cost)
		},
	}, nil
}

// Stream implements dsgo.LM
func (m *MockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- dsgo.Chunk{
			Content: result.Content,
			Usage:   result.Usage,
		}
	}()

	return chunkChan, errChan
}

// Name implements dsgo.LM
func (m *MockLM) Name() string {
	return "mock-lm"
}

// SupportsJSON implements dsgo.LM
func (m *MockLM) SupportsJSON() bool {
	return m.SupportsJSONValue
}

// SupportsTools implements dsgo.LM
func (m *MockLM) SupportsTools() bool {
	return m.SupportsToolsValue
}

// IsOpenAI implements dsgo.LM
func (m *MockLM) IsOpenAI() bool {
	return false
}

// HistoryCollector is a test double that collects history entries
type HistoryCollector struct {
	Entries []*dsgo.HistoryEntry
}

// Collect implements dsgo.Collector
func (hc *HistoryCollector) Collect(entry *dsgo.HistoryEntry) error {
	hc.Entries = append(hc.Entries, entry)
	return nil
}

// Close implements dsgo.Collector
func (hc *HistoryCollector) Close() error {
	return nil
}

// GetEntries returns all collected entries
func (hc *HistoryCollector) GetEntries() []*dsgo.HistoryEntry {
	return hc.Entries
}

// GetLastEntry returns the most recent entry
func (hc *HistoryCollector) GetLastEntry() *dsgo.HistoryEntry {
	if len(hc.Entries) == 0 {
		return nil
	}
	return hc.Entries[len(hc.Entries)-1]
}

// GetTotalCost returns the sum of all entry costs
func (hc *HistoryCollector) GetTotalCost() float64 {
	total := 0.0
	for _, entry := range hc.Entries {
		total += entry.Usage.Cost
	}
	return total
}

// GetTotalTokens returns the sum of all entry tokens
func (hc *HistoryCollector) GetTotalTokens() int {
	total := 0
	for _, entry := range hc.Entries {
		total += entry.Usage.TotalTokens
	}
	return total
}

// Clear resets the collector
func (hc *HistoryCollector) Clear() {
	hc.Entries = []*dsgo.HistoryEntry{}
}

// ============================================================================
// Specialized Mock LM Implementations for Phase 1 Coverage
// ============================================================================

// MalformedJSONMockLM returns JSON with specific malformations to test repair
type MalformedJSONMockLM struct {
	MalformationType string // "single-quotes", "unquoted-keys", "trailing-comma", "newlines"
}

// Generate returns malformed JSON based on type
func (m *MalformedJSONMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	var response string

	switch m.MalformationType {
	case "single-quotes":
		// Single quotes instead of double quotes
		response = "{'answer': 'test value', 'confidence': '0.95'}"
	case "unquoted-keys":
		// Unquoted keys
		response = "{answer: \"test value\", confidence: \"0.95\"}"
	case "trailing-comma":
		// Trailing comma in object
		response = "{\"answer\": \"test value\", \"confidence\": 0.95,}"
	case "newlines":
		// Literal newlines in string values that might confuse parsers
		response = "{\"answer\": \"test\nvalue\nwith\nnewlines\", \"confidence\": 0.95}"
	case "mixed":
		// Mix of multiple issues
		response = "{'answer': \"test value\",\n \"confidence\": 0.95,}"
	default:
		response = "{\"answer\": \"test value\", \"confidence\": 0.95}"
	}

	return &dsgo.GenerateResult{
		Content: response,
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             0.001,
		},
	}, nil
}

// Stream implements dsgo.LM
func (m *MalformedJSONMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- dsgo.Chunk{
			Content: result.Content,
			Usage:   result.Usage,
		}
	}()

	return chunkChan, errChan
}

// Name implements dsgo.LM
func (m *MalformedJSONMockLM) Name() string {
	return "malformed-json-mock"
}

// SupportsJSON implements dsgo.LM
func (m *MalformedJSONMockLM) SupportsJSON() bool {
	return true
}

// SupportsTools implements dsgo.LM
func (m *MalformedJSONMockLM) SupportsTools() bool {
	return false
}

// IsOpenAI implements dsgo.LM
func (m *MalformedJSONMockLM) IsOpenAI() bool {
	return false
}

// TypeCoercionMockLM returns responses with intentionally wrong types for testing coercion
type TypeCoercionMockLM struct {
	CoercionType string // "string-as-int", "string-as-float", "bool-as-string", "int-as-string"
}

// Generate returns values with type mismatches
func (m *TypeCoercionMockLM) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	var response string

	switch m.CoercionType {
	case "string-as-int":
		// Int field as string
		response = `{"count": "42", "name": "test"}`
	case "string-as-float":
		// Float field as string
		response = `{"score": "95.5", "name": "test"}`
	case "string-as-percentage":
		// Percentage string to float conversion
		response = `{"confidence": "95%", "name": "test"}`
	case "bool-as-string":
		// Boolean as string
		response = `{"enabled": "true", "name": "test"}`
	case "bool-as-int":
		// Boolean as int (1/0)
		response = `{"enabled": 1, "name": "test"}`
	case "qualitative-confidence":
		// Qualitative description to numeric confidence
		response = `{"confidence": "high", "name": "test"}`
	case "int-as-string":
		// Int as string
		response = `{"count": "100", "name": "test"}`
	default:
		response = `{"count": 42, "score": 95.5, "enabled": true, "name": "test"}`
	}

	return &dsgo.GenerateResult{
		Content: response,
		Usage: dsgo.Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             0.001,
		},
	}, nil
}

// Stream implements dsgo.LM
func (m *TypeCoercionMockLM) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		result, err := m.Generate(ctx, messages, options)
		if err != nil {
			errChan <- err
			return
		}

		chunkChan <- dsgo.Chunk{
			Content: result.Content,
			Usage:   result.Usage,
		}
	}()

	return chunkChan, errChan
}

// Name implements dsgo.LM
func (m *TypeCoercionMockLM) Name() string {
	return "type-coercion-mock"
}

// SupportsJSON implements dsgo.LM
func (m *TypeCoercionMockLM) SupportsJSON() bool {
	return true
}

// SupportsTools implements dsgo.LM
func (m *TypeCoercionMockLM) SupportsTools() bool {
	return false
}

// IsOpenAI implements dsgo.LM
func (m *TypeCoercionMockLM) IsOpenAI() bool {
	return false
}

// ============================================================================
// HTTP Mock Transport for Retry Testing
// ============================================================================

// HTTPMockTransport mocks HTTP responses for testing retry logic
// Cycles through predefined responses on each call
type HTTPMockTransport struct {
	Responses []http.Response
	Index     int
	mu        sync.Mutex
}

// RoundTrip implements http.RoundTripper
func (h *HTTPMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.Index >= len(h.Responses) {
		h.Index = 0
	}

	resp := h.Responses[h.Index]
	h.Index++

	// Ensure body is set for reading
	if resp.Body == nil {
		resp.Body = io.NopCloser(bytes.NewReader([]byte{}))
	}

	return &resp, nil
}

// NewHTTPMockTransport creates a new HTTP mock transport with predefined responses
func NewHTTPMockTransport(responses []http.Response) *HTTPMockTransport {
	return &HTTPMockTransport{
		Responses: responses,
		Index:     0,
	}
}
