package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/assagman/dsgo"
)

const (
	DefaultBaseURL = "https://api.openai.com/v1"
)

// OpenAI implements the LM interface for OpenAI models
type OpenAI struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

// NewOpenAI creates a new OpenAI LM
func NewOpenAI(model string) *OpenAI {
	apiKey := os.Getenv("OPENAI_API_KEY")
	return &OpenAI{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: DefaultBaseURL,
		Client:  &http.Client{},
	}
}

// Name returns the model name
func (o *OpenAI) Name() string {
	return o.Model
}

// SupportsJSON indicates OpenAI supports native JSON mode
func (o *OpenAI) SupportsJSON() bool {
	return true
}

// SupportsTools indicates OpenAI supports tool calling
func (o *OpenAI) SupportsTools() bool {
	return true
}

// Generate generates a response from OpenAI
func (o *OpenAI) Generate(ctx context.Context, messages []dsgo.Message, options *dsgo.GenerateOptions) (*dsgo.GenerateResult, error) {
	reqBody := o.buildRequest(messages, options)

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return o.parseResponse(&apiResp)
}

func (o *OpenAI) buildRequest(messages []dsgo.Message, options *dsgo.GenerateOptions) map[string]interface{} {
	req := map[string]interface{}{
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
		req["response_format"] = map[string]string{"type": "json_object"}
	}
	if options.FrequencyPenalty != 0 {
		req["frequency_penalty"] = options.FrequencyPenalty
	}
	if options.PresencePenalty != 0 {
		req["presence_penalty"] = options.PresencePenalty
	}

	// Add tools if supported
	if len(options.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(options.Tools))
		for _, tool := range options.Tools {
			tools = append(tools, o.convertTool(&tool))
		}
		req["tools"] = tools

		if options.ToolChoice != "" && options.ToolChoice != "auto" {
			if options.ToolChoice == "none" {
				req["tool_choice"] = "none"
			} else {
				req["tool_choice"] = map[string]interface{}{
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

func (o *OpenAI) convertMessages(messages []dsgo.Message) []map[string]interface{} {
	converted := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		m := map[string]interface{}{
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
			toolCalls := make([]map[string]interface{}, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				argsBytes, _ := json.Marshal(tc.Arguments)
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
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

func (o *OpenAI) convertTool(tool *dsgo.Tool) map[string]interface{} {
	properties := make(map[string]interface{})
	required := []string{}

	for _, param := range tool.Parameters {
		prop := map[string]interface{}{
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

	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

func (o *OpenAI) parseResponse(resp *openAIResponse) (*dsgo.GenerateResult, error) {
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
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
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

// OpenAI API response structures
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}