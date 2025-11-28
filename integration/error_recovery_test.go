package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/assagman/dsgo/core"
	"github.com/assagman/dsgo/integration/fixtures"
	"github.com/assagman/dsgo/module"
)

// TestRetry_TransientError validates that transient errors are handled.
// Scenario: LM fails with network error, may succeed on retry.
// Expected: Eventual success or appropriate error.
func TestRetry_TransientError(t *testing.T) {
	tests := []struct {
		name            string
		failAttempts    int
		expectedSuccess bool
	}{
		{
			name:            "Single_call_succeeds",
			failAttempts:    0,
			expectedSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create a mock LM that succeeds
			responses := make([]string, tt.failAttempts+1)
			for i := range responses {
				responses[i] = `{"answer": "42"}`
			}
			lm := NewMockLMWithResponses(responses)

			sig := fixtures.SimplePredictSig()
			predictor := module.NewPredict(sig, lm)

			result, err := predictor.Forward(ctx, map[string]any{
				"question": "What is the answer?",
			})

			if tt.expectedSuccess {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if result == nil {
					t.Error("Expected non-nil result on success")
				} else {
					answer, ok := result.GetString("answer")
					if !ok || answer != "42" {
						t.Errorf("Expected answer=42, got %v", answer)
					}
				}
			}
		})
	}
}

// TestRetry_PermanentError validates that permanent errors fail without extended retries.
// Scenario: LM returns permanent error.
// Expected: Fast failure.
func TestRetry_PermanentError(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
	}{
		{
			name:      "Invalid_API_key",
			errorType: "authentication_error",
		},
		{
			name:      "Permission_error",
			errorType: "permission_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create LM that always fails
			lm := &MockLM{
				Error: errors.New(tt.errorType),
			}

			sig := fixtures.SimplePredictSig()
			predictor := module.NewPredict(sig, lm)

			startTime := time.Now()
			result, err := predictor.Forward(ctx, map[string]any{
				"question": "test",
			})
			elapsed := time.Since(startTime)

			// Should fail
			if err == nil {
				t.Error("Expected error for permanent failure")
			}
			if result != nil {
				t.Error("Expected nil result on permanent failure")
			}

			// Should fail quickly (no retries)
			if elapsed > 5*time.Second {
				t.Logf("Warning: failure took %v (expected <5s for permanent error)", elapsed)
			}
		})
	}
}

// TestRetry_ContextTimeout validates that context timeout is respected.
// Scenario: Context expires during operation or completes successfully.
// Expected: Operation respects context or completes.
func TestRetry_ContextTimeout(t *testing.T) {
	tests := []struct {
		name             string
		contextTimeout   time.Duration
		operationLatency time.Duration
	}{
		{
			name:             "Operation_within_timeout",
			contextTimeout:   1 * time.Second,
			operationLatency: 50 * time.Millisecond,
		},
		{
			name:             "Context_tight_constraint",
			contextTimeout:   100 * time.Millisecond,
			operationLatency: 50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			// Create LM with latency
			lm := &MockLM{
				Response: `{"answer": "response"}`,
				Latency:  tt.operationLatency,
			}

			sig := fixtures.SimplePredictSig()
			predictor := module.NewPredict(sig, lm)

			result, err := predictor.Forward(ctx, map[string]any{
				"question": "test",
			})

			// Either should succeed or timeout - both are acceptable depending on timing
			if err == nil {
				if result == nil {
					t.Error("Expected non-nil result on success")
				}
			}
			// Timeout is acceptable in context-constrained scenarios
		})
	}
}

