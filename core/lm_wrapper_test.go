package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/assagman/dsgo/internal/cost"
)

// mockWrapperLM is a mock LM for testing the wrapper
type mockWrapperLM struct {
	name          string
	generateFunc  func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error)
	supportsJSON  bool
	supportsTools bool
}

func (m *mockWrapperLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, messages, options)
	}
	return &GenerateResult{
		Content:      "test response",
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

func (m *mockWrapperLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunkChan := make(chan Chunk)
	errChan := make(chan error, 1)
	close(chunkChan)
	close(errChan)
	return chunkChan, errChan
}

func (m *mockWrapperLM) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-model"
}

func (m *mockWrapperLM) SupportsJSON() bool {
	return m.supportsJSON
}

func (m *mockWrapperLM) SupportsTools() bool {
	return m.supportsTools
}

func TestNewLMWrapper(t *testing.T) {
	mock := &mockWrapperLM{name: "test-model"}
	memCollector := NewMemoryCollector(10)

	wrapper := NewLMWrapper(mock, memCollector)

	if wrapper == nil {
		t.Fatal("Expected wrapper to be created")
	}

	if wrapper.Name() != "test-model" {
		t.Errorf("Expected name 'test-model', got '%s'", wrapper.Name())
	}
}

func TestLMWrapper_Generate_Success(t *testing.T) {
	mock := &mockWrapperLM{
		name: "gpt-4",
		generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
			return &GenerateResult{
				Content:      "Hello, world!",
				FinishReason: "stop",
				Usage: Usage{
					PromptTokens:     50,
					CompletionTokens: 100,
					TotalTokens:      150,
				},
			}, nil
		},
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	options := DefaultGenerateOptions()

	result, err := wrapper.Generate(ctx, messages, options)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", result.Content)
	}

	// Check that cost and latency were added
	if result.Usage.Cost <= 0 {
		t.Errorf("Expected cost > 0, got %f", result.Usage.Cost)
	}

	// Latency should be >= 0 (can be 0 on very fast systems)
	if result.Usage.Latency < 0 {
		t.Errorf("Expected latency >= 0, got %d", result.Usage.Latency)
	}

	// Check that history was collected
	if memCollector.Count() != 1 {
		t.Errorf("Expected 1 history entry, got %d", memCollector.Count())
	}

	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	// Verify entry fields
	if entry.ID == "" {
		t.Error("Expected entry ID to be set")
	}

	if entry.SessionID == "" {
		t.Error("Expected session ID to be set")
	}

	if entry.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", entry.Model)
	}

	if entry.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", entry.Provider)
	}

	if entry.Request.MessageCount != 1 {
		t.Errorf("Expected 1 message, got %d", entry.Request.MessageCount)
	}

	if entry.Response.Content != "Hello, world!" {
		t.Errorf("Expected response content 'Hello, world!', got '%s'", entry.Response.Content)
	}

	if entry.Usage.PromptTokens != 50 {
		t.Errorf("Expected 50 prompt tokens, got %d", entry.Usage.PromptTokens)
	}

	if entry.Usage.CompletionTokens != 100 {
		t.Errorf("Expected 100 completion tokens, got %d", entry.Usage.CompletionTokens)
	}

	if entry.Usage.Cost <= 0 {
		t.Errorf("Expected cost > 0, got %f", entry.Usage.Cost)
	}

	if entry.Usage.Latency < 0 {
		t.Errorf("Expected latency >= 0, got %d", entry.Usage.Latency)
	}

	if entry.Error != nil {
		t.Errorf("Expected no error, got %v", entry.Error)
	}
}

func TestLMWrapper_Generate_Error(t *testing.T) {
	expectedErr := errors.New("generation failed")
	mock := &mockWrapperLM{
		name: "gpt-4",
		generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
			return nil, expectedErr
		},
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hello"}}
	options := DefaultGenerateOptions()

	result, err := wrapper.Generate(ctx, messages, options)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	// Check that error was recorded in history
	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Error == nil {
		t.Fatal("Expected error metadata to be set")
	}

	if entry.Error.Message != "generation failed" {
		t.Errorf("Expected error message 'generation failed', got '%s'", entry.Error.Message)
	}
}

