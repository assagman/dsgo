package dsgo

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/assagman/dsgo/internal/core"
	"github.com/assagman/dsgo/internal/logging"
	"github.com/assagman/dsgo/internal/module"
)

// This file contains comprehensive tests for the dsgo package's public API surface.
// It serves as a smoke test to ensure that:
// 1. All providers are properly registered via init() functions
// 2. All re-exported types, functions, and constants are accessible
// 3. The public API maintains backward compatibility
// 4. Type aliases work correctly at runtime
//
// These tests are intentionally lightweight and don't require external API keys,
// making them suitable for CI/CD environments and quick validation.

// TestPackageInit verifies that importing the dsgo package initializes
// default settings and registers all standard providers via init()
func TestPackageInit(t *testing.T) {
	ctx := context.Background()

	// Test that providers are registered by attempting to create LMs
	// This indirectly tests that the init() functions in provider packages ran
	tests := []struct {
		name           string
		model          string
		shouldWork     bool
		expectedError  string // substring that should appear in error message
		skipIfNoAPIKey bool   // skip test if API key not available
	}{
		{
			name:           "OpenAI provider registered",
			model:          "openai/gpt-4o-mini",
			shouldWork:     true,
			expectedError:  "",
			skipIfNoAPIKey: true,
		},
		{
			name:           "OpenRouter provider registered",
			model:          "openrouter/anthropic/claude-3.7-sonnet",
			shouldWork:     true,
			expectedError:  "",
			skipIfNoAPIKey: true,
		},
		{
			name:           "Non-existent provider fails",
			model:          "nonexistent/test-model",
			shouldWork:     false,
			expectedError:  "provider 'nonexistent' not registered",
			skipIfNoAPIKey: false,
		},
		{
			name:           "Empty model string fails",
			model:          "",
			shouldWork:     false,
			expectedError:  "model string is required",
			skipIfNoAPIKey: false,
		},
		{
			name:           "Invalid format fails",
			model:          "invalid-format-no-slash",
			shouldWork:     false,
			expectedError:  "model string must include provider",
			skipIfNoAPIKey: false,
		},
		{
			name:           "Provider only fails",
			model:          "openai",
			shouldWork:     false,
			expectedError:  "model string must include provider",
			skipIfNoAPIKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Attempt to create an LM instance
			lm, err := core.NewLM(ctx, tt.model)

			if tt.shouldWork {
				if err != nil {
					// Check if it's an API key error - that's acceptable for this test
					if strings.Contains(err.Error(), "API key") || strings.Contains(err.Error(), "environment variable") {
						t.Skipf("Skipping %s due to missing API key: %v", tt.name, err)
						return
					}
					t.Errorf("Expected to create LM with model %s, but got error: %v", tt.model, err)
					return
				}
				if lm == nil {
					t.Errorf("Expected non-nil LM for model %s", tt.model)
				}
			} else {
				if err == nil {
					t.Errorf("Expected model %s to not be created, but no error was returned", tt.model)
					return
				}
				if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s' for model %s, got: %v", tt.expectedError, tt.model, err)
				}
			}
		})
	}
}

// TestModelCatalog verifies model catalog and cost APIs
func TestModelCatalog(t *testing.T) {
	if !IsValidModel("openai/gpt-4o") {
		t.Fatal("expected openai/gpt-4o to be valid")
	}
	if !IsValidModel("gpt-4o") {
		t.Fatal("expected alias gpt-4o to be valid")
	}
	if IsValidModel("openai/does-not-exist") {
		t.Fatal("expected unknown model to be invalid")
	}

	models := ListModelsByProvider("openai")
	if len(models) == 0 {
		t.Fatal("expected some openai models")
	}

	found := false
	for _, m := range models {
		if m.ID == "openai/gpt-4o" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected openai/gpt-4o to be listed")
	}

	pricing, ok := DefaultCostCalculator.GetPricing("openai/gpt-4o")
	if !ok || pricing.PromptPrice == 0 {
		t.Fatal("expected pricing for openai/gpt-4o")
	}
}

