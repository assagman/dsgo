package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"testing"

	"github.com/assagman/dsgo/core"
)

// MockTransport is a mock implementation of Transport.
type MockTransport struct {
	SendFunc func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error)
}

func (m *MockTransport) Send(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
	return m.SendFunc(ctx, request)
}

func (m *MockTransport) Close() error {
	return nil
}

func TestClient_Initialize(t *testing.T) {
	mockTransport := &MockTransport{
		SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
			if request.Method == "initialize" {
				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      request.ID,
					Result:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
				}, nil
			}
			if request.Method == "tools/list" {
				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      request.ID,
					Result: json.RawMessage(`{
						"tools": [
							{
								"name": "test_tool",
								"description": "A test tool",
								"inputSchema": {
									"type": "object",
									"properties": {
										"arg1": {"type": "string", "description": "Argument 1"}
									},
									"required": ["arg1"]
								}
							}
						]
					}`),
				}, nil
			}
			return nil, nil
		},
	}

	client, err := NewClient(ClientConfig{Transport: mockTransport})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	tools := client.GetTools()
	_ = tools // Ensure core.Tool is used

	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", tools[0].Name)
	}
}

func TestClient_CallTool(t *testing.T) {
	mockTransport := &MockTransport{
		SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
			if request.Method == "tools/call" {
				// Verify params
				var params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				}
				if err := json.Unmarshal(request.Params, &params); err != nil {
					t.Errorf("Failed to unmarshal params: %v", err)
				}
				if params.Name != "test_tool" {
					t.Errorf("Expected tool name 'test_tool', got '%s'", params.Name)
				}
				if params.Arguments["arg1"] != "value1" {
					t.Errorf("Expected arg1 'value1', got '%v'", params.Arguments["arg1"])
				}

				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      request.ID,
					Result: json.RawMessage(`{
						"content": [
							{"type": "text", "text": "Tool output"}
						]
					}`),
				}, nil
			}
			return nil, nil
		},
	}

	client, err := NewClient(ClientConfig{Transport: mockTransport})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// We can manually populate schemas/tools to skip Initialize in this test
	client.SetSchemas([]MCPToolSchema{
		{
			Name: "test_tool",
			InputSchema: MCPInputSchema{
				Type: "object",
				Properties: map[string]any{
					"arg1": map[string]any{"type": "string"},
				},
			},
		},
	})

	// Call the DSGo tool wrapper
	result, err := client.tools[0].Function(context.Background(), map[string]any{"arg1": "value1"})
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if result.(string) != "Tool output" {
		t.Errorf("Expected 'Tool output', got '%v'", result)
	}
}

// Helper function to get a parameter by name
func getParameterByName(params []core.ToolParameter, name string) *core.ToolParameter {
	for _, param := range params {
		if param.Name == name {
			return &param
		}
	}
	return nil
}

func TestConvertMCPToolsToDSGo_JinaStyleUnionSchemas(t *testing.T) {
	// Simulate Jina-style union schemas with a read_url tool
	mockSchemas := []MCPToolSchema{
		{
			Name:        "read_url",
			Description: "Read content from a URL",
			InputSchema: MCPInputSchema{
				Type: "object",
				Properties: map[string]any{
					"url": map[string]any{
						"anyOf": []any{
							map[string]any{
								"type":   "string",
								"format": "uri",
							},
							map[string]any{
								"type": "array",
								"items": map[string]any{
									"type":   "string",
									"format": "uri",
								},
							},
						},
					},
					"withAllLinks": map[string]any{
						"type":        "boolean",
						"description": "Include all links in response",
					},
					"withAllImages": map[string]any{
						"type":        "boolean",
						"description": "Include all images in response",
					},
				},
				Required: []string{"url"},
			},
		},
	}

	// We need to create a mock client just for this test
	mockTransport := &MockTransport{}
	mockClient, _ := NewClient(ClientConfig{Transport: mockTransport})
	mockClient.SetSchemas(mockSchemas)

	tools := ConvertMCPToolsToDSGo(mockSchemas, mockClient)

	// Assertions:
	// - There is exactly one converted tool named `read_url`
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "read_url" {
		t.Errorf("Expected tool name 'read_url', got '%s'", tools[0].Name)
	}

	// - Its parameters include "url", "withAllLinks", "withAllImages"
	expectedParamNames := []string{"url", "withAllLinks", "withAllImages"}
	actualParamNames := make([]string, len(tools[0].Parameters))
	for i, param := range tools[0].Parameters {
		actualParamNames[i] = param.Name
	}

	for _, expected := range expectedParamNames {
		if !slices.Contains(actualParamNames, expected) {
			t.Errorf("Expected parameter '%s' not found", expected)
		}
	}

	// - The "url" parameter has Type equal to "array" with ElementType "string"
	urlParam := getParameterByName(tools[0].Parameters, "url")
	if urlParam == nil {
		t.Fatalf("Expected parameter 'url' not found")
	}
	if urlParam.Type != "array" {
		t.Errorf("Expected url parameter type 'array', got '%s'", urlParam.Type)
	}
	if urlParam.ElementType != "string" {
		t.Errorf("Expected url parameter elementType 'string', got '%s'", urlParam.ElementType)
	}

	// - The booleans have Type "bool" or "boolean"
	for _, paramName := range []string{"withAllLinks", "withAllImages"} {
		param := getParameterByName(tools[0].Parameters, paramName)
		if param == nil {
			t.Fatalf("Expected parameter '%s' not found", paramName)
		}
		if param.Type != "boolean" && param.Type != "bool" {
			t.Errorf("Expected %s parameter type 'boolean' or 'bool', got '%s'", paramName, param.Type)
		}
	}
}

