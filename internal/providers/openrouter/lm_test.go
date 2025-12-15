package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assagman/dsgo/internal/core"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func TestNewOpenRouter(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("OPENROUTER_SITE_NAME", "test-site")
	t.Setenv("OPENROUTER_SITE_URL", "https://test.com")

	lm := newOpenRouter("gpt-4")
	if lm.APIKey != "test-key" {
		t.Errorf("expected APIKey test-key, got %s", lm.APIKey)
	}
	if lm.Model != "gpt-4" {
		t.Errorf("expected Model gpt-4, got %s", lm.Model)
	}
	if lm.BaseURL != defaultBaseURL {
		t.Errorf("expected BaseURL %s, got %s", defaultBaseURL, lm.BaseURL)
	}
	if lm.SiteName != "test-site" {
		t.Errorf("expected SiteName test-site, got %s", lm.SiteName)
	}
	if lm.SiteURL != "https://test.com" {
		t.Errorf("expected SiteURL https://test.com, got %s", lm.SiteURL)
	}
}

func TestOpenRouter_Name(t *testing.T) {
	t.Parallel()
	lm := &openRouter{Model: "gpt-4-turbo"}
	if lm.Name() != "gpt-4-turbo" {
		t.Errorf("expected Name gpt-4-turbo, got %s", lm.Name())
	}
}

func TestOpenRouter_SupportsJSON(t *testing.T) {
	t.Parallel()
	lm := &openRouter{}
	if !lm.SupportsJSON() {
		t.Error("expected SupportsJSON to return true")
	}
}

func TestOpenRouter_SupportsTools(t *testing.T) {
	t.Parallel()
	lm := &openRouter{}
	if !lm.SupportsTools() {
		t.Error("expected SupportsTools to return true")
	}
}

func TestOpenRouter_IsOpenAI(t *testing.T) {
	t.Parallel()
	lm := &openRouter{}
	if lm.IsOpenAI() {
		t.Error("expected IsOpenAI to return false for OpenRouter")
	}
}

// Helper to create a test OpenRouter client pointing to a test server
func createTestOpenRouter(t *testing.T, server *httptest.Server) *openRouter {
	t.Helper()
	client := openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)
	return &openRouter{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  client,
	}
}

func TestOpenRouter_Generate_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello, world!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := createTestOpenRouter(t, server)

	messages := []core.Message{
		{Role: "user", Content: "Hello"},
	}
	options := core.DefaultGenerateOptions()

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %s", result.Content)
	}
	if result.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %s", result.FinishReason)
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("expected 10 prompt tokens, got %d", result.Usage.PromptTokens)
	}
}

func TestOpenRouter_Generate_WithTools(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		if _, ok := req["tools"]; !ok {
			t.Error("expected tools in request")
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]any{
									"name":      "get_weather",
									"arguments": `{"location":"NYC"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := createTestOpenRouter(t, server)

	messages := []core.Message{{Role: "user", Content: "What's the weather?"}}
	options := core.DefaultGenerateOptions()
	weatherFunc := func(ctx context.Context, args map[string]any) (any, error) {
		return "sunny", nil
	}
	options.Tools = []core.Tool{
		*core.NewTool("get_weather", "Get weather", weatherFunc),
	}

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected tool name get_weather, got %s", result.ToolCalls[0].Name)
	}
}

func TestOpenRouter_Generate_ToolCallsWithMalformedJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{
								"id":   "call_456",
								"type": "function",
								"function": map[string]any{
									"name":      "search",
									"arguments": `{'query': 'test query', 'limit': 10,}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := createTestOpenRouter(t, server)

	messages := []core.Message{{Role: "user", Content: "Search for test query"}}
	options := core.DefaultGenerateOptions()
	searchFunc := func(ctx context.Context, args map[string]any) (any, error) {
		return "results", nil
	}
	options.Tools = []core.Tool{
		*core.NewTool("search", "Search tool", searchFunc).AddParameter("query", "string", "Search query", true),
	}

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	if result.ToolCalls[0].Arguments["query"] != "test query" {
		t.Errorf("expected query 'test query', got %v", result.ToolCalls[0].Arguments["query"])
	}
	if result.ToolCalls[0].Arguments["limit"] != float64(10) {
		t.Errorf("expected limit 10, got %v", result.ToolCalls[0].Arguments["limit"])
	}
}

func TestOpenRouter_Generate_ErrorResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid request"}}`))
	}))
	defer server.Close()

	lm := createTestOpenRouter(t, server)

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOpenRouter_Generate_NoChoices(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4",
			"choices": []any{},
			"usage": map[string]any{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := createTestOpenRouter(t, server)

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err == nil || err.Error() != "no choices in response" {
		t.Fatalf("expected 'no choices in response' error, got %v", err)
	}
}

func TestOpenRouter_BuildParams(t *testing.T) {
	t.Parallel()
	lm := &openRouter{Model: "gpt-4"}

	t.Run("basic request", func(t *testing.T) {
		messages := []core.Message{{Role: "user", Content: "test"}}
		options := core.DefaultGenerateOptions()
		params := lm.buildParams(messages, options)

		if params.Model != "gpt-4" {
			t.Errorf("expected model gpt-4, got %v", params.Model)
		}
	})

	t.Run("with temperature", func(t *testing.T) {
		messages := []core.Message{{Role: "user", Content: "test"}}
		options := &core.GenerateOptions{Temperature: 0.7}
		params := lm.buildParams(messages, options)

		data, _ := json.Marshal(params)
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		if temp, ok := m["temperature"].(float64); !ok || temp != 0.7 {
			t.Errorf("expected temperature 0.7, got %v", m["temperature"])
		}
	})

	t.Run("with max tokens", func(t *testing.T) {
		messages := []core.Message{{Role: "user", Content: "test"}}
		options := &core.GenerateOptions{MaxTokens: 100}
		params := lm.buildParams(messages, options)

		data, _ := json.Marshal(params)
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		if tokens, ok := m["max_tokens"].(float64); !ok || int(tokens) != 100 {
			t.Errorf("expected max_tokens 100, got %v", m["max_tokens"])
		}
	})
}

func TestOpenRouter_ConvertMessages(t *testing.T) {
	t.Parallel()
	lm := &openRouter{}

	t.Run("basic messages", func(t *testing.T) {
		messages := []core.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		}
		converted := lm.convertMessages(messages)

		if len(converted) != 3 {
			t.Errorf("expected 3 messages, got %d", len(converted))
		}
	})

	t.Run("with tool calls", func(t *testing.T) {
		messages := []core.Message{
			{Role: "user", Content: "What's the weather?"},
			{
				Role: "assistant",
				ToolCalls: []core.ToolCall{
					{ID: "call_123", Name: "get_weather", Arguments: map[string]any{"location": "NYC"}},
				},
			},
			{Role: "tool", Content: `{"temp": 72}`, ToolID: "call_123"},
		}
		converted := lm.convertMessages(messages)

		if len(converted) != 3 {
			t.Errorf("expected 3 messages, got %d", len(converted))
		}
	})
}