// TestReexportedTypes verifies that all re-exported types are accessible and properly typed
func TestReexportedTypes(t *testing.T) {
	// This test ensures that the type aliases in dsgo.go compile and are accessible
	// If this compiles and runs, it means all the re-exported types are properly defined

	// Test core types - these are type aliases to the actual types
	// We verify that we can create values of these types and assign them to the re-exported types
	var _ LM = nil                  // interface
	var _ = core.Message{}          // struct
	var _ = core.Signature{}        // struct value
	var _ = core.Prediction{}       // struct value
	var _ = core.Field{}            // struct value
	var _ = core.Tool{}             // struct value
	var _ = core.Example{}          // struct value
	var _ = core.History{}          // struct value
	var _ = &core.MemoryCollector{} // pointer to struct
	var _ = &core.LMCache{}         // pointer to struct

	// Test module types - these are type aliases to module types (structs)
	var _ = module.Predict{}          // struct value
	var _ = module.ChainOfThought{}   // struct value
	var _ = module.ReAct{}            // struct value
	var _ = module.Refine{}           // struct value
	var _ = module.BestOfN{}          // struct value
	var _ = module.Program{}          // struct value
	var _ = module.ProgramOfThought{} // struct value
	var _ = module.Parallel{}         // struct value

	// Test logging types
	var _ Logger = nil // interface
	var _ = LevelDebug // int constant

	// Test that re-exported functions are accessible
	if NewLM == nil {
		t.Error("NewLM function not re-exported")
	}
	if NewSignature == nil {
		t.Error("NewSignature function not re-exported")
	}
	if Configure == nil {
		t.Error("Configure function not re-exported")
	}
	if NewPredict == nil {
		t.Error("NewPredict function not re-exported")
	}
	if NewDefaultLogger == nil {
		t.Error("NewDefaultLogger function not re-exported")
	}

	// Runtime type safety checks
	t.Run("runtime type verification", func(t *testing.T) {
		// Verify that re-exported types have the correct underlying types
		tests := []struct {
			name     string
			reexport reflect.Type
			expected reflect.Type
		}{
			{"LM type", reflect.TypeOf((*LM)(nil)).Elem(), reflect.TypeOf((*core.LM)(nil)).Elem()},
			{"Message type", reflect.TypeOf(Message{}), reflect.TypeOf(core.Message{})},
			{"Signature type", reflect.TypeOf(Signature{}), reflect.TypeOf(core.Signature{})},
			{"Prediction type", reflect.TypeOf(Prediction{}), reflect.TypeOf(core.Prediction{})},
			{"Predict type", reflect.TypeOf(Predict{}), reflect.TypeOf(module.Predict{})},
			{"Logger type", reflect.TypeOf((*Logger)(nil)).Elem(), reflect.TypeOf((*logging.Logger)(nil)).Elem()},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.reexport != tt.expected {
					t.Errorf("Type mismatch: expected %v, got %v", tt.expected, tt.reexport)
				}
			})
		}
	})

	// Test that we can actually create instances using re-exported constructors
	t.Run("constructor functionality", func(t *testing.T) {
		// Test signature creation
		sig := NewSignature("test signature")
		if sig == nil {
			t.Error("NewSignature returned nil")
		} else if sig.Description != "test signature" {
			t.Errorf("Expected description 'test signature', got '%s'", sig.Description)
		}

		// Test prediction creation
		pred := NewPrediction(map[string]any{"test": "value"})
		if pred == nil {
			t.Error("NewPrediction returned nil")
		} else if pred.Outputs["test"] != "value" {
			t.Errorf("Expected prediction output 'value', got '%v'", pred.Outputs["test"])
		}

		// Test logger creation
		logger := NewDefaultLogger(LevelInfo)
		if logger == nil {
			t.Error("NewDefaultLogger returned nil")
		}
	})
}

