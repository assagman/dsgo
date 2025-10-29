package module

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/assagman/dsgo"
)

type MockModule struct {
	ForwardFunc    func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error)
	SignatureValue *dsgo.Signature
	CallCount      int
	mu             sync.Mutex
}

func (m *MockModule) Forward(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	m.mu.Lock()
	m.CallCount++
	m.mu.Unlock()
	if m.ForwardFunc != nil {
		return m.ForwardFunc(ctx, inputs)
	}
	return map[string]interface{}{"result": "test"}, nil
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			callCount++
			return map[string]interface{}{"answer": callCount}, nil
		},
	}

	scorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
		return float64(outputs["answer"].(int)), nil
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs["answer"].(int) != 3 {
		t.Errorf("Expected best answer=3, got %v", outputs["answer"])
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			callCount++
			if callCount <= 2 {
				return nil, errors.New("temporary error")
			}
			return map[string]interface{}{"answer": "success"}, nil
		},
	}

	bon := NewBestOfN(module, 5).WithScorer(DefaultScorer()).WithMaxFailures(3)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs["answer"] != "success" {
		t.Error("Should succeed with partial failures below max")
	}
}

func TestBestOfN_Forward_ExceedMaxFailures(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"answer": "test"}, nil
		},
	}

	scorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			callCount++
			return map[string]interface{}{"score": callCount}, nil
		},
	}

	scorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
		return float64(outputs["score"].(int)), nil
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer).WithReturnAll(true)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if _, exists := outputs["_best_of_n_score"]; !exists {
		t.Error("ReturnAll should include score metadata")
	}

	if _, exists := outputs["_best_of_n_all_scores"]; !exists {
		t.Error("ReturnAll should include all scores metadata")
	}
}

func TestBestOfN_Forward_Parallel(t *testing.T) {
	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "test"}, nil
		},
	}

	bon := NewBestOfN(module, 5).WithScorer(DefaultScorer()).WithParallel(true)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if outputs["result"] != "test" {
		t.Error("Parallel execution should produce valid output")
	}
}

func TestBestOfN_Forward_ParallelWithFailures(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	module := &MockModule{
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			mu.Lock()
			callCount++
			count := callCount
			mu.Unlock()

			if count <= 2 {
				return nil, errors.New("simulated failure")
			}
			return map[string]interface{}{"value": count}, nil
		},
	}

	scorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
		return float64(outputs["value"].(int)), nil
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			return nil, errors.New("always fail")
		},
	}

	scorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{"value": "test"}, nil
		},
	}

	scorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
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
		ForwardFunc: func(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
			mu.Lock()
			callCount++
			count := callCount
			mu.Unlock()
			return map[string]interface{}{"value": count}, nil
		},
	}

	scorer := func(inputs map[string]interface{}, outputs map[string]interface{}) (float64, error) {
		return float64(outputs["value"].(int)), nil
	}

	bon := NewBestOfN(module, 3).WithScorer(scorer).WithParallel(true).WithReturnAll(true)
	outputs, err := bon.Forward(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if _, exists := outputs["_best_of_n_score"]; !exists {
		t.Error("ReturnAll should include score metadata")
	}

	if allScores, exists := outputs["_best_of_n_all_scores"]; !exists {
		t.Error("ReturnAll should include all scores metadata")
	} else if len(allScores.([]float64)) != 3 {
		t.Errorf("Expected 3 scores, got %d", len(allScores.([]float64)))
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

	score, err := scorer(nil, map[string]interface{}{
		"answer": "short",
	})

	if err != nil {
		t.Fatalf("DefaultScorer error = %v", err)
	}

	if score <= 0 {
		t.Error("DefaultScorer should return positive score for non-empty output")
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
			score, err := scorer(nil, tt.outputs)

			if (err != nil) != tt.wantErr {
				t.Errorf("ConfidenceScorer() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && score != tt.want {
				t.Errorf("ConfidenceScorer() = %v, want %v", score, tt.want)
			}
		})
	}
}
