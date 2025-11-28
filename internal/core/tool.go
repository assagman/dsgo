package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// ParamType represents a tool parameter type
type ParamType string

const (
	ParamString ParamType = "string"
	ParamInt    ParamType = "int"
	ParamFloat  ParamType = "float"
	ParamBool   ParamType = "bool"
	ParamJSON   ParamType = "json"
	ParamArray  ParamType = "array"
)

// ToolParameter represents a parameter for a tool
type ToolParameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Enum        []string
	ElementType string // Optional: for array element type validation
}

// Tool represents a tool/function that can be called by the LM
type Tool struct {
	Name        string
	Description string
	Parameters  []ToolParameter
	Function    ToolFunction `json:"-"` // Exclude from JSON serialization
}

// ToolFunction is the actual function implementation
type ToolFunction func(ctx context.Context, args map[string]any) (any, error)

// NewTool creates a new tool
func NewTool(name, description string, fn ToolFunction) *Tool {
	return &Tool{
		Name:        name,
		Description: description,
		Parameters:  []ToolParameter{},
		Function:    fn,
	}
}

// AddParameter adds a parameter to the tool
func (t *Tool) AddParameter(name, paramType, description string, required bool) *Tool {
	t.Parameters = append(t.Parameters, ToolParameter{
		Name:        name,
		Type:        paramType,
		Description: description,
		Required:    required,
	})
	return t
}

// AddEnumParameter adds an enum parameter to the tool
func (t *Tool) AddEnumParameter(name, description string, enum []string, required bool) *Tool {
	t.Parameters = append(t.Parameters, ToolParameter{
		Name:        name,
		Type:        "string",
		Description: description,
		Required:    required,
		Enum:        enum,
	})
	return t
}

// AddArrayParameter adds an array parameter to the tool with optional element type
func (t *Tool) AddArrayParameter(name, description string, elementType string, required bool) *Tool {
	t.Parameters = append(t.Parameters, ToolParameter{
		Name:        name,
		Type:        "array",
		Description: description,
		Required:    required,
		ElementType: elementType,
	})
	return t
}

// normalizeParamType maps type synonyms to canonical types
func normalizeParamType(t string) ParamType {
	switch strings.ToLower(t) {
	case "string":
		return ParamString
	case "int", "integer":
		return ParamInt
	case "float", "number", "double":
		return ParamFloat
	case "bool", "boolean":
		return ParamBool
	case "json", "object":
		return ParamJSON
	case "array", "list":
		return ParamArray
	default:
		return ParamType(t) // Keep as-is for custom types
	}
}

// Validate validates the arguments against the tool's parameters
func (t *Tool) Validate(args map[string]any) error {
	// Check required parameters are present
	for _, param := range t.Parameters {
		if param.Required {
			if _, exists := args[param.Name]; !exists {
				return fmt.Errorf("missing required parameter: %s", param.Name)
			}
		}
	}

	// Validate type for each parameter
	for _, param := range t.Parameters {
		val, exists := args[param.Name]
		if !exists {
			continue // Skip optional parameters
		}

		paramType := normalizeParamType(param.Type)
		if err := t.validateType(param.Name, paramType, val); err != nil {
			return err
		}

		// Check enum values (only for string parameters)
		if len(param.Enum) > 0 && paramType == ParamString {
			valStr, ok := val.(string)
			if !ok {
				return fmt.Errorf("parameter %s: enum validation requires string type", param.Name)
			}
			valid := false
			for _, enumVal := range param.Enum {
				if valStr == enumVal {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("parameter %s has invalid value: %v (must be one of %v)", param.Name, valStr, param.Enum)
			}
		}
	}

	return nil
}

// validateType validates that a value matches the expected parameter type
func (t *Tool) validateType(name string, paramType ParamType, value any) error {
	switch paramType {
	case ParamString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter %s has invalid type: expected string, got %T", name, value)
		}

	case ParamInt:
		if _, ok := value.(int64); !ok {
			return fmt.Errorf("parameter %s has invalid type: expected int, got %T (value: %v)", name, value, value)
		}

	case ParamFloat:
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("parameter %s has invalid type: expected float, got %T (value: %v)", name, value, value)
		}

	case ParamBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter %s has invalid type: expected bool, got %T (value: %v)", name, value, value)
		}

	case ParamJSON:
		// JSON accepts map or slice
		switch value.(type) {
		case map[string]any, []any:
			// Valid
		default:
			return fmt.Errorf("parameter %s has invalid type: expected JSON (object/array), got %T", name, value)
		}

	case ParamArray:
		// Array accepts various slice types
		v := reflect.ValueOf(value)
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return fmt.Errorf("parameter %s has invalid type: expected array, got %T", name, value)
		}

	default:
		// Unknown type - skip validation
	}

	return nil
}