// TestConstants verifies that all re-exported constants are accessible
func TestConstants(t *testing.T) {
	// Test field type constants
	if FieldTypeString != core.FieldTypeString {
		t.Error("FieldTypeString constant not properly re-exported")
	}
	if FieldTypeInt != core.FieldTypeInt {
		t.Error("FieldTypeInt constant not properly re-exported")
	}
	if FieldTypeFloat != core.FieldTypeFloat {
		t.Error("FieldTypeFloat constant not properly re-exported")
	}
	if FieldTypeBool != core.FieldTypeBool {
		t.Error("FieldTypeBool constant not properly re-exported")
	}
	if FieldTypeClass != core.FieldTypeClass {
		t.Error("FieldTypeClass constant not properly re-exported")
	}
	if FieldTypeJSON != core.FieldTypeJSON {
		t.Error("FieldTypeJSON constant not properly re-exported")
	}
	if FieldTypeImage != core.FieldTypeImage {
		t.Error("FieldTypeImage constant not properly re-exported")
	}
	if FieldTypeDatetime != core.FieldTypeDatetime {
		t.Error("FieldTypeDatetime constant not properly re-exported")
	}

	// Test logging level constants
	if LevelDebug != 0 {
		t.Error("LevelDebug constant has unexpected value")
	}
	if LevelInfo != 1 {
		t.Error("LevelInfo constant has unexpected value")
	}
	if LevelWarn != 2 {
		t.Error("LevelWarn constant has unexpected value")
	}
	if LevelError != 3 {
		t.Error("LevelError constant has unexpected value")
	}
}

// TestConfigurationAndSettings tests configuration-related re-exports
func TestConfigurationAndSettings(t *testing.T) {
	t.Run("configuration functions", func(t *testing.T) {
		// Test that configuration functions are accessible
		if Configure == nil {
			t.Error("Configure function not re-exported")
		}
		if GetSettings == nil {
			t.Error("GetSettings function not re-exported")
		}
		if ResetConfig == nil {
			t.Error("ResetConfig function not re-exported")
		}

		// Test that we can get current settings
		settings := GetSettings()
		// Settings is a struct, not a pointer, so we check if it's zero value
		if settings.DefaultTimeout == 0 {
			t.Error("GetSettings returned zero-value settings")
		}

		// Test that we can reset configuration
		ResetConfig()
		// Should not panic
	})

	t.Run("option functions", func(t *testing.T) {
		// Test that option functions are accessible
		optionFuncs := []struct {
			name string
			fn   func() any
		}{
			{"WithProvider", func() any { return WithProvider("test") }},
			{"WithModel", func() any { return WithModel("test") }},
			{"WithTimeout", func() any { return WithTimeout(0) }},
			{"WithAPIKey", func() any { return WithAPIKey("test", "key") }},
			{"WithMaxRetries", func() any { return WithMaxRetries(0) }},
			{"WithTracing", func() any { return WithTracing(false) }},
			{"WithCache", func() any { return WithCache(100) }},
			{"WithCacheTTL", func() any { return WithCacheTTL(0) }},
		}

		for _, opt := range optionFuncs {
			t.Run(opt.name, func(t *testing.T) {
				result := opt.fn()
				if result == nil {
					t.Errorf("%s returned nil", opt.name)
				}
			})
		}
	})

	t.Run("adapter functions", func(t *testing.T) {
		// Test that adapter constructor functions are accessible
		if NewFallbackAdapter == nil {
			t.Error("NewFallbackAdapter function not re-exported")
		}
		if NewJSONAdapter == nil {
			t.Error("NewJSONAdapter function not re-exported")
		}
		if NewChatAdapter == nil {
			t.Error("NewChatAdapter function not re-exported")
		}
		if NewTwoStepAdapter == nil {
			t.Error("NewTwoStepAdapter function not re-exported")
		}

		// Test that we can create adapters
		fallbackAdapter := NewFallbackAdapter()
		if fallbackAdapter == nil {
			t.Error("NewFallbackAdapter returned nil")
		}

		jsonAdapter := NewJSONAdapter()
		if jsonAdapter == nil {
			t.Error("NewJSONAdapter returned nil")
		}

		chatAdapter := NewChatAdapter()
		if chatAdapter == nil {
			t.Error("NewChatAdapter returned nil")
		}

		// NewTwoStepAdapter requires an LM parameter, so we'll just test the function exists
		// We can't create a real LM here without API keys, so we skip the creation test
	})

	t.Run("collector functions", func(t *testing.T) {
		// Test that collector constructor functions are accessible
		if NewMemoryCollector == nil {
			t.Error("NewMemoryCollector function not re-exported")
		}
		if NewJSONLCollector == nil {
			t.Error("NewJSONLCollector function not re-exported")
		}
		if NewCompositeCollector == nil {
			t.Error("NewCompositeCollector function not re-exported")
		}

		// Test that we can create collectors
		memoryCollector := NewMemoryCollector(10)
		if memoryCollector == nil {
			t.Error("NewMemoryCollector returned nil")
		}

		// NewJSONLCollector returns (collector, error), so we need to handle the error
		jsonlCollector, err := NewJSONLCollector("/tmp/test.jsonl")
		if err != nil {
			t.Errorf("NewJSONLCollector failed: %v", err)
		}
		if jsonlCollector == nil {
			t.Error("NewJSONLCollector returned nil")
		}

		compositeCollector := NewCompositeCollector(memoryCollector)
		if compositeCollector == nil {
			t.Error("NewCompositeCollector returned nil")
		}
	})
}

