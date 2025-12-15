package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/assagman/dsgo/internal/core"
	openaiSDK "github.com/openai/openai-go/v3"
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

func TestSanitizeToolCallID(t *testing.T) {
	t.Parallel()

	t.Run("already valid", func(t *testing.T) {
		got := sanitizeToolCallID("call_123")
		if got != "call_123" {
			t.Errorf("expected call_123, got %q", got)
		}
	})

	t.Run("replaces invalid characters", func(t *testing.T) {
		got := sanitizeToolCallID("call 123!!")
		if got != "call_123__" {
			t.Errorf("expected call_123__ , got %q", got)
		}
	})

	t.Run("too long becomes deterministic", func(t *testing.T) {
		long := strings.Repeat("a", maxToolCallIDLength+10)
		got1 := sanitizeToolCallID(long)
		got2 := sanitizeToolCallID(long)
		if got1 != got2 {
			t.Errorf("expected deterministic output, got %q vs %q", got1, got2)
		}
		if len(got1) > maxToolCallIDLength {
			t.Errorf("expected length <= %d, got %d", maxToolCallIDLength, len(got1))
		}
	})
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