// Execute executes the tool with given arguments
func (t *Tool) Execute(ctx context.Context, args map[string]any) (any, error) {
	// Normalize arguments (convert arrays to strings for string parameters)
	normalizedArgs := t.normalizeArguments(args)

	// Validate arguments before execution
	if err := t.Validate(normalizedArgs); err != nil {
		return nil, fmt.Errorf("argument validation failed: %w", err)
	}

	return t.Function(ctx, normalizedArgs)
}

// normalizeArguments converts arguments to match their expected parameter types
// Supports: string, int, float, bool, json, array
func (t *Tool) normalizeArguments(args map[string]any) map[string]any {
	normalized := make(map[string]any)

	for key, value := range args {
		// Find the parameter definition
		var paramType ParamType
		for _, param := range t.Parameters {
			if param.Name == key {
				paramType = normalizeParamType(param.Type)
				break
			}
		}

		// Normalize based on parameter type
		switch paramType {
		case ParamString:
			// Arrays to comma-separated strings (backward compatibility)
			normalized[key] = t.normalizeToString(value)

		case ParamInt:
			normalized[key] = t.normalizeToInt(value)

		case ParamFloat:
			normalized[key] = t.normalizeToFloat(value)

		case ParamBool:
			normalized[key] = t.normalizeToBool(value)

		case ParamJSON:
			normalized[key] = t.normalizeToJSON(value)

		case ParamArray:
			normalized[key] = t.normalizeToArray(value)

		default:
			normalized[key] = value
		}
	}

	return normalized
}

// normalizeToString converts value to string, handling arrays
func (t *Tool) normalizeToString(value any) any {
	switch v := value.(type) {
	case []interface{}:
		parts := make([]string, len(v))
		for i, val := range v {
			parts[i] = fmt.Sprintf("%v", val)
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(v, ",")
	case []int:
		parts := make([]string, len(v))
		for i, val := range v {
			parts[i] = fmt.Sprintf("%d", val)
		}
		return strings.Join(parts, ",")
	case []float64:
		parts := make([]string, len(v))
		for i, val := range v {
			parts[i] = fmt.Sprintf("%v", val)
		}
		return strings.Join(parts, ",")
	default:
		return value
	}
}

// normalizeToInt converts value to int64
func (t *Tool) normalizeToInt(value any) any {
	switch v := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return parsed
		}
		return value
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		if v <= math.MaxInt64 {
			return int64(v)
		}
		return value
	case float32:
		if v == float32(int64(v)) {
			return int64(v)
		}
		return value
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
		return value
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return i
		}
		return value
	default:
		return value
	}
}

// normalizeToFloat converts value to float64
func (t *Tool) normalizeToFloat(value any) any {
	switch v := value.(type) {
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return parsed
		}
		return value
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return f
		}
		return value
	default:
		return value
	}
}

// normalizeToBool converts value to bool
func (t *Tool) normalizeToBool(value any) any {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		if lower == "true" || lower == "1" {
			return true
		}
		if lower == "false" || lower == "0" {
			return false
		}
		return value
	case int, int8, int16, int32, int64:
		intVal := reflect.ValueOf(v).Int()
		if intVal == 1 {
			return true
		}
		if intVal == 0 {
			return false
		}
		return value
	case uint, uint8, uint16, uint32, uint64:
		uintVal := reflect.ValueOf(v).Uint()
		if uintVal == 1 {
			return true
		}
		if uintVal == 0 {
			return false
		}
		return value
	case float32, float64:
		floatVal := reflect.ValueOf(v).Float()
		if floatVal == 1.0 {
			return true
		}
		if floatVal == 0.0 {
			return false
		}
		return value
	default:
		return value
	}
}

// normalizeToJSON parses JSON strings or passes through maps/slices
func (t *Tool) normalizeToJSON(value any) any {
	switch v := value.(type) {
	case string:
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parsed
		}
		return value
	case map[string]any, []any:
		return v
	default:
		return value
	}
}

// normalizeToArray ensures value is a slice
func (t *Tool) normalizeToArray(value any) any {
	// Already a slice type - keep it
	switch v := value.(type) {
	case []string, []int, []int8, []int16, []int32, []int64,
		[]uint, []uint8, []uint16, []uint32, []uint64, []float32, []float64, []bool:
		return v
	case []interface{}: // []any and []interface{} are the same
		return v
	case string:
		// Split CSV string into array
		parts := strings.Split(v, ",")
		result := make([]string, len(parts))
		for i, part := range parts {
			result[i] = strings.TrimSpace(part)
		}
		return result
	default:
		return value
	}
}
