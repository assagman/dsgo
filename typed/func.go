package typed

import (
	"context"
	"fmt"
	"reflect"

	"github.com/assagman/dsgo"
	"github.com/assagman/dsgo/module"
)

// Func is a generic, type-safe module wrapper for DSGo modules
// I is the input struct type, O is the output struct type
type Func[I, O any] struct {
	module      dsgo.Module
	inputType   reflect.Type
	outputType  reflect.Type
	description string
}

// NewPredict creates a new typed function module using Predict
// The I and O types must be structs with dsgo tags
func NewPredict[I, O any](lm dsgo.LM) (*Func[I, O], error) {
	sig, inputType, outputType, err := buildTypedSignature[I, O]()
	if err != nil {
		return nil, err
	}

	// Create the underlying Predict module
	predict := module.NewPredict(sig, lm)

	return &Func[I, O]{
		module:      predict,
		inputType:   inputType,
		outputType:  outputType,
		description: sig.Description,
	}, nil
}

// NewCoT creates a new typed function module using ChainOfThought
// The I and O types must be structs with dsgo tags
func NewCoT[I, O any](lm dsgo.LM) (*Func[I, O], error) {
	sig, inputType, outputType, err := buildTypedSignature[I, O]()
	if err != nil {
		return nil, err
	}

	// Create the underlying ChainOfThought module
	cot := module.NewChainOfThought(sig, lm)

	return &Func[I, O]{
		module:      cot,
		inputType:   inputType,
		outputType:  outputType,
		description: sig.Description,
	}, nil
}

// NewReAct creates a new typed function module using ReAct
// The I and O types must be structs with dsgo tags
func NewReAct[I, O any](lm dsgo.LM, tools []dsgo.Tool) (*Func[I, O], error) {
	sig, inputType, outputType, err := buildTypedSignature[I, O]()
	if err != nil {
		return nil, err
	}

	// Create the underlying ReAct module
	react := module.NewReAct(sig, lm, tools)

	return &Func[I, O]{
		module:      react,
		inputType:   inputType,
		outputType:  outputType,
		description: sig.Description,
	}, nil
}

// buildTypedSignature is a helper to build signature and extract types
func buildTypedSignature[I, O any]() (*dsgo.Signature, reflect.Type, reflect.Type, error) {
	var i I
	var o O

	inputType := reflect.TypeOf(i)
	outputType := reflect.TypeOf(o)

	// Validate that I and O are structs
	if inputType.Kind() != reflect.Struct {
		return nil, nil, nil, fmt.Errorf("input type must be a struct, got %s", inputType.Kind())
	}
	if outputType.Kind() != reflect.Struct {
		return nil, nil, nil, fmt.Errorf("output type must be a struct, got %s", outputType.Kind())
	}

	// Build combined signature from both input and output types
	sig, err := buildCombinedSignature(inputType, outputType)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build signature: %w", err)
	}

	return sig, inputType, outputType, nil
}

// NewPredictWithDescription creates a typed Predict function with a custom description
func NewPredictWithDescription[I, O any](lm dsgo.LM, description string) (*Func[I, O], error) {
	fn, err := NewPredict[I, O](lm)
	if err != nil {
		return nil, err
	}
	fn.description = description
	fn.module.GetSignature().Description = description
	return fn, nil
}

// buildCombinedSignature builds a signature by combining input and output struct tags
func buildCombinedSignature(inputType, outputType reflect.Type) (*dsgo.Signature, error) {
	// Parse input fields
	inputFields, err := ParseStructTags(inputType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input type: %w", err)
	}

	// Parse output fields
	outputFields, err := ParseStructTags(outputType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output type: %w", err)
	}

	sig := dsgo.NewSignature("")

	// Add input fields (only those marked as input)
	for _, field := range inputFields {
		if field.IsInput {
			sig.InputFields = append(sig.InputFields, dsgo.Field{
				Name:         field.Name,
				Type:         field.Type,
				Description:  field.Description,
				Optional:     field.Optional,
				Classes:      field.Classes,
				ClassAliases: field.ClassAliases,
			})
		}
	}

	// Add output fields (only those marked as output)
	for _, field := range outputFields {
		if field.IsOutput {
			sig.OutputFields = append(sig.OutputFields, dsgo.Field{
				Name:         field.Name,
				Type:         field.Type,
				Description:  field.Description,
				Optional:     field.Optional,
				Classes:      field.Classes,
				ClassAliases: field.ClassAliases,
			})
		}
	}

	return sig, nil
}

// Run executes the typed module with type-safe input and output
func (f *Func[I, O]) Run(ctx context.Context, input I) (O, error) {
	var zero O

	// Convert input struct to map
	inputMap, err := StructToMap(input)
	if err != nil {
		return zero, fmt.Errorf("failed to convert input to map: %w", err)
	}

	// Execute the module
	pred, err := f.module.Forward(ctx, inputMap)
	if err != nil {
		return zero, fmt.Errorf("module execution failed: %w", err)
	}

	// Convert output map to struct
	var output O
	if err := MapToStruct(pred.Outputs, &output); err != nil {
		return zero, fmt.Errorf("failed to convert output to struct: %w", err)
	}

	return output, nil
}

