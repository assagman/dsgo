package openai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/jsonutil"
	"github.com/assagman/dsgo/internal/logging"
	"github.com/assagman/dsgo/internal/providers/util"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// Tool call ID constraints:
// - OpenAI: max 40 characters, alphanumeric + underscore + hyphen
const maxToolCallIDLength = 40

var toolCallIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func init() {
	core.RegisterLM("openai", func(model string) core.LM {
		return newOpenAI(model)
	})
}

const (
	defaultBaseURL = "https://api.openai.com/v1"
)

// openAI implements the LM interface for OpenAI models using the official SDK
type openAI struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  openai.Client
	Cache   core.Cache
}

// newOpenAI creates a new OpenAI LM using the official SDK
func newOpenAI(model string) *openAI {
	apiKey := os.Getenv("OPENAI_API_KEY")

	timeout := 300 * time.Second
	if v := os.Getenv("DSGO_HTTP_TIMEOUT_MS"); v != "" {
		if d, err := time.ParseDuration(v + "ms"); err == nil && d > 0 {
			timeout = d
		}
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(defaultBaseURL),
		option.WithRequestTimeout(timeout),
	)

	return &openAI{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: defaultBaseURL,
		Client:  client,
	}
}

// Match exact model patterns: o1, o1-*, o3, o3-*, o4, o4-*, gpt-5, gpt-5-*
var reasoningModelRegex = regexp.MustCompile(`^(o1|o3|o4|gpt-5)(-|$)`)

// isReasoningModel checks if the model requires max_completion_tokens instead of max_tokens
func (o *openAI) isReasoningModel() bool {
	return reasoningModelRegex.MatchString(strings.ToLower(o.Model))
}

// Name returns the model name
func (o *openAI) Name() string {
	return o.Model
}

// SupportsJSON indicates OpenAI supports native JSON mode
func (o *openAI) SupportsJSON() bool {
	return true
}

// SupportsTools indicates OpenAI supports tool calling
func (o *openAI) SupportsTools() bool {
	return true
}

// IsOpenAI indicates this is an OpenAI provider
func (o *openAI) IsOpenAI() bool {
	return true
}

// SetCache sets the cache instance for this LM
func (o *openAI) SetCache(cache core.Cache) {
	o.Cache = cache
}

// Generate generates a response from OpenAI using the official SDK
func (o *openAI) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	startTime := time.Now()

	promptLength := 0
	for _, msg := range messages {
		promptLength += len(msg.Content)
	}

	logging.LogAPIRequest(ctx, "provider.OpenAI", o.Model, promptLength)

	if options == nil {
		options = core.DefaultGenerateOptions()
	}

	cacheModelName := "openai/" + o.Model

	if o.Cache != nil {
		cacheKey := core.GenerateCacheKey(cacheModelName, messages, options)
		if cached, ok := o.Cache.Get(cacheKey); ok {
			// Mark as cache hit and clear usage (no API call was made)
			return core.MarkCacheHit(cached), nil
		}
	}

	params := o.buildParams(messages, options)

	var rawResp *http.Response
	reqOpts := []option.RequestOption{option.WithResponseInto(&rawResp)}
	if o.BaseURL != "" {
		reqOpts = append(reqOpts, option.WithBaseURL(o.BaseURL))
	}
	if options.RetryConfig != nil {
		reqOpts = append(reqOpts, option.WithMaxRetries(options.RetryConfig.MaxRetries))
	}

	chatCompletion, err := o.Client.Chat.Completions.New(ctx, params, reqOpts...)
	if err != nil {
		logging.LogAPIError(ctx, "provider.OpenAI", o.Model, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	result, err := o.parseResponse(chatCompletion)
	if err != nil {
		logging.LogAPIError(ctx, "provider.OpenAI", o.Model, err)
		return nil, err
	}

	if rawResp != nil {
		if metadata := o.extractMetadata(rawResp.Header); len(metadata) > 0 {
			result.Metadata = metadata
		}
	}

	duration := time.Since(startTime)
	logging.LogAPIResponse(ctx, "provider.OpenAI", o.Model, 200, duration, logging.Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
		Cost:             result.Usage.Cost,
		Latency:          result.Usage.Latency,
	})

	if o.Cache != nil {
		cacheKey := core.GenerateCacheKey(cacheModelName, messages, options)
		o.Cache.Set(cacheKey, result)
	}

	return result, nil
}