func TestLMWrapper_Generate_WithTools(t *testing.T) {
	mock := &mockWrapperLM{
		name:          "gpt-4",
		supportsTools: true,
		generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
			return &GenerateResult{
				Content:      "",
				FinishReason: "tool_calls",
				ToolCalls: []ToolCall{
					{
						ID:   "call_123",
						Name: "get_weather",
						Arguments: map[string]interface{}{
							"location": "San Francisco",
						},
					},
				},
				Usage: Usage{
					PromptTokens:     30,
					CompletionTokens: 15,
					TotalTokens:      45,
				},
			}, nil
		},
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "What's the weather?"}}
	options := DefaultGenerateOptions()
	options.Tools = []Tool{
		{
			Name:        "get_weather",
			Description: "Get weather",
			Parameters: []ToolParameter{
				{Name: "location", Type: "string", Description: "Location", Required: true},
			},
		},
	}

	result, err := wrapper.Generate(ctx, messages, options)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}

	// Check history entry
	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	if !entry.Request.HasTools {
		t.Error("Expected HasTools to be true")
	}

	if entry.Request.ToolCount != 1 {
		t.Errorf("Expected 1 tool, got %d", entry.Request.ToolCount)
	}

	if entry.Response.ToolCallCount != 1 {
		t.Errorf("Expected 1 tool call, got %d", entry.Response.ToolCallCount)
	}

	if len(entry.Response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call in response, got %d", len(entry.Response.ToolCalls))
	}
}

func TestLMWrapper_Generate_NilCollector(t *testing.T) {
	mock := &mockWrapperLM{name: "gpt-4"}
	wrapper := NewLMWrapper(mock, nil)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hello"}}
	options := DefaultGenerateOptions()

	// Should not panic with nil collector
	result, err := wrapper.Generate(ctx, messages, options)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}
}

func TestLMWrapper_ProviderExtraction(t *testing.T) {
	tests := []struct {
		modelName string
		provider  string
	}{
		{"gpt-4", "openai"},
		{"gpt-3.5-turbo", "openai"},
		{"claude-3-opus", "anthropic"},
		{"claude-3.5-sonnet", "anthropic"},
		{"gemini-pro", "google"},
		{"gemini-2.5-flash", "google"},
		{"llama-3.1-70b", "meta"},
		{"meta-llama-3.1", "meta"},
		{"some-random-model", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			mock := &mockWrapperLM{name: tt.modelName}
			memCollector := NewMemoryCollector(10)
			wrapper := NewLMWrapper(mock, memCollector)

			ctx := context.Background()
			messages := []Message{{Role: "user", Content: "test"}}
			options := DefaultGenerateOptions()

			_, _ = wrapper.Generate(ctx, messages, options)

			entries := memCollector.GetAll()
			if len(entries) != 1 {
				t.Fatalf("Expected 1 entry, got %d", len(entries))
			}

			if entries[0].Provider != tt.provider {
				t.Errorf("Expected provider '%s', got '%s'", tt.provider, entries[0].Provider)
			}
		})
	}
}

func TestLMWrapper_CustomSession(t *testing.T) {
	mock := &mockWrapperLM{name: "gpt-4"}
	memCollector := NewMemoryCollector(10)
	sessionID := "custom-session-123"

	wrapper := NewLMWrapperWithSession(mock, memCollector, sessionID)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hello"}}
	options := DefaultGenerateOptions()

	_, err := wrapper.Generate(ctx, messages, options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].SessionID != sessionID {
		t.Errorf("Expected session ID '%s', got '%s'", sessionID, entries[0].SessionID)
	}
}

