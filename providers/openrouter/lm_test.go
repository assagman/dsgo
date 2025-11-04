package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/assagman/dsgo"
)

func TestNewOpenRouter(t *testing.T) {
	originalKey := os.Getenv("OPENROUTER_API_KEY")
	originalSiteName := os.Getenv("OPENROUTER_SITE_NAME")
	originalSiteURL := os.Getenv("OPENROUTER_SITE_URL")
	defer func() {
		_ = os.Setenv("OPENROUTER_API_KEY", originalKey)
		_ = os.Setenv("OPENROUTER_SITE_NAME", originalSiteName)
		_ = os.Setenv("OPENROUTER_SITE_URL", originalSiteURL)
	}()

	_ = os.Setenv("OPENROUTER_API_KEY", "test-key")
	_ = os.Setenv("OPENROUTER_SITE_NAME", "test-site")
	_ = os.Setenv("OPENROUTER_SITE_URL", "https://test.com")

	lm := NewOpenRouter("gpt-4")
	if lm.APIKey != "test-key" {
		t.Errorf("expected APIKey test-key, got %s", lm.APIKey)
	}
	if lm.Model != "gpt-4" {
		t.Errorf("expected Model gpt-4, got %s", lm.Model)
	}
	if lm.BaseURL != DefaultBaseURL {
		t.Errorf("expected BaseURL %s, got %s", DefaultBaseURL, lm.BaseURL)
	}
	if lm.SiteName != "test-site" {
		t.Errorf("expected SiteName test-site, got %s", lm.SiteName)
	}
	if lm.SiteURL != "https://test.com" {
		t.Errorf("expected SiteURL https://test.com, got %s", lm.SiteURL)
	}
	if lm.Client == nil {
		t.Error("expected Client to be initialized")
	}
}

func TestOpenRouter_Name(t *testing.T) {
	lm := &OpenRouter{Model: "gpt-4-turbo"}
	if lm.Name() != "gpt-4-turbo" {
		t.Errorf("expected Name gpt-4-turbo, got %s", lm.Name())
	}
}

func TestOpenRouter_SupportsJSON(t *testing.T) {
	lm := &OpenRouter{}
	if !lm.SupportsJSON() {
		t.Error("expected SupportsJSON to return true")
	}
}

func TestOpenRouter_SupportsTools(t *testing.T) {
	lm := &OpenRouter{}
	if !lm.SupportsTools() {
		t.Error("expected SupportsTools to return true")
	}
}

func TestOpenRouter_ExtractMetadata(t *testing.T) {
	lm := &OpenRouter{}

	t.Run("extracts all headers", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("CF-Cache-Status", "HIT")
		headers.Set("X-Cache", "HIT from cloudflare")
		headers.Set("X-RateLimit-Limit", "1000")
		headers.Set("X-RateLimit-Remaining", "999")
		headers.Set("X-RateLimit-Reset", "1234567890")
		headers.Set("X-OpenRouter-Generation-ID", "gen-123")

		metadata := lm.extractMetadata(headers)

		if metadata["cache_status"] != "HIT" {
			t.Errorf("expected cache_status HIT, got %v", metadata["cache_status"])
		}
		if metadata["cache_hit"] != true {
			t.Errorf("expected cache_hit true, got %v", metadata["cache_hit"])
		}
		if metadata["x_cache"] != "HIT from cloudflare" {
			t.Errorf("expected x_cache, got %v", metadata["x_cache"])
		}
		if metadata["rate_limit_limit"] != "1000" {
			t.Errorf("expected rate_limit_limit 1000, got %v", metadata["rate_limit_limit"])
		}
		if metadata["rate_limit_remaining"] != "999" {
			t.Errorf("expected rate_limit_remaining 999, got %v", metadata["rate_limit_remaining"])
		}
		if metadata["rate_limit_reset"] != "1234567890" {
			t.Errorf("expected rate_limit_reset, got %v", metadata["rate_limit_reset"])
		}
		if metadata["generation_id"] != "gen-123" {
			t.Errorf("expected generation_id gen-123, got %v", metadata["generation_id"])
		}
	})

	t.Run("cache miss", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("CF-Cache-Status", "MISS")

		metadata := lm.extractMetadata(headers)

		if metadata["cache_status"] != "MISS" {
			t.Errorf("expected cache_status MISS, got %v", metadata["cache_status"])
		}
		if metadata["cache_hit"] != false {
			t.Errorf("expected cache_hit false, got %v", metadata["cache_hit"])
		}
	})

	t.Run("empty headers", func(t *testing.T) {
		headers := http.Header{}
		metadata := lm.extractMetadata(headers)

		if len(metadata) != 0 {
			t.Errorf("expected empty metadata, got %v", metadata)
		}
	})

	t.Run("partial headers", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-RateLimit-Limit", "500")

		metadata := lm.extractMetadata(headers)

		if metadata["rate_limit_limit"] != "500" {
			t.Errorf("expected rate_limit_limit 500, got %v", metadata["rate_limit_limit"])
		}
		if len(metadata) != 1 {
			t.Errorf("expected 1 metadata entry, got %d", len(metadata))
		}
	})
}

