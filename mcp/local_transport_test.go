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
	tr, err := NewLocalTransport(testHandler{})
	if err != nil {
		t.Fatalf("NewLocalTransport() error: %v", err)
	}
	resp, err := tr.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tools/list"})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if resp == nil || resp.Result == nil {
		t.Fatalf("expected response result")
	}
}

func TestLocalTransport_NilHandler(t *testing.T) {
	_, err := NewLocalTransport(nil)
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}
