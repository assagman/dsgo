package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/assagman/dsgo/core"
)

func TestNewOpenAI(t *testing.T) {
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer func() { _ = os.Setenv("OPENAI_API_KEY", originalKey) }()

	_ = os.Setenv("OPENAI_API_KEY", "test-key")

	lm := newOpenAI("gpt-4")
	if lm.APIKey != "test-key" {
		t.Errorf("expected APIKey test-key, got %s", lm.APIKey)
	}
	if lm.Model != "gpt-4" {
		t.Errorf("expected Model gpt-4, got %s", lm.Model)
	}
	if lm.BaseURL != defaultBaseURL {
		t.Errorf("expected BaseURL %s, got %s", defaultBaseURL, lm.BaseURL)
	}
	if lm.Client == nil {
		t.Error("expected Client to be initialized")
	}
}

func TestOpenAI_Name(t *testing.T) {
	lm := &openAI{Model: "gpt-4-turbo"}
	if lm.Name() != "gpt-4-turbo" {
		t.Errorf("expected Name gpt-4-turbo, got %s", lm.Name())
	}
}

func TestOpenAI_SupportsJSON(t *testing.T) {
	lm := &openAI{}
	if !lm.SupportsJSON() {
		t.Error("expected SupportsJSON to return true")
	}
}

func TestOpenAI_SupportsTools(t *testing.T) {
	lm := &openAI{}
	if !lm.SupportsTools() {
		t.Error("expected SupportsTools to return true")
	}
}