// RunWithPrediction executes and returns both the typed output and raw prediction
func (f *Func[I, O]) RunWithPrediction(ctx context.Context, input I) (O, *dsgo.Prediction, error) {
	var zero O

	// Convert input struct to map
	inputMap, err := StructToMap(input)
	if err != nil {
		return zero, nil, fmt.Errorf("failed to convert input to map: %w", err)
	}

	// Execute the module
	pred, err := f.module.Forward(ctx, inputMap)
	if err != nil {
		return zero, nil, fmt.Errorf("module execution failed: %w", err)
	}

	// Convert output map to struct
	var output O
	if err := MapToStruct(pred.Outputs, &output); err != nil {
		return zero, pred, fmt.Errorf("failed to convert output to struct: %w", err)
	}

	return output, pred, nil
}

// WithOptions sets custom generation options
// Works with all module types (Predict, ChainOfThought, ReAct, etc.)
func (f *Func[I, O]) WithOptions(options *dsgo.GenerateOptions) *Func[I, O] {
	switch m := f.module.(type) {
	case *module.Predict:
		m.WithOptions(options)
	case *module.ChainOfThought:
		m.WithOptions(options)
	case *module.ReAct:
		m.WithOptions(options)
	}
	return f
}

// WithAdapter sets a custom adapter
// Works with all module types (Predict, ChainOfThought, ReAct, etc.)
func (f *Func[I, O]) WithAdapter(adapter dsgo.Adapter) *Func[I, O] {
	switch m := f.module.(type) {
	case *module.Predict:
		m.WithAdapter(adapter)
	case *module.ChainOfThought:
		m.WithAdapter(adapter)
	case *module.ReAct:
		m.WithAdapter(adapter)
	}
	return f
}

// WithHistory sets conversation history
// Works with all module types (Predict, ChainOfThought, ReAct, etc.)
func (f *Func[I, O]) WithHistory(history *dsgo.History) *Func[I, O] {
	switch m := f.module.(type) {
	case *module.Predict:
		m.WithHistory(history)
	case *module.ChainOfThought:
		m.WithHistory(history)
	case *module.ReAct:
		m.WithHistory(history)
	}
	return f
}

// WithDemos sets few-shot examples (using map-based examples)
// Works with all module types (Predict, ChainOfThought, ReAct, etc.)
func (f *Func[I, O]) WithDemos(demos []dsgo.Example) *Func[I, O] {
	switch m := f.module.(type) {
	case *module.Predict:
		m.WithDemos(demos)
	case *module.ChainOfThought:
		m.WithDemos(demos)
	case *module.ReAct:
		m.WithDemos(demos)
	}
	return f
}

// WithMaxIterations sets maximum iterations for ReAct module
// Only applicable when using NewReAct
func (f *Func[I, O]) WithMaxIterations(max int) *Func[I, O] {
	if react, ok := f.module.(*module.ReAct); ok {
		react.WithMaxIterations(max)
	}
	return f
}

// WithVerbose enables verbose logging for ReAct module
// Only applicable when using NewReAct
func (f *Func[I, O]) WithVerbose(verbose bool) *Func[I, O] {
	if react, ok := f.module.(*module.ReAct); ok {
		react.WithVerbose(verbose)
	}
	return f
}

// WithDemosTyped sets few-shot examples using typed inputs/outputs
func (f *Func[I, O]) WithDemosTyped(inputs []I, outputs []O) (*Func[I, O], error) {
	if len(inputs) != len(outputs) {
		return nil, fmt.Errorf("inputs and outputs must have the same length")
	}

	demos := make([]dsgo.Example, len(inputs))
	for i := range inputs {
		inputMap, err := StructToMap(inputs[i])
		if err != nil {
			return nil, fmt.Errorf("failed to convert input %d: %w", i, err)
		}
		outputMap, err := StructToMap(outputs[i])
		if err != nil {
			return nil, fmt.Errorf("failed to convert output %d: %w", i, err)
		}
		demos[i] = dsgo.Example{
			Inputs:  inputMap,
			Outputs: outputMap,
		}
	}

	if predict, ok := f.module.(*module.Predict); ok {
		predict.WithDemos(demos)
	}
	return f, nil
}

// GetSignature returns the underlying signature
func (f *Func[I, O]) GetSignature() *dsgo.Signature {
	return f.module.GetSignature()
}

// Forward implements the Module interface for compatibility
func (f *Func[I, O]) Forward(ctx context.Context, inputs map[string]any) (*dsgo.Prediction, error) {
	return f.module.Forward(ctx, inputs)
}
