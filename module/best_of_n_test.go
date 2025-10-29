package module

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/assagman/dsgo"
)

type MockModule struct {
	ForwardFunc    func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error)
	SignatureValue *dsgo.Signature
	CallCount      int
	mu             sync.Mutex
}

func (m *MockModule) Forward(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
	m.mu.Lock()
	m.CallCount++
	m.mu.Unlock()
	if m.ForwardFunc != nil {
		return m.ForwardFunc(ctx, inputs)
	}
	return dsgo.NewPrediction(map[string]interface{}{"result": "test"}), nil
}

func (m *MockModule) GetSignature() *dsgo.Signature {
	if m.SignatureValue != nil {
		return m.SignatureValue
	}
	return dsgo.NewSignature("Mock")
}

func TestBestOfN_Forward_Success(t *testing.T) {
	callCount := 0
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			callCount++
			return dsgo.NewPrediction(map[string]interface{}{"answer": callCount}), nil
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return float64(prediction.Outputs["answer"].(int)), nil
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"].(int) != 3 {
		t.Errorf("Expected best answer=3, got %v", outputs.Outputs["answer"])
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestBestOfN_Forward_NoScorer(t *testing.T) {
	module := &MockModule{}
	bon := NewBestOfN(module, 3)

	_, err := bon.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error when scorer is not set")
	}
}

func TestBestOfN_Forward_InvalidN(t *testing.T) {
	module := &MockModule{}
	bon := NewBestOfN(module, 0).WithScorer(DefaultScorer())

	_, err := bon.Forward(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Forward() should error when N <= 0")
	}
}

func TestBestOfN_Forward_ModuleError(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			return nil, errors.New("module error")
		},
	}

	bon := NewBestOfN(module, 3).WithScorer(DefaultScorer())
	_, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err == nil {
		t.Error("Forward() should error when all attempts fail")
	}
}

func TestBestOfN_Forward_PartialFailures(t *testing.T) {
	callCount := 0
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			callCount++
			if callCount <= 2 {
				return nil, errors.New("temporary error")
			}
			return dsgo.NewPrediction(map[string]interface{}{"answer": "success"}), nil
		},
	}

	bon := NewBestOfN(module, 5).WithScorer(DefaultScorer()).WithMaxFailures(3)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["answer"] != "success" {
		t.Error("Should succeed with partial failures below max")
	}
}

func TestBestOfN_Forward_ExceedMaxFailures(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			return nil, errors.New("always fail")
		},
	}

	bon := NewBestOfN(module, 5).WithScorer(DefaultScorer()).WithMaxFailures(2)
	_, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err == nil {
		t.Error("Forward() should error when max failures exceeded")
	}
}

func TestBestOfN_Forward_ScorerError(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			return dsgo.NewPrediction(map[string]interface{}{"answer": "test"}), nil
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return 0, errors.New("scorer error")
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer)
	_, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err == nil {
		t.Error("Forward() should error when scorer fails")
	}
}

func TestBestOfN_Forward_ReturnAll(t *testing.T) {
	callCount := 0
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			callCount++
			return dsgo.NewPrediction(map[string]interface{}{"score": callCount}), nil
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return float64(prediction.Outputs["score"].(int)), nil
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer).WithReturnAll(true)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Score <= 0 {
		t.Error("ReturnAll should include score")
	}

	if len(outputs.Completions) != 3 {
		t.Errorf("ReturnAll should include all completions, got %d", len(outputs.Completions))
	}
}

func TestBestOfN_Forward_Parallel(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			return dsgo.NewPrediction(map[string]interface{}{"result": "test"}), nil
		},
	}

	bon := NewBestOfN(module, 5).WithScorer(DefaultScorer()).WithParallel(true)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Outputs["result"] != "test" {
		t.Error("Parallel execution should produce valid output")
	}
}

func TestBestOfN_Forward_ParallelWithFailures(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			mu.Lock()
			callCount++
			count := callCount
			mu.Unlock()

			if count <= 2 {
				return nil, errors.New("simulated failure")
			}
			return dsgo.NewPrediction(map[string]interface{}{"value": count}), nil
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return float64(prediction.Outputs["value"].(int)), nil
	}

	bon := NewBestOfN(module, 5).WithScorer(scorer).WithParallel(true).WithMaxFailures(2)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs == nil {
		t.Error("Expected valid output despite failures")
	}
}

