package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assagman/dsgo/internal/core"
	openaiSDK "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func TestNewOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

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
}

func TestOpenAI_Name(t *testing.T) {
	t.Parallel()
	lm := &openAI{Model: "gpt-4-turbo"}
	if lm.Name() != "gpt-4-turbo" {
		t.Errorf("expected Name gpt-4-turbo, got %s", lm.Name())
	}
}

func TestOpenAI_SupportsJSON(t *testing.T) {
	t.Parallel()
	lm := &openAI{}
	if !lm.SupportsJSON() {
		t.Error("expected SupportsJSON to return true")
	}
}

func TestOpenAI_SupportsTools(t *testing.T) {
	t.Parallel()
	lm := &openAI{}
	if !lm.SupportsTools() {
		t.Error("expected SupportsTools to return true")
	}
}

func TestOpenAI_IsOpenAI(t *testing.T) {
	t.Parallel()
	lm := &openAI{}
	if !lm.IsOpenAI() {
		t.Error("expected IsOpenAI to return true")
	}
}

func TestOpenAI_BuildParams_ReasoningModelsUseMaxCompletionTokens(t *testing.T) {
	t.Parallel()
	lm := &openAI{Model: "o1"}

	params := lm.buildParams([]core.Message{{Role: "user", Content: "hi"}}, &core.GenerateOptions{MaxTokens: 123})

	if !params.MaxCompletionTokens.Valid() {
		t.Fatal("expected MaxCompletionTokens to be set")
	}
	if params.MaxCompletionTokens.Value != 123 {
		t.Errorf("expected MaxCompletionTokens 123, got %d", params.MaxCompletionTokens.Value)
	}
	if params.MaxTokens.Valid() {
		t.Errorf("expected MaxTokens to be omitted for reasoning models, got %d", params.MaxTokens.Value)
	}
}

func TestOpenAI_BuildParams_NonReasoningModelsUseMaxTokens(t *testing.T) {
	t.Parallel()
	lm := &openAI{Model: "gpt-4o"}

	params := lm.buildParams([]core.Message{{Role: "user", Content: "hi"}}, &core.GenerateOptions{MaxTokens: 123})

	if !params.MaxTokens.Valid() {
		t.Fatal("expected MaxTokens to be set")
	}
	if params.MaxTokens.Value != 123 {
		t.Errorf("expected MaxTokens 123, got %d", params.MaxTokens.Value)
	}
	if params.MaxCompletionTokens.Valid() {
		t.Errorf("expected MaxCompletionTokens to be omitted for non-reasoning models, got %d", params.MaxCompletionTokens.Value)
	}
}