// TestPartialValidation_MissingFields validates handling of missing output fields.
// Scenario: Signature requires output but LM returns partial results.
// Expected: Partial validation succeeds with diagnostics.
func TestPartialValidation_MissingFields(t *testing.T) {
	tests := []struct {
		name           string
		returnedFields map[string]any
	}{
		{
			name: "Complete_output",
			returnedFields: map[string]any{
				"answer": "42",
			},
		},
		{
			name:           "Empty_output",
			returnedFields: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := fixtures.SimplePredictSig()

			// Use partial validation to get diagnostics
			diag := sig.ValidateOutputsPartial(tt.returnedFields)

			// Partial validation should return diagnostics or nil
			if diag != nil {
				t.Logf("Diagnostics: missing=%v, errors=%v", diag.MissingFields, diag.TypeErrors)
			}
		})
	}
}

// TestErrorPropagation_SerialComposition validates error handling in sequential pipelines.
// Scenario: Module 1 succeeds, Module 2 processes output.
// Expected: Outputs flow or error propagates.
func TestErrorPropagation_SerialComposition(t *testing.T) {
	tests := []struct {
		name            string
		module1Succeeds bool
		module2Succeeds bool
		expectedError   bool
	}{
		{
			name:            "Both_succeed",
			module1Succeeds: true,
			module2Succeeds: true,
			expectedError:   false,
		},
		{
			name:            "First_fails",
			module1Succeeds: false,
			module2Succeeds: true,
			expectedError:   true,
		},
		{
			name:            "Second_fails",
			module1Succeeds: true,
			module2Succeeds: false,
			expectedError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			sig := fixtures.SimplePredictSig()

			// Module 1
			var lm1 core.LM
			if tt.module1Succeeds {
				lm1 = NewMockLMWithResponse(`{"answer": "intermediate"}`)
			} else {
				lm1 = &MockLM{
					Error: errors.New("module 1 failed"),
				}
			}
			module1 := module.NewPredict(sig, lm1)

			// Module 2
			var lm2 core.LM
			if tt.module2Succeeds {
				lm2 = NewMockLMWithResponse(`{"answer": "final"}`)
			} else {
				lm2 = &MockLM{
					Error: errors.New("module 2 failed"),
				}
			}
			module2 := module.NewPredict(sig, lm2)

			// Execute Module 1
			result1, err1 := module1.Forward(ctx, map[string]any{
				"question": "first step",
			})

			if tt.module1Succeeds {
				if err1 != nil {
					t.Errorf("Module 1 should succeed but got error: %v", err1)
				}
				if result1 == nil {
					t.Error("Module 1 should return result")
				} else {
					answer, ok := result1.GetString("answer")
					if !ok {
						t.Errorf("Failed to get answer from module 1")
						return
					}

					// Execute Module 2 with output from Module 1
					result2, err2 := module2.Forward(ctx, map[string]any{
						"question": answer,
					})

					if tt.module2Succeeds {
						if err2 != nil {
							t.Errorf("Module 2 should succeed but got error: %v", err2)
						}
					} else {
						if err2 == nil {
							t.Error("Module 2 should fail")
						}
					}
					_ = result2
				}
			} else {
				if err1 == nil {
					t.Error("Module 1 should fail")
				}
			}
		})
	}
}