func TestBestOfN_Forward_ParallelExceedMaxFailures(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			return nil, errors.New("always fail")
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return 1.0, nil
	}

	bon := NewBestOfN(module, 5).WithScorer(scorer).WithParallel(true).WithMaxFailures(2)
	_, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err == nil {
		t.Error("Expected error when exceeding max failures")
	}
}

func TestBestOfN_Forward_ParallelScorerError(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			return dsgo.NewPrediction(map[string]interface{}{"value": "test"}), nil
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return 0, errors.New("scorer error")
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer).WithParallel(true)
	_, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err == nil {
		t.Error("Expected error when all parallel scorers fail")
	}
}

func TestBestOfN_Forward_ParallelReturnAll(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			mu.Lock()
			callCount++
			count := callCount
			mu.Unlock()
			return dsgo.NewPrediction(map[string]interface{}{"value": count}), nil
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return float64(prediction.Outputs["value"].(int)), nil
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer).WithParallel(true).WithReturnAll(true)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs.Score <= 0 {
		t.Error("ReturnAll should include score")
	}

	if len(outputs.Completions) != 3 {
		t.Errorf("Expected 3 completions, got %d", len(outputs.Completions))
	}
}

func TestBestOfN_GetSignature(t *testing.T) {
	sig := dsgo.NewSignature("Test")
	module := &MockModule{SignatureValue: sig}
	bon := NewBestOfN(module, 3)

	if bon.GetSignature() != sig {
		t.Error("GetSignature should return module's signature")
	}
}

func TestDefaultScorer(t *testing.T) {
	scorer := DefaultScorer()

	pred := dsgo.NewPrediction(map[string]interface{}{
		"answer": "short",
	})

	score, err := scorer(nil, pred)

	if err != nil {
		t.Fatalf("DefaultScorer error = %v", err)
	}

	if score <= 0 {
		t.Error("DefaultScorer should return positive score for non-empty output")
	}
}

func TestBestOfN_WithThreshold_EarlyStop(t *testing.T) {
	callCount := 0
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (*dsgo.Prediction, error) {
			callCount++
			return dsgo.NewPrediction(map[string]interface{}{"score": callCount * 10}), nil
		},
	}

	scorer := func(inputs map[string]interface{}, prediction *dsgo.Prediction) (float64, error) {
		return float64(prediction.Outputs["score"].(int)), nil
	}

	bon := NewBestOfN(module, 10).WithScorer(scorer).WithThreshold(25.0)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should stop early at score 30 (third call) because it exceeds threshold of 25
	if callCount > 3 {
		t.Errorf("expected early stop at 3 calls, got %d", callCount)
	}

	if outputs.Outputs["score"].(int) < 25 {
		t.Errorf("expected score >= 25, got %v", outputs.Outputs["score"])
	}
}

func TestConfidenceScorer(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string]interface{}
		wantErr bool
		want    float64
	}{
		{
			name:    "float64 confidence",
			outputs: map[string]interface{}{"confidence": 0.95},
			wantErr: false,
			want:    0.95,
		},
		{
			name:    "int confidence",
			outputs: map[string]interface{}{"confidence": 5},
			wantErr: false,
			want:    5.0,
		},
		{
			name:    "string confidence",
			outputs: map[string]interface{}{"confidence": "0.85"},
			wantErr: false,
			want:    0.85,
		},
		{
			name:    "missing field",
			outputs: map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "invalid type",
			outputs: map[string]interface{}{"confidence": []int{1, 2}},
			wantErr: true,
		},
		{
			name:    "invalid string",
			outputs: map[string]interface{}{"confidence": "not-a-number"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := ConfidenceScorer("confidence")
			pred := dsgo.NewPrediction(tt.outputs)
			score, err := scorer(nil, pred)

			if (err != nil) != tt.wantErr {
				t.Errorf("ConfidenceScorer() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && score != tt.want {
				t.Errorf("ConfidenceScorer() = %v, want %v", score, tt.want)
			}
		})
	}
}
