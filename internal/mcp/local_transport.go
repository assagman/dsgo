package mcp

import "context"

// LocalHandler handles MCP JSON-RPC requests in-process.
//
// It enables building built-in MCP servers without HTTP/SSE/Stdio transports.
// LocalHandler implementations must be thread-safe.
type LocalHandler interface {
	Handle(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error)
}

// LocalTransport is an MCP transport that routes requests to an in-process handler.
//
// It is useful for built-in tools like a safe shell runner.
type LocalTransport struct {
	handler LocalHandler
}

// NewLocalTransport creates a new LocalTransport.
func NewLocalTransport(handler LocalHandler) *LocalTransport {
	return &LocalTransport{handler: handler}
}

// Send routes the request to the local handler.
func (t *LocalTransport) Send(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
	return t.handler.Handle(ctx, request)
}

// Close closes the transport.
func (t *LocalTransport) Close() error { return nil }
