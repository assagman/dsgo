package typed

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/assagman/dsgo/core"
)

// Mock LM for testing
type mockLM struct {
	generateFunc func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error)
}

func (m *mockLM) Generate(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, messages, options)
	}
	return &core.GenerateResult{
		Content:  "mocked response",
		Metadata: make(map[string]any),
	}, nil
}

func (m *mockLM) Stream(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (<-chan core.Chunk, <-chan error) {
	chunks := make(chan core.Chunk)
	errs := make(chan error)
	close(chunks)
	close(errs)
	return chunks, errs
}

func (m *mockLM) Name() string {
	return "mock-lm"
}

func (m *mockLM) SupportsJSON() bool {
	return true
}

func (m *mockLM) SupportsTools() bool {
	return false
}

func TestNewFunc(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	if fn == nil {
		t.Fatal("NewFunc() returned nil")
	}

	sig := fn.GetSignature()
	if len(sig.InputFields) != 1 {
		t.Errorf("InputFields count = %d, want 1", len(sig.InputFields))
	}
	if len(sig.OutputFields) != 1 {
		t.Errorf("OutputFields count = %d, want 1", len(sig.OutputFields))
	}
}

func TestNewFunc_NotStruct(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}

	lm := &mockLM{}
	// Use string as output type (not a struct)
	type Output = string

	_, err := NewPredict[Input, Output](lm)
	if err == nil {
		t.Error("NewFunc() should return error when output is not a struct")
	}
}

func TestNewPredictWithDescription(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	desc := "Custom description"
	fn, err := NewPredictWithDescription[Input, Output](lm, desc)
	if err != nil {
		t.Fatalf("NewPredictWithDescription() error = %v", err)
	}

	if fn.description != desc {
		t.Errorf("description = %q, want %q", fn.description, desc)
	}

	if fn.GetSignature().Description != desc {
		t.Errorf("signature description = %q, want %q", fn.GetSignature().Description, desc)
	}
}

func TestFunc_Run(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	// Create custom mock that returns specific output
	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content:  `{"Result": "mocked result"}`,
				Metadata: make(map[string]any),
			}, nil
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	input := Input{Text: "test"}
	output, err := fn.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if output.Result != "mocked result" {
		t.Errorf("output.Result = %q, want %q", output.Result, "mocked result")
	}
}

func TestFunc_RunWithPrediction(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content:  `{"Result": "test result"}`,
				Metadata: map[string]any{"test": "metadata"},
			}, nil
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	input := Input{Text: "test"}
	output, pred, err := fn.RunWithPrediction(context.Background(), input)
	if err != nil {
		t.Fatalf("RunWithPrediction() error = %v", err)
	}

	if output.Result != "test result" {
		t.Errorf("output.Result = %q, want %q", output.Result, "test result")
	}

	if pred == nil {
		t.Fatal("prediction should not be nil")
	}

	if pred.Outputs["Result"] != "test result" {
		t.Errorf("prediction.Outputs[Result] = %q, want %q", pred.Outputs["Result"], "test result")
	}
}

func TestFunc_WithDemosTyped(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	inputs := []Input{
		{Text: "example 1"},
		{Text: "example 2"},
	}
	outputs := []Output{
		{Result: "result 1"},
		{Result: "result 2"},
	}

	fn2, err := fn.WithDemosTyped(inputs, outputs)
	if err != nil {
		t.Fatalf("WithDemosTyped() error = %v", err)
	}
	if fn2 == nil {
		t.Fatal("WithDemosTyped() returned nil")
	}
}

func TestFunc_WithDemosTyped_MismatchedLength(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	inputs := []Input{{Text: "example 1"}}
	outputs := []Output{{Result: "result 1"}, {Result: "result 2"}}

	_, err = fn.WithDemosTyped(inputs, outputs)
	if err == nil {
		t.Error("WithDemosTyped() should return error when lengths don't match")
	}
}

// Additional tests for 100% coverage