func TestOpenRouter_ConvertTool(t *testing.T) {
	t.Parallel()
	lm := &openRouter{}

	tool := core.NewTool("test_tool", "A test tool", nil)
	tool.AddParameter("param1", "string", "First param", true)
	tool.AddEnumParameter("param2", "Second param", []string{"a", "b"}, false)

	converted := lm.convertTool(tool)

	if converted.GetFunction() == nil {
		t.Error("expected function to be set")
	}
}

func TestOpenRouter_ConvertTool_WithEmptyType(t *testing.T) {
	t.Parallel()
	lm := &openRouter{}
	tool := core.NewTool("test_tool", "A test tool with empty type", nil)
	tool.AddParameter("param1", "", "First param with empty type", true)

	converted := lm.convertTool(tool)

	if converted.GetFunction() == nil {
		t.Error("expected function to be set")
	}
}

func TestOpenRouter_ConvertTool_ArrayWithElementType(t *testing.T) {
	t.Parallel()
	lm := &openRouter{}
	tool := core.NewTool("test_tool", "A test tool with array parameter", nil)
	tool.AddArrayParameter("urls", "List of URLs", "string", true)

	converted := lm.convertTool(tool)

	if converted.GetFunction() == nil {
		t.Error("expected function to be set")
	}
}

func TestOpenRouter_SanitizeToolsForOpenRouter(t *testing.T) {
	t.Parallel()

	originalTool := core.NewTool("test_tool", "A test tool", nil)
	originalTool.AddParameter("param1", "", "Param with empty type", true)

	tools := []core.Tool{*originalTool}

	sanitizedTools := sanitizeToolsForOpenRouter(tools)

	if len(sanitizedTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(sanitizedTools))
	}

	if len(sanitizedTools[0].Parameters) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(sanitizedTools[0].Parameters))
	}

	param := sanitizedTools[0].Parameters[0]
	if param.Type != "string" {
		t.Errorf("expected parameter type 'string', got '%s'", param.Type)
	}
}

func TestOpenRouter_Stream_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	lm := createTestOpenRouter(t, server)

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	options := core.DefaultGenerateOptions()

	chunkChan, errChan := lm.Stream(context.Background(), messages, options)

	var content string
	var finalUsage core.Usage
	chunkCount := 0

	for chunk := range chunkChan {
		content += chunk.Content
		chunkCount++
		if chunk.Usage.TotalTokens > 0 {
			finalUsage = chunk.Usage
		}
	}

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
	}

	if content != "Hello world!" {
		t.Errorf("expected content 'Hello world!', got '%s'", content)
	}
	if chunkCount != 3 {
		t.Errorf("expected 3 chunks, got %d", chunkCount)
	}
	if finalUsage.TotalTokens != 13 {
		t.Errorf("expected 13 total tokens, got %d", finalUsage.TotalTokens)
	}
}