func TestLMWrapper_Latency(t *testing.T) {
	mock := &mockWrapperLM{
		name: "gpt-4",
		generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			return &GenerateResult{
				Content:      "response",
				FinishReason: "stop",
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			}, nil
		},
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hello"}}
	options := DefaultGenerateOptions()

	result, err := wrapper.Generate(ctx, messages, options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have at least 10ms latency
	if result.Usage.Latency < 10 {
		t.Errorf("Expected latency >= 10ms, got %d ms", result.Usage.Latency)
	}

	// Check history entry also has latency
	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Usage.Latency < 10 {
		t.Errorf("Expected entry latency >= 10ms, got %d ms", entries[0].Usage.Latency)
	}
}

func TestLMWrapper_InterfaceMethods(t *testing.T) {
	mock := &mockWrapperLM{
		name:          "gpt-4",
		supportsJSON:  true,
		supportsTools: true,
	}

	wrapper := NewLMWrapper(mock, nil)

	if wrapper.Name() != "gpt-4" {
		t.Errorf("Expected name 'gpt-4', got '%s'", wrapper.Name())
	}

	if !wrapper.SupportsJSON() {
		t.Error("Expected SupportsJSON to be true")
	}

	if !wrapper.SupportsTools() {
		t.Error("Expected SupportsTools to be true")
	}
}

func TestLMWrapper_Stream(t *testing.T) {
	mock := &mockWrapperLM{name: "gpt-4"}
	wrapper := NewLMWrapper(mock, nil)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hello"}}
	options := DefaultGenerateOptions()

	// Should delegate to underlying LM
	chunkChan, errChan := wrapper.Stream(ctx, messages, options)

	if chunkChan == nil {
		t.Error("Expected chunk channel")
	}

	if errChan == nil {
		t.Error("Expected error channel")
	}
}

func TestLMWrapper_ProviderMeta_Population(t *testing.T) {
	tests := []struct {
		name             string
		metadata         map[string]any
		expectedProvMeta map[string]any
	}{
		{
			name: "with request ID and rate limits",
			metadata: map[string]any{
				"request_id":         "req_123",
				"x-ratelimit-limit":  "100",
				"x-ratelimit-remain": "95",
			},
			expectedProvMeta: map[string]any{
				"request_id":         "req_123",
				"x-ratelimit-limit":  "100",
				"x-ratelimit-remain": "95",
			},
		},
		{
			name: "with cache headers",
			metadata: map[string]any{
				"cache_status": "hit",
				"request_id":   "req_456",
			},
			expectedProvMeta: map[string]any{
				"cache_status": "hit",
				"request_id":   "req_456",
			},
		},
		{
			name:             "empty metadata",
			metadata:         map[string]any{},
			expectedProvMeta: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockWrapperLM{
				name: "gpt-4",
				generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
					return &GenerateResult{
						Content:      "response",
						FinishReason: "stop",
						Usage: Usage{
							PromptTokens:     10,
							CompletionTokens: 20,
							TotalTokens:      30,
						},
						Metadata: tt.metadata,
					}, nil
				},
			}

			memCollector := NewMemoryCollector(10)
			wrapper := NewLMWrapper(mock, memCollector)

			ctx := context.Background()
			messages := []Message{{Role: "user", Content: "test"}}
			options := DefaultGenerateOptions()

			_, err := wrapper.Generate(ctx, messages, options)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			entries := memCollector.GetAll()
			if len(entries) != 1 {
				t.Fatalf("Expected 1 entry, got %d", len(entries))
			}

			entry := entries[0]
			if entry.ProviderMeta == nil && len(tt.expectedProvMeta) > 0 {
				t.Fatal("Expected ProviderMeta to be populated")
			}

			for key, expectedVal := range tt.expectedProvMeta {
				actualVal, ok := entry.ProviderMeta[key]
				if !ok {
					t.Errorf("Expected key '%s' in ProviderMeta", key)
					continue
				}
				if actualVal != expectedVal {
					t.Errorf("For key '%s': expected %v, got %v", key, expectedVal, actualVal)
				}
			}
		})
	}
}

