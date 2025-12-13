package fixtures

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// OpenAIChatCompletionJSON returns an OpenAI-compatible /chat/completions response.
//
// This is intended for the internal/providers/mock provider, which expects a
// minimal subset of the OpenAI schema.
func OpenAIChatCompletionJSON(content string, finishReason string, promptTokens, completionTokens int) string {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type choice struct {
		Index        int     `json:"index"`
		Message      message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}
	type usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	type response struct {
		Choices []choice `json:"choices"`
		Usage   usage    `json:"usage"`
	}

	if finishReason == "" {
		finishReason = "stop"
	}

	payload := response{
		Choices: []choice{{
			Index: 0,
			Message: message{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: finishReason,
		}},
		Usage: usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		// Fixtures must never fail; fallback to a minimal string.
		return fmt.Sprintf(`{"choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":%q}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`, content, finishReason, promptTokens, completionTokens, promptTokens+completionTokens)
	}
	return string(b)
}

// OpenAIToolCall describes a single OpenAI tool call fixture.
//
// Arguments is raw JSON (string) so we can easily generate malformed JSON for
// json repair tests.
type OpenAIToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// OpenAIChatCompletionWithToolCallsJSON returns an OpenAI-compatible response
// whose first choice contains tool calls.
func OpenAIChatCompletionWithToolCallsJSON(toolCalls []OpenAIToolCall, finishReason string, promptTokens, completionTokens int) string {
	type toolFunction struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	type toolCall struct {
		ID       string       `json:"id"`
		Type     string       `json:"type"`
		Function toolFunction `json:"function"`
	}
	type message struct {
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []toolCall `json:"tool_calls,omitempty"`
	}
	type choice struct {
		Index        int     `json:"index"`
		Message      message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}
	type usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	type response struct {
		Choices []choice `json:"choices"`
		Usage   usage    `json:"usage"`
	}

	if finishReason == "" {
		finishReason = "tool_calls"
	}

	outCalls := make([]toolCall, 0, len(toolCalls))
	for i, tc := range toolCalls {
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i+1)
		}
		outCalls = append(outCalls, toolCall{
			ID:   id,
			Type: "function",
			Function: toolFunction{
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})
	}

	payload := response{
		Choices: []choice{{
			Index: 0,
			Message: message{
				Role:      "assistant",
				Content:   "",
				ToolCalls: outCalls,
			},
			FinishReason: finishReason,
		}},
		Usage: usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// OpenAIStreamHeaders returns the default headers for an SSE streaming response.
func OpenAIStreamHeaders() http.Header {
	h := make(http.Header)
	h.Set("Content-Type", "text/event-stream")
	return h
}

// OpenAIChatCompletionSSE returns an OpenAI-style Server-Sent Events body.
//
// Each element of contentChunks becomes a streamed delta.content value.
// The stream is terminated with a final chunk containing usage and finish_reason,
// then a [DONE] marker.
func OpenAIChatCompletionSSE(contentChunks []string, finishReason string, promptTokens, completionTokens int) string {
	type delta struct {
		Content string `json:"content"`
		Role    string `json:"role,omitempty"`
	}
	type choice struct {
		Index        int    `json:"index"`
		Delta        delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	}
	type usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	type chunk struct {
		Choices []choice `json:"choices"`
		Usage   *usage   `json:"usage,omitempty"`
	}

	if finishReason == "" {
		finishReason = "stop"
	}

	var out string
	for _, c := range contentChunks {
		payload := chunk{Choices: []choice{{Index: 0, Delta: delta{Content: c}, FinishReason: ""}}}
		b, _ := json.Marshal(payload)
		out += "data: " + string(b) + "\n\n"
	}

	final := chunk{
		Choices: []choice{{Index: 0, Delta: delta{Content: ""}, FinishReason: finishReason}},
		Usage: &usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
	b, _ := json.Marshal(final)
	out += "data: " + string(b) + "\n\n"
	out += "data: [DONE]\n\n"

	return out
}

// OpenAIErrorBodyJSON returns an OpenAI-ish error response body that is useful
// when testing non-200 HTTP paths.
func OpenAIErrorBodyJSON(message string) string {
	if message == "" {
		message = "test error"
	}
	return fmt.Sprintf(`{"error":{"message":%q,"type":"test_error","code":"test_error"}}`, message)
}
