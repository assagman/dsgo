package mcp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSSETransport_EndpointResolution(t *testing.T) {
	// Create a mock SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send endpoint event
		_, _ = w.Write([]byte("event: endpoint\n"))
		_, _ = w.Write([]byte("data: /api/messages\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Keep connection open
		time.Sleep(1 * time.Second)
	}))
	defer server.Close()

	// Create SSE transport
	transport := NewSSETransport(server.URL+"/sse", "test-key")

	// Start transport
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Verify postURL
	expected := server.URL + "/api/messages"
	if transport.postURL != expected {
		t.Errorf("Expected postURL %q, got %q", expected, transport.postURL)
	}

	_ = transport.Close()
}

func TestSSETransport_Send(t *testing.T) {
	// Channel to coordinate response
	respCh := make(chan string)

	// Create a mock server with mux
	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Send endpoint event
		_, _ = w.Write([]byte("event: endpoint\n"))
		_, _ = w.Write([]byte("data: /api/messages\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Wait for response to send
		select {
		case respData := <-respCh:
			_, _ = w.Write([]byte("event: message\n"))
			_, _ = w.Write([]byte("data: " + respData + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-time.After(5 * time.Second):
			// Timeout
		}
	})

	// POST endpoint
	mux.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
		// Respond with 202 Accepted
		w.WriteHeader(http.StatusAccepted)

		// Send response via SSE
		// Note: We use string ID "1" here to test the type matching fix
		// If Client used int, we'd have to ensure unmarshaling handles it.
		// Since we fixed Client to use string, this response should be matched.
		respCh <- `{"jsonrpc":"2.0","id":"1","result":{"status":"ok"}}`
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Create SSE transport
	transport := NewSSETransport(server.URL+"/sse", "test-key")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Send request with ID "1"
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1", // String ID
		Method:  "test",
	}

	resp, err := transport.Send(ctx, req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if resp.ID != "1" {
		t.Errorf("Expected response ID '1', got %v", resp.ID)
	}
}

func TestHTTPTransport_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Authorization 'Bearer test-key', got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":{"status":"ok"}}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "test-key")

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "test",
	}

	resp, err := transport.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if resp.ID != "1" {
		t.Errorf("Expected response ID '1', got %v", resp.ID)
	}
}

func TestHTTPTransport_RetryOnServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("temporary failure"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":{"status":"ok"}}`))
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "")

	resp, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "test"})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}

	if resp.ID != "1" {
		t.Errorf("Expected response ID '1', got %v", resp.ID)
	}
}

func TestHTTPTransport_NoRetryOnClientError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "")

	_, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "test"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if attempts != 1 {
		t.Fatalf("expected 1 attempt for client error, got %d", attempts)
	}
}

func TestHTTPTransport_Close(t *testing.T) {
	transport := NewHTTPTransport("http://example.com", "key")
	if err := transport.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestHTTPTransport_MarshalError(t *testing.T) {
	transport := NewHTTPTransport("http://example.com", "")
	_, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: make(chan int), Method: "test"})
	if err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestHTTPTransport_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	transport := NewHTTPTransport(server.URL, "")
	_, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "test"})
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestShouldRetryStatus(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{http.StatusTooManyRequests, true},
		{http.StatusBadGateway, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadRequest, false},
		{http.StatusOK, false},
	}
	for _, tc := range cases {
		if got := shouldRetryStatus(tc.status); got != tc.want {
			t.Fatalf("status %d got %v want %v", tc.status, got, tc.want)
		}
	}
}

func TestShouldRetryError(t *testing.T) {
	timeoutErr := &net.DNSError{IsTimeout: true}
	if !shouldRetryError(timeoutErr) {
		t.Fatalf("expected timeout to retry")
	}
	if !shouldRetryError(context.DeadlineExceeded) {
		t.Fatalf("expected deadline to retry")
	}
	if !shouldRetryError(syscall.ECONNRESET) {
		t.Fatalf("expected ECONNRESET to retry")
	}
	if shouldRetryError(errors.New("other")) {
		t.Fatalf("unexpected retry for generic error")
	}
}

func TestDecodeJSONRPCResponse_SSE_NoResponse(t *testing.T) {
	body := strings.NewReader("event: message\ndata: {\"not\":\"rpc\"}\n\n")
	if _, err := decodeJSONRPCResponse("text/event-stream", body); err == nil {
		t.Fatalf("expected error when no valid response")
	}
}

func TestDecodeJSONRPCResponse_JSONDecodeFailure(t *testing.T) {
	if _, err := decodeJSONRPCResponse("application/json", strings.NewReader("not json")); err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestSSETransport_StartErrors(t *testing.T) {
	t.Run("status error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		transport := NewSSETransport(server.URL, "")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := transport.Start(ctx); err == nil {
			t.Fatalf("expected status error")
		}
	})

	t.Run("timeout waiting endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(2 * time.Second)
		}))
		defer server.Close()

		transport := NewSSETransport(server.URL, "")
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if err := transport.Start(ctx); err == nil {
			t.Fatalf("expected timeout error")
		}
	})
}

func TestSSETransport_SendErrors(t *testing.T) {
	transport := NewSSETransport("http://example.com/sse", "")
	_, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "test"})
	if err == nil {
		t.Fatalf("expected not initialized error")
	}

	close(transport.initCh)
	transport.running.Store(false)
	resp, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "test"})
	if err == nil || resp != nil {
		t.Fatalf("expected closed transport error")
	}
}

func writeHelperProgram(t *testing.T, dir string) string {
	t.Helper()
	code := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req map[string]any
		_ = json.Unmarshal(line, &req)
		resp := map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": map[string]any{"ok": true}}
		b, _ := json.Marshal(resp)
		fmt.Println(string(b))
		return
	}
}
`
	path := filepath.Join(dir, "helper.go")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatalf("failed to write helper: %v", err)
	}
	return path
}

func TestStdioTransport(t *testing.T) {
	dir := t.TempDir()
	helper := writeHelperProgram(t, dir)

	cmdArgs := []string{"run", helper}
	transport, err := NewStdioTransport("go", cmdArgs, os.Environ())
	if err != nil {
		t.Fatalf("failed to start stdio transport: %v", err)
	}

	t.Cleanup(func() { _ = transport.Close() })

	resp, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "ping"})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if resp == nil || resp.Result == nil {
		t.Fatalf("expected result")
	}

	// Wait for helper process to exit and transport to mark itself not running
	deadline := time.Now().Add(time.Second)
	for transport.Running() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := transport.Send(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "2", Method: "ping"}); err == nil {
		t.Fatalf("expected error after transport stopped")
	}
}