func TestLMWrapper_CacheHit_FromMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metadata    map[string]any
		expectedHit bool
		expectedSrc string
	}{
		{
			name: "cache_status hit",
			metadata: map[string]any{
				"cache_status": "hit",
			},
			expectedHit: true,
			expectedSrc: "provider",
		},
		{
			name: "cache_status miss",
			metadata: map[string]any{
				"cache_status": "miss",
			},
			expectedHit: false,
			expectedSrc: "",
		},
		{
			name: "cache_hit boolean true",
			metadata: map[string]any{
				"cache_hit": true,
			},
			expectedHit: true,
			expectedSrc: "provider",
		},
		{
			name: "cache_hit boolean false",
			metadata: map[string]any{
				"cache_hit": false,
			},
			expectedHit: false,
			expectedSrc: "provider",
		},
		{
			name:        "no cache metadata",
			metadata:    map[string]any{},
			expectedHit: false,
			expectedSrc: "",
		},
		{
			name:        "nil metadata",
			metadata:    nil,
			expectedHit: false,
			expectedSrc: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockWrapperLM{
				name: "gpt-4",
				generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
					return &GenerateResult{
						Content:      "response",
						FinishReason: "stop",
						Usage: Usage{
							PromptTokens:     10,
							CompletionTokens: 20,
							TotalTokens:      30,
						},
						Metadata: tt.metadata,
					}, nil
				},
			}

			memCollector := NewMemoryCollector(10)
			wrapper := NewLMWrapper(mock, memCollector)

			ctx := context.Background()
			messages := []Message{{Role: "user", Content: "test"}}
			options := DefaultGenerateOptions()

			_, err := wrapper.Generate(ctx, messages, options)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			entries := memCollector.GetAll()
			if len(entries) != 1 {
				t.Fatalf("Expected 1 entry, got %d", len(entries))
			}

			entry := entries[0]
			if entry.Cache.Hit != tt.expectedHit {
				t.Errorf("Expected Cache.Hit=%v, got %v", tt.expectedHit, entry.Cache.Hit)
			}

			if tt.expectedSrc != "" && entry.Cache.Source != tt.expectedSrc {
				t.Errorf("Expected Cache.Source='%s', got '%s'", tt.expectedSrc, entry.Cache.Source)
			}
		})
	}
}

func TestLMWrapper_Provider_FromSettings(t *testing.T) {
	// Save current settings
	oldSettings := GetSettings()
	defer func() {
		// Restore settings
		globalSettings.mu.Lock()
		globalSettings.DefaultProvider = oldSettings.DefaultProvider
		globalSettings.mu.Unlock()
	}()

	tests := []struct {
		name             string
		modelName        string
		settingsProvider string
		expectedProvider string
	}{
		{
			name:             "use settings provider",
			modelName:        "some-model",
			settingsProvider: "openrouter",
			expectedProvider: "openrouter",
		},
		{
			name:             "settings override model heuristic",
			modelName:        "gpt-4",
			settingsProvider: "openrouter",
			expectedProvider: "openrouter",
		},
		{
			name:             "fallback to model heuristic when no settings",
			modelName:        "gpt-4",
			settingsProvider: "",
			expectedProvider: "openai",
		},
		{
			name:             "fallback for claude model",
			modelName:        "claude-3-opus",
			settingsProvider: "",
			expectedProvider: "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global settings
			globalSettings.mu.Lock()
			globalSettings.DefaultProvider = tt.settingsProvider
			globalSettings.mu.Unlock()

			mock := &mockWrapperLM{name: tt.modelName}
			memCollector := NewMemoryCollector(10)
			wrapper := NewLMWrapper(mock, memCollector)

			ctx := context.Background()
			messages := []Message{{Role: "user", Content: "test"}}
			options := DefaultGenerateOptions()

			_, err := wrapper.Generate(ctx, messages, options)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			entries := memCollector.GetAll()
			if len(entries) != 1 {
				t.Fatalf("Expected 1 entry, got %d", len(entries))
			}

			if entries[0].Provider != tt.expectedProvider {
				t.Errorf("Expected provider '%s', got '%s'", tt.expectedProvider, entries[0].Provider)
			}
		})
	}
}

