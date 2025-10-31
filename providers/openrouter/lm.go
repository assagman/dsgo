package openrouter

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

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/internal/jsonutil"
	"github.com/assagman/dsgo/internal/retry"
	"github.com/assagman/dsgo/logging"
)

const (
	DefaultBaseURL = "https://openrouter.ai/api/v1"
)

// OpenRouter implements the LM interface for OpenRouter models
type OpenRouter struct {
	APIKey   string
	Model    string
	BaseURL  string
	Client   *http.Client
	SiteName string
	SiteURL  string
	Cache    dsgo.Cache
}

// NewOpenRouter creates a new OpenRouter LM
func NewOpenRouter(model string) *OpenRouter {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	return &OpenRouter{
		APIKey:   apiKey,
		Model:    model,
		BaseURL:  DefaultBaseURL,
		Client:   &http.Client{},
		SiteName: os.Getenv("OPENROUTER_SITE_NAME"),
		SiteURL:  os.Getenv("OPENROUTER_SITE_URL"),
	}
}

// Name returns the model name
func (o *OpenRouter) Name() string {
	return o.Model
}

// SupportsJSON indicates OpenRouter supports native JSON mode
func (o *OpenRouter) SupportsJSON() bool {
	return true
}

// SupportsTools indicates OpenRouter supports tool calling
func (o *OpenRouter) SupportsTools() bool {
	return true
}

// Generate generates a response from OpenRouter
func (o *OpenRouter) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	startTime := time.Now()

	// Calculate prompt length for logging
	promptLength := 0
	for _, msg := range messages {
		promptLength += len(msg.Content)
	}

	// Log API request start
	logging.LogAPIRequest(ctx, o.Model, promptLength)

	// Check cache if available
	if o.Cache != nil {
		cacheKey := dsgo.GenerateCacheKey(o.Model, messages, options)
		if cached, ok := o.Cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	reqBody := o.buildRequest(messages, options)

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retry.WithExponentialBackoff(ctx, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
		if o.SiteName != "" {
			req.Header.Set("X-Title", o.SiteName)
		}
		if o.SiteURL != "" {
			req.Header.Set("HTTP-Referer", o.SiteURL)
		}
		return o.Client.Do(req)
	})
	if err != nil {
		logging.LogAPIError(ctx, o.Model, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		logging.LogAPIError(ctx, o.Model, err)
		return nil, err
	}

	var apiResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		logging.LogAPIError(ctx, o.Model, err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result, err := o.parseResponse(&apiResp)
	if err != nil {
		logging.LogAPIError(ctx, o.Model, err)
		return nil, err
	}

	// Log API response
	duration := time.Since(startTime)
	logging.LogAPIResponse(ctx, o.Model, resp.StatusCode, duration, result.Usage)

	// Store in cache if available
	if o.Cache != nil {
		cacheKey := dsgo.GenerateCacheKey(o.Model, messages, options)
		o.Cache.Set(cacheKey, result)
	}

	return result, nil
}

func (o *OpenRouter) buildRequest(messages []dsgo.Message, options *dsgo.GenerateOptions) map[string]any {
	req := map[string]any{
		"model":    o.Model,
		"messages": o.convertMessages(messages),
	}

	if options.Temperature > 0 {
		req["temperature"] = options.Temperature
	}
	if options.MaxTokens > 0 {
		req["max_tokens"] = options.MaxTokens
	}
	if options.TopP > 0 && options.TopP != 1.0 {
		req["top_p"] = options.TopP
	}
	if len(options.Stop) > 0 {
		req["stop"] = options.Stop
	}
	if options.ResponseFormat == "json" {
		if options.ResponseSchema != nil {
			// Full structured output with JSON schema
			req["response_format"] = map[string]any{
				"type":   "json_schema",
				"schema": options.ResponseSchema,
			}
		} else {
			// Basic JSON mode without schema
			req["response_format"] = map[string]string{"type": "json_schema"}
		}
	}
	if options.FrequencyPenalty != 0 {
		req["frequency_penalty"] = options.FrequencyPenalty
	}
	if options.PresencePenalty != 0 {
		req["presence_penalty"] = options.PresencePenalty
	}

	// Add tools if supported
	if len(options.Tools) > 0 {
		tools := make([]map[string]any, 0, len(options.Tools))
		for _, tool := range options.Tools {
			tools = append(tools, o.convertTool(&tool))
		}
		req["tools"] = tools

		if options.ToolChoice != "" && options.ToolChoice != "auto" {
			if options.ToolChoice == "none" {
				req["tool_choice"] = "none"
			} else {
				req["tool_choice"] = map[string]any{
					"type": "function",
					"function": map[string]string{
						"name": options.ToolChoice,
					},
				}
			}
		}
	}

	return req
}

func (o *OpenRouter) convertMessages(messages []dsgo.Message) []map[string]any {
	converted := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := map[string]any{
			"role": msg.Role,
		}

		// Handle tool responses
		if msg.Role == "tool" {
			m["content"] = msg.Content
			if msg.ToolID != "" {
				m["tool_call_id"] = msg.ToolID
			}
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Assistant message with tool calls
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
		} else {
			// Regular message
			m["content"] = msg.Content
		}

		converted = append(converted, m)
	}
	return converted
}