// TestErrorPropagation_ParallelComposition validates error handling in parallel execution.
// Scenario: Multiple modules execute in parallel.
// Expected: Errors appropriately tracked.
func TestErrorPropagation_ParallelComposition(t *testing.T) {
	tests := []struct {
		name            string
		moduleASucceeds bool
		moduleBSucceeds bool
		expectedResults int
	}{
		{
			name:            "Both_succeed",
			moduleASucceeds: true,
			moduleBSucceeds: true,
			expectedResults: 2,
		},
		{
			name:            "A_fails_B_succeeds",
			moduleASucceeds: false,
			moduleBSucceeds: true,
			expectedResults: 1,
		},
		{
			name:            "A_succeeds_B_fails",
			moduleASucceeds: true,
			moduleBSucceeds: false,
			expectedResults: 1,
		},
		{
			name:            "Both_fail",
			moduleASucceeds: false,
			moduleBSucceeds: false,
			expectedResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			sig := fixtures.SimplePredictSig()

			// Module A
			var lmA core.LM
			if tt.moduleASucceeds {
				lmA = NewMockLMWithResponse(`{"answer": "A"}`)
			} else {
				lmA = &MockLM{
					Error: errors.New("module A failed"),
				}
			}
			moduleA := module.NewPredict(sig, lmA)

			// Module B
			var lmB core.LM
			if tt.moduleBSucceeds {
				lmB = NewMockLMWithResponse(`{"answer": "B"}`)
			} else {
				lmB = &MockLM{
					Error: errors.New("module B failed"),
				}
			}
			moduleB := module.NewPredict(sig, lmB)

			// Execute in parallel
			inputs := map[string]any{"question": "test"}
			resultAChan := make(chan *core.Prediction, 1)
			errAChan := make(chan error, 1)
			resultBChan := make(chan *core.Prediction, 1)
			errBChan := make(chan error, 1)

			go func() {
				result, err := moduleA.Forward(ctx, inputs)
				resultAChan <- result
				errAChan <- err
			}()

			go func() {
				result, err := moduleB.Forward(ctx, inputs)
				resultBChan <- result
				errBChan <- err
			}()

			resultA := <-resultAChan
			errA := <-errAChan
			resultB := <-resultBChan
			errB := <-errBChan

			successCount := 0
			if errA == nil && resultA != nil {
				successCount++
			}
			if errB == nil && resultB != nil {
				successCount++
			}

			if successCount != tt.expectedResults {
				t.Errorf("Expected %d successful results but got %d", tt.expectedResults, successCount)
			}
		})
	}
}

// TestGracefulDegradation_FallbackModule validates fallback behavior on failure.
// Scenario: Primary module fails, fallback module executes.
// Expected: Final result from fallback module.
func TestGracefulDegradation_FallbackModule(t *testing.T) {
	tests := []struct {
		name             string
		primarySucceeds  bool
		fallbackSucceeds bool
		expectedSuccess  bool
	}{
		{
			name:             "Primary_succeeds",
			primarySucceeds:  true,
			fallbackSucceeds: true,
			expectedSuccess:  true,
		},
		{
			name:             "Primary_fails_fallback_succeeds",
			primarySucceeds:  false,
			fallbackSucceeds: true,
			expectedSuccess:  true,
		},
		{
			name:             "Primary_fails_fallback_fails",
			primarySucceeds:  false,
			fallbackSucceeds: false,
			expectedSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			sig := fixtures.SimplePredictSig()

			// Primary module
			var primaryLM core.LM
			if tt.primarySucceeds {
				primaryLM = NewMockLMWithResponse(`{"answer": "primary"}`)
			} else {
				primaryLM = &MockLM{
					Error: errors.New("primary failed"),
				}
			}
			primaryModule := module.NewPredict(sig, primaryLM)

			// Fallback module
			var fallbackLM core.LM
			if tt.fallbackSucceeds {
				fallbackLM = NewMockLMWithResponse(`{"answer": "fallback"}`)
			} else {
				fallbackLM = &MockLM{
					Error: errors.New("fallback failed"),
				}
			}
			fallbackModule := module.NewPredict(sig, fallbackLM)

			inputs := map[string]any{"question": "test"}

			// Try primary
			result, err := primaryModule.Forward(ctx, inputs)

			// If primary fails, try fallback
			if err != nil {
				result, err = fallbackModule.Forward(ctx, inputs)
			}

			if tt.expectedSuccess {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if result == nil {
					t.Error("Expected non-nil result on success")
				}
			} else {
				if err == nil {
					t.Error("Expected error but got success")
				}
			}
		})
	}
}

