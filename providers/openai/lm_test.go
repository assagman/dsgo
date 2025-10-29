package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/assagman/dsgo"
)

func TestNewOpenAI(t *testing.T) {
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalKey)

	os.Setenv("OPENAI_API_KEY", "test-key")

	lm := NewOpenAI("gpt-4")
	if lm.APIKey != "test-key" {
		t.Errorf("expected APIKey test-key, got %s", lm.APIKey)
	}
	if lm.Model != "gpt-4" {
		t.Errorf("expected Model gpt-4, got %s", lm.Model)
	}
	if lm.BaseURL != DefaultBaseURL {
		t.Errorf("expected BaseURL %s, got %s", DefaultBaseURL, lm.BaseURL)
	}
	if lm.Client == nil {
		t.Error("expected Client to be initialized")
	}
}

func TestOpenAI_Name(t *testing.T) {
	lm := &OpenAI{Model: "gpt-4-turbo"}
	if lm.Name() != "gpt-4-turbo" {
		t.Errorf("expected Name gpt-4-turbo, got %s", lm.Name())
	}
}

func TestOpenAI_SupportsJSON(t *testing.T) {
	lm := &OpenAI{}
	if !lm.SupportsJSON() {
		t.Error("expected SupportsJSON to return true")
	}
}

func TestOpenAI_SupportsTools(t *testing.T) {
	lm := &OpenAI{}
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
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenAI{
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

func TestOpenAI_Generate_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

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
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenAI{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	messages := []dsgo.Message{{Role: "user", Content: "What's the weather?"}}
	options := dsgo.DefaultGenerateOptions()
	weatherFunc := func(ctx context.Context, args map[string]any) (any, error) {
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

func TestOpenAI_Generate_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	lm := &OpenAI{
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

func TestOpenAI_Generate_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenAI{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Client:  &http.Client{},
	}

	_, err := lm.Generate(context.Background(), []dsgo.Message{{Role: "user", Content: "test"}}, dsgo.DefaultGenerateOptions())
	if err == nil || err.Error() != "no choices in response" {
		t.Fatalf("expected 'no choices in response' error, got %v", err)
	}
}

func TestOpenAI_BuildRequest(t *testing.T) {
	lm := &OpenAI{Model: "gpt-4"}

	tests := []struct {
		name     string
		messages []dsgo.Message
		options  *dsgo.GenerateOptions
		check    func(t *testing.T, req map[string]any)
	}{
		{
			name:     "basic request",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options:  dsgo.DefaultGenerateOptions(),
			check: func(t *testing.T, req map[string]any) {
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
			check: func(t *testing.T, req map[string]any) {
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
			check: func(t *testing.T, req map[string]any) {
				if req["max_tokens"] != 100 {
					t.Errorf("expected max_tokens 100, got %v", req["max_tokens"])
				}
			},
		},
		{
			name:     "with json format",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
				ResponseFormat: "json",
			},
			check: func(t *testing.T, req map[string]any) {
				rf, ok := req["response_format"].(map[string]string)
				if !ok || rf["type"] != "json_object" {
					t.Error("expected response_format to be json_object")
				}
			},
		},
		{
			name:     "with stop sequences",
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
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
			messages: []dsgo.Message{{Role: "user", Content: "test"}},
			options: &dsgo.GenerateOptions{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := lm.buildRequest(tt.messages, tt.options)
			tt.check(t, req)
		})
	}
}

func TestOpenAI_ConvertMessages(t *testing.T) {
	lm := &OpenAI{}

	tests := []struct {
		name     string
		messages []dsgo.Message
		check    func(t *testing.T, converted []map[string]any)
	}{
		{
			name: "basic message",
			messages: []dsgo.Message{
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
			messages: []dsgo.Message{
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
			messages: []dsgo.Message{
				{
					Role:    "assistant",
					Content: "Let me check",
					ToolCalls: []dsgo.ToolCall{
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
	lm := &OpenAI{}
	tool := dsgo.NewTool("test_tool", "A test tool", nil)
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
	lm := &OpenAI{}
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
		json.NewDecoder(r.Body).Decode(&req)

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
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenAI{
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

func TestOpenAI_Generate_ToolChoiceNone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

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
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	lm := &OpenAI{
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