func TestOpenAI_BuildParams_ToolChoice(t *testing.T) {
	t.Parallel()

	t.Run("none", func(t *testing.T) {
		lm := &openAI{Model: "gpt-4o"}
		tool := core.NewTool("test_tool", "A test tool", nil)

		opts := &core.GenerateOptions{Tools: []core.Tool{*tool}, ToolChoice: "none"}
		params := lm.buildParams([]core.Message{{Role: "user", Content: "hi"}}, opts)

		if !params.ToolChoice.OfAuto.Valid() {
			t.Fatal("expected ToolChoice.OfAuto to be set")
		}
		if params.ToolChoice.OfAuto.Value != "none" {
			t.Errorf("expected tool choice none, got %q", params.ToolChoice.OfAuto.Value)
		}
	})

	t.Run("specific tool", func(t *testing.T) {
		lm := &openAI{Model: "gpt-4o"}
		tool := core.NewTool("specific_tool", "A test tool", nil)

		opts := &core.GenerateOptions{Tools: []core.Tool{*tool}, ToolChoice: "specific_tool"}
		params := lm.buildParams([]core.Message{{Role: "user", Content: "hi"}}, opts)

		fn := params.ToolChoice.GetFunction()
		if fn == nil {
			t.Fatal("expected function tool choice")
		}
		if fn.Name != "specific_tool" {
			t.Errorf("expected specific_tool, got %q", fn.Name)
		}
	})

	t.Run("with provider params", func(t *testing.T) {
		lm := &openAI{Model: "gpt-4o"}
		messages := []core.Message{{Role: "user", Content: "test"}}
		options := &core.GenerateOptions{
			Temperature: 0.7,
			ProviderParams: map[string]any{
				"seed":        42,
				"temperature": 1.2, // should be ignored
				"top_k":       50,
			},
		}
		params := lm.buildParams(messages, options)

		data, _ := json.Marshal(params)
		var m map[string]any
		_ = json.Unmarshal(data, &m)

		if seed, ok := m["seed"].(float64); !ok || int(seed) != 42 {
			t.Errorf("expected seed 42, got %v", m["seed"])
		}
		if topK, ok := m["top_k"].(float64); !ok || int(topK) != 50 {
			t.Errorf("expected top_k 50, got %v", m["top_k"])
		}
		if temp, ok := m["temperature"].(float64); !ok || temp != 0.7 {
			t.Errorf("expected temperature 0.7 (DSGo-managed), got %v", m["temperature"])
		}
	})

	t.Run("with max tokens", func(t *testing.T) {
		lm := &openAI{Model: "gpt-4o"}
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

	t.Run("with penalties", func(t *testing.T) {
		lm := &openAI{Model: "gpt-4o"}
		messages := []core.Message{{Role: "user", Content: "test"}}
		options := &core.GenerateOptions{
			FrequencyPenalty: 0.5,
			PresencePenalty:  0.3,
		}
		params := lm.buildParams(messages, options)

		data, _ := json.Marshal(params)
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		if fp, ok := m["frequency_penalty"].(float64); !ok || fp != 0.5 {
			t.Errorf("expected frequency_penalty 0.5, got %v", m["frequency_penalty"])
		}
		if pp, ok := m["presence_penalty"].(float64); !ok || pp != 0.3 {
			t.Errorf("expected presence_penalty 0.3, got %v", m["presence_penalty"])
		}
	})
}

func TestOpenAI_ConvertMessages(t *testing.T) {
	t.Parallel()
	lm := &openAI{}

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

func TestOpenAI_ConvertTool(t *testing.T) {
	t.Parallel()
	lm := &openAI{}

	tool := core.NewTool("test_tool", "A test tool", nil)
	tool.AddParameter("param1", "string", "First param", true)
	tool.AddEnumParameter("param2", "Second param", []string{"a", "b"}, false)

	converted := lm.convertTool(tool)

	fn := converted.GetFunction()
	if fn == nil {
		t.Fatal("expected function tool")
	}
	if fn.Name != "test_tool" {
		t.Errorf("expected name test_tool, got %q", fn.Name)
	}

	properties, ok := fn.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected parameters.properties to be an object, got %T", fn.Parameters["properties"])
	}

	param2, ok := properties["param2"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties.param2 to be an object, got %T", properties["param2"])
	}
	if _, ok := param2["enum"]; !ok {
		t.Error("expected enum in param2")
	}

	required, ok := fn.Parameters["required"].([]string)
	if !ok {
		t.Fatalf("expected parameters.required to be []string, got %T", fn.Parameters["required"])
	}
	if len(required) != 1 || required[0] != "param1" {
		t.Errorf("expected required to be [param1], got %v", required)
	}
}

func TestOpenAI_ParseResponse_InvalidToolArgs(t *testing.T) {
	t.Parallel()
	lm := &openAI{Model: "gpt-4o"}

	resp := &openaiSDK.ChatCompletion{
		Choices: []openaiSDK.ChatCompletionChoice{
			{
				Message: openaiSDK.ChatCompletionMessage{
					ToolCalls: []openaiSDK.ChatCompletionMessageToolCallUnion{
						{
							ID:   "call_123",
							Type: "function",
							Function: openaiSDK.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "test",
								Arguments: "invalid json",
							},
						},
					},
				},
			},
		},
	}

	_, err := lm.parseResponse(resp)
	if err == nil {
		t.Fatal("expected error for invalid tool arguments")
	}
}

func TestOpenAI_ParseResponse_ToolCalls(t *testing.T) {
	t.Parallel()
	lm := &openAI{Model: "gpt-4o"}

	args := map[string]any{"location": "NYC"}
	argsBytes, _ := json.Marshal(args)
	badID := "call 123"
	wantID := sanitizeToolCallID(badID)

	resp := &openaiSDK.ChatCompletion{
		Choices: []openaiSDK.ChatCompletionChoice{
			{
				FinishReason: "tool_calls",
				Message: openaiSDK.ChatCompletionMessage{
					ToolCalls: []openaiSDK.ChatCompletionMessageToolCallUnion{
						{
							ID:   badID,
							Type: "function",
							Function: openaiSDK.ChatCompletionMessageFunctionToolCallFunction{
								Name:      "get_weather",
								Arguments: string(argsBytes),
							},
						},
					},
				},
			},
		},
	}

	result, err := lm.parseResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != wantID {
		t.Errorf("expected tool call ID %q, got %q", wantID, result.ToolCalls[0].ID)
	}
	if result.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected tool name get_weather, got %q", result.ToolCalls[0].Name)
	}
	if got := result.ToolCalls[0].Arguments["location"]; got != "NYC" {
		t.Errorf("expected location NYC, got %v", got)
	}
}

