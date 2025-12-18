package openrouter

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

// Tool call ID constraints across providers:
// - OpenAI: max 40 characters
// - Azure: max 64 characters
// - Anthropic: pattern ^[a-zA-Z0-9_-]+$
// We use the most restrictive: max 40 chars, alphanumeric + underscore + hyphen only
const maxToolCallIDLength = 40

var toolCallIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func init() {
	core.RegisterLM("openrouter", func(model string) core.LM {
		return newOpenRouter(model)
	})
}

const (
	defaultBaseURL = "https://openrouter.ai/api/v1"
)

// openRouter implements the LM interface for OpenRouter models using the official OpenAI SDK
type openRouter struct {
	APIKey   string
	Model    string
	BaseURL  string
	Client   openai.Client
	SiteName string
	SiteURL  string
	Cache    core.Cache
}

// newOpenRouter creates a new OpenRouter LM using the official OpenAI SDK
func newOpenRouter(model string) *openRouter {
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	timeout := 300 * time.Second
	if v := os.Getenv("DSGO_HTTP_TIMEOUT_MS"); v != "" {
		if d, err := time.ParseDuration(v + "ms"); err == nil && d > 0 {
			timeout = d
		}
	}

	siteName := os.Getenv("OPENROUTER_SITE_NAME")
	siteURL := os.Getenv("OPENROUTER_SITE_URL")

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(defaultBaseURL),
		option.WithRequestTimeout(timeout),
	}

	if siteName != "" {
		opts = append(opts, option.WithHeader("X-Title", siteName))
	}
	if siteURL != "" {
		opts = append(opts, option.WithHeader("HTTP-Referer", siteURL))
	}

	client := openai.NewClient(opts...)

	return &openRouter{
		APIKey:   apiKey,
		Model:    model,
		BaseURL:  defaultBaseURL,
		Client:   client,
		SiteName: siteName,
		SiteURL:  siteURL,
	}
}

// Name returns the model name
func (o *openRouter) Name() string {
	return o.Model
}

// SupportsJSON indicates OpenRouter supports native JSON mode
func (o *openRouter) SupportsJSON() bool {
	return true
}

// SupportsTools indicates OpenRouter supports tool calling
func (o *openRouter) SupportsTools() bool {
	return true
}

// IsOpenAI indicates this is not an OpenAI provider (OpenRouter is compatible but not strict)
func (o *openRouter) IsOpenAI() bool {
	return false
}

// SetCache sets the cache instance for this LM
func (o *openRouter) SetCache(cache core.Cache) {
	o.Cache = cache
}

