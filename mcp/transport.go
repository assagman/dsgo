package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Transport defines the interface for MCP communication.
type Transport interface {
	Send(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error)
	Close() error
}

// HTTPTransport implements Transport over HTTP.
type HTTPTransport struct {
	url       string
	client    *http.Client
	apiKey    string
	sessionID string
}

const defaultHTTPTimeout = 300 * time.Second

// NewHTTPTransport creates a new HTTPTransport.
func NewHTTPTransport(url string, apiKey string) *HTTPTransport {
	return NewHTTPTransportWithTimeout(url, apiKey, defaultHTTPTimeout)
}

// NewHTTPTransportWithTimeout creates a new HTTPTransport with a custom client timeout.
// If timeout <= 0, the default timeout is used.
func NewHTTPTransportWithTimeout(url string, apiKey string, timeout time.Duration) *HTTPTransport {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &HTTPTransport{
		url:       url,
		client:    &http.Client{Timeout: timeout},
		apiKey:    apiKey,
		sessionID: fmt.Sprintf("sess_%d", time.Now().UnixNano()),
	}
}

// Send sends a JSON-RPC request over HTTP with retries.
func (t *HTTPTransport) Send(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	const maxAttempts = 3
	baseBackoff := 200 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Short-circuit if context is already cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Session-Id", t.sessionID) // Required by some providers
		if t.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+t.apiKey)
			req.Header.Set("x-api-key", t.apiKey) // Some providers use this
		}

		resp, err := t.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			if attempt == maxAttempts || !shouldRetryError(err) {
				return nil, lastErr
			}
			time.Sleep(baseBackoff << (attempt - 1))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
			if attempt == maxAttempts || !shouldRetryStatus(resp.StatusCode) {
				return nil, lastErr
			}
			time.Sleep(baseBackoff << (attempt - 1))
			continue
		}

		jsonRpcResp, decodeErr := decodeJSONRPCResponse(resp.Header.Get("Content-Type"), resp.Body)
		_ = resp.Body.Close()
		if decodeErr == nil {
			return jsonRpcResp, nil
		}

		lastErr = decodeErr
		if attempt == maxAttempts || !shouldRetryError(decodeErr) {
			return nil, lastErr
		}
		time.Sleep(baseBackoff << (attempt - 1))
	}

	return nil, lastErr
}

// Close closes the HTTPTransport.
func (t *HTTPTransport) Close() error {
	return nil
}

func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func shouldRetryError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Retry on common transient network conditions
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}

	return false
}