func TestLMWrapper_ProviderMeta_WithNilResult(t *testing.T) {
	expectedErr := errors.New("generation failed")
	mock := &mockWrapperLM{
		name: "gpt-4",
		generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
			return nil, expectedErr
		},
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "test"}}
	options := DefaultGenerateOptions()

	_, err := wrapper.Generate(ctx, messages, options)
	if err == nil {
		t.Fatal("Expected error")
	}

	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	// ProviderMeta should not be set when result is nil
	entry := entries[0]
	if entry.ProviderMeta != nil {
		t.Error("Expected ProviderMeta to be nil when result is nil")
	}
}

func TestLMWrapper_ExtractProviderFromModel(t *testing.T) {
	tests := []struct {
		modelName string
		expected  string
	}{
		{"gpt-4", "openai"},
		{"GPT-4", "openai"},
		{"gpt-3.5-turbo", "openai"},
		{"openai/gpt-4", "openai"},
		{"claude-3-opus", "anthropic"},
		{"CLAUDE-3.5-SONNET", "anthropic"},
		{"anthropic/claude", "anthropic"},
		{"gemini-pro", "google"},
		{"GEMINI-2.5-FLASH", "google"},
		{"google/gemini-pro", "google"},
		{"llama-3.1-70b", "meta"},
		{"LLAMA-3", "meta"},
		{"meta/llama-3", "meta"},
		{"random-model-123", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			mock := &mockWrapperLM{name: tt.modelName}
			memCollector := NewMemoryCollector(10)
			wrapper := NewLMWrapper(mock, memCollector).(*LMWrapper)

			result := wrapper.extractProviderFromModel()
			if result != tt.expected {
				t.Errorf("For model '%s': expected provider '%s', got '%s'", tt.modelName, tt.expected, result)
			}
		})
	}
}