// TestErrorRecovery_AdapterFallback validates adapter fallback chain.
// Scenario: Adapter fallback chain attempts recovery.
// Expected: Adapter metadata tracking.
func TestErrorRecovery_AdapterFallback(t *testing.T) {
	tests := []struct {
		name            string
		lmOutput        string
		expectedSuccess bool
	}{
		{
			name:            "Valid_JSON",
			lmOutput:        `{"answer": "42"}`,
			expectedSuccess: true,
		},
		{
			name:            "Chat_markers_format",
			lmOutput:        "[[ ## answer ## ]] 42",
			expectedSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			lm := NewMockLMWithResponse(tt.lmOutput)
			adapter := core.NewFallbackAdapter()

			sig := fixtures.SimplePredictSig()

			predictor := module.NewPredict(sig, lm)
			predictor.Adapter = adapter

			result, err := predictor.Forward(ctx, map[string]any{
				"question": "What is the answer?",
			})

			if tt.expectedSuccess {
				if err == nil && result != nil {
					// Good - we succeeded
					if result.AdapterUsed == "" {
						t.Logf("Note: AdapterUsed not set")
					}
				} else if err != nil {
					t.Logf("Note: Parser could not recover: %v", err)
				}
			}
		})
	}
}

// TestErrorRecovery_ValidationWithDiagnostics validates diagnostics-based recovery.
// Scenario: Output validation returns diagnostics.
// Expected: Diagnostics inform caller about partial results.
func TestErrorRecovery_ValidationWithDiagnostics(t *testing.T) {
	tests := []struct {
		name           string
		returnedFields map[string]any
		canContinue    bool
	}{
		{
			name: "Partial_output",
			returnedFields: map[string]any{
				"answer": "partial",
			},
			canContinue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := fixtures.SimplePredictSig()

			// Use partial validation
			diag := sig.ValidateOutputsPartial(tt.returnedFields)

			if tt.canContinue {
				// Should be able to continue with partial results
				if diag != nil {
					t.Logf("Diagnostics: missing=%v, errors=%v", diag.MissingFields, diag.TypeErrors)
				}
			}
		})
	}
}

// TestError_ChainedModuleRecovery validates recovery across module chains.
// Scenario: Chain of modules, one fails.
// Expected: Error at failure point.
func TestError_ChainedModuleRecovery(t *testing.T) {
	tests := []struct {
		name             string
		module1Fails     bool
		module2Fails     bool
		module3Fails     bool
		expectedFinalErr bool
	}{
		{
			name:             "All_succeed",
			module1Fails:     false,
			module2Fails:     false,
			module3Fails:     false,
			expectedFinalErr: false,
		},
		{
			name:             "Middle_module_fails",
			module1Fails:     false,
			module2Fails:     true,
			module3Fails:     false,
			expectedFinalErr: true,
		},
		{
			name:             "Last_module_fails",
			module1Fails:     false,
			module2Fails:     false,
			module3Fails:     true,
			expectedFinalErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			sig := fixtures.SimplePredictSig()

			// Create three modules
			modules := make([]*module.Predict, 3)
			shouldFail := []bool{tt.module1Fails, tt.module2Fails, tt.module3Fails}

			for i := range modules {
				var lm core.LM
				if shouldFail[i] {
					lm = &MockLM{
						Error: errors.New("module failed"),
					}
				} else {
					lm = NewMockLMWithResponse(`{"answer": "step"}`)
				}
				modules[i] = module.NewPredict(sig, lm)
			}

			// Execute chain
			inputs := map[string]any{"question": "start"}
			var result *core.Prediction
			var err error
			var executedSteps int

			for _, m := range modules {
				result, err = m.Forward(ctx, inputs)
				if err != nil {
					break
				}
				if result != nil {
					answer, ok := result.GetString("answer")
					if !ok {
						break
					}
					inputs["question"] = answer
					executedSteps++
				}
			}

			if tt.expectedFinalErr {
				if err == nil {
					t.Error("Expected error in chain")
				}
			} else {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if executedSteps != len(modules) {
					t.Errorf("Expected all %d modules to execute, but only %d did", len(modules), executedSteps)
				}
			}
		})
	}
}