func TestInit_RegistersLM(t *testing.T) {
	// The init function should register "openrouter" as an LM factory
	// We can test this indirectly by configuring and creating an LM
	ctx := context.Background()

	// Configure with openrouter provider
	dsgo.Configure(
		dsgo.WithProvider("openrouter"),
		dsgo.WithModel("test-model"),
	)

	lm, err := dsgo.NewLM(ctx)
	if err != nil {
		t.Fatalf("expected LM to be created, got error: %v", err)
	}

	if lm.Name() != "test-model" {
		t.Errorf("expected model name test-model, got %s", lm.Name())
	}

	// Verify it's actually an OpenRouter instance (or wrapped)
	// Could be LMWrapper, so we check the base type
	switch v := lm.(type) {
	case *OpenRouter:
		// Direct OpenRouter
	case interface{ Unwrap() dsgo.LM }:
		// Wrapped LM, check the base
		if _, ok := v.Unwrap().(*OpenRouter); !ok {
			t.Errorf("expected wrapped *OpenRouter, got %T", v.Unwrap())
		}
	default:
		t.Errorf("expected *OpenRouter or wrapper, got %T", lm)
	}
}

func TestOpenRouter_Generate_Success(t *testing.T) {
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

		resp := openRouterResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []struct {
				Index        int               `json:"index"`
				Message      openRouterMessage `json:"message"`
				FinishReason string            `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: openRouterMessage{
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

	lm := &OpenRouter{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	messages := []dsgo.Message{
		{Role: "user", Content: "Hello"},
	}
	options := dsgo.DefaultGenerateOptions()

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

func TestOpenRouter_Generate_WithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Title") != "test-site" {
			t.Errorf("expected X-Title test-site, got %s", r.Header.Get("X-Title"))
		}
		if r.Header.Get("HTTP-Referer") != "https://test.com" {
			t.Errorf("expected HTTP-Referer https://test.com, got %s", r.Header.Get("HTTP-Referer"))
		}

		resp := openRouterResponse{
			Choices: []struct {
				Index        int               `json:"index"`
				Message      openRouterMessage `json:"message"`
				FinishReason string            `json:"finish_reason"`
			}{{Message: openRouterMessage{Content: "ok"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenRouter{
		APIKey:   "test-key",
		BaseURL:  server.URL,
		Client:   &http.Client{},
		SiteName: "test-site",
		SiteURL:  "https://test.com",
	}

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "test"}}, dsgo.DefaultGenerateOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenRouter_Generate_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if _, ok := req["tools"]; !ok {
			t.Error("expected tools in request")
		}

		resp := openRouterResponse{
			Choices: []struct {
				Index        int               `json:"index"`
				Message      openRouterMessage `json:"message"`
				FinishReason string            `json:"finish_reason"`
			}{
				{
					Message: openRouterMessage{
						Role: "assistant",
						ToolCalls: []openRouterToolCall{
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

	lm := &OpenRouter{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	messages := []dsgo.Message{{Role: "user", Content: "What's the weather?"}}
	options := dsgo.DefaultGenerateOptions()
	weatherFunc := func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return "sunny", nil
	}
	options.Tools = []dsgo.Tool{
		*dsgo.NewTool("get_weather", "Get weather", weatherFunc),
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return tool call with malformed JSON arguments (single quotes, trailing comma)
		resp := openRouterResponse{
			Choices: []struct {
				Index        int               `json:"index"`
				Message      openRouterMessage `json:"message"`
				FinishReason string            `json:"finish_reason"`
			}{
				{
					Message: openRouterMessage{
						Role: "assistant",
						ToolCalls: []openRouterToolCall{
							{
								ID:   "call_456",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
									Name:      "search",
									Arguments: `{'query': 'test query', 'limit': 10,}`, // Malformed: single quotes + trailing comma
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

	lm := &OpenRouter{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	messages := []dsgo.Message{{Role: "user", Content: "Search for test query"}}
	options := dsgo.DefaultGenerateOptions()
	searchFunc := func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return "results", nil
	}
	options.Tools = []dsgo.Tool{
		*dsgo.NewTool("search", "Search tool", searchFunc).AddParameter("query", "string", "Search query", true),
	}

	result, err := lm.Generate(context.Background(), messages, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	// Verify arguments were repaired and parsed correctly
	if result.ToolCalls[0].Arguments["query"] != "test query" {
		t.Errorf("expected query 'test query', got %v", result.ToolCalls[0].Arguments["query"])
	}
	if result.ToolCalls[0].Arguments["limit"] != float64(10) {
		t.Errorf("expected limit 10, got %v", result.ToolCalls[0].Arguments["limit"])
	}
}

func TestOpenRouter_Generate_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	lm := &OpenRouter{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "test"}}, dsgo.DefaultGenerateOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOpenRouter_Generate_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openRouterResponse{
			Choices: []struct {
				Index        int               `json:"index"`
				Message      openRouterMessage `json:"message"`
				FinishReason string            `json:"finish_reason"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenRouter{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "test"}}, dsgo.DefaultGenerateOptions())
	if err == nil || err.Error() != "no choices in response" {
		t.Fatalf("expected 'no choices in response' error, got %v", err)
	}
}

