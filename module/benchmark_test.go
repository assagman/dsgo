package module

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/assagman/dsgo/core"
)

// BenchmarkHistoryConcurrentAdd benchmarks concurrent History.Add operations
func BenchmarkHistoryConcurrentAdd(b *testing.B) {
	history := core.NewHistory()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			history.AddUserMessage(fmt.Sprintf("msg-%d", i))
			i++
		}
	})
}

// BenchmarkHistorySequential benchmarks sequential History.Add operations
func BenchmarkHistorySequential(b *testing.B) {
	history := core.NewHistory()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history.AddUserMessage(fmt.Sprintf("msg-%d", i))
	}
}

// BenchmarkStreamingBufferConcurrentWrite benchmarks concurrent StreamingBuffer.Write operations
func BenchmarkStreamingBufferConcurrentWrite(b *testing.B) {
	buffer := core.NewStreamingBuffer()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer.Write("test chunk")
		}
	})
}

// BenchmarkStreamingBufferSequentialWrite benchmarks sequential StreamingBuffer.Write operations
func BenchmarkStreamingBufferSequentialWrite(b *testing.B) {
	buffer := core.NewStreamingBuffer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Write("test chunk")
	}
}

// BenchmarkPredictSequential benchmarks sequential Predict operations
func BenchmarkPredictSequential(b *testing.B) {
	sig := core.NewSignature("BenchmarkTest").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]success",
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	ctx := context.Background()
	inputs := map[string]any{"text": "test input"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := predictor.Forward(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPredictParallel benchmarks parallel Predict operations
func BenchmarkPredictParallel(b *testing.B) {
	sig := core.NewSignature("BenchmarkTest").
		AddInput("text", core.FieldTypeString, "Input text").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]success",
				Usage:   core.Usage{TotalTokens: 10},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).WithMaxWorkers(runtime.NumCPU())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]map[string]any, 10)
		for j := 0; j < 10; j++ {
			batch[j] = map[string]any{"text": fmt.Sprintf("test input %d", j)}
		}
		inputs := map[string]any{"_batch": batch}
		_, err := parallel.Forward(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBestOfNSequential benchmarks sequential BestOfN operations
func BenchmarkBestOfNSequential(b *testing.B) {
	sig := core.NewSignature("BenchmarkTest").
		AddInput("prompt", core.FieldTypeString, "Prompt").
		AddOutput("response", core.FieldTypeString, "Response")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## response ## ]]response",
				Usage:   core.Usage{TotalTokens: 8},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	bestof := NewBestOfN(predictor, 3).
		WithScorer(func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
			return 0.8, nil
		}).
		WithParallel(false) // Sequential

	ctx := context.Background()
	inputs := map[string]any{"prompt": "test prompt"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := bestof.Forward(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBestOfNParallel benchmarks parallel BestOfN operations
func BenchmarkBestOfNParallel(b *testing.B) {
	sig := core.NewSignature("BenchmarkTest").
		AddInput("prompt", core.FieldTypeString, "Prompt").
		AddOutput("response", core.FieldTypeString, "Response")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## response ## ]]response",
				Usage:   core.Usage{TotalTokens: 8},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	bestof := NewBestOfN(predictor, 3).
		WithScorer(func(inputs map[string]any, prediction *core.Prediction) (float64, error) {
			return 0.8, nil
		}).
		WithParallel(true) // Parallel

	ctx := context.Background()
	inputs := map[string]any{"prompt": "test prompt"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := bestof.Forward(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelModule benchmarks Parallel module performance
func BenchmarkParallelModule(b *testing.B) {
	sig := core.NewSignature("BenchmarkTest").
		AddInput("value", core.FieldTypeInt, "Input value").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]processed",
				Usage:   core.Usage{TotalTokens: 5},
			}, nil
		},
	}

	predictor := NewPredict(sig, lm)
	parallel := NewParallel(predictor).WithMaxWorkers(runtime.NumCPU())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]map[string]any, 100)
		for j := 0; j < 100; j++ {
			batch[j] = map[string]any{"value": j}
		}
		inputs := map[string]any{"_batch": batch}
		_, err := parallel.Forward(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelModuleFactory benchmarks Parallel module with factory pattern
func BenchmarkParallelModuleFactory(b *testing.B) {
	sig := core.NewSignature("BenchmarkTest").
		AddInput("value", core.FieldTypeInt, "Input value").
		AddOutput("result", core.FieldTypeString, "Result")

	factory := func(i int) core.Module {
		lm := &MockLM{
			GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
				return &core.GenerateResult{
					Content: "[[ ## result ## ]]processed",
					Usage:   core.Usage{TotalTokens: 5},
				}, nil
			},
		}
		return NewPredict(sig, lm)
	}

	parallel := NewParallelWithFactory(factory).WithMaxWorkers(runtime.NumCPU())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]map[string]any, 100)
		for j := 0; j < 100; j++ {
			batch[j] = map[string]any{"value": j}
		}
		inputs := map[string]any{"_batch": batch}
		_, err := parallel.Forward(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkThreadSafetyOverhead measures the overhead of thread safety mechanisms
func BenchmarkThreadSafetyOverhead(b *testing.B) {
	sig := core.NewSignature("OverheadTest").
		AddInput("text", core.FieldTypeString, "Input").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]result",
				Usage:   core.Usage{TotalTokens: 5},
			}, nil
		},
	}

	// Test with History (thread-safe)
	history := core.NewHistory()
	predictorWithHistory := NewPredict(sig, lm).WithHistory(history)

	// Test without History (minimal thread safety)
	predictorWithoutHistory := NewPredict(sig, lm)

	ctx := context.Background()
	inputs := map[string]any{"text": "test"}

	b.Run("WithHistory", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := predictorWithHistory.Forward(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithoutHistory", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := predictorWithoutHistory.Forward(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConcurrentAccess measures performance under concurrent access
func BenchmarkConcurrentAccess(b *testing.B) {
	sig := core.NewSignature("ConcurrentTest").
		AddInput("id", core.FieldTypeInt, "ID").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]ok",
				Usage:   core.Usage{TotalTokens: 3},
			}, nil
		},
	}

	history := core.NewHistory()
	predictor := NewPredict(sig, lm).WithHistory(history)
	parallel := NewParallel(predictor).WithMaxWorkers(runtime.NumCPU() * 2)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			batch := make([]map[string]any, 10)
			for j := 0; j < 10; j++ {
				batch[j] = map[string]any{"id": i*10 + j}
			}
			inputs := map[string]any{"_batch": batch}
			_, err := parallel.Forward(ctx, inputs)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	sig := core.NewSignature("MemoryTest").
		AddInput("data", core.FieldTypeString, "Input data").
		AddOutput("result", core.FieldTypeString, "Result")

	lm := &MockLM{
		GenerateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content: "[[ ## result ## ]]processed data",
				Usage:   core.Usage{TotalTokens: 8},
			}, nil
		},
	}

	history := core.NewHistory()
	predictor := NewPredict(sig, lm).WithHistory(history)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inputs := map[string]any{"data": fmt.Sprintf("test data %d", i)}
		_, err := predictor.Forward(ctx, inputs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