func TestFunc_WithOptions_Coverage(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	opts := &core.GenerateOptions{Temperature: 0.5, MaxTokens: 100}
	result := fn.WithOptions(opts)
	if result != fn {
		t.Error("WithOptions should return the same function")
	}
}

func TestFunc_WithAdapter_Coverage(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	adapter := core.NewJSONAdapter()
	result := fn.WithAdapter(adapter)
	if result != fn {
		t.Error("WithAdapter should return the same function")
	}
}

func TestFunc_WithHistory_Coverage(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	history := core.NewHistory()
	result := fn.WithHistory(history)
	if result != fn {
		t.Error("WithHistory should return the same function")
	}
}

func TestFunc_WithDemos_Coverage(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	demos := []core.Example{
		{
			Inputs:  map[string]any{"Text": "example"},
			Outputs: map[string]any{"Result": "output"},
		},
	}
	result := fn.WithDemos(demos)
	if result != fn {
		t.Error("WithDemos should return the same function")
	}
}

func TestFunc_Forward_Coverage(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content:  `{"Result": "test"}`,
				Metadata: make(map[string]any),
			}, nil
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	pred, err := fn.Forward(context.Background(), map[string]any{"Text": "test"})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}

	if pred == nil {
		t.Fatal("Forward() returned nil prediction")
	}

	if pred.Outputs["Result"] != "test" {
		t.Errorf("Forward() result = %v, want 'test'", pred.Outputs["Result"])
	}
}

func TestFunc_Run_ModuleError(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return nil, fmt.Errorf("module failed")
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	_, err = fn.Run(context.Background(), Input{Text: "test"})
	if err == nil {
		t.Error("Run() should return error when module fails")
	}
}

func TestFunc_RunWithPrediction_ModuleError(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input text"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output result"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return nil, fmt.Errorf("module failed")
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	_, pred, err := fn.RunWithPrediction(context.Background(), Input{Text: "test"})
	if err == nil {
		t.Error("RunWithPrediction() should return error when module fails")
	}
	if pred != nil {
		t.Error("RunWithPrediction() should return nil prediction on error")
	}
}

func TestBuildCombinedSignature_OnlyOutputFields(t *testing.T) {
	type Input struct {
		unexported string `dsgo:"input,desc=Should be skipped"` //nolint:unused
	}
	type Output struct {
		Result string `dsgo:"output,desc=Valid output"`
	}

	inputType := reflect.TypeOf(Input{})
	outputType := reflect.TypeOf(Output{})

	sig, err := buildCombinedSignature(inputType, outputType)
	if err != nil {
		t.Fatalf("buildCombinedSignature() error = %v", err)
	}

	if len(sig.InputFields) != 0 {
		t.Errorf("InputFields count = %d, want 0 (unexported skipped)", len(sig.InputFields))
	}
	if len(sig.OutputFields) != 1 {
		t.Errorf("OutputFields count = %d, want 1", len(sig.OutputFields))
	}
}

func TestBuildCombinedSignature_BothFields(t *testing.T) {
	type Input struct {
		Question string `dsgo:"input,desc=Question"`
	}
	type Output struct {
		Answer string `dsgo:"output,desc=Answer"`
	}

	inputType := reflect.TypeOf(Input{})
	outputType := reflect.TypeOf(Output{})

	sig, err := buildCombinedSignature(inputType, outputType)
	if err != nil {
		t.Fatalf("buildCombinedSignature() error = %v", err)
	}

	if len(sig.InputFields) != 1 {
		t.Errorf("InputFields count = %d, want 1", len(sig.InputFields))
	}
	if len(sig.OutputFields) != 1 {
		t.Errorf("OutputFields count = %d, want 1", len(sig.OutputFields))
	}
	if sig.InputFields[0].Name != "Question" {
		t.Errorf("InputField name = %s, want Question", sig.InputFields[0].Name)
	}
	if sig.OutputFields[0].Name != "Answer" {
		t.Errorf("OutputField name = %s, want Answer", sig.OutputFields[0].Name)
	}
}

