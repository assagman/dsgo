package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

type testHandler struct{}

func (h testHandler) Handle(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	payload, _ := json.Marshal(map[string]any{"ok": true, "method": req.Method})
	return &JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: payload}, nil
}

func TestLocalTransport_Send(t *testing.T) {
	tr := NewLocalTransport(testHandler{})
	resp, err := tr.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tools/list"})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if resp == nil || resp.Result == nil {
		t.Fatalf("expected response result")
	}
}