func TestOpenRouter_Stream_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "bad request"}}`))
	}))
	defer server.Close()

	lm := createTestOpenRouter(t, server)

	chunkChan, errChan := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())

	for range chunkChan {
		t.Error("should not receive chunks on error")
	}

	err := <-errChan
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInit_RegistersLM(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	core.Configure(
		core.WithProvider("openrouter"),
		core.WithModel("test-model"),
	)

	lm, err := core.NewLM(ctx, "openrouter/test-model")
	if err != nil {
		t.Fatalf("expected LM to be created, got error: %v", err)
	}

	if lm.Name() != "test-model" {
		t.Errorf("expected model name test-model, got %s", lm.Name())
	}
}

func TestMapParamTypeToJSONType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"string", "string"},
		{"int", "integer"},
		{"integer", "integer"},
		{"float", "number"},
		{"number", "number"},
		{"double", "number"},
		{"bool", "boolean"},
		{"boolean", "boolean"},
		{"json", "object"},
		{"object", "object"},
		{"array", "array"},
		{"list", "array"},
		{"unknown", "string"},
		{"", "string"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := mapParamTypeToJSONType(tt.input)
			if result != tt.expected {
				t.Errorf("mapParamTypeToJSONType(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBalanceDelimiters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{`{"key": "value"`, `{"key": "value"}`},
		{`{"nested": {"inner": "val"`, `{"nested": {"inner": "val"}}`},
		{`["item1", "item2"`, `["item1", "item2"]`},
		{`{"arr": [1, 2, 3`, `{"arr": [1, 2, 3}]`},
		{`{"complete": true}`, `{"complete": true}`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := balanceDelimiters(tt.input)
			if result != tt.expected {
				t.Errorf("balanceDelimiters(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeToolCallID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantLen  int  // expected max length
		wantSame bool // expect input to be returned unchanged
	}{
		{
			name:     "valid short ID",
			input:    "call_abc123",
			wantSame: true,
		},
		{
			name:     "valid ID with underscores and hyphens",
			input:    "call_abc-123_xyz",
			wantSame: true,
		},
		{
			name:     "ID at max length",
			input:    "abcdefghij0123456789abcdefghij0123456789", // exactly 40 chars
			wantSame: true,
		},
		{
			name:    "ID exceeds max length",
			input:   "this_is_a_very_long_tool_call_id_that_exceeds_the_maximum_allowed_length_of_40_characters",
			wantLen: maxToolCallIDLength,
		},
		{
			name:    "ID with invalid characters (spaces)",
			input:   "call with spaces",
			wantLen: maxToolCallIDLength,
		},
		{
			name:    "ID with invalid characters (special chars)",
			input:   "call@#$%^&*()",
			wantLen: maxToolCallIDLength,
		},
		{
			name:    "very long ID (1000+ chars like Gemini generates)",
			input:   string(make([]byte, 1220)), // simulate long Gemini ID
			wantLen: maxToolCallIDLength,
		},
		{
			name:    "ID with dots and slashes",
			input:   "call.function/name",
			wantLen: maxToolCallIDLength,
		},
		{
			name:     "empty ID",
			input:    "",
			wantSame: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := sanitizeToolCallID(tt.input)

			if tt.wantSame {
				if result != tt.input {
					t.Errorf("expected unchanged ID %q, got %q", tt.input, result)
				}
				return
			}

			// Check length constraint
			if len(result) > maxToolCallIDLength {
				t.Errorf("result length %d exceeds max %d", len(result), maxToolCallIDLength)
			}

			// Check character validity
			if toolCallIDPattern.MatchString(result) {
				t.Errorf("result %q contains invalid characters", result)
			}

			// Check determinism - same input should produce same output
			result2 := sanitizeToolCallID(tt.input)
			if result != result2 {
				t.Errorf("non-deterministic: got %q then %q for same input", result, result2)
			}
		})
	}
}

func TestSanitizeToolCallID_Determinism(t *testing.T) {
	t.Parallel()

	// Test that different long IDs produce different sanitized IDs
	id1 := "very_long_id_number_one_that_needs_truncation_abc123"
	id2 := "very_long_id_number_two_that_needs_truncation_xyz789"

	result1 := sanitizeToolCallID(id1)
	result2 := sanitizeToolCallID(id2)

	if result1 == result2 {
		t.Errorf("different inputs produced same output: %q", result1)
	}
}

func TestSanitizeToolCallID_UniqueHash(t *testing.T) {
	t.Parallel()

	// Test that IDs with same prefix but different content produce unique hashes
	base := "call_abcdefghijklmnopqrstuvwxyz"
	id1 := base + "_suffix_one"
	id2 := base + "_suffix_two"

	result1 := sanitizeToolCallID(id1)
	result2 := sanitizeToolCallID(id2)

	if result1 == result2 {
		t.Errorf("different inputs with same prefix produced same output: %q", result1)
	}
}