// Generate generates a response from OpenRouter using the official OpenAI SDK
func (o *openRouter) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	startTime := time.Now()

	promptLength := 0
	for _, msg := range messages {
		promptLength += len(msg.Content)
	}

	logging.LogAPIRequest(ctx, "provider.OpenRouter", o.Model, promptLength)

	if options == nil {
		options = core.DefaultGenerateOptions()
	}

	cacheModelName := "openrouter/" + o.Model

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
		logging.LogAPIError(ctx, "provider.OpenRouter", o.Model, err)

		// Check for JSON mode unsupported - fallback gracefully
		errStr := err.Error()
		if options != nil && options.ResponseFormat == "json" {
			if strings.Contains(errStr, "json_schema") || strings.Contains(errStr, "response_format") {
				if options.ResponseSchema != nil {
					fmt.Fprintf(os.Stderr, "Warning: model %s doesn't support json_schema, falling back to json_object\n", o.Model)
					fallbackOpts := *options
					fallbackOpts.ResponseSchema = nil
					return o.Generate(ctx, messages, &fallbackOpts)
				}
				if strings.Contains(errStr, "json_object") || strings.Contains(errStr, "response format") {
					fmt.Fprintf(os.Stderr, "Warning: model %s doesn't support JSON mode, using adapter-based parsing\n", o.Model)
					fallbackOpts := *options
					fallbackOpts.ResponseFormat = ""
					fallbackOpts.ResponseSchema = nil
					return o.Generate(ctx, messages, &fallbackOpts)
				}
			}
		}

		return nil, fmt.Errorf("request failed: %w", err)
	}

	result, err := o.parseResponse(chatCompletion)
	if err != nil {
		logging.LogAPIError(ctx, "provider.OpenRouter", o.Model, err)
		return nil, err
	}

	if rawResp != nil {
		if metadata := o.extractMetadata(rawResp.Header); len(metadata) > 0 {
			result.Metadata = metadata
		}
	}

	duration := time.Since(startTime)
	logging.LogAPIResponse(ctx, "provider.OpenRouter", o.Model, 200, duration, logging.Usage{
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

func (o *openRouter) buildParams(messages []core.Message, options *core.GenerateOptions) openai.ChatCompletionNewParams {
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
		params.MaxTokens = openai.Int(int64(options.MaxTokens))
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

	// Detect provider-specific models that don't support tool_choice: "none"
	isZAIModel := strings.Contains(o.Model, "z-ai/")

	if len(options.Tools) > 0 {
		sanitizedTools := sanitizeToolsForOpenRouter(options.Tools)
		tools := make([]openai.ChatCompletionToolUnionParam, 0, len(sanitizedTools))
		for _, tool := range sanitizedTools {
			tools = append(tools, o.convertTool(&tool))
		}
		params.Tools = tools

		if options.ToolChoice != "" && options.ToolChoice != "auto" {
			switch options.ToolChoice {
			case "none":
				if !isZAIModel {
					params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
						OfAuto: openai.String("none"),
					}
				}
			case "required":
				params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
					OfAuto: openai.String("required"),
				}
			default:
				params.ToolChoice = openai.ToolChoiceOptionFunctionToolChoice(
					openai.ChatCompletionNamedToolChoiceFunctionParam{
						Name: options.ToolChoice,
					},
				)
			}
		}
	}

	// Handle Z.AI specific requirements
	hasToolContent := false
	if isZAIModel {
		for _, msg := range messages {
			if msg.Role == "tool" || (msg.Role == "assistant" && len(msg.ToolCalls) > 0) {
				hasToolContent = true
				break
			}
		}
	}

	if isZAIModel && (len(options.Tools) > 0 || hasToolContent) {
		// Z.AI models have strict tool_choice handling and may reject "none".
		// We only override when the caller didn't explicitly ask for a non-auto policy.
		if options.ToolChoice == "" || options.ToolChoice == "auto" || options.ToolChoice == "none" {
			params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: openai.String("auto"),
			}
		}
	}

	util.ApplyChatCompletionProviderParams(&params, options.ProviderParams)

	return params
}

func (o *openRouter) convertMessages(messages []core.Message) []openai.ChatCompletionMessageParamUnion {
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
					argsHash := sha256.Sum256(argsBytes)
					argsHashStr := hex.EncodeToString(argsHash[:])[:16]
					// Sanitize tool call ID to meet provider constraints.
					// If the provider returns an empty/whitespace ID, we deterministically derive
					// a collision-resistant ID from call index + a short hash of arguments.
					fallback := fmt.Sprintf("dsgo_toolcall_%d_%d_%s_%s", msgIndex, tcIndex, tc.Name, argsHashStr)
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
				// Only set content if non-empty to avoid empty text field issues
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
			// Sanitize tool call ID to meet provider constraints.
			contentHash := sha256.Sum256([]byte(msg.Content))
			contentHashStr := hex.EncodeToString(contentHash[:])[:16]
			fallback := fmt.Sprintf("dsgo_tool_message_%d_%s", msgIndex, contentHashStr)
			sanitizedToolID := sanitizeToolCallIDWithFallback(msg.ToolID, fallback)
			// Note: ToolMessage signature is (content, toolCallID) - not (toolCallID, content)
			converted = append(converted, openai.ToolMessage(msg.Content, sanitizedToolID))
		default:
			converted = append(converted, openai.UserMessage(msg.Content))
		}
	}

	return converted
}

