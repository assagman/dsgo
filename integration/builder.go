package integration

import (
	"context"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/integration/fixtures"
	"github.com/assagman/dsgo/module"
)

// TestBuilder provides a fluent interface for setting up integration tests.
// It reduces boilerplate and ensures consistent test configuration.
type TestBuilder struct {
	signature  *core.Signature
	responses  []string
	errorMsg   string
	timeout    time.Duration
	adapter    core.Adapter
	moduleType string
	latency    time.Duration
}

// TestContext holds all configured components for a test.
type TestContext struct {
	Ctx       context.Context
	Cancel    context.CancelFunc
	LM        core.LM
	Module    core.Module
	Signature *core.Signature
	T         *testing.T
}

// NewTestBuilder creates a new test builder with sensible defaults.
func NewTestBuilder() *TestBuilder {
	return &TestBuilder{
		signature:  fixtures.SimplePredictSig(),
		responses:  []string{`{"answer": "test response"}`},
		timeout:    10 * time.Second,
		moduleType: "predict",
	}
}

// WithSignature sets the signature for the test module.
func (b *TestBuilder) WithSignature(sig *core.Signature) *TestBuilder {
	b.signature = sig
	return b
}

// WithMockResponse sets a single mock response.
func (b *TestBuilder) WithMockResponse(response string) *TestBuilder {
	b.responses = []string{response}
	return b
}

// WithMockResponses sets multiple mock responses (used sequentially).
func (b *TestBuilder) WithMockResponses(responses []string) *TestBuilder {
	b.responses = responses
	return b
}

// WithError configures the mock to return an error.
func (b *TestBuilder) WithError(errMsg string) *TestBuilder {
	b.errorMsg = errMsg
	return b
}

// WithTimeout sets the context timeout.
func (b *TestBuilder) WithTimeout(d time.Duration) *TestBuilder {
	b.timeout = d
	return b
}

// WithAdapter sets a specific adapter.
func (b *TestBuilder) WithAdapter(adapter core.Adapter) *TestBuilder {
	b.adapter = adapter
	return b
}

// WithLatency adds artificial latency to mock responses.
func (b *TestBuilder) WithLatency(d time.Duration) *TestBuilder {
	b.latency = d
	return b
}

// WithModuleType sets the module type ("predict", "cot", "refine").
func (b *TestBuilder) WithModuleType(moduleType string) *TestBuilder {
	b.moduleType = moduleType
	return b
}

// Build creates the test context with all configured components.
func (b *TestBuilder) Build(t *testing.T) *TestContext {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), b.timeout)

	// Create mock LM
	var lm core.LM
	if b.errorMsg != "" {
		lm = NewAlwaysFailLM(b.errorMsg)
	} else if len(b.responses) > 1 {
		lm = NewMockLMWithResponses(b.responses)
	} else if len(b.responses) == 1 {
		lm = NewMockLMWithResponse(b.responses[0])
	} else {
		lm = NewMockLMWithResponse(`{"answer": "default"}`)
	}

	// Add latency if configured
	if b.latency > 0 {
		lm = NewLatencyLM(b.latency, b.responses[0])
	}

	// Create module based on type
	var mod core.Module
	switch b.moduleType {
	case "predict":
		pred := module.NewPredict(b.signature, lm)
		if b.adapter != nil {
			pred = pred.WithAdapter(b.adapter)
		}
		mod = pred
	case "cot":
		cot := module.NewChainOfThought(b.signature, lm)
		if b.adapter != nil {
			cot = cot.WithAdapter(b.adapter)
		}
		mod = cot
	case "refine":
		refine := module.NewRefine(b.signature, lm)
		if b.adapter != nil {
			refine = refine.WithAdapter(b.adapter)
		}
		mod = refine
	default:
		t.Fatalf("unknown module type: %s", b.moduleType)
	}

	return &TestContext{
		Ctx:       ctx,
		Cancel:    cancel,
		LM:        lm,
		Module:    mod,
		Signature: b.signature,
		T:         t,
	}
}

// Cleanup releases resources. Call with defer after Build().
func (tc *TestContext) Cleanup() {
	tc.Cancel()
}

// Forward executes the module with given inputs and returns the result.
func (tc *TestContext) Forward(inputs map[string]any) (*core.Prediction, error) {
	return tc.Module.Forward(tc.Ctx, inputs)
}

// MustForward executes the module and fails the test on error.
func (tc *TestContext) MustForward(inputs map[string]any) *core.Prediction {
	tc.T.Helper()
	result, err := tc.Module.Forward(tc.Ctx, inputs)
	if err != nil {
		tc.T.Fatalf("Forward failed: %v", err)
	}
	return result
}

// AssertSuccess verifies the module execution succeeds with expected fields.
func (tc *TestContext) AssertSuccess(inputs map[string]any, expectedFields []string) *core.Prediction {
	tc.T.Helper()
	result := tc.MustForward(inputs)
	AssertPredictionValid(tc.T, result, expectedFields)
	return result
}

// AssertError verifies the module execution fails with expected error.
func (tc *TestContext) AssertError(inputs map[string]any, expectedErrSubstring string) {
	tc.T.Helper()
	_, err := tc.Module.Forward(tc.Ctx, inputs)
	if err == nil {
		tc.T.Fatal("expected error, got nil")
	}
	if expectedErrSubstring != "" && !containsSubstring(err.Error(), expectedErrSubstring) {
		tc.T.Errorf("error %q does not contain %q", err.Error(), expectedErrSubstring)
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