func TestConvertMCPToolsToDSGo_FallbackBehavior(t *testing.T) {
	// Test fallback behavior for schema property without type and without anyOf/oneOf
	mockSchemas := []MCPToolSchema{
		{
			Name:        "fallback_tool",
			Description: "A tool with fallback schema",
			InputSchema: MCPInputSchema{
				Type: "object",
				Properties: map[string]any{
					"unknown_type_field": map[string]any{
						"description": "A field without type",
						// No type field, no anyOf/oneOf/allOf
					},
				},
				Required: []string{"unknown_type_field"},
			},
		},
	}

	mockTransport := &MockTransport{}
	mockClient, _ := NewClient(ClientConfig{Transport: mockTransport})
	mockClient.SetSchemas(mockSchemas)

	tools := ConvertMCPToolsToDSGo(mockSchemas, mockClient)

	// The unknown_type_field should default to "string" type
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	unknownFieldParam := getParameterByName(tools[0].Parameters, "unknown_type_field")
	if unknownFieldParam == nil {
		t.Fatalf("Expected parameter 'unknown_type_field' not found")
	}
	if unknownFieldParam.Type != "string" {
		t.Errorf("Expected unknown_type_field parameter type 'string', got '%s'", unknownFieldParam.Type)
	}
}

func TestInferParameterTypeVariants(t *testing.T) {
	tests := []struct {
		name         string
		def          map[string]any
		wantType     string
		wantElemType string
	}{
		{"direct string", map[string]any{"type": "string"}, "string", ""},
		{"array with element", map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}, "array", "integer"},
		{"union array string", map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}, "array", "string"},
		{"union object", map[string]any{"oneOf": []any{map[string]any{"type": "object"}}}, "json", ""},
		{"properties fallback", map[string]any{"properties": map[string]any{"x": map[string]any{"type": "string"}}}, "json", ""},
		{"default string", map[string]any{"description": "only desc"}, "string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotElem := inferParameterType(tt.def)
			if gotType != tt.wantType || gotElem != tt.wantElemType {
				t.Fatalf("got (%s,%s), want (%s,%s)", gotType, gotElem, tt.wantType, tt.wantElemType)
			}
		})
	}
}

func TestConvertTool_IgnoresNonMapProperty(t *testing.T) {
	mockTransport := &MockTransport{}
	client, _ := NewClient(ClientConfig{Transport: mockTransport})

	schema := MCPToolSchema{
		Name: "odd",
		InputSchema: MCPInputSchema{
			Type: "object",
			Properties: map[string]any{
				"good": map[string]any{"type": "string"},
				"bad":  "not-a-map",
			},
			Required: []string{"good"},
		},
	}

	tool := convertTool(schema, client)
	if getParameterByName(tool.Parameters, "good") == nil {
		t.Fatalf("expected good parameter present")
	}
	if getParameterByName(tool.Parameters, "bad") != nil {
		t.Fatalf("expected bad parameter to be ignored")
	}
}

func TestNewClient_NilTransport(t *testing.T) {
	_, err := NewClient(ClientConfig{Transport: nil})
	if err == nil {
		t.Error("Expected error for nil transport")
	}
}

func TestClient_GetSchemas(t *testing.T) {
	mockTransport := &MockTransport{}
	client, _ := NewClient(ClientConfig{Transport: mockTransport})

	schemas := []MCPToolSchema{
		{Name: "test", Description: "test desc"},
	}
	client.SetSchemas(schemas)

	got := client.GetSchemas()
	if len(got) != 1 || got[0].Name != "test" {
		t.Errorf("GetSchemas returned unexpected result: %v", got)
	}
}

func TestClient_Initialize_SSEStartError(t *testing.T) {
	badSSE := NewSSETransport("://bad-url", "")
	client, err := NewClient(ClientConfig{Transport: badSSE})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if err := client.Initialize(context.Background()); err == nil {
		t.Fatalf("expected SSE start error")
	}
}