func TestOpenRouter_BuildRequest(t *testing.T) {
	lm := &OpenRouter{Model: "gpt-4"}

	tests := []struct {
		name     string
		messages []dsgo.Message
		options  *dsgo.GenerateOptions
		check    func(t *testing.T, req map[string]interface{})
	}{
		{
			name:     "basic request",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options:  dsgo.DefaultGenerateOptions(),
			check: func(t *testing.T, req map[string]interface{}) {
				if req["model"] != "gpt-4" {
					t.Errorf("expected model gpt-4, got %v", req["model"])
				}
			},
		},
		{
			name:     "with temperature",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
				Temperature: 0.7,
			},
			check: func(t *testing.T, req map[string]interface{}) {
				if req["temperature"] != 0.7 {
					t.Errorf("expected temperature 0.7, got %v", req["temperature"])
				}
			},
		},
		{
			name:     "with max tokens",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
				MaxTokens: 100,
			},
			check: func(t *testing.T, req map[string]interface{}) {
				if req["max_tokens"] != 100 {
					t.Errorf("expected max_tokens 100, got %v", req["max_tokens"])
				}
			},
		},
		{
			name:     "with top_p",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
				TopP: 0.9,
			},
			check: func(t *testing.T, req map[string]interface{}) {
				if req["top_p"] != 0.9 {
					t.Errorf("expected top_p 0.9, got %v", req["top_p"])
				}
			},
		},
		{
			name:     "with json format (no schema)",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
				ResponseFormat: "json",
			},
			check: func(t *testing.T, req map[string]interface{}) {
				rf, ok := req["response_format"].(map[string]string)
				if !ok || rf["type"] != "json_object" {
					t.Errorf("expected response_format to be json_object, got %v", req["response_format"])
				}
			},
		},
		{
			name:     "with json schema",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
				ResponseFormat: "json",
				ResponseSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"age":  map[string]any{"type": "number"},
					},
					"required": []string{"name"},
				},
			},
			check: func(t *testing.T, req map[string]interface{}) {
				rf, ok := req["response_format"].(map[string]any)
				if !ok {
					t.Fatal("expected response_format to be map[string]any")
				}
				if rf["type"] != "json_schema" {
					t.Errorf("expected type json_schema, got %v", rf["type"])
				}
				schema, ok := rf["schema"].(map[string]any)
				if !ok {
					t.Fatal("expected schema in response_format")
				}
				if schema["type"] != "object" {
					t.Errorf("expected schema type object, got %v", schema["type"])
				}
				props, ok := schema["properties"].(map[string]any)
				if !ok {
					t.Fatal("expected properties in schema")
				}
				if len(props) != 2 {
					t.Errorf("expected 2 properties, got %d", len(props))
				}
			},
		},
		{
			name:     "with penalties",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
				FrequencyPenalty: 0.5,
				PresencePenalty:  0.3,
			},
			check: func(t *testing.T, req map[string]interface{}) {
				if req["frequency_penalty"] != 0.5 {
					t.Errorf("expected frequency_penalty 0.5, got %v", req["frequency_penalty"])
				}
				if req["presence_penalty"] != 0.3 {
					t.Errorf("expected presence_penalty 0.3, got %v", req["presence_penalty"])
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

func TestOpenRouter_ConvertMessages(t *testing.T) {
	lm := &OpenRouter{}

	tests := []struct {
		name     string
		messages []dsgo.Message
		check    func(t *testing.T, converted []map[string]interface{})
	}{
		{
			name: "basic message",
			messages: []dsgo.Message{
				{Role: "user", Content: "Hello"},
			},
			check: func(t *testing.T, converted []map[string]interface{}) {
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
			messages: []dsgo.Message{
				{Role: "tool", Content: "result", ToolID: "call_123"},
			},
			check: func(t *testing.T, converted []map[string]interface{}) {
				if converted[0]["tool_call_id"] != "call_123" {
					t.Errorf("expected tool_call_id call_123, got %v", converted[0]["tool_call_id"])
				}
			},
		},
		{
			name: "assistant with tool calls",
			messages: []dsgo.Message{
				{
					Role:    "assistant",
					Content: "Let me check",
					ToolCalls: []dsgo.ToolCall{
						{
							ID:        "call_123",
							Name:      "search",
							Arguments: map[string]interface{}{"query": "test"},
						},
					},
				},
			},
			check: func(t *testing.T, converted []map[string]interface{}) {
				if converted[0]["content"] != "Let me check" {
					t.Errorf("expected content, got %v", converted[0]["content"])
				}
				toolCalls, ok := converted[0]["tool_calls"].([]map[string]interface{})
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

func TestOpenRouter_ConvertTool(t *testing.T) {
	lm := &OpenRouter{}
	tool := dsgo.NewTool("test_tool", "A test tool", nil)
	tool.AddParameter("param1", "string", "First param", true)
	tool.AddEnumParameter("param2", "Second param", []string{"a", "b"}, false)

	converted := lm.convertTool(tool)

	if converted["type"] != "function" {
		t.Errorf("expected type function, got %v", converted["type"])
	}

	fn, ok := converted["function"].(map[string]interface{})
	if !ok {
		t.Fatal("expected function object")
	}
	if fn["name"] != "test_tool" {
		t.Errorf("expected name test_tool, got %v", fn["name"])
	}

	params, ok := fn["parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("expected parameters object")
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties object")
	}

	param2, ok := props["param2"].(map[string]interface{})
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

func TestOpenRouter_ParseResponse_InvalidToolArgs(t *testing.T) {
	lm := &OpenRouter{}
	resp := &openRouterResponse{
		Choices: []struct {
			Index        int               `json:"index"`
			Message      openRouterMessage `json:"message"`
			FinishReason string            `json:"finish_reason"`
		}{
			{
				Message: openRouterMessage{
					ToolCalls: []openRouterToolCall{
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

func TestOpenRouter_Generate_WithToolChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if tc, ok := req["tool_choice"].(map[string]interface{}); !ok {
			t.Error("expected tool_choice object")
		} else if fn, ok := tc["function"].(map[string]interface{}); !ok {
			t.Error("expected function object in tool_choice")
		} else if fn["name"] != "specific_tool" {
			t.Errorf("expected tool name specific_tool, got %v", fn["name"])
		}

		resp := openRouterResponse{
			Choices: []struct {
				Index        int               `json:"index"`
				Message      openRouterMessage `json:"message"`
				FinishReason string            `json:"finish_reason"`
			}{{Message: openRouterMessage{Content: "ok"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenRouter{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	options := dsgo.DefaultGenerateOptions()
	options.Tools = []dsgo.Tool{*dsgo.NewTool("specific_tool", "desc", nil)}
	options.ToolChoice = "specific_tool"

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "test"}}, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenRouter_Generate_ToolChoiceNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req["tool_choice"] != "none" {
			t.Errorf("expected tool_choice none, got %v", req["tool_choice"])
		}

		resp := openRouterResponse{
			Choices: []struct {
				Index        int               `json:"index"`
				Message      openRouterMessage `json:"message"`
				FinishReason string            `json:"finish_reason"`
			}{{Message: openRouterMessage{Content: "ok"}, FinishReason: "stop"}},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenRouter{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	options := dsgo.DefaultGenerateOptions()
	options.Tools = []dsgo.Tool{*dsgo.NewTool("tool", "desc", nil)}
	options.ToolChoice = "none"

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "test"}}, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenRouter_Stream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Write SSE stream response
		w.Header().Set("Content-Type", "text/event-stream")

		chunks := []string{
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":""}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":""}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n\n"))
		}
	}))
	defer server.Close()

	lm := &OpenRouter{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	messages := []dsgo.Message{{Role: "user", Content: "Hello"}}
	options := dsgo.DefaultGenerateOptions()

	chunkChan, errChan := lm.Stream(context.Background(), messages, options)

	var content string
	var finalUsage dsgo.Usage
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	lm := &OpenRouter{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	chunkChan, errChan := lm.Stream(context.Background(), []dsgo.Message{{Role: "user", Content: "test"}}, dsgo.DefaultGenerateOptions())

	// Should get error
	for range chunkChan {
		t.Error("should not receive chunks on error")
	}

	err := <-errChan
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