func decodeJSONRPCResponse(contentType string, body io.Reader) (*JSONRPCResponse, error) {
	var jsonRpcResp JSONRPCResponse
	if strings.HasPrefix(contentType, "text/event-stream") {
		scanner := bufio.NewScanner(body)
		// Increase buffer size for large MCP tool responses (default 64KB is too small)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // 4MB max
		found := false
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			if data, hasPrefix := bytes.CutPrefix(line, []byte("data: ")); hasPrefix {
				if string(data) == "[DONE]" {
					continue
				}
				if err := json.Unmarshal(data, &jsonRpcResp); err != nil {
					continue // Skip malformed or non-response events
				}
				if jsonRpcResp.ID != nil || jsonRpcResp.Result != nil || jsonRpcResp.Error != nil {
					found = true
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading SSE stream: %w", err)
		}
		if !found {
			return nil, fmt.Errorf("no valid JSON-RPC response found in SSE stream")
		}
		return &jsonRpcResp, nil
	}

	if err := json.NewDecoder(body).Decode(&jsonRpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &jsonRpcResp, nil
}

// SSETransport implements Transport over SSE.
type SSETransport struct {
	sseURL  string
	postURL string
	client  *http.Client
	apiKey  string

	// Timeouts for the POST side of SSE MCP.
	// These are intentionally separate from caller context since SSE streams can be long-lived.
	postTimeout     time.Duration
	responseTimeout time.Duration

	running   atomic.Bool
	mu        sync.Mutex
	pending   map[any]chan *JSONRPCResponse
	stopCh    chan struct{}
	readError error
	initCh    chan struct{} // Closed when endpoint is received
}

// NewSSETransport creates a new SSETransport.
func NewSSETransport(url string, apiKey string) *SSETransport {
	return NewSSETransportWithTimeouts(url, apiKey, defaultHTTPTimeout, defaultHTTPTimeout)
}

// NewSSETransportWithTimeouts creates a new SSETransport with custom POST and response timeouts.
// If either timeout <= 0, the default is used.
func NewSSETransportWithTimeouts(url string, apiKey string, postTimeout time.Duration, responseTimeout time.Duration) *SSETransport {
	if postTimeout <= 0 {
		postTimeout = defaultHTTPTimeout
	}
	if responseTimeout <= 0 {
		responseTimeout = defaultHTTPTimeout
	}

	t := &SSETransport{
		sseURL:          url,
		client:          &http.Client{Timeout: 0}, // No timeout for SSE stream
		apiKey:          apiKey,
		postTimeout:     postTimeout,
		responseTimeout: responseTimeout,
		pending:         make(map[any]chan *JSONRPCResponse),
		stopCh:          make(chan struct{}),
		initCh:          make(chan struct{}),
	}
	t.running.Store(true)
	return t
}

func closeStopCh(ch chan struct{}) {
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}

// Start initiates the SSE connection and waits for the endpoint event.
func (t *SSETransport) Start(ctx context.Context) error {

	// Use a transport with dial timeout but no response timeout
	// The SSE stream needs to stay open indefinitely for reading events
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		IdleConnTimeout:       0, // Keep connection alive
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	sseClient := &http.Client{
		Transport: transport,
		Timeout:   0, // No timeout - SSE streams indefinitely
	}

	// Don't use context for the request - we want the connection to persist
	// after Start() returns. We'll handle cancellation via t.stopCh.
	req, err := http.NewRequest("GET", t.sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	resp, err := sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	go t.readLoop(resp.Body)

	// Wait for the endpoint event with timeout
	select {
	case <-t.initCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for SSE endpoint: %w", ctx.Err())
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timeout waiting for SSE endpoint event from %s", t.sseURL)
	}
}

// Send sends a JSON-RPC request.
func (t *SSETransport) Send(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
	// Verify initialization completed (should be done in Start())
	select {
	case <-t.initCh:
		// Good, initialized
	default:
		return nil, fmt.Errorf("SSE transport not initialized - call Start() first")
	}

	t.mu.Lock()
	if !t.running.Load() {
		t.mu.Unlock()
		return nil, fmt.Errorf("transport is closed: %w", t.readError)
	}
	respCh := make(chan *JSONRPCResponse, 1)
	t.pending[request.ID] = respCh
	t.mu.Unlock()

	// Ensure cleanup of pending map entry on all return paths (success, error, timeout)
	defer func() {
		t.mu.Lock()
		delete(t.pending, request.ID)
		t.mu.Unlock()
	}()

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use a separate timeout for the POST request.
	// Don't use the parent context directly as it may have a shorter timeout.
	postTimeout := t.postTimeout
	if postTimeout <= 0 {
		postTimeout = defaultHTTPTimeout
	}
	postCtx, postCancel := context.WithTimeout(context.Background(), postTimeout)
	defer postCancel()

	req, err := http.NewRequestWithContext(postCtx, "POST", t.postURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	// Send POST request
	postResp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = postResp.Body.Close()
	}()

	if postResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(postResp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", postResp.StatusCode, string(bodyBytes))
	}

	// Wait for response with our own timeout (don't rely on caller's context for SSE)
	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("transport closed while waiting for response")
		}
		if resp == nil {
			return nil, fmt.Errorf("received nil response - transport may have closed")
		}
		return resp, nil
	case <-time.After(func() time.Duration {
		if t.responseTimeout > 0 {
			return t.responseTimeout
		}
		return defaultHTTPTimeout
	}()):
		return nil, fmt.Errorf("timeout waiting for response to request %v", request.ID)
	}
}

func (t *SSETransport) readLoop(body io.ReadCloser) {
	defer func() {
		_ = body.Close()
	}()
	scanner := bufio.NewScanner(body)
	// Increase buffer size for large MCP tool responses (default 64KB is too small)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // 4MB max
	var eventType string

	for scanner.Scan() {
		line := scanner.Bytes()
		lineStr := string(line)

		if len(line) == 0 {
			eventType = "" // Reset event type on empty line (end of event)
			continue
		}

		// Handle colon-prefixed comments (keep-alive)
		if len(line) > 0 && line[0] == ':' {
			continue
		}

		if bytes.HasPrefix(line, []byte("event:")) {
			// Handle with or without space after colon
			eventType = strings.TrimSpace(strings.TrimPrefix(lineStr, "event:"))
			continue
		}

		if bytes.HasPrefix(line, []byte("data:")) {
			// Handle with or without space after colon
			data := []byte(strings.TrimPrefix(lineStr, "data:"))
			if len(data) > 0 && data[0] == ' ' {
				data = data[1:]
			}

			// Handle endpoint event (standard MCP SSE pattern)
			if eventType == "endpoint" {
				// Handle endpoint event
				endpoint := string(data)
				// Handle relative URL
				if len(endpoint) > 0 && endpoint[0] == '/' {
					// Parse base URL using net/url
					u, err := url.Parse(t.sseURL)
					if err != nil {
						// Fallback to simple string manipulation if parsing fails
						fmt.Fprintf(os.Stderr, "dsgo: warning: failed to parse SSE URL %q: %v\n", t.sseURL, err)
					} else {
						// Construct new URL
						t.postURL = u.Scheme + "://" + u.Host + endpoint
					}

					// If fallback needed or just use the robust way exclusively:
					if t.postURL == "" {
						// Only if url.Parse failed
						schemeEnd := len("https://")
						// Handle http:// case
						if strings.HasPrefix(t.sseURL, "http://") {
							schemeEnd = len("http://")
						}

						pathStart := -1
						for i := schemeEnd; i < len(t.sseURL); i++ {
							if t.sseURL[i] == '/' {
								pathStart = i
								break
							}
						}
						var host string
						if pathStart != -1 {
							host = t.sseURL[:pathStart]
						} else {
							host = t.sseURL
						}
						t.postURL = host + endpoint
					}
				} else {
					t.postURL = endpoint
				}

				// Signal initialization done (only once)
				select {
				case <-t.initCh:
				default:
					close(t.initCh)
				}
				continue
			}

			// Handle message event (or default)
			var resp JSONRPCResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				continue
			}

			if resp.ID != nil {
				t.mu.Lock()
				ch, ok := t.pending[resp.ID]
				t.mu.Unlock()
				if ok {
					ch <- &resp
				}
			}
		}
	}

	t.mu.Lock()
	t.readError = scanner.Err()
	if t.readError == nil {
		t.readError = io.EOF
	}
	t.running.Store(false)
	for _, ch := range t.pending {
		close(ch)
	}
	t.mu.Unlock()
	closeStopCh(t.stopCh)
}