func (o *openAI) buildParams(messages []core.Message, options *core.GenerateOptions) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:    o.Model,
		Messages: o.convertMessages(messages),
	}

	if options == nil {
		return params
	}

	if options.Temperature > 0 {
		params.Temperature = openai.Float(options.Temperature)
	}

	if options.MaxTokens > 0 {
		if o.isReasoningModel() {
			params.MaxCompletionTokens = openai.Int(int64(options.MaxTokens))
		} else {
			params.MaxTokens = openai.Int(int64(options.MaxTokens))
		}
	}

	if options.TopP > 0 && options.TopP != 1.0 {
		params.TopP = openai.Float(options.TopP)
	}

	if len(options.Stop) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: options.Stop,
		}
	}

	if options.ResponseFormat == "json" {
		if options.ResponseSchema != nil {
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
					JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
						Name:   "response",
						Schema: options.ResponseSchema,
						Strict: openai.Bool(true),
					},
				},
			}
		} else {
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
			}
		}
	}

	if options.FrequencyPenalty != 0 {
		params.FrequencyPenalty = openai.Float(options.FrequencyPenalty)
	}

	if options.PresencePenalty != 0 {
		params.PresencePenalty = openai.Float(options.PresencePenalty)
	}

	if len(options.Tools) > 0 {
		tools := make([]openai.ChatCompletionToolUnionParam, 0, len(options.Tools))
		for _, tool := range options.Tools {
			tools = append(tools, o.convertTool(&tool))
		}
		params.Tools = tools

		if options.ToolChoice != "" && options.ToolChoice != "auto" {
			if options.ToolChoice == "none" {
				params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
					OfAuto: openai.String("none"),
				}
			} else {
				params.ToolChoice = openai.ToolChoiceOptionFunctionToolChoice(
					openai.ChatCompletionNamedToolChoiceFunctionParam{
						Name: options.ToolChoice,
					},
				)
			}
		}
	}

	util.ApplyChatCompletionProviderParams(&params, options.ProviderParams)

	return params
}

func (o *openAI) convertMessages(messages []core.Message) []openai.ChatCompletionMessageParamUnion {
	converted := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for msgIndex, msg := range messages {
		switch msg.Role {
		case "system":
			converted = append(converted, openai.SystemMessage(msg.Content))
		case "user":
			converted = append(converted, openai.UserMessage(msg.Content))
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.ToolCalls))
				for tcIndex, tc := range msg.ToolCalls {
					argsBytes, _ := json.Marshal(tc.Arguments)
					// Sanitize tool call ID to meet provider constraints.
					// If the provider returns an empty/whitespace ID, we deterministically derive
					// a collision-resistant ID from call index + content.
					fallback := fmt.Sprintf("dsgo_toolcall_%d_%d_%s_%s", msgIndex, tcIndex, tc.Name, string(argsBytes))
					sanitizedID := sanitizeToolCallIDWithFallback(tc.ID, fallback)
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID:   sanitizedID,
							Type: "function",
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: string(argsBytes),
							},
						},
					})
				}
				assistantMsg := &openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCalls,
				}
				// Only set content if non-empty - some providers reject empty text fields
				if msg.Content != "" {
					assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(msg.Content)}
				}
				converted = append(converted, openai.ChatCompletionMessageParamUnion{
					OfAssistant: assistantMsg,
				})
			} else {
				converted = append(converted, openai.AssistantMessage(msg.Content))
			}
		case "tool":
			// Sanitize tool call ID to meet provider constraints
			fallback := fmt.Sprintf("dsgo_tool_message_%d_%s", msgIndex, msg.Content)
			sanitizedToolID := sanitizeToolCallIDWithFallback(msg.ToolID, fallback)
			// Note: ToolMessage signature is (content, toolCallID) - not (toolCallID, content)
			converted = append(converted, openai.ToolMessage(msg.Content, sanitizedToolID))
		default:
			converted = append(converted, openai.UserMessage(msg.Content))
		}
	}

	return converted
}

func (o *openAI) convertTool(tool *core.Tool) openai.ChatCompletionToolUnionParam {
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

	return openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        tool.Name,
		Description: openai.String(tool.Description),
		Parameters: shared.FunctionParameters{
			"type":       "object",
			"properties": properties,
			"required":   required,
		},
	})
}