func TestLMWrapper_FullObservabilityIntegration(t *testing.T) {
	// Save current settings
	oldSettings := GetSettings()
	defer func() {
		globalSettings.mu.Lock()
		globalSettings.DefaultProvider = oldSettings.DefaultProvider
		globalSettings.mu.Unlock()
	}()

	// Set global provider
	globalSettings.mu.Lock()
	globalSettings.DefaultProvider = "openrouter"
	globalSettings.mu.Unlock()

	// Create mock with full metadata
	mock := &mockWrapperLM{
		name: "gpt-4",
		generateFunc: func(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
			return &GenerateResult{
				Content:      "Test response",
				FinishReason: "stop",
				Usage: Usage{
					PromptTokens:     50,
					CompletionTokens: 100,
					TotalTokens:      150,
				},
				Metadata: map[string]any{
					"cache_status":          "hit",
					"x-request-id":          "req_abc123",
					"x-ratelimit-limit":     "100",
					"x-ratelimit-remaining": "95",
					"x-ratelimit-reset":     "1234567890",
				},
			}, nil
		},
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hello"}}
	options := DefaultGenerateOptions()

	result, err := wrapper.Generate(ctx, messages, options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify result
	if result.Content != "Test response" {
		t.Errorf("Expected content 'Test response', got '%s'", result.Content)
	}

	// Verify history entry
	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	// 1. Provider should come from settings
	if entry.Provider != "openrouter" {
		t.Errorf("Expected provider 'openrouter', got '%s'", entry.Provider)
	}

	// 2. Cache hit should be extracted from metadata
	if !entry.Cache.Hit {
		t.Error("Expected Cache.Hit=true")
	}
	if entry.Cache.Source != "provider" {
		t.Errorf("Expected Cache.Source='provider', got '%s'", entry.Cache.Source)
	}

	// 3. ProviderMeta should contain all metadata
	if entry.ProviderMeta == nil {
		t.Fatal("Expected ProviderMeta to be populated")
	}

	expectedMetaKeys := []string{
		"cache_status",
		"x-request-id",
		"x-ratelimit-limit",
		"x-ratelimit-remaining",
		"x-ratelimit-reset",
	}

	for _, key := range expectedMetaKeys {
		if _, ok := entry.ProviderMeta[key]; !ok {
			t.Errorf("Expected key '%s' in ProviderMeta", key)
		}
	}

	// 4. Verify specific metadata values
	if reqID, ok := entry.ProviderMeta["x-request-id"].(string); !ok || reqID != "req_abc123" {
		t.Errorf("Expected request ID 'req_abc123', got %v", entry.ProviderMeta["x-request-id"])
	}

	// 5. Verify usage and cost are calculated
	if entry.Usage.Cost <= 0 {
		t.Errorf("Expected cost > 0, got %f", entry.Usage.Cost)
	}

	if entry.Usage.Latency < 0 {
		t.Errorf("Expected latency >= 0, got %d", entry.Usage.Latency)
	}
}

func TestLMWrapper_Stream_Success(t *testing.T) {
	mock := &mockStreamSuccessLM{
		name: "gpt-4",
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}
	options := DefaultGenerateOptions()

	chunkChan, errChan := wrapper.Stream(ctx, messages, options)

	var content string

	// Consume stream
	for chunk := range chunkChan {
		content += chunk.Content
	}

	// Check for errors
	if err := <-errChan; err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Allow goroutine time to collect history
	time.Sleep(50 * time.Millisecond)

	// Verify accumulated content
	if content != "Hello world!" {
		t.Errorf("Expected 'Hello world!', got '%s'", content)
	}

	// Verify history was collected
	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	// Verify entry fields
	if entry.Response.Content != "Hello world!" {
		t.Errorf("Expected response 'Hello world!', got '%s'", entry.Response.Content)
	}

	if entry.Response.FinishReason != "stop" {
		t.Errorf("Expected finish reason 'stop', got '%s'", entry.Response.FinishReason)
	}

	if entry.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", entry.Usage.PromptTokens)
	}

	if entry.Usage.CompletionTokens != 5 {
		t.Errorf("Expected 5 completion tokens, got %d", entry.Usage.CompletionTokens)
	}

	if entry.Usage.TotalTokens != 15 {
		t.Errorf("Expected 15 total tokens, got %d", entry.Usage.TotalTokens)
	}

	if entry.Usage.Cost <= 0 {
		t.Errorf("Expected cost > 0, got %f", entry.Usage.Cost)
	}

	if entry.Usage.Latency < 0 {
		t.Errorf("Expected latency >= 0, got %d", entry.Usage.Latency)
	}

	if entry.Error != nil {
		t.Errorf("Expected no error, got %v", entry.Error)
	}
}

func TestLMWrapper_Stream_Error(t *testing.T) {
	expectedErr := errors.New("stream failed")

	memCollector := NewMemoryCollector(10)

	// Create wrapper with custom stream behavior
	wrapper := &LMWrapper{
		lm: &mockStreamErrorLM{
			name: "gpt-4",
			err:  expectedErr,
		},
		collector:  memCollector,
		calculator: cost.NewCalculator(),
		sessionID:  "test-session",
	}

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}
	options := DefaultGenerateOptions()

	chunkChan, errChan := wrapper.Stream(ctx, messages, options)

	// Consume stream
	for range chunkChan {
		// Just consume
	}

	// Check for error
	err := <-errChan
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
	}

	// Allow goroutine time to collect history
	time.Sleep(50 * time.Millisecond)

	// Verify error was recorded in history
	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Error == nil {
		t.Fatal("Expected error metadata to be set")
	}

	if entry.Error.Message != "stream failed" {
		t.Errorf("Expected error message 'stream failed', got '%s'", entry.Error.Message)
	}
}