// Close closes the SSETransport.
func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running.Load() {
		return nil
	}
	t.running.Store(false)
	closeStopCh(t.stopCh)
	return nil
}

// StdioTransport implements Transport over stdio of a subprocess.
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	running   atomic.Bool
	mu        sync.Mutex
	pending   map[any]chan *JSONRPCResponse
	stopCh    chan struct{}
	readError error
}

// NewStdioTransport creates a new StdioTransport.
func NewStdioTransport(command string, args []string, env []string) (*StdioTransport, error) {
	return NewStdioTransportWithDir(command, args, env, "")
}

// NewStdioTransportWithDir creates a new StdioTransport with a specific working directory.
// If dir is empty, the current working directory is used.
func NewStdioTransportWithDir(command string, args []string, env []string, dir string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = env
	if dir != "" {
		cmd.Dir = dir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	t := &StdioTransport{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		pending: make(map[any]chan *JSONRPCResponse),
		stopCh:  make(chan struct{}),
	}
	t.running.Store(true)

	go t.readLoop()
	go t.logStderr()

	return t, nil
}

// Running reports whether the transport is still active.
func (t *StdioTransport) Running() bool {
	return t.running.Load()
}

// Send sends a JSON-RPC request over stdio.
func (t *StdioTransport) Send(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
	t.mu.Lock()
	if !t.running.Load() {
		t.mu.Unlock()
		return nil, fmt.Errorf("transport is closed: %w", t.readError)
	}
	respCh := make(chan *JSONRPCResponse, 1)
	t.pending[request.ID] = respCh
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pending, request.ID)
		t.mu.Unlock()
	}()

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Append newline as per JSON-RPC over stdio convention
	body = append(body, '\n')

	if _, err := t.stdin.Write(body); err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.stopCh:
		return nil, fmt.Errorf("transport closed while waiting for response")
	}
}

// Close closes the StdioTransport.
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running.Load() {
		return nil
	}

	t.running.Store(false)
	closeStopCh(t.stopCh)
	_ = t.stdin.Close()
	_ = t.cmd.Process.Kill()
	return t.cmd.Wait()
}

func (t *StdioTransport) readLoop() {
	scanner := bufio.NewScanner(t.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			// Ignore malformed lines or logs
			continue
		}

		if resp.ID != nil {
			t.mu.Lock()
			ch, ok := t.pending[resp.ID]
			t.mu.Unlock()
			if ok {
				ch <- &resp
			}
		}
	}

	t.mu.Lock()
	t.readError = scanner.Err()
	if t.readError == nil {
		t.readError = io.EOF
	}
	t.running.Store(false)
	// Close all pending channels
	for _, ch := range t.pending {
		close(ch)
	}
	t.mu.Unlock()
	closeStopCh(t.stopCh)
}

func (t *StdioTransport) logStderr() {
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		// In a real app, we might log this
		// fmt.Fprintf(os.Stderr, "MCP Stderr: %s\n", scanner.Text())
	}
}