// TestTypedGenericFunctionality tests the typed generic functionality
func TestTypedGenericFunctionality(t *testing.T) {
	t.Run("typed utility functions", func(t *testing.T) {
		// Test that typed utility functions are accessible
		if StructToSignature == nil {
			t.Error("StructToSignature function not re-exported")
		}
		if StructToMap == nil {
			t.Error("StructToMap function not re-exported")
		}
		if MapToStruct == nil {
			t.Error("MapToStruct function not re-exported")
		}
		if ParseStructTags == nil {
			t.Error("ParseStructTags function not re-exported")
		}

		// Test that we can use the generic Func type
		var _ Func[string, int] // This should compile without error

		// Test that we can use the FieldInfo type
		fieldInfo := FieldInfo{
			Name: "test",
			Type: core.FieldTypeString,
		}
		if fieldInfo.Name != "test" {
			t.Errorf("Expected field name 'test', got '%s'", fieldInfo.Name)
		}
		if fieldInfo.Type != core.FieldTypeString {
			t.Errorf("Expected field type '%s', got '%s'", core.FieldTypeString, fieldInfo.Type)
		}
	})

	t.Run("typed struct operations", func(t *testing.T) {
		// Define test structs for typed operations
		type TestInput struct {
			Text  string `dsgo:"input,desc=Input text"`
			Count int    `dsgo:"input,desc=Input count"`
		}

		type TestOutput struct {
			Result string  `dsgo:"output,desc=Output result"`
			Score  float64 `dsgo:"output,desc=Output score"`
		}

		input := TestInput{
			Text:  "hello",
			Count: 5,
		}

		// Test StructToMap
		inputMap, err := StructToMap(input)
		if err != nil {
			t.Errorf("StructToMap failed: %v", err)
		}
		if inputMap["Text"] != "hello" {
			t.Errorf("Expected 'hello', got '%v'", inputMap["Text"])
		}
		if inputMap["Count"] != 5 {
			t.Errorf("Expected 5, got '%v'", inputMap["Count"])
		}

		// Test MapToStruct
		outputMap := map[string]any{
			"Result": "success",
			"Score":  0.95,
		}
		var output TestOutput
		err = MapToStruct(outputMap, &output)
		if err != nil {
			t.Errorf("MapToStruct failed: %v", err)
		}
		if output.Result != "success" {
			t.Errorf("Expected 'success', got '%s'", output.Result)
		}
		if output.Score != 0.95 {
			t.Errorf("Expected 0.95, got '%f'", output.Score)
		}
	})

	t.Run("struct tag parsing", func(t *testing.T) {
		type TestStruct struct {
			Name     string `dsgo:"input,desc=Person name"`
			Age      int    `dsgo:"input,desc=Person age"`
			Email    string `dsgo:"input,desc=Email address"`
			Optional string `dsgo:"output,optional,desc=Optional field"`
		}

		tags, err := ParseStructTags(reflect.TypeOf(TestStruct{}))
		if err != nil {
			t.Errorf("ParseStructTags failed: %v", err)
		}

		// Check that we got the expected number of fields
		if len(tags) != 4 {
			t.Errorf("Expected 4 fields, got %d", len(tags))
		}

		// Check specific field tags by finding the field by name
		var nameField *FieldInfo
		for _, field := range tags {
			if field.Name == "Name" {
				nameField = &field
				break
			}
		}

		if nameField == nil {
			t.Error("Name field not found")
		} else {
			if !nameField.IsInput {
				t.Error("Expected Name field to be input")
			}
			if nameField.Description != "Person name" {
				t.Errorf("Expected description 'Person name', got '%s'", nameField.Description)
			}
		}
	})
}
