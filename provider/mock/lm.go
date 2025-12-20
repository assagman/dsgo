package mock

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/internal/jsonutil"
	"github.com/assagman/dsgo/modelcatalog"
)

func init() {
	core.RegisterLM("mock", func(model string) core.LM {
		return newMockHTTP(model)
	})

	// Register mock provider models for testing
	mockModels := []modelcatalog.Model{
		{ID: "mock/gpt-4o"},
		{ID: "mock/gpt-4o-mini"},
		{ID: "mock/test-model"},
	}
	for _, m := range mockModels {
		_ = modelcatalog.RegisterModel(m)
	}
}

const (
	defaultChatCompletionsPath = "/chat/completions"
)

// mockHTTP is a lightweight OpenAI-compatible LM implementation intended for
// hermetic integration tests. It talks to a user-provided httptest.Server
// (configured via environment variables) instead of real provider APIs.
//
// Environment variables:
//   - DSGO_MOCK_BASE_URL: required (unless a custom HTTP transport is configured via SetHTTPTransport).
//     Example: http://127.0.0.1:12345
//   - DSGO_MOCK_API_KEY: optional. Default: "test"
//   - DSGO_HTTP_TIMEOUT_MS: optional. Default: 30000
type mockHTTP struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
	cache   core.Cache

	supportsJSON  bool
	supportsTools bool
	isOpenAI      bool
}

func newMockHTTP(model string) *mockHTTP {
	apiKey := os.Getenv("DSGO_MOCK_API_KEY")
	if apiKey == "" {
		apiKey = "test"
	}

	baseURL := os.Getenv("DSGO_MOCK_BASE_URL")

	timeout := 30 * time.Second
	if v := os.Getenv("DSGO_HTTP_TIMEOUT_MS"); v != "" {
		if d, err := time.ParseDuration(v + "ms"); err == nil && d > 0 {
			timeout = d
		}
	}

	rt := getHTTPTransportOverride()
	if baseURL == "" && rt != nil {
		// When a scripted transport is configured, we can use a dummy base URL.
		baseURL = "http://mock.local"
	}

	client := &http.Client{Timeout: timeout}
	if rt != nil {
		client.Transport = rt
	}

	return &mockHTTP{
		apiKey:        apiKey,
		model:         model,
		baseURL:       baseURL,
		client:        client,
		supportsJSON:  true,
		supportsTools: true,
		isOpenAI:      true,
	}
}

func (m *mockHTTP) Name() string {
	return m.model
}

func (m *mockHTTP) SupportsJSON() bool {
	return m.supportsJSON
}

func (m *mockHTTP) SupportsTools() bool {
	return m.supportsTools
}

func (m *mockHTTP) IsOpenAI() bool {
	return m.isOpenAI
}

// SetCache sets the cache instance for this LM.
func (m *mockHTTP) SetCache(cache core.Cache) {
	m.cache = cache
}

func (m *mockHTTP) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	if m.baseURL == "" {
		return nil, fmt.Errorf("mock provider base URL not configured: set DSGO_MOCK_BASE_URL")
	}

	if options == nil {
		options = core.DefaultGenerateOptions()
	}

	cacheModelName := "mock/" + m.model

	// Check cache if available.
	if m.cache != nil {
		cacheKey := core.GenerateCacheKey(cacheModelName, messages, options)
		if cached, ok := m.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	reqBody := m.buildRequest(messages, options)

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(m.baseURL, "/") + defaultChatCompletionsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	m.applyHeaders(req)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	var apiResp chatCompletionResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result, err := m.parseResponse(&apiResp)
	if err != nil {
		return nil, err
	}

	result.Metadata = map[string]any{
		"mock": true,
	}

	// Store in cache if available.
	if m.cache != nil {
		cacheKey := core.GenerateCacheKey(cacheModelName, messages, options)
		m.cache.Set(cacheKey, result)
	}

	return result, nil
}

func (m *mockHTTP) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		if m.baseURL == "" {
			errChan <- fmt.Errorf("mock provider base URL not configured: set DSGO_MOCK_BASE_URL")
			return
		}

		if options == nil {
			options = core.DefaultGenerateOptions()
		}

		reqBody := m.buildRequest(messages, options)
		reqBody["stream"] = true

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			errChan <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		url := strings.TrimRight(m.baseURL, "/") + defaultChatCompletionsPath
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			errChan <- fmt.Errorf("failed to build request: %w", err)
			return
		}
		m.applyHeaders(req)

		resp, err := m.client.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("request failed: %w", err)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var streamResp chatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				errChan <- fmt.Errorf("failed to parse stream chunk: %w", err)
				return
			}

			if len(streamResp.Choices) == 0 {
				continue
			}

			choice := streamResp.Choices[0]
			chunk := core.Chunk{
				Content:      choice.Delta.Content,
				FinishReason: choice.FinishReason,
			}
			if streamResp.Usage != nil {
				chunk.Usage = core.Usage{
					PromptTokens:     streamResp.Usage.PromptTokens,
					CompletionTokens: streamResp.Usage.CompletionTokens,
					TotalTokens:      streamResp.Usage.TotalTokens,
				}
			}

			// Forward chunk
			chunkChan <- chunk

			// Best-effort callback support for callers who configure it
			if options.StreamCallback != nil {
				options.StreamCallback(chunk)
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("stream reading error: %w", err)
			return
		}
	}()

	return chunkChan, errChan
}