func TestClient_Initialize_ToolsListFailures(t *testing.T) {
	t.Run("transport error", func(t *testing.T) {
		mockTransport := &MockTransport{
			SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
				if request.Method == "tools/list" {
					return nil, fmt.Errorf("boom")
				}
				return &JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{}`)}, nil
			},
		}
		client, _ := NewClient(ClientConfig{Transport: mockTransport})
		if err := client.Initialize(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("json-rpc error", func(t *testing.T) {
		mockTransport := &MockTransport{
			SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
				if request.Method == "tools/list" {
					return &JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Error: &JSONRPCError{Code: -1, Message: "fail"}}, nil
				}
				return &JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{}`)}, nil
			},
		}
		client, _ := NewClient(ClientConfig{Transport: mockTransport})
		if err := client.Initialize(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("unmarshal error", func(t *testing.T) {
		mockTransport := &MockTransport{
			SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
				if request.Method == "tools/list" {
					return &JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{`)}, nil
				}
				return &JSONRPCResponse{JSONRPC: "2.0", ID: request.ID, Result: json.RawMessage(`{}`)}, nil
			},
		}
		client, _ := NewClient(ClientConfig{Transport: mockTransport})
		if err := client.Initialize(context.Background()); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestClient_CallTool_ErrorBranches(t *testing.T) {
	baseReq := &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: mustMarshal(map[string]any{"name": "tool", "arguments": map[string]any{}})}

	t.Run("transport error", func(t *testing.T) {
		client, _ := NewClient(ClientConfig{Transport: &MockTransport{SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
			return nil, fmt.Errorf("transport boom")
		}}})
		_, err := client.CallTool(context.Background(), "tool", map[string]any{})
		if err == nil || err.Error() == "" {
			t.Fatalf("expected transport error")
		}
	})

	t.Run("json-rpc error", func(t *testing.T) {
		client, _ := NewClient(ClientConfig{Transport: &MockTransport{SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
			return &JSONRPCResponse{JSONRPC: "2.0", ID: baseReq.ID, Error: &JSONRPCError{Code: 1, Message: "nope"}}, nil
		}}})
		_, err := client.CallTool(context.Background(), "tool", map[string]any{})
		if err == nil || err.Error() == "" {
			t.Fatalf("expected json-rpc error")
		}
	})

	t.Run("unmarshal error", func(t *testing.T) {
		client, _ := NewClient(ClientConfig{Transport: &MockTransport{SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
			return &JSONRPCResponse{JSONRPC: "2.0", ID: baseReq.ID, Result: json.RawMessage(`{`)}, nil
		}}})
		_, err := client.CallTool(context.Background(), "tool", map[string]any{})
		if err == nil {
			t.Fatalf("expected unmarshal error")
		}
	})

	t.Run("isError true", func(t *testing.T) {
		client, _ := NewClient(ClientConfig{Transport: &MockTransport{SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
			return &JSONRPCResponse{JSONRPC: "2.0", ID: baseReq.ID, Result: json.RawMessage(`{"content":[],"isError":true}`)}, nil
		}}})
		_, err := client.CallTool(context.Background(), "tool", map[string]any{})
		if err == nil {
			t.Fatalf("expected tool error status")
		}
	})

	t.Run("multiple text parts", func(t *testing.T) {
		client, _ := NewClient(ClientConfig{Transport: &MockTransport{SendFunc: func(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
			return &JSONRPCResponse{JSONRPC: "2.0", ID: baseReq.ID, Result: json.RawMessage(`{"content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`)}, nil
		}}})
		out, err := client.CallTool(context.Background(), "tool", map[string]any{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "ab" {
			t.Fatalf("expected concatenated output, got %q", out)
		}
	})
}

func TestErrorFormatting(t *testing.T) {
	err := NewError(42, "oops")
	if err.Error() != "MCP Error 42: oops" {
		t.Fatalf("unexpected error string: %s", err.Error())
	}
}

func TestNewTavilyClient(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{"simple key", "test-api-key", false},
		{"key with special chars", "key+with&special=chars", false},
		{"empty key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewTavilyClient(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewTavilyClient error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if client == nil {
				t.Fatal("Expected non-nil client")
			}
			if client.transport == nil {
				t.Fatal("Expected non-nil transport")
			}

			// Verify it's an HTTP transport with the correct URL format
			httpTransport, ok := client.transport.(*HTTPTransport)
			if !ok {
				t.Fatal("Expected HTTPTransport for Tavily client")
			}

			// Extract and verify the query parameter
			parsedURL, err := url.Parse(httpTransport.url)
			if err != nil {
				t.Fatalf("Failed to parse transport URL: %v", err)
			}

			apiKeyParam := parsedURL.Query().Get("tavilyApiKey")
			if apiKeyParam != tt.apiKey {
				t.Errorf("Expected apiKey %q, got %q", tt.apiKey, apiKeyParam)
			}

			// API key should be empty since it's passed as query param
			if httpTransport.apiKey != "" {
				t.Errorf("Expected empty apiKey in transport, got %q", httpTransport.apiKey)
			}
		})
	}
}
