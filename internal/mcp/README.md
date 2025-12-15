# MCP (Model Context Protocol) Client

This package provides an implementation of the Model Context Protocol (MCP) for use with DSGo. MCP allows LLMs to access external tools and resources through standardized interfaces.

## Features

- **HTTP and SSE Transport**: Support for both HTTP request/response and Server-Sent Events (SSE) transports
- **Stdio Transport**: Support for communication with MCP servers via standard input/output
- **Tool Conversion**: Automatic conversion of MCP tool schemas to DSGo tools
- **Complex Schema Handling**: Advanced type inference for complex JSON Schema definitions including union types (anyOf, oneOf, allOf)

## Tool Schema Conversion

DSGo collapses complex MCP JSON Schema types into compatible parameter types for LLM tool-calling:

- **Union Types**: Complex union schemas (like Jina's `string` OR `array<string>`) are converted to appropriate DSGo parameter types
- **Array Handling**: Arrays-of-strings are preferred when available in union types
- **Fallback Behavior**: Unknown/complex schemas default to `string` type for compatibility
- **Provider Compatibility**: Ensures schemas work with strict providers like OpenAI, Mistral, and xAI over OpenRouter

## Supported MCP Clients

- **Exa**: Search and web content extraction
- **Jina**: URL reading and content extraction
- **Tavily**: Web search (`tavily-search`) and content extraction (`tavily-extract`)

## Usage

This package is re-exported through the main `dsgo` package. Use it via the main package:

```go
import "github.com/assagman/dsgo"

// Create Exa MCP client
exaClient, err := dsgo.NewMCPExaClient(apiKey)
if err != nil {
    log.Fatalf("Failed to create Exa client: %v", err)
}

// Initialize client
if err := exaClient.Initialize(ctx); err != nil {
    log.Fatalf("Failed to initialize Exa client: %v", err)
}

// Get tools for use with DSGo
tools := exaClient.GetTools()

// Use with DSGo ReAct module
react := dsgo.NewReAct(signature, lm, tools)
```

## Custom MCP Servers

For custom MCP servers, use the transport constructors directly:

```go
import "github.com/assagman/dsgo"

// HTTP Transport
transport := dsgo.NewMCPHTTPTransport("https://your-mcp-server.com/mcp", "api-key")

// SSE Transport
transport := dsgo.NewMCPSSETransport("https://your-mcp-server.com/sse", "api-key")

// Create client with custom transport
client, err := dsgo.NewMCPClient(dsgo.MCPClientConfig{Transport: transport})
```

## Transport Types

### HTTPTransport

Used for standard HTTP request/response MCP servers. Supports both JSON and SSE response formats.

### SSETransport

Used for Server-Sent Events based MCP servers. The transport:
1. Connects to the SSE endpoint
2. Waits for the `endpoint` event containing the POST URL
3. Sends requests via POST and receives responses via SSE

### StdioTransport

Used for local MCP servers that communicate via stdin/stdout. Useful for running MCP servers as subprocesses.

## Architecture

```
                    ┌──────────────────┐
                    │   MCP Client     │
                    │                  │
                    │ - Initialize()   │
                    │ - CallTool()     │
                    │ - GetTools()     │
                    └────────┬─────────┘
                             │
                             │ uses
                             ▼
                    ┌──────────────────┐
                    │   Transport      │
                    │   (interface)    │
                    └────────┬─────────┘
                             │
            ┌────────────────┼────────────────┐
            │                │                │
            ▼                ▼                ▼
    ┌───────────┐    ┌───────────┐    ┌───────────┐
    │   HTTP    │    │    SSE    │    │   Stdio   │
    │ Transport │    │ Transport │    │ Transport │
    └───────────┘    └───────────┘    └───────────┘
```

## Thread Safety

All transports are thread-safe and can be used concurrently:

- `HTTPTransport`: Stateless, each request is independent
- `SSETransport`: Uses mutex for pending request tracking
- `StdioTransport`: Uses mutex for pending request tracking

## Error Handling

The package defines standard JSON-RPC 2.0 error codes:

- `ErrCodeParseError` (-32700): Invalid JSON
- `ErrCodeInvalidRequest` (-32600): Invalid request object
- `ErrCodeMethodNotFound` (-32601): Method not found
- `ErrCodeInvalidParams` (-32602): Invalid parameters
- `ErrCodeInternalError` (-32603): Internal server error