func TestLMWrapper_Stream_WithToolCalls(t *testing.T) {
	mock := &mockStreamToolCallsLM{
		name: "gpt-4",
	}

	memCollector := NewMemoryCollector(10)
	wrapper := NewLMWrapper(mock, memCollector)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "What's the weather?"}}
	options := DefaultGenerateOptions()

	chunkChan, errChan := wrapper.Stream(ctx, messages, options)

	var toolCalls []ToolCall

	// Consume stream
	for chunk := range chunkChan {
		if len(chunk.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.ToolCalls...)
		}
	}

	// Check for errors
	if err := <-errChan; err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Allow goroutine time to collect history
	time.Sleep(50 * time.Millisecond)

	// Verify tool calls were accumulated
	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got '%s'", toolCalls[0].Name)
	}

	// Verify history entry
	entries := memCollector.GetAll()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	if len(entry.Response.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call in history, got %d", len(entry.Response.ToolCalls))
	}

	if entry.Response.ToolCalls[0].Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got '%s'", entry.Response.ToolCalls[0].Name)
	}

	if entry.Response.ToolCallCount != 1 {
		t.Errorf("Expected tool call count 1, got %d", entry.Response.ToolCallCount)
	}
}

func TestLMWrapper_Stream_NoCollector(t *testing.T) {
	mock := &mockWrapperLM{name: "gpt-4"}

	// Create wrapper without collector
	wrapper := NewLMWrapper(mock, nil)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}
	options := DefaultGenerateOptions()

	chunkChan, errChan := wrapper.Stream(ctx, messages, options)

	// Consume stream
	for range chunkChan {
		// Just consume
	}

	// Check for errors
	if err := <-errChan; err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should not panic even without collector
	time.Sleep(50 * time.Millisecond)
}

// mockStreamSuccessLM is a mock that returns successful streaming chunks
type mockStreamSuccessLM struct {
	name string
}

func (m *mockStreamSuccessLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	return nil, nil
}

func (m *mockStreamSuccessLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunkChan := make(chan Chunk, 3)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		// Send chunks
		chunkChan <- Chunk{Content: "Hello"}
		chunkChan <- Chunk{Content: " world"}
		chunkChan <- Chunk{
			Content:      "!",
			FinishReason: "stop",
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
	}()

	return chunkChan, errChan
}

func (m *mockStreamSuccessLM) Name() string {
	return m.name
}

func (m *mockStreamSuccessLM) SupportsJSON() bool {
	return false
}

func (m *mockStreamSuccessLM) SupportsTools() bool {
	return false
}

// mockStreamErrorLM is a mock that returns an error during streaming
type mockStreamErrorLM struct {
	name string
	err  error
}

func (m *mockStreamErrorLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	return nil, m.err
}

func (m *mockStreamErrorLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunkChan := make(chan Chunk)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)
		errChan <- m.err
	}()

	return chunkChan, errChan
}

func (m *mockStreamErrorLM) Name() string {
	return m.name
}

func (m *mockStreamErrorLM) SupportsJSON() bool {
	return false
}

func (m *mockStreamErrorLM) SupportsTools() bool {
	return false
}

// mockStreamToolCallsLM is a mock that returns tool calls during streaming
type mockStreamToolCallsLM struct {
	name string
}

func (m *mockStreamToolCallsLM) Generate(ctx context.Context, messages []Message, options *GenerateOptions) (*GenerateResult, error) {
	return nil, nil
}

func (m *mockStreamToolCallsLM) Stream(ctx context.Context, messages []Message, options *GenerateOptions) (<-chan Chunk, <-chan error) {
	chunkChan := make(chan Chunk, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		chunkChan <- Chunk{
			ToolCalls: []ToolCall{
				{
					ID:   "call_123",
					Name: "get_weather",
					Arguments: map[string]interface{}{
						"location": "San Francisco",
					},
				},
			},
			FinishReason: "tool_calls",
			Usage: Usage{
				PromptTokens:     20,
				CompletionTokens: 10,
				TotalTokens:      30,
			},
		}
	}()

	return chunkChan, errChan
}

func (m *mockStreamToolCallsLM) Name() string {
	return m.name
}

func (m *mockStreamToolCallsLM) SupportsJSON() bool {
	return false
}

func (m *mockStreamToolCallsLM) SupportsTools() bool {
	return true
}
