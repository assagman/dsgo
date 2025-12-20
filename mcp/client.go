package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"sync/atomic"

	"github.com/assagman/dsgo/core"
)

// Client represents an MCP client.
type Client struct {
	transport Transport
	tools     []core.Tool
	schemas   []MCPToolSchema
	nextID    int64
}

// ClientConfig configuration for creating a new Client.
type ClientConfig struct {
	Transport Transport
}

// NewClient creates a new MCP client.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Transport == nil {
		return nil, fmt.Errorf("transport is required")
	}
	return &Client{
		transport: cfg.Transport,
	}, nil
}

// NewExaClient creates a new MCP client for Exa.
func NewExaClient(apiKey string) (*Client, error) {
	// Exa MCP URL: https://mcp.exa.ai/mcp
	transport := NewHTTPTransport("https://mcp.exa.ai/mcp", apiKey)
	return NewClient(ClientConfig{Transport: transport})
}

// NewJinaClient creates a new MCP client for Jina.
func NewJinaClient(apiKey string) (*Client, error) {
	// Jina MCP URL: https://mcp.jina.ai/sse (Using SSE as required by server)
	transport := NewSSETransport("https://mcp.jina.ai/sse", apiKey)
	return NewClient(ClientConfig{Transport: transport})
}

// NewTavilyClient creates a new MCP client for Tavily.
// Provides tavily-search and tavily-extract tools for web search and content extraction.
func NewTavilyClient(apiKey string) (*Client, error) {
	// Tavily MCP URL: https://mcp.tavily.com/mcp?tavilyApiKey=<api-key>
	// The API key is passed as a query parameter, not as a header
	baseURL, err := url.Parse("https://mcp.tavily.com/mcp")
	if err != nil {
		return nil, fmt.Errorf("failed to parse Tavily MCP URL: %w", err)
	}
	q := baseURL.Query()
	q.Set("tavilyApiKey", apiKey)
	baseURL.RawQuery = q.Encode()

	transport := NewHTTPTransport(baseURL.String(), "")
	return NewClient(ClientConfig{Transport: transport})
}

// NewFilesystemClient creates a new MCP client for local filesystem operations.
// Uses the official @modelcontextprotocol/server-filesystem via npx/bunx with stdio transport.
// The directory parameter specifies both the allowed directory and the working directory
// for the MCP server, so relative paths resolve correctly within it.
// If empty, defaults to current working directory.
func NewFilesystemClient(directory string) (*Client, error) {
	// Default to current directory if none specified
	if directory == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		directory = cwd
	}

	// Try bunx first (Bun's npx equivalent), fall back to npx
	command := "bunx"
	if _, err := exec.LookPath("bunx"); err != nil {
		command = "npx"
		if _, err := exec.LookPath("npx"); err != nil {
			return nil, fmt.Errorf("neither bunx nor npx found in PATH - please install Bun or Node.js")
		}
	}

	// Build args: npx -y @modelcontextprotocol/server-filesystem <directory>
	args := []string{"-y", "@modelcontextprotocol/server-filesystem", directory}

	// Create stdio transport with working directory set to the allowed directory.
	// This ensures relative paths are resolved correctly from the directory root.
	transport, err := NewStdioTransportWithDir(command, args, os.Environ(), directory)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdio transport: %w", err)
	}

	return NewClient(ClientConfig{Transport: transport})
}

// Initialize initializes the client and fetches tools.
func (c *Client) Initialize(ctx context.Context) error {
	// Start SSE transport if needed
	if sse, ok := c.transport.(*SSETransport); ok {
		if err := sse.Start(ctx); err != nil {
			return fmt.Errorf("failed to start SSE transport: %w", err)
		}
	}

	// 1. Send initialize request (optional/required by spec?)
	// For HTTP/Stateless MCP, we might just call tools/list directly if handshake isn't strict.
	// But let's follow spec if possible.
	// Spec says: initialize -> initialized -> ...

	initReq := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprint(c.getNextID()),
		Method:  "initialize",
		Params: mustMarshal(MCPInitializeParams{
			ProtocolVersion: "2024-11-05", // Example version
			Capabilities:    MCPCapabilities{},
			ClientInfo: MCPClientInfo{
				Name:    "dsgo-client",
				Version: "0.1.0",
			},
		}),
	}

	// Some MCP servers over HTTP might not support initialize or might be stateless.
	// We'll try initialize, if it fails or returns valid result, we proceed to tools/list.
	// For simple HTTP proxies, they often just expose tools/list.

	_, err := c.transport.Send(ctx, initReq)
	if err != nil {
		// Log error but maybe continue if it's just method not found (stateless)?
		// But strictly speaking we should fail.
		// However, for known HTTP endpoints like Exa/Jina, let's see.
		// Exa MCP usually works with direct calls.
		_ = err // Suppress errcheck warning for now
	}

	// 2. Send initialized notification (if stateful)
	// ...

	// 3. List tools
	listReq := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprint(c.getNextID()),
		Method:  "tools/list",
	}

	resp, err := c.transport.Send(ctx, listReq)
	if err != nil {
		return fmt.Errorf("tools/list failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("tools/list error: %d %s", resp.Error.Code, resp.Error.Message)
	}

	var result MCPListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("failed to unmarshal tools list: %w", err)
	}

	c.schemas = result.Tools
	c.tools = ConvertMCPToolsToDSGo(result.Tools, c)

	return nil
}

// CallTool calls a tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprint(c.getNextID()),
		Method:  "tools/call",
		Params: mustMarshal(map[string]any{
			"name":      name,
			"arguments": args,
		}),
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("tool execution error: %d %s", resp.Error.Code, resp.Error.Message)
	}

	var result MCPCallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	if result.IsError {
		// Many MCP servers return error details in the content array.
		// Preserve that information so callers can see the real failure cause.
		var errText string
		for _, content := range result.Content {
			if content.Type == "text" && content.Text != "" {
				if errText != "" {
					errText += "\n"
				}
				errText += content.Text
			}
		}
		if errText == "" {
			errText = "tool returned error status"
		}
		return "", fmt.Errorf("tool returned error status: %s", errText)
	}

	// Combine text content
	var output string
	for _, content := range result.Content {
		if content.Type == "text" {
			output += content.Text
		}
	}

	return output, nil
}

// GetTools returns the converted DSGo tools.
func (c *Client) GetTools() []core.Tool {
	return c.tools
}

// GetSchemas returns the raw MCP tool schemas.
func (c *Client) GetSchemas() []MCPToolSchema {
	return c.schemas
}

// SetSchemas sets the tool schemas (useful for testing).
func (c *Client) SetSchemas(schemas []MCPToolSchema) {
	c.schemas = schemas
	c.tools = ConvertMCPToolsToDSGo(schemas, c)
}

func (c *Client) getNextID() int64 {
	return atomic.AddInt64(&c.nextID, 1)
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