func TestOpenAI_IsReasoningModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{"o1 basic", "o1", true},
		{"o1 with dash", "o1-preview", true},
		{"o1 mini", "o1-mini", true},
		{"o3 basic", "o3", true},
		{"o3 with dash", "o3-preview", true},
		{"o3 mini", "o3-mini", true},
		{"o4 basic", "o4", true},
		{"o4 mini", "o4-mini", true},
		{"gpt-5 basic", "gpt-5", true},
		{"gpt-5 with dash", "gpt-5-turbo", true},
		{"gpt-3.5-turbo", "gpt-3.5-turbo", false},
		{"gpt-4", "gpt-4", false},
		{"gpt-4-turbo", "gpt-4-turbo", false},
		{"gpt-4o", "gpt-4o", false},
		{"gpt-4o-mini", "gpt-4o-mini", false},
		{"custom o1 model", "my-o1-custom", false},
		{"o1 in middle", "model-o1-test", false},
		{"uppercase O1", "O1", true},
		{"mixed case O3", "O3-pReViEw", true},
		{"uppercase GPT-5", "GPT-5", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lm := &openAI{Model: tt.model}
			result := lm.isReasoningModel()
			if result != tt.expected {
				t.Errorf("isReasoningModel() for model %q = %v, expected %v", tt.model, result, tt.expected)
			}
		})
	}
}

func createTestOpenAI(t *testing.T, server *httptest.Server) *openAI {
	t.Helper()

	client := openaiSDK.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
	)

	return &openAI{
		APIKey:  "test-key",
		Model:   "gpt-4o",
		BaseURL: server.URL,
		Client:  client,
	}
}

func TestOpenAI_Stream_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":123,"model":"gpt-4","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
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

	lm := createTestOpenAI(t, server)

	messages := []core.Message{{Role: "user", Content: "Hello"}}
	options := core.DefaultGenerateOptions()

	chunkChan, errChan := lm.Stream(context.Background(), messages, options)

	var chunks []core.Chunk
	for chunk := range chunkChan {
		chunks = append(chunks, chunk)
	}

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	var fullContent string
	for _, chunk := range chunks {
		fullContent += chunk.Content
	}

	if fullContent != "Hello" {
		t.Errorf("expected content 'Hello', got %s", fullContent)
	}
}

func TestOpenAI_Stream_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "bad request"}}`))
	}))
	defer server.Close()

	lm := createTestOpenAI(t, server)

	chunkChan, errChan := lm.Stream(context.Background(), []core.Message{{Role: "user", Content: "test"}}, core.DefaultGenerateOptions())

	for range chunkChan {
	}

	err := <-errChan
	if err == nil {
		t.Fatal("expected error for bad request")
	}
}

func TestOpenAI_InitRegistration(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-registration-key")

	lm, err := core.NewLM(context.Background(), "openai/gpt-4-registration")
	if err != nil {
		t.Fatalf("NewLM failed: %v", err)
	}
	if lm == nil {
		t.Fatal("NewLM returned nil for openai provider")
	}

	if lm.Name() != "gpt-4-registration" {
		t.Errorf("expected model name gpt-4-registration, got %s", lm.Name())
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
			name:    "very long ID",
			input:   string(make([]byte, 1220)),
			wantLen: maxToolCallIDLength,
		},
		{
			name:  "empty ID",
			input: "",
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