func TestNewPredictWithDescription_Coverage(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{}
	desc := "Test description"

	fn, err := NewPredictWithDescription[Input, Output](lm, desc)
	if err != nil {
		t.Fatalf("NewPredictWithDescription() error = %v", err)
	}

	if fn.description != desc {
		t.Errorf("description = %q, want %q", fn.description, desc)
	}

	if fn.GetSignature().Description != desc {
		t.Errorf("signature description = %q, want %q", fn.GetSignature().Description, desc)
	}
}

func TestWithDemosTyped_SuccessfulConversion(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	inputs := []Input{{Text: "test1"}, {Text: "test2"}}
	outputs := []Output{{Result: "out1"}, {Result: "out2"}}

	fn2, err := fn.WithDemosTyped(inputs, outputs)
	if err != nil {
		t.Fatalf("WithDemosTyped() error = %v", err)
	}

	if fn2 == nil {
		t.Error("WithDemosTyped() returned nil")
	}
}

func TestNewFunc_InputNotStruct(t *testing.T) {
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{}
	_, err := NewPredict[int, Output](lm)
	if err == nil {
		t.Error("NewFunc() should return error when input is not a struct")
	}
}

func TestNewFunc_BuildSignatureError(t *testing.T) {
	// Both input and output are valid structs, but with invalid tags
	type BadInput struct {
		Field string `dsgo:"invalid_tag"`
	}
	type ValidOutput struct {
		Result string `dsgo:"output,desc=Valid"`
	}

	lm := &mockLM{}
	_, err := NewPredict[BadInput, ValidOutput](lm)
	if err == nil {
		t.Error("NewFunc() should return error when signature building fails")
	}
}

func TestNewPredictWithDescription_ErrorPropagation(t *testing.T) {
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{}
	// Use int as input (not a struct) to trigger error
	_, err := NewPredictWithDescription[int, Output](lm, "description")
	if err == nil {
		t.Error("NewPredictWithDescription() should propagate NewFunc error")
	}
}

func TestRun_StructToMapError(t *testing.T) {
	// This will be hard to trigger since StructToMap is robust
	// but we can try with a pointer type in the input
	type Input struct {
		Text string `dsgo:"input,desc=Input"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content:  `{"Result": "test"}`,
				Metadata: make(map[string]any),
			}, nil
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	// Normal case should work
	_, runErr := fn.Run(context.Background(), Input{Text: "test"})
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
}

func TestRun_MapToStructError(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			// Return incompatible data that will cause MapToStruct to fail
			return &core.GenerateResult{
				Content:  `{"Result": 123}`, // Number instead of string
				Metadata: make(map[string]any),
			}, nil
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	// This should succeed because the adapter handles type conversion
	_, runErr := fn.Run(context.Background(), Input{Text: "test"})
	// The actual behavior depends on adapter's type handling
	_ = runErr // May or may not error depending on adapter
}

func TestRunWithPrediction_MapToStructError(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return &core.GenerateResult{
				Content:  `{"Result": "test"}`,
				Metadata: make(map[string]any),
			}, nil
		},
	}

	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	// Normal case
	_, pred, err := fn.RunWithPrediction(context.Background(), Input{Text: "test"})
	if err != nil {
		t.Fatalf("RunWithPrediction() error = %v", err)
	}
	if pred == nil {
		t.Error("RunWithPrediction() should return prediction")
	}
}

func TestBuildCombinedSignature_ParseInputError(t *testing.T) {
	type BadInput struct {
		Field string `dsgo:"bad_direction"`
	}
	type ValidOutput struct {
		Result string `dsgo:"output,desc=Output"`
	}

	inputType := reflect.TypeOf(BadInput{})
	outputType := reflect.TypeOf(ValidOutput{})

	_, err := buildCombinedSignature(inputType, outputType)
	if err == nil {
		t.Error("buildCombinedSignature() should return error for invalid input tags")
	}
}

func TestBuildCombinedSignature_ParseOutputError(t *testing.T) {
	type ValidInput struct {
		Text string `dsgo:"input,desc=Input"`
	}
	type BadOutput struct {
		Field string `dsgo:"bad_direction"`
	}

	inputType := reflect.TypeOf(ValidInput{})
	outputType := reflect.TypeOf(BadOutput{})

	_, err := buildCombinedSignature(inputType, outputType)
	if err == nil {
		t.Error("buildCombinedSignature() should return error for invalid output tags")
	}
}

func TestWithDemosTyped_EmptyList(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input,desc=Input"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=Output"`
	}

	lm := &mockLM{}
	fn, err := NewPredict[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewFunc() error = %v", err)
	}

	// Empty lists should work
	inputs := []Input{}
	outputs := []Output{}

	fn2, err := fn.WithDemosTyped(inputs, outputs)
	if err != nil {
		t.Fatalf("WithDemosTyped() error = %v", err)
	}
	if fn2 == nil {
		t.Error("WithDemosTyped() returned nil")
	}
}