// mapParamTypeToJSONType maps internal parameter types to JSON Schema types
func mapParamTypeToJSONType(t string) string {
	switch strings.ToLower(t) {
	case "string":
		return "string"
	case "int", "integer":
		return "integer"
	case "float", "number", "double":
		return "number"
	case "bool", "boolean":
		return "boolean"
	case "json", "object":
		return "object"
	case "array", "list":
		return "array"
	default:
		return "string"
	}
}

// sanitizeToolsForOpenRouter ensures all tool parameter types are valid before sending
func sanitizeToolsForOpenRouter(tools []core.Tool) []core.Tool {
	sanitized := make([]core.Tool, len(tools))
	for i, t := range tools {
		sanitizedParams := make([]core.ToolParameter, len(t.Parameters))
		for j, p := range t.Parameters {
			paramType := strings.TrimSpace(p.Type)
			if paramType == "" {
				paramType = "string"
			}
			normalizedType := mapParamTypeToJSONType(paramType)

			sanitizedParam := p
			sanitizedParam.Type = normalizedType
			sanitizedParams[j] = sanitizedParam
		}

		sanitizedTool := t
		sanitizedTool.Parameters = sanitizedParams
		sanitized[i] = sanitizedTool
	}
	return sanitized
}

func (o *openRouter) convertTool(tool *core.Tool) openai.ChatCompletionToolUnionParam {
	properties := make(map[string]any)
	required := []string{}

	for _, param := range tool.Parameters {
		jsonType := mapParamTypeToJSONType(param.Type)

		prop := map[string]any{
			"type":        jsonType,
			"description": param.Description,
		}

		if jsonType == "array" && param.ElementType != "" {
			itemType := mapParamTypeToJSONType(param.ElementType)
			prop["items"] = map[string]any{"type": itemType}
		}

		if len(param.Enum) > 0 && jsonType == "string" {
			prop["enum"] = param.Enum
		}

		properties[param.Name] = prop

		if param.Required {
			required = append(required, param.Name)
		}
	}

	required = core.DedupeStringsPreserveOrder(required)

	return openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        tool.Name,
		Description: openai.String(tool.Description),
		Parameters: shared.FunctionParameters{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
	})
}