func (o *openAI) parseResponse(resp *openai.ChatCompletion) (*core.GenerateResult, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	result := &core.GenerateResult{
		Content:      choice.Message.Content,
		FinishReason: string(choice.FinishReason),
		Usage: core.Usage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}

	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]core.ToolCall, 0, len(choice.Message.ToolCalls))
		for tcIndex, tc := range choice.Message.ToolCalls {
			var args map[string]any
			repairedArgs := jsonutil.RepairJSON(tc.Function.Arguments)

			if err := json.Unmarshal([]byte(repairedArgs), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments (after repair): %w", err)
			}
			// Sanitize tool call ID to meet provider constraints.
			// If the provider returns an empty/whitespace ID, derive a stable ID based on
			// tool index + name + arguments.
			fallback := fmt.Sprintf("dsgo_response_toolcall_%d_%s_%s", tcIndex, tc.Function.Name, tc.Function.Arguments)
			sanitizedID := sanitizeToolCallIDWithFallback(tc.ID, fallback)
			result.ToolCalls = append(result.ToolCalls, core.ToolCall{
				ID:        sanitizedID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

func (o *openAI) extractMetadata(headers http.Header) map[string]any {
	metadata := make(map[string]any)

	if v := headers.Get("X-RateLimit-Limit-Requests"); v != "" {
		metadata["rate_limit_requests"] = v
	}
	if v := headers.Get("X-RateLimit-Remaining-Requests"); v != "" {
		metadata["rate_limit_remaining_requests"] = v
	}
	if v := headers.Get("X-RateLimit-Limit-Tokens"); v != "" {
		metadata["rate_limit_tokens"] = v
	}
	if v := headers.Get("X-RateLimit-Remaining-Tokens"); v != "" {
		metadata["rate_limit_remaining_tokens"] = v
	}
	if v := headers.Get("X-Request-ID"); v != "" {
		metadata["request_id"] = v
	}
	if v := headers.Get("Openai-Organization"); v != "" {
		metadata["organization"] = v
	}

	if cacheStatus := headers.Get("CF-Cache-Status"); cacheStatus != "" {
		metadata["cache_status"] = cacheStatus
		metadata["cache_hit"] = cacheStatus == "HIT"
	}
	if cache := headers.Get("X-Cache"); cache != "" {
		metadata["x_cache"] = cache
	}

	return metadata
}

// sanitizeToolCallID ensures a tool call ID meets provider constraints.
// It handles IDs that are too long or contain invalid characters by creating
// a deterministic hash-based ID that preserves uniqueness.
func sanitizeToolCallID(id string) string {
	return sanitizeToolCallIDWithFallback(id, "")
}

func sanitizeToolCallIDWithFallback(id, fallback string) string {
	if strings.TrimSpace(id) == "" {
		seed := strings.TrimSpace(fallback)
		if seed == "" {
			seed = "dsgo_empty_tool_call_id"
		}
		hash := sha256.Sum256([]byte(seed))
		return "toolcall_" + hex.EncodeToString(hash[:])[:16]
	}

	// Check if ID is already valid
	if len(id) <= maxToolCallIDLength && !toolCallIDPattern.MatchString(id) {
		return id
	}

	// Replace invalid characters first
	sanitized := toolCallIDPattern.ReplaceAllString(id, "_")

	// If still too long, create a hash-based ID
	if len(sanitized) > maxToolCallIDLength {
		// Use SHA-256 to create a deterministic short ID
		// Keep a prefix for readability, append hash for uniqueness
		hash := sha256.Sum256([]byte(id))
		hashStr := hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash

		// Calculate max prefix length: maxLength - hash length - separator
		maxPrefixLen := maxToolCallIDLength - len(hashStr) - 1
		if maxPrefixLen < 0 {
			maxPrefixLen = 0
		}

		prefix := sanitized
		if len(prefix) > maxPrefixLen {
			prefix = prefix[:maxPrefixLen]
		}

		if prefix == "" {
			sanitized = hashStr
		} else {
			sanitized = prefix + "_" + hashStr
		}
	}

	return sanitized
}

// Stream generates a streaming response from OpenAI using the official SDK
func (o *openAI) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunkChan := make(chan core.Chunk)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		if options == nil {
			options = core.DefaultGenerateOptions()
		}

		params := o.buildParams(messages, options)

		var rawResp *http.Response
		reqOpts := []option.RequestOption{option.WithResponseInto(&rawResp)}
		if o.BaseURL != "" {
			reqOpts = append(reqOpts, option.WithBaseURL(o.BaseURL))
		}
		if options.RetryConfig != nil {
			reqOpts = append(reqOpts, option.WithMaxRetries(options.RetryConfig.MaxRetries))
		}

		stream := o.Client.Chat.Completions.NewStreaming(ctx, params, reqOpts...)

		for stream.Next() {
			chunk := stream.Current()

			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				coreChunk := core.Chunk{
					Content:      choice.Delta.Content,
					FinishReason: string(choice.FinishReason),
				}

				if chunk.Usage.TotalTokens > 0 {
					coreChunk.Usage = core.Usage{
						PromptTokens:     int(chunk.Usage.PromptTokens),
						CompletionTokens: int(chunk.Usage.CompletionTokens),
						TotalTokens:      int(chunk.Usage.TotalTokens),
					}
				}

				chunkChan <- coreChunk
			}
		}

		if err := stream.Err(); err != nil {
			errChan <- fmt.Errorf("stream error: %w", err)
			return
		}
	}()

	return chunkChan, errChan
}