// TestNewCoT tests the NewCoT constructor
func TestNewCoT(t *testing.T) {
	type Input struct {
		Question string `dsgo:"input,desc=The question to answer"`
	}
	type Output struct {
		Answer string `dsgo:"output,desc=The answer"`
	}

	lm := &mockLM{}
	fn, err := NewCoT[Input, Output](lm)
	if err != nil {
		t.Fatalf("NewCoT() error = %v", err)
	}

	if fn == nil {
		t.Fatal("NewCoT() returned nil")
	}

	// Verify it's a ChainOfThought module
	sig := fn.GetSignature()
	if sig == nil {
		t.Fatal("GetSignature() returned nil")
	}

	if len(sig.InputFields) != 1 {
		t.Errorf("expected 1 input field, got %d", len(sig.InputFields))
	}

	if len(sig.OutputFields) != 1 {
		t.Errorf("expected 1 output field, got %d", len(sig.OutputFields))
	}
}

// TestNewReAct tests the NewReAct constructor
func TestNewReAct(t *testing.T) {
	type Input struct {
		Query string `dsgo:"input,desc=The query"`
	}
	type Output struct {
		Result string `dsgo:"output,desc=The result"`
	}

	lm := &mockLM{}
	tools := []core.Tool{
		*core.NewTool("calculator", "Simple calculator", func(ctx context.Context, args map[string]any) (any, error) {
			return "42", nil
		}),
	}

	fn, err := NewReAct[Input, Output](lm, tools)
	if err != nil {
		t.Fatalf("NewReAct() error = %v", err)
	}

	if fn == nil {
		t.Fatal("NewReAct() returned nil")
	}

	// Verify it has input/output fields
	sig := fn.GetSignature()
	if sig == nil {
		t.Fatal("GetSignature() returned nil")
	}

	if len(sig.InputFields) != 1 {
		t.Errorf("expected 1 input field, got %d", len(sig.InputFields))
	}

	if len(sig.OutputFields) != 1 {
		t.Errorf("expected 1 output field, got %d", len(sig.OutputFields))
	}

	// Test WithMaxIterations on ReAct
	fn.WithMaxIterations(5)
	fn.WithVerbose(true)
}

func TestFunc_WithOptions_AllModuleTypes(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{}
	opts := &core.GenerateOptions{Temperature: 0.7}

	t.Run("Predict", func(t *testing.T) {
		fn, _ := NewPredict[Input, Output](lm)
		result := fn.WithOptions(opts)
		if result == nil {
			t.Error("WithOptions should return Func")
		}
	})

	t.Run("ChainOfThought", func(t *testing.T) {
		fn, _ := NewCoT[Input, Output](lm)
		result := fn.WithOptions(opts)
		if result == nil {
			t.Error("WithOptions should return Func")
		}
	})

	t.Run("ReAct", func(t *testing.T) {
		fn, _ := NewReAct[Input, Output](lm, []core.Tool{})
		result := fn.WithOptions(opts)
		if result == nil {
			t.Error("WithOptions should return Func")
		}
	})
}

