package mock

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

var (
	httpTransportMu       sync.RWMutex
	httpTransportOverride http.RoundTripper
)

// SetHTTPTransport overrides the HTTP transport used by the mock provider.
//
// This is primarily intended for tests that want to script responses without
// running an httptest.Server.
//
// The returned reset function restores the previous transport.
func SetHTTPTransport(rt http.RoundTripper) (reset func()) {
	httpTransportMu.Lock()
	prev := httpTransportOverride
	httpTransportOverride = rt
	httpTransportMu.Unlock()

	return func() {
		httpTransportMu.Lock()
		httpTransportOverride = prev
		httpTransportMu.Unlock()
	}
}

func getHTTPTransportOverride() http.RoundTripper {
	httpTransportMu.RLock()
	rt := httpTransportOverride
	httpTransportMu.RUnlock()
	return rt
}

// HTTPResponseStep describes a single scripted HTTP response.
//
// When StatusCode is 0 it defaults to 200.
// If Err is non-nil, RoundTrip returns Err and ignores the other fields.
type HTTPResponseStep struct {
	StatusCode int
	Header     http.Header
	Body       string
	Err        error
}

// CapturedRequest is a recorded outbound HTTP request made by the mock LM.
type CapturedRequest struct {
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// ScriptedTransport is an http.RoundTripper that returns a fixed sequence of
// responses. It is safe for concurrent use.
//
// If the number of requests exceeds the number of steps, RoundTrip returns an
// error.
type ScriptedTransport struct {
	mu       sync.Mutex
	steps    []HTTPResponseStep
	next     int
	requests []CapturedRequest
}

func NewScriptedTransport(steps ...HTTPResponseStep) *ScriptedTransport {
	copied := make([]HTTPResponseStep, len(steps))
	copy(copied, steps)
	return &ScriptedTransport{steps: copied}
}

func (t *ScriptedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		bodyBytes = b
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Capture request
	t.requests = append(t.requests, CapturedRequest{
		Method: req.Method,
		URL:    req.URL.String(),
		Header: req.Header.Clone(),
		Body:   bodyBytes,
	})

	if t.next >= len(t.steps) {
		return nil, fmt.Errorf("no scripted response available for request %d", t.next+1)
	}
	step := t.steps[t.next]
	t.next++

	if step.Err != nil {
		return nil, step.Err
	}

	statusCode := step.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	h := make(http.Header)
	for k, v := range step.Header {
		h[k] = append([]string(nil), v...)
	}
	if h.Get("Content-Type") == "" {
		// Default to JSON for convenience. Streaming tests can override.
		h.Set("Content-Type", "application/json")
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(step.Body)),
		Request:    req,
	}

	return resp, nil
}

// Requests returns the recorded outbound requests.
func (t *ScriptedTransport) Requests() []CapturedRequest {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]CapturedRequest, len(t.requests))
	copy(out, t.requests)
	return out
}