func (o *openRouter) parseResponse(resp *openai.ChatCompletion) (*core.GenerateResult, error) {
	if len(resp.Choices) == 0 {
		if debugEnv := os.Getenv("DSGO_DEBUG_PARSE"); debugEnv == "1" || debugEnv == "true" {
			fmt.Fprintf(os.Stderr, "\n=== NO CHOICES ERROR DEBUG ===\n")
			fmt.Fprintf(os.Stderr, "Model: %s\n", o.Model)
			fmt.Fprintf(os.Stderr, "Response ID: %s\n", resp.ID)
			fmt.Fprintf(os.Stderr, "==============================\n\n")
		}
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
			args, err := parseToolArguments(o.Model, resp.ID, string(choice.FinishReason), tc)
			if err != nil {
				return nil, err
			}
			// Sanitize tool call ID to meet provider constraints.
			// If the provider returns an empty/whitespace ID, derive a stable ID based on
			// tool index + name + a short hash of arguments.
			argHash := sha256.Sum256([]byte(tc.Function.Arguments))
			argHashStr := hex.EncodeToString(argHash[:])[:16]
			fallback := fmt.Sprintf("dsgo_response_toolcall_%d_%s_%s", tcIndex, tc.Function.Name, argHashStr)
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

// extractMetadata extracts provider-specific metadata from HTTP response headers
func (o *openRouter) extractMetadata(headers http.Header) map[string]any {
	metadata := make(map[string]any)

	// Cache detection (OpenRouter uses Cloudflare)
	if cacheStatus := headers.Get("CF-Cache-Status"); cacheStatus != "" {
		metadata["cache_status"] = cacheStatus
		metadata["cache_hit"] = (cacheStatus == "HIT")
	}
	if cache := headers.Get("X-Cache"); cache != "" {
		metadata["x_cache"] = cache
	}

	// Rate limit headers (OpenRouter specific)
	if rateLimit := headers.Get("X-RateLimit-Limit"); rateLimit != "" {
		metadata["rate_limit_limit"] = rateLimit
	}
	if rateRemaining := headers.Get("X-RateLimit-Remaining"); rateRemaining != "" {
		metadata["rate_limit_remaining"] = rateRemaining
	}
	if rateReset := headers.Get("X-RateLimit-Reset"); rateReset != "" {
		metadata["rate_limit_reset"] = rateReset
	}

	// OpenRouter-specific headers
	if genID := headers.Get("X-OpenRouter-Generation-ID"); genID != "" {
		metadata["generation_id"] = genID
	}

	return metadata
}

// parseToolArguments attempts to parse tool call arguments with multiple fallback strategies
func parseToolArguments(model, responseID, finishReason string, tc openai.ChatCompletionMessageToolCallUnion) (map[string]any, error) {
	raw := tc.Function.Arguments

	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	repaired := jsonutil.RepairJSON(raw)
	var args map[string]any

	if err := json.Unmarshal([]byte(repaired), &args); err == nil {
		return args, nil
	}

	var inner string
	if json.Unmarshal([]byte(repaired), &inner) == nil && strings.Contains(inner, "{") {
		innerRepaired := jsonutil.RepairJSON(inner)
		if err := json.Unmarshal([]byte(innerRepaired), &args); err == nil {
			return args, nil
		}
	}

	if jsonStr, err := jsonutil.ExtractJSON(repaired); err == nil {
		if err := json.Unmarshal([]byte(jsonStr), &args); err == nil {
			return args, nil
		}
	}

	if strings.Contains(repaired, "{") || strings.Contains(repaired, "[") {
		balanced := balanceDelimiters(repaired)
		if err := json.Unmarshal([]byte(balanced), &args); err == nil {
			return args, nil
		}
	}

	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "\n=== TOOL ARGS PARSE ERROR DEBUG ===\n")
		fmt.Fprintf(os.Stderr, "Model: %s\nResponse ID: %s\nFinish Reason: %s\n", model, responseID, finishReason)
		fmt.Fprintf(os.Stderr, "Tool Call ID: %s  Name: %s\n", tc.ID, tc.Function.Name)
		fmt.Fprintf(os.Stderr, "Raw args length: %d\n", len(raw))
		fmt.Fprintf(os.Stderr, "===================================\n\n")
	}

	return nil, fmt.Errorf("failed to parse tool arguments (model=%s, tool=%s, finish_reason=%s): all parsing attempts failed",
		model, tc.Function.Name, finishReason)
}

func balanceDelimiters(s string) string {
	openCurly := strings.Count(s, "{")
	closeCurly := strings.Count(s, "}")
	if closeCurly < openCurly {
		s += strings.Repeat("}", openCurly-closeCurly)
	}

	openSquare := strings.Count(s, "[")
	closeSquare := strings.Count(s, "]")
	if closeSquare < openSquare {
		s += strings.Repeat("]", openSquare-closeSquare)
	}

	return s
}

func debugEnabled() bool {
	d := os.Getenv("DSGO_DEBUG_PARSE")
	return d == "1" || strings.ToLower(d) == "true"
}

// sanitizeToolCallID ensures a tool call ID meets all provider constraints.
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

// Stream generates a streaming response from OpenRouter using the official OpenAI SDK
func (o *openRouter) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
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
		defer func() { _ = stream.Close() }()

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