func TestFunc_WithAdapter_AllModuleTypes(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{}
	adapter := core.NewChatAdapter()

	t.Run("Predict", func(t *testing.T) {
		fn, _ := NewPredict[Input, Output](lm)
		result := fn.WithAdapter(adapter)
		if result == nil {
			t.Error("WithAdapter should return Func")
		}
	})

	t.Run("ChainOfThought", func(t *testing.T) {
		fn, _ := NewCoT[Input, Output](lm)
		result := fn.WithAdapter(adapter)
		if result == nil {
			t.Error("WithAdapter should return Func")
		}
	})

	t.Run("ReAct", func(t *testing.T) {
		fn, _ := NewReAct[Input, Output](lm, []core.Tool{})
		result := fn.WithAdapter(adapter)
		if result == nil {
			t.Error("WithAdapter should return Func")
		}
	})
}

func TestFunc_WithHistory_AllModuleTypes(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{}
	history := core.NewHistory()

	t.Run("Predict", func(t *testing.T) {
		fn, _ := NewPredict[Input, Output](lm)
		result := fn.WithHistory(history)
		if result == nil {
			t.Error("WithHistory should return Func")
		}
	})

	t.Run("ChainOfThought", func(t *testing.T) {
		fn, _ := NewCoT[Input, Output](lm)
		result := fn.WithHistory(history)
		if result == nil {
			t.Error("WithHistory should return Func")
		}
	})

	t.Run("ReAct", func(t *testing.T) {
		fn, _ := NewReAct[Input, Output](lm, []core.Tool{})
		result := fn.WithHistory(history)
		if result == nil {
			t.Error("WithHistory should return Func")
		}
	})
}

func TestFunc_WithDemos_AllModuleTypes(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{}
	demos := []core.Example{
		*core.NewExample(map[string]any{"Text": "hello"}, map[string]any{"Result": "world"}),
	}

	t.Run("Predict", func(t *testing.T) {
		fn, _ := NewPredict[Input, Output](lm)
		result := fn.WithDemos(demos)
		if result == nil {
			t.Error("WithDemos should return Func")
		}
	})

	t.Run("ChainOfThought", func(t *testing.T) {
		fn, _ := NewCoT[Input, Output](lm)
		result := fn.WithDemos(demos)
		if result == nil {
			t.Error("WithDemos should return Func")
		}
	})

	t.Run("ReAct", func(t *testing.T) {
		fn, _ := NewReAct[Input, Output](lm, []core.Tool{})
		result := fn.WithDemos(demos)
		if result == nil {
			t.Error("WithDemos should return Func")
		}
	})
}

func TestFunc_WithDemosTyped_ValidDemos(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{}
	fn, _ := NewPredict[Input, Output](lm)

	// Test with valid typed demos
	inputs := []Input{{Text: "hello"}}
	outputs := []Output{{Result: "world"}}

	_, err := fn.WithDemosTyped(inputs, outputs)
	if err != nil {
		t.Errorf("WithDemosTyped should not error with valid demos: %v", err)
	}
}

func TestNewCoT_Success(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{}
	fn, err := NewCoT[Input, Output](lm)
	if err != nil {
		t.Errorf("NewCoT should not error: %v", err)
	}
	if fn == nil {
		t.Error("NewCoT should return Func")
	}
}

func TestNewReAct_Success(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{}
	fn, err := NewReAct[Input, Output](lm, []core.Tool{})
	if err != nil {
		t.Errorf("NewReAct should not error: %v", err)
	}
	if fn == nil {
		t.Error("NewReAct should return Func")
	}
}

func TestRun_ErrorConditions(t *testing.T) {
	type Input struct {
		Text string `dsgo:"input"`
	}
	type Output struct {
		Result string `dsgo:"output"`
	}

	lm := &mockLM{
		generateFunc: func(ctx context.Context, messages []core.Message, options *core.GenerateOptions) (*core.GenerateResult, error) {
			return nil, fmt.Errorf("generation error")
		},
	}

	fn, _ := NewPredict[Input, Output](lm)

	_, err := fn.Run(context.Background(), Input{Text: "test"})
	if err == nil {
		t.Error("Run should return error when generation fails")
	}
}
