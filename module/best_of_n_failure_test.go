package module

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/assagman/dsgo/core"
)

// MockFailingModule always fails
type MockFailingModule struct {
	sig         *core.Signature
	failureType string // "parse" or "http"
}

func (m *MockFailingModule) GetSignature() *core.Signature {
	return m.sig
}

func (m *MockFailingModule) Forward(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
	if m.failureType == "parse" {
		return nil, errors.New("failed to parse: all adapters failed")
	}
	return nil, errors.New("API request failed with status 429: rate limited")
}

// TestBestOfN_AllAttemptsFail tests behavior when all N attempts fail
func TestBestOfN_AllAttemptsFail(t *testing.T) {
	sig := core.NewSignature("Test")

	mockModule := &MockFailingModule{
		sig:         sig,
		failureType: "parse",
	}

	scorer := func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		return 1.0, nil
	}

	// Set MaxFailures to allow all 3 to fail
	bon := NewBestOfN(mockModule, 3).
		WithScorer(scorer).
		WithMaxFailures(3)
	inputs := map[string]any{"question": "test"}

	result, err := bon.Forward(context.Background(), inputs)

	// Should return error when all attempts fail
	if err == nil {
		t.Error("Expected error when all attempts fail, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result when all fail, got %v", result)
	}
	if !strings.Contains(err.Error(), "all 3 attempts failed") {
		t.Errorf("Error should mention all attempts failed, got: %v", err)
	}
}

// TestBestOfN_AllAttemptsFail_HTTP tests HTTP errors
func TestBestOfN_AllAttemptsFail_HTTP(t *testing.T) {
	sig := core.NewSignature("Test")

	mockModule := &MockFailingModule{
		sig:         sig,
		failureType: "http",
	}

	scorer := func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		return 1.0, nil
	}

	bon := NewBestOfN(mockModule, 5).
		WithScorer(scorer).
		WithMaxFailures(5)
	inputs := map[string]any{"question": "test"}

	result, err := bon.Forward(context.Background(), inputs)

	// Should return error when all attempts fail
	if err == nil {
		t.Error("Expected error when all attempts fail (HTTP), got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result when all fail (HTTP), got %v", result)
	}
	if !strings.Contains(err.Error(), "all 5 attempts failed") {
		t.Errorf("Error should mention all attempts failed, got: %v", err)
	}
}

// TestBestOfN_PartialFailures tests when some attempts fail but not all
func TestBestOfN_PartialFailures(t *testing.T) {
	sig := core.NewSignature("Test")

	attemptCount := 0
	mockModule := &MockModule{
		SignatureValue: sig,
		ForwardFunc: func(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
			attemptCount++
			// Fail first 2 attempts, succeed on 3rd
			if attemptCount <= 2 {
				return nil, errors.New("parse failed")
			}
			return &core.Prediction{
				Outputs: map[string]any{"answer": "success"},
			}, nil
		},
	}

	scorer := func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		return 1.0, nil
	}

	bon := NewBestOfN(mockModule, 5).WithScorer(scorer)
	inputs := map[string]any{"question": "test"}

	result, err := bon.Forward(context.Background(), inputs)

	// Should succeed with the successful attempt
	if err != nil {
		t.Fatalf("Expected success with partial failures, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Outputs["answer"] != "success" {
		t.Errorf("Expected answer='success', got %v", result.Outputs["answer"])
	}
}

// TestBestOfN_ExceedMaxFailures tests exceeding max failures threshold
func TestBestOfN_ExceedMaxFailures(t *testing.T) {
	sig := core.NewSignature("Test")

	mockModule := &MockFailingModule{
		sig:         sig,
		failureType: "parse",
	}

	scorer := func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		return 1.0, nil
	}

	// Set max failures to 1 (should fail after 2 failures)
	bon := NewBestOfN(mockModule, 10).
		WithScorer(scorer).
		WithMaxFailures(1)
	inputs := map[string]any{"question": "test"}

	result, err := bon.Forward(context.Background(), inputs)

	// Should return error when max failures exceeded
	if err == nil {
		t.Error("Expected error when max failures exceeded, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
	if !strings.Contains(err.Error(), "exceeded maximum failures") {
		t.Errorf("Error should mention exceeded max failures, got: %v", err)
	}
}

// TestBestOfN_ScorerFails tests when scorer function fails
func TestBestOfN_ScorerFails(t *testing.T) {
	sig := core.NewSignature("Test")

	mockModule := &MockModule{
		SignatureValue: sig,
		ForwardFunc: func(ctx context.Context, inputs map[string]any) (*core.Prediction, error) {
			return &core.Prediction{
				Outputs: map[string]any{"answer": "test"},
			}, nil
		},
	}

	// Scorer that always fails
	scorer := func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		return 0, errors.New("scoring failed: missing confidence field")
	}

	bon := NewBestOfN(mockModule, 3).WithScorer(scorer)
	inputs := map[string]any{"question": "test"}

	result, err := bon.Forward(context.Background(), inputs)

	// Should return error when scoring fails
	if err == nil {
		t.Error("Expected error when scorer fails, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result when scorer fails, got %v", result)
	}
	if !strings.Contains(err.Error(), "scoring failed") {
		t.Errorf("Error should mention scoring failure, got: %v", err)
	}
}

// TestBestOfN_Parallel_AllAttemptsFail tests parallel execution with all failures
func TestBestOfN_Parallel_AllAttemptsFail(t *testing.T) {
	sig := core.NewSignature("Test")

	mockModule := &MockFailingModule{
		sig:         sig,
		failureType: "parse",
	}

	scorer := func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
		return 1.0, nil
	}

	bon := NewBestOfN(mockModule, 3).
		WithScorer(scorer).
		WithParallel(true).
		WithMaxFailures(3)
	inputs := map[string]any{"question": "test"}

	result, err := bon.Forward(context.Background(), inputs)

	// Should return error when all attempts fail (parallel)
	if err == nil {
		t.Error("Expected error when all attempts fail (parallel), got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result when all fail (parallel), got %v", result)
	}
	if !strings.Contains(err.Error(), "all 3 attempts failed") {
		t.Errorf("Error should mention all attempts failed, got: %v", err)
	}
}