func (o *OpenRouter) convertTool(tool *dsgo.Tool) map[string]any {
	properties := make(map[string]any)
	required := []string{}

	for _, param := range tool.Parameters {
		prop := map[string]any{
			"type":        param.Type,
			"description": param.Description,
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
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

func (o *OpenRouter) parseResponse(resp *openRouterResponse) (*dsgo.GenerateResult, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	result := &dsgo.GenerateResult{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: dsgo.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Parse tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]dsgo.ToolCall, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]any

			// Apply JSON repair to handle malformed tool arguments from models
			repairedArgs := jsonutil.RepairJSON(tc.Function.Arguments)

			if err := json.Unmarshal([]byte(repairedArgs), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments (after repair): %w", err)
			}
			result.ToolCalls = append(result.ToolCalls, dsgo.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

// Stream generates a streaming response from OpenRouter
func (o *OpenRouter) Stream(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (<-chan dsgo.Chunk, <-chan error) {
	chunkChan := make(chan dsgo.Chunk)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		reqBody := o.buildRequest(messages, options)
		reqBody["stream"] = true

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			errChan <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		resp, err := retry.WithExponentialBackoff(ctx, func() (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
			if err != nil {
				return nil, err
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+o.APIKey)
			if o.SiteName != "" {
				req.Header.Set("X-Title", o.SiteName)
			}
			if o.SiteURL != "" {
				req.Header.Set("HTTP-Referer", o.SiteURL)
			}

			return o.Client.Do(req)
		})
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

		// Read SSE stream
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// Parse SSE format: "data: {json}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				break
			}

			// Parse JSON chunk
			var streamResp openRouterStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				errChan <- fmt.Errorf("failed to parse stream chunk: %w", err)
				return
			}

			// Extract chunk data
			if len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]
				chunk := dsgo.Chunk{
					Content:      choice.Delta.Content,
					FinishReason: choice.FinishReason,
				}

				// Add usage if present (typically in last chunk)
				if streamResp.Usage != nil {
					chunk.Usage = dsgo.Usage{
						PromptTokens:     streamResp.Usage.PromptTokens,
						CompletionTokens: streamResp.Usage.CompletionTokens,
						TotalTokens:      streamResp.Usage.TotalTokens,
					}
				}

				chunkChan <- chunk
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("stream reading error: %w", err)
			return
		}
	}()

	return chunkChan, errChan
}

// OpenRouter API response structures
type openRouterResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int               `json:"index"`
		Message      openRouterMessage `json:"message"`
		FinishReason string            `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openRouterMessage struct {
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	ToolCalls []openRouterToolCall `json:"tool_calls,omitempty"`
}

type openRouterToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openRouterStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
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