func (m *mockHTTP) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
}

func (m *mockHTTP) buildRequest(messages []core.Message, options *core.GenerateOptions) map[string]any {
	req := map[string]any{
		"model":    m.model,
		"messages": m.convertMessages(messages),
	}

	if options.Temperature != 0 {
		req["temperature"] = options.Temperature
	}
	if options.MaxTokens > 0 {
		req["max_tokens"] = options.MaxTokens
	}
	if options.TopP != 0 && options.TopP != 1.0 {
		req["top_p"] = options.TopP
	}
	if len(options.Stop) > 0 {
		req["stop"] = options.Stop
	}

	// Response format
	if options.ResponseFormat == "json" {
		if options.ResponseSchema != nil {
			req["response_format"] = map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "response",
					"schema": options.ResponseSchema,
					"strict": true,
				},
			}
		} else {
			req["response_format"] = map[string]string{"type": "json_object"}
		}
	}

	// Tools
	if len(options.Tools) > 0 {
		tools := make([]map[string]any, 0, len(options.Tools))
		for _, tool := range options.Tools {
			tools = append(tools, convertTool(&tool))
		}
		req["tools"] = tools

		if options.ToolChoice != "" && options.ToolChoice != "auto" {
			switch options.ToolChoice {
			case "none", "required":
				req["tool_choice"] = options.ToolChoice
			default:
				req["tool_choice"] = map[string]any{
					"type": "function",
					"function": map[string]string{
						"name": options.ToolChoice,
					},
				}
			}
		}
	}

	if options.FrequencyPenalty != 0 {
		req["frequency_penalty"] = options.FrequencyPenalty
	}
	if options.PresencePenalty != 0 {
		req["presence_penalty"] = options.PresencePenalty
	}

	return req
}

func (m *mockHTTP) convertMessages(messages []core.Message) []map[string]any {
	converted := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := map[string]any{"role": msg.Role}

		if msg.Role == "tool" {
			m["content"] = msg.Content
			if msg.ToolID != "" {
				m["tool_call_id"] = msg.ToolID
			}
			converted = append(converted, m)
			continue
		}

		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			if msg.Content != "" {
				m["content"] = msg.Content
			}
			toolCalls := make([]map[string]any, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				argsBytes, _ := json.Marshal(tc.Arguments)
				toolCalls = append(toolCalls, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": string(argsBytes),
					},
				})
			}
			m["tool_calls"] = toolCalls
			converted = append(converted, m)
			continue
		}

		m["content"] = msg.Content
		converted = append(converted, m)
	}
	return converted
}

func convertTool(tool *core.Tool) map[string]any {
	properties := make(map[string]any)
	required := []string{}

	for _, param := range tool.Parameters {
		jsonType := param.Type
		switch param.Type {
		case "int":
			jsonType = "integer"
		case "float":
			jsonType = "number"
		case "bool":
			jsonType = "boolean"
		case "json":
			jsonType = "object"
		}

		prop := map[string]any{
			"type":        jsonType,
			"description": param.Description,
		}

		if jsonType == "array" || param.Type == "array" {
			elemType := "string"
			if param.ElementType != "" {
				elemType = param.ElementType
			}
			switch elemType {
			case "int":
				elemType = "integer"
			case "float":
				elemType = "number"
			case "bool":
				elemType = "boolean"
			case "json":
				elemType = "object"
			}
			prop["items"] = map[string]any{"type": elemType}
		}

		if len(param.Enum) > 0 {
			prop["enum"] = param.Enum
		}
		properties[param.Name] = prop

		if param.Required {
			required = append(required, param.Name)
		}
	}

	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters": map[string]any{
				"type":                 "object",
				"properties":           properties,
				"required":             required,
				"additionalProperties": false,
			},
		},
	}
}

func (m *mockHTTP) parseResponse(resp *chatCompletionResponse) (*core.GenerateResult, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	result := &core.GenerateResult{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: core.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]core.ToolCall, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			repairedArgs := jsonutil.RepairJSON(tc.Function.Arguments)
			var args map[string]any
			if err := json.Unmarshal([]byte(repairedArgs), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments (after repair): %w", err)
			}
			result.ToolCalls = append(result.ToolCalls, core.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

// Minimal OpenAI-compatible response structures

type chatCompletionResponse struct {
	Choices []struct {
		Index        int               `json:"index"`
		Message      chatCompletionMsg `json:"message"`
		FinishReason string            `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type chatCompletionMsg struct {
	Role      string                   `json:"role"`
	Content   string                   `json:"content"`
	ToolCalls []chatCompletionToolCall `json:"tool_calls,omitempty"`
}

type chatCompletionToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatCompletionStreamResponse struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content"`
			Role    string `json:"role,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}