func TestOpenAI_Generate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		resp := openAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: openAIMessage{
						Role:    "assistant",
						Content: "Hello, world!",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

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

func TestOpenAI_Generate_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		if _, ok := req["tools"]; !ok {
			t.Error("expected tools in request")
		}

		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Message: openAIMessage{
						Role: "assistant",
						ToolCalls: []openAIToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
									Name:      "get_weather",
									Arguments: `{"location":"NYC"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

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

func TestOpenAI_Generate_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOpenAI_Generate_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err == nil || err.Error() != "no choices in response" {
		t.Fatalf("expected 'no choices in response' error, got %v", err)
	}
}

func TestOpenAI_BuildRequest(t *testing.T) {
	lm := &openAI{Model: "gpt-4"}

	tests := []struct {
		name     string
		messages []core.Message
		options  *core.GenerateOptions
		check    func(t *testing.T, req map[string]any)
	}{
		{
			name:     "basic request",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options:  core.DefaultGenerateOptions(),
			check: func(t *testing.T, req map[string]any) {
				if req["model"] != "gpt-4" {
					t.Errorf("expected model gpt-4, got %v", req["model"])
				}
			},
		},
		{
			name:     "with temperature",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				Temperature: 0.7,
			},
			check: func(t *testing.T, req map[string]any) {
				if req["temperature"] != 0.7 {
					t.Errorf("expected temperature 0.7, got %v", req["temperature"])
				}
			},
		},
		{
			name:     "with max tokens",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				MaxTokens: 100,
			},
			check: func(t *testing.T, req map[string]any) {
				if req["max_tokens"] != 100 {
					t.Errorf("expected max_tokens 100, got %v", req["max_tokens"])
				}
			},
		},
		{
			name:     "with json format (no schema)",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				ResponseFormat: "json",
			},
			check: func(t *testing.T, req map[string]any) {
				rf, ok := req["response_format"].(map[string]string)
				if !ok || rf["type"] != "json_object" {
					t.Errorf("expected response_format to be json_object, got %v", req["response_format"])
				}
			},
		},
		{
			name:     "with json format and schema",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				ResponseFormat: "json",
				ResponseSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"answer": map[string]any{"type": "string"},
					},
					"required": []string{"answer"},
				},
			},
			check: func(t *testing.T, req map[string]any) {
				rf, ok := req["response_format"].(map[string]any)
				if !ok {
					t.Fatal("expected response_format to be map[string]any")
				}
				if rf["type"] != "json_schema" {
					t.Errorf("expected type json_schema, got %v", rf["type"])
				}
				jsonSchema, ok := rf["json_schema"].(map[string]any)
				if !ok {
					t.Fatal("expected json_schema field")
				}
				if jsonSchema["name"] != "response" {
					t.Errorf("expected name 'response', got %v", jsonSchema["name"])
				}
				if jsonSchema["strict"] != true {
					t.Error("expected strict to be true")
				}
				schema, ok := jsonSchema["schema"].(map[string]any)
				if !ok {
					t.Fatal("expected schema in json_schema")
				}
				if schema["type"] != "object" {
					t.Errorf("expected schema type object, got %v", schema["type"])
				}
			},
		},
		{
			name:     "with stop sequences",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				Stop: []string{"END", "STOP"},
			},
			check: func(t *testing.T, req map[string]any) {
				stop, ok := req["stop"].([]string)
				if !ok || len(stop) != 2 {
					t.Error("expected stop sequences")
				}
			},
		},
		{
			name:     "with penalties",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				FrequencyPenalty: 0.5,
				PresencePenalty:  0.3,
			},
			check: func(t *testing.T, req map[string]any) {
				if req["frequency_penalty"] != 0.5 {
					t.Errorf("expected frequency_penalty 0.5, got %v", req["frequency_penalty"])
				}
				if req["presence_penalty"] != 0.3 {
					t.Errorf("expected presence_penalty 0.3, got %v", req["presence_penalty"])
				}
			},
		},
		{
			name:     "with TopP exactly 1.0 (should be omitted)",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				TopP: 1.0,
			},
			check: func(t *testing.T, req map[string]any) {
				if _, exists := req["top_p"]; exists {
					t.Error("expected top_p to be omitted when exactly 1.0")
				}
			},
		},
		{
			name:     "with TopP non-default (should be included)",
			messages: []core.Message{{Role: "user", Content: "test"}},
			options: &core.GenerateOptions{
				TopP: 0.7,
			},
			check: func(t *testing.T, req map[string]any) {
				if req["top_p"] != 0.7 {
					t.Errorf("expected top_p 0.7, got %v", req["top_p"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := lm.buildRequest(tt.messages, tt.options)
			tt.check(t, req)
		})
	}
}

func TestOpenAI_ConvertMessages(t *testing.T) {
	lm := &openAI{}

	tests := []struct {
		name     string
		messages []core.Message
		check    func(t *testing.T, converted []map[string]any)
	}{
		{
			name: "basic message",
			messages: []core.Message{
				{Role: "user", Content: "Hello"},
			},
			check: func(t *testing.T, converted []map[string]any) {
				if len(converted) != 1 {
					t.Fatalf("expected 1 message, got %d", len(converted))
				}
				if converted[0]["role"] != "user" {
					t.Errorf("expected role user, got %v", converted[0]["role"])
				}
				if converted[0]["content"] != "Hello" {
					t.Errorf("expected content Hello, got %v", converted[0]["content"])
				}
			},
		},
		{
			name: "tool response",
			messages: []core.Message{
				{Role: "tool", Content: "result", ToolID: "call_123"},
			},
			check: func(t *testing.T, converted []map[string]any) {
				if converted[0]["tool_call_id"] != "call_123" {
					t.Errorf("expected tool_call_id call_123, got %v", converted[0]["tool_call_id"])
				}
			},
		},
		{
			name: "assistant with tool calls",
			messages: []core.Message{
				{
					Role:    "assistant",
					Content: "Let me check",
					ToolCalls: []core.ToolCall{
						{
							ID:        "call_123",
							Name:      "search",
							Arguments: map[string]any{"query": "test"},
						},
					},
				},
			},
			check: func(t *testing.T, converted []map[string]any) {
				if converted[0]["content"] != "Let me check" {
					t.Errorf("expected content, got %v", converted[0]["content"])
				}
				toolCalls, ok := converted[0]["tool_calls"].([]map[string]any)
				if !ok || len(toolCalls) != 1 {
					t.Fatal("expected tool_calls")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted := lm.convertMessages(tt.messages)
			tt.check(t, converted)
		})
	}
}

func TestOpenAI_ConvertTool(t *testing.T) {
	lm := &openAI{}
	tool := core.NewTool("test_tool", "A test tool", nil)
	tool.AddParameter("param1", "string", "First param", true)
	tool.AddEnumParameter("param2", "Second param", []string{"a", "b"}, false)

	converted := lm.convertTool(tool)

	if converted["type"] != "function" {
		t.Errorf("expected type function, got %v", converted["type"])
	}

	fn, ok := converted["function"].(map[string]any)
	if !ok {
		t.Fatal("expected function object")
	}
	if fn["name"] != "test_tool" {
		t.Errorf("expected name test_tool, got %v", fn["name"])
	}

	params, ok := fn["parameters"].(map[string]any)
	if !ok {
		t.Fatal("expected parameters object")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties object")
	}

	param2, ok := props["param2"].(map[string]any)
	if !ok {
		t.Fatal("expected param2 object")
	}
	if _, ok := param2["enum"]; !ok {
		t.Error("expected enum in param2")
	}

	required, ok := params["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "param1" {
		t.Error("expected param1 to be required")
	}
}

func TestOpenAI_ParseResponse_InvalidToolArgs(t *testing.T) {
	lm := &openAI{}
	resp := &openAIResponse{
		Choices: []struct {
			Index        int           `json:"index"`
			Message      openAIMessage `json:"message"`
			FinishReason string        `json:"finish_reason"`
		}{
			{
				Message: openAIMessage{
					ToolCalls: []openAIToolCall{
						{
							ID: "call_123",
							Function: struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							}{
								Name:      "test",
								Arguments: "invalid json",
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{},
	}

	_, err := lm.parseResponse(resp)
	if err == nil {
		t.Fatal("expected error for invalid tool arguments")
	}
}

func TestOpenAI_Generate_WithToolChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		if tc, ok := req["tool_choice"].(map[string]any); !ok {
			t.Error("expected tool_choice object")
		} else if fn, ok := tc["function"].(map[string]any); !ok {
			t.Error("expected function object in tool_choice")
		} else if fn["name"] != "specific_tool" {
			t.Errorf("expected tool name specific_tool, got %v", fn["name"])
		}

		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{{Message: openAIMessage{Content: "ok"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	options := core.DefaultGenerateOptions()
	options.Tools = []core.Tool{*core.NewTool("specific_tool", "desc", nil)}
	options.ToolChoice = "specific_tool"

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAI_Generate_ToolChoiceNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req["tool_choice"] != "none" {
			t.Errorf("expected tool_choice none, got %v", req["tool_choice"])
		}

		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{{Message: openAIMessage{Content: "ok"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	options := core.DefaultGenerateOptions()
	options.Tools = []core.Tool{*core.NewTool("tool", "desc", nil)}
	options.ToolChoice = "none"

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// fakeCache is a simple in-memory cache for testing
type fakeCache struct {
	data   map[string]*core.GenerateResult
	setKey string
	setVal *core.GenerateResult
}

func (f *fakeCache) Get(key string) (*core.GenerateResult, bool) {
	if f.data == nil {
		return nil, false
	}
	val, ok := f.data[key]
	return val, ok
}

func (f *fakeCache) Set(key string, result *core.GenerateResult) {
	f.setKey = key
	f.setVal = result
}

func (f *fakeCache) Clear() {
	if f.data != nil {
		f.data = make(map[string]*core.GenerateResult)
	}
}

func (f *fakeCache) Size() int {
	if f.data == nil {
		return 0
	}
	return len(f.data)
}

func (f *fakeCache) Capacity() int {
	return 1000 // Fixed capacity for fake cache
}

func (f *fakeCache) Stats() core.CacheStats {
	return core.CacheStats{
		Hits:   0,
		Misses: 0,
		Size:   f.Size(),
	}
}

func TestOpenAI_Generate_CacheHit(t *testing.T) {
	cachedResult := &core.GenerateResult{
		Content:      "cached response",
		FinishReason: "stop",
		Usage: core.Usage{
			PromptTokens:     5,
			CompletionTokens: 3,
			TotalTokens:      8,
		},
	}

	messages := []core.Message{{Role: "user", Content: "test"}}
	options := core.DefaultGenerateOptions()
	cacheKey := core.GenerateCacheKey("gpt-4", messages, options)

	cache := &fakeCache{
		data: map[string]*core.GenerateResult{
			cacheKey: cachedResult,
		},
	}

	lm := &openAI{
		APIKey: "test-key",
		Model:  "gpt-4",
		Cache:  cache,
		Client: nil, // Should not be used if cache hits
	}

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "cached response" {
		t.Errorf("expected cached content, got %s", result.Content)
	}
	if result.Usage.PromptTokens != 5 {
		t.Errorf("expected 5 prompt tokens, got %d", result.Usage.PromptTokens)
	}
}

func TestOpenAI_Generate_CacheSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{{Message: openAIMessage{Content: "fresh response"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cache := &fakeCache{data: map[string]*core.GenerateResult{}}
	messages := []core.Message{{Role: "user", Content: "test"}}
	options := core.DefaultGenerateOptions()
	expectedKey := core.GenerateCacheKey("gpt-4", messages, options)

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
		Cache:   cache,
	}

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "fresh response" {
		t.Errorf("expected fresh response, got %s", result.Content)
	}

	if cache.setKey != expectedKey {
		t.Errorf("expected cache key %s, got %s", expectedKey, cache.setKey)
	}
	if cache.setVal != result {
		t.Error("expected result to be cached")
	}
}

func TestOpenAI_Generate_JSONDecodeError(t *testing.T) {
	originalEnv := os.Getenv("DSGO_SAVE_RAW_RESPONSES")
	defer func() { _ = os.Setenv("DSGO_SAVE_RAW_RESPONSES", originalEnv) }()

	_ = os.Setenv("DSGO_SAVE_RAW_RESPONSES", "1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{invalid json"))
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !containsString(err.Error(), "failed to decode response") {
		t.Errorf("expected 'failed to decode response' error, got %v", err)
	}
}

func TestOpenAI_Generate_ParseResponseError(t *testing.T) {
	originalEnv := os.Getenv("DSGO_SAVE_RAW_RESPONSES")
	defer func() { _ = os.Setenv("DSGO_SAVE_RAW_RESPONSES", originalEnv) }()

	_ = os.Setenv("DSGO_SAVE_RAW_RESPONSES", "1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Message: openAIMessage{
						ToolCalls: []openAIToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
									Name:      "test_tool",
									Arguments: "not valid json",
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err == nil {
		t.Fatal("expected error for invalid tool arguments")
	}
	if !containsString(err.Error(), "failed to parse tool arguments") {
		t.Errorf("expected 'failed to parse tool arguments' error, got %v", err)
	}
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestOpenAI_Stream_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req["stream"] != true {
			t.Error("expected stream to be true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"},\"finish_reason\":\"\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	chunkChan, errChan := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())

	var chunks []core.Chunk
	var streamErr error
	done := false

	for !done {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				done = true
				break
			}
			chunks = append(chunks, chunk)
		case err, ok := <-errChan:
			if ok && err != nil {
				streamErr = err
			}
			done = true
		}
	}

	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}

	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk, got %d", len(chunks))
	}

	var fullContent string
	var lastChunk core.Chunk
	for _, chunk := range chunks {
		fullContent += chunk.Content
		lastChunk = chunk
	}

	if fullContent != "Hello" {
		t.Errorf("expected content 'Hello', got %s", fullContent)
	}

	if lastChunk.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %s", lastChunk.FinishReason)
	}

	if lastChunk.Usage.PromptTokens != 1 || lastChunk.Usage.CompletionTokens != 2 || lastChunk.Usage.TotalTokens != 3 {
		t.Errorf("expected usage 1/2/3, got %d/%d/%d",
			lastChunk.Usage.PromptTokens, lastChunk.Usage.CompletionTokens, lastChunk.Usage.TotalTokens)
	}
}

func TestOpenAI_Stream_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	chunkChan, errChan := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())

	var streamErr error
	done := false

	for !done {
		select {
		case _, ok := <-chunkChan:
			if !ok {
				done = true
			}
		case err, ok := <-errChan:
			if ok && err != nil {
				streamErr = err
			}
			done = true
		}
	}

	if streamErr == nil {
		t.Fatal("expected error for non-OK status")
	}
	if !containsString(streamErr.Error(), "API request failed with status") {
		t.Errorf("expected 'API request failed' error, got %v", streamErr)
	}
}

func TestOpenAI_Stream_InvalidJSONChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte("data: invalid json\n\n"))
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	chunkChan, errChan := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())

	var streamErr error
	done := false

	for !done {
		select {
		case _, ok := <-chunkChan:
			if !ok {
				done = true
			}
		case err, ok := <-errChan:
			if ok && err != nil {
				streamErr = err
			}
			done = true
		}
	}

	if streamErr == nil {
		t.Fatal("expected error for invalid JSON chunk")
	}
	if !containsString(streamErr.Error(), "failed to parse stream chunk") {
		t.Errorf("expected 'failed to parse stream chunk' error, got %v", streamErr)
	}
}

func TestOpenAI_Stream_SkipsNonDataLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte("\n"))
		_, _ = w.Write([]byte("event: message\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	chunkChan, errChan := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())

	var chunks []core.Chunk
	var streamErr error
	done := false

	for !done {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				done = true
				break
			}
			chunks = append(chunks, chunk)
		case err, ok := <-errChan:
			if ok {
				streamErr = err
			}
			done = true
		}
	}

	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0].Content != "OK" {
		t.Errorf("expected content 'OK', got %s", chunks[0].Content)
	}
}

func TestOpenAI_Stream_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-4\",\"choices\":[]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	chunkChan, errChan := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())

	var chunks []core.Chunk
	var streamErr error
	done := false

	for !done {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				done = true
				break
			}
			chunks = append(chunks, chunk)
		case err, ok := <-errChan:
			if ok {
				streamErr = err
			}
			done = true
		}
	}

	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}

	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks, got %d", len(chunks))
	}
}

// TestOpenAI_InitRegistration tests that OpenAI provider is registered
// This verifies the init() function properly registers the provider
func TestOpenAI_InitRegistration(t *testing.T) {
	// Test that the factory function registered in init() works
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer func() { _ = os.Setenv("OPENAI_API_KEY", originalKey) }()

	_ = os.Setenv("OPENAI_API_KEY", "test-registration-key")

	// Get LM through the registered factory
	lm, err := core.NewLM(context.Background(), "openai/gpt-4-registration")
	if err != nil {
		t.Fatalf("NewLM failed: %v", err)
	}
	if lm == nil {
		t.Fatal("NewLM returned nil for openai provider")
	}

	// Verify the model name is set correctly
	if lm.Name() != "gpt-4-registration" {
		t.Errorf("expected model name gpt-4-registration, got %s", lm.Name())
	}

	// Test direct construction
	lm2 := newOpenAI("gpt-4-test")

	if lm2 == nil {
		t.Fatal("NewOpenAI returned nil")
	}

	if lm2.Model != "gpt-4-test" {
		t.Errorf("expected model gpt-4-test, got %s", lm2.Model)
	}

	if lm2.BaseURL != defaultBaseURL {
		t.Errorf("expected BaseURL %s, got %s", defaultBaseURL, lm2.BaseURL)
	}

	if lm2.Client == nil {
		t.Error("expected Client to be initialized")
	}
}

func TestOpenAI_ExtractMetadata(t *testing.T) {
	lm := &openAI{}

	tests := []struct {
		name     string
		headers  http.Header
		expected map[string]any
	}{
		{
			name: "all OpenAI-specific headers",
			headers: http.Header{
				"X-Ratelimit-Limit-Requests":     []string{"3000"},
				"X-Ratelimit-Remaining-Requests": []string{"2999"},
				"X-Ratelimit-Limit-Tokens":       []string{"90000"},
				"X-Ratelimit-Remaining-Tokens":   []string{"89000"},
				"X-Request-Id":                   []string{"req_12345"},
				"Openai-Organization":            []string{"org-abc123"},
				"Cf-Cache-Status":                []string{"HIT"},
			},
			expected: map[string]any{
				"rate_limit_requests":           "3000",
				"rate_limit_remaining_requests": "2999",
				"rate_limit_tokens":             "90000",
				"rate_limit_remaining_tokens":   "89000",
				"request_id":                    "req_12345",
				"organization":                  "org-abc123",
				"cache_status":                  "HIT",
				"cache_hit":                     true,
			},
		},
		{
			name: "cache MISS",
			headers: http.Header{
				"Cf-Cache-Status": []string{"MISS"},
			},
			expected: map[string]any{
				"cache_status": "MISS",
				"cache_hit":    false,
			},
		},
		{
			name: "subset of headers",
			headers: http.Header{
				"X-Request-Id":    []string{"req_99999"},
				"X-Cache":         []string{"HIT from cloudflare"},
				"Cf-Cache-Status": []string{"DYNAMIC"},
			},
			expected: map[string]any{
				"request_id":   "req_99999",
				"x_cache":      "HIT from cloudflare",
				"cache_status": "DYNAMIC",
				"cache_hit":    false,
			},
		},
		{
			name:     "empty headers",
			headers:  http.Header{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := lm.extractMetadata(tt.headers)

			if len(metadata) != len(tt.expected) {
				t.Errorf("expected %d metadata entries, got %d", len(tt.expected), len(metadata))
			}

			for key, expectedVal := range tt.expected {
				actualVal, exists := metadata[key]
				if !exists {
					t.Errorf("expected metadata key %s to exist", key)
					continue
				}
				if actualVal != expectedVal {
					t.Errorf("for key %s: expected %v, got %v", key, expectedVal, actualVal)
				}
			}
		})
	}
}

func TestOpenAI_Generate_SaveRawExchange(t *testing.T) {
	originalEnv := os.Getenv("DSGO_SAVE_RAW_RESPONSES")
	defer func() { _ = os.Setenv("DSGO_SAVE_RAW_RESPONSES", originalEnv) }()

	_ = os.Setenv("DSGO_SAVE_RAW_RESPONSES", "1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{{Message: openAIMessage{Content: "ok"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAI_Generate_WithMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit-Requests", "5000")
		w.Header().Set("X-RateLimit-Remaining-Requests", "4999")
		w.Header().Set("X-RateLimit-Limit-Tokens", "100000")
		w.Header().Set("X-RateLimit-Remaining-Tokens", "99500")
		w.Header().Set("X-Request-ID", "req_abc123")
		w.Header().Set("Openai-Organization", "org-test")
		w.Header().Set("CF-Cache-Status", "MISS")
		w.Header().Set("X-Cache", "MISS")

		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{{Message: openAIMessage{Content: "response"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	result, err := lm.Generate(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Metadata == nil {
		t.Fatal("expected metadata to be populated")
	}

	expectedMetadata := map[string]any{
		"rate_limit_requests":           "5000",
		"rate_limit_remaining_requests": "4999",
		"rate_limit_tokens":             "100000",
		"rate_limit_remaining_tokens":   "99500",
		"request_id":                    "req_abc123",
		"organization":                  "org-test",
		"cache_status":                  "MISS",
		"cache_hit":                     false,
		"x_cache":                       "MISS",
	}

	for key, expectedVal := range expectedMetadata {
		actualVal, exists := result.Metadata[key]
		if !exists {
			t.Errorf("expected metadata key %s to exist", key)
			continue
		}
		if actualVal != expectedVal {
			t.Errorf("for key %s: expected %v, got %v", key, expectedVal, actualVal)
		}
	}
}
