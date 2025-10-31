package dsgo

import (
	"fmt"
	"reflect"
	"strings"
)

// FieldType represents the type of a signature field
type FieldType string

const (
	FieldTypeString   FieldType = "string"
	FieldTypeInt      FieldType = "int"
	FieldTypeFloat    FieldType = "float"
	FieldTypeBool     FieldType = "bool"
	FieldTypeJSON     FieldType = "json"
	FieldTypeClass    FieldType = "class"
	FieldTypeImage    FieldType = "image"
	FieldTypeDatetime FieldType = "datetime"
)

// Field represents a signature field (input or output)
type Field struct {
	Name         string
	Type         FieldType
	Description  string
	Optional     bool
	Classes      []string          // For class/enum types
	ClassAliases map[string]string // Synonym mapping for class values (e.g., "pos" -> "positive")
}

// Signature defines the structure of inputs and outputs for an LM call
type Signature struct {
	Description  string
	InputFields  []Field
	OutputFields []Field
}

// NewSignature creates a new signature with description
func NewSignature(description string) *Signature {
	return &Signature{
		Description:  description,
		InputFields:  []Field{},
		OutputFields: []Field{},
	}
}

// AddInput adds an input field to the signature
func (s *Signature) AddInput(name string, fieldType FieldType, description string) *Signature {
	s.InputFields = append(s.InputFields, Field{
		Name:        name,
		Type:        fieldType,
		Description: description,
		Optional:    false,
	})
	return s
}

// AddOptionalInput adds an optional input field
func (s *Signature) AddOptionalInput(name string, fieldType FieldType, description string) *Signature {
	s.InputFields = append(s.InputFields, Field{
		Name:        name,
		Type:        fieldType,
		Description: description,
		Optional:    true,
	})
	return s
}

// AddOutput adds an output field to the signature
func (s *Signature) AddOutput(name string, fieldType FieldType, description string) *Signature {
	s.OutputFields = append(s.OutputFields, Field{
		Name:        name,
		Type:        fieldType,
		Description: description,
		Optional:    false,
	})
	return s
}

// AddOptionalOutput adds an optional output field
func (s *Signature) AddOptionalOutput(name string, fieldType FieldType, description string) *Signature {
	s.OutputFields = append(s.OutputFields, Field{
		Name:        name,
		Type:        fieldType,
		Description: description,
		Optional:    true,
	})
	return s
}

// AddClassOutput adds a class/enum output field
func (s *Signature) AddClassOutput(name string, classes []string, description string) *Signature {
	s.OutputFields = append(s.OutputFields, Field{
		Name:        name,
		Type:        FieldTypeClass,
		Description: description,
		Optional:    false,
		Classes:     classes,
	})
	return s
}

// ValidateInputs validates that all required inputs are present and of correct type
func (s *Signature) ValidateInputs(inputs map[string]any) error {
	for _, field := range s.InputFields {
		value, exists := inputs[field.Name]
		if !exists && !field.Optional {
			return fmt.Errorf("missing required input field: %s", field.Name)
		}
		if !exists {
			continue
		}

		// Basic type validation
		if err := s.validateFieldType(field, value); err != nil {
			return err
		}
	}
	return nil
}

// GetOutputField returns the output field with the given name, or nil if not found
func (s *Signature) GetOutputField(name string) *Field {
	for i := range s.OutputFields {
		if s.OutputFields[i].Name == name {
			return &s.OutputFields[i]
		}
	}
	return nil
}

// ValidationDiagnostics contains detailed validation error information
type ValidationDiagnostics struct {
	MissingFields []string         // Required fields that are missing
	TypeErrors    map[string]error // Type validation errors by field name
	ClassErrors   map[string]error // Class/enum validation errors by field name
}

// HasErrors returns true if there are any validation errors
func (d *ValidationDiagnostics) HasErrors() bool {
	return len(d.MissingFields) > 0 || len(d.TypeErrors) > 0 || len(d.ClassErrors) > 0
}

// ValidateOutputs validates that all required outputs are present and of correct type
func (s *Signature) ValidateOutputs(outputs map[string]any) error {
	for _, field := range s.OutputFields {
		value, exists := outputs[field.Name]
		if !exists && !field.Optional {
			return fmt.Errorf("missing required output field: %s", field.Name)
		}
		if !exists {
			continue
		}

		// Validate class types
		if field.Type == FieldTypeClass && len(field.Classes) > 0 {
			valueStr := fmt.Sprintf("%v", value)
			// Normalize the value for comparison
			normalized := normalizeClassValue(valueStr, field)
			valid := false
			for _, class := range field.Classes {
				if normalized == class {
					valid = true
					// Update output with normalized value
					outputs[field.Name] = normalized
					break
				}
			}
			if !valid {
				return fmt.Errorf("field %s has invalid class value: %v (must be one of %v)", field.Name, valueStr, field.Classes)
			}
		}

		// Basic type validation
		if err := s.validateFieldType(field, value); err != nil {
			return err
		}
	}
	return nil
}

// ValidateOutputsPartial performs validation but allows missing fields and captures diagnostics.
// Missing required fields are set to nil in the outputs map.
func (s *Signature) ValidateOutputsPartial(outputs map[string]any) *ValidationDiagnostics {
	diag := &ValidationDiagnostics{
		MissingFields: []string{},
		TypeErrors:    make(map[string]error),
		ClassErrors:   make(map[string]error),
	}

	for _, field := range s.OutputFields {
		value, exists := outputs[field.Name]
		if !exists && !field.Optional {
			diag.MissingFields = append(diag.MissingFields, field.Name)
			outputs[field.Name] = nil // Set to nil for partial output
			continue
		}
		if !exists {
			continue
		}

		// Skip validation if value is nil
		if value == nil {
			continue
		}

		// Validate class types
		if field.Type == FieldTypeClass && len(field.Classes) > 0 {
			valueStr := fmt.Sprintf("%v", value)
			// Normalize the value for comparison
			normalized := normalizeClassValue(valueStr, field)
			valid := false
			for _, class := range field.Classes {
				if normalized == class {
					valid = true
					// Update output with normalized value
					outputs[field.Name] = normalized
					break
				}
			}
			if !valid {
				diag.ClassErrors[field.Name] = fmt.Errorf("invalid class value: %v (must be one of %v)", valueStr, field.Classes)
			}
		}

		// Basic type validation
		if err := s.validateFieldType(field, value); err != nil {
			diag.TypeErrors[field.Name] = err
		}
	}

	return diag
}

func (s *Signature) validateFieldType(field Field, value any) error {
	if value == nil {
		if field.Optional {
			return nil
		}
		return fmt.Errorf("field %s cannot be nil", field.Name)
	}

	kind := reflect.TypeOf(value).Kind()

	switch field.Type {
	case FieldTypeString, FieldTypeClass, FieldTypeImage, FieldTypeDatetime:
		if kind != reflect.String {
			return fmt.Errorf("field %s expected string, got %T", field.Name, value)
		}

	case FieldTypeInt:
		// Accept all int kinds + float64 (adapters coerce to int)
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// OK
		case reflect.Float64:
			// OK - adapters will coerce to int
		default:
			return fmt.Errorf("field %s expected int-like (int/int32/int64/float64), got %T", field.Name, value)
		}

	case FieldTypeFloat:
		// Accept all float kinds + int kinds (can convert to float)
		switch kind {
		case reflect.Float32, reflect.Float64:
			// OK
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// OK - can convert to float
		default:
			return fmt.Errorf("field %s expected float-like (float32/float64/int), got %T", field.Name, value)
		}

	case FieldTypeBool:
		if kind != reflect.Bool {
			return fmt.Errorf("field %s expected bool, got %T", field.Name, value)
		}

	case FieldTypeJSON:
		// Accept map, slice, or string (JSON)
		switch kind {
		case reflect.Map, reflect.Slice, reflect.String:
			// OK
		default:
			return fmt.Errorf("field %s expected JSON (map/slice/string), got %T", field.Name, value)
		}
	}
	return nil
}

// normalizeClassValue normalizes a class value for comparison using case-insensitive matching and aliases
func normalizeClassValue(value string, field Field) string {
	v := strings.ToLower(strings.TrimSpace(value))

	// Check if there's an exact case-insensitive match first
	for _, class := range field.Classes {
		if strings.EqualFold(v, class) {
			return class
		}
	}

	// Check aliases
	if field.ClassAliases != nil {
		if normalized, ok := field.ClassAliases[v]; ok {
			return normalized
		}
	}

	// Return original if no match found
	return value
}

// SignatureToJSONSchema generates a JSON schema from the signature's output fields
// This enables structured output mode for OpenAI/OpenRouter compatible LMs
func (s *Signature) SignatureToJSONSchema() map[string]any {
	properties := make(map[string]any)
	required := []string{}

	for _, field := range s.OutputFields {
		prop := make(map[string]any)

		// Map DSGo field types to JSON schema types
		switch field.Type {
		case FieldTypeString, FieldTypeImage, FieldTypeDatetime:
			prop["type"] = "string"
		case FieldTypeInt:
			prop["type"] = "integer"
		case FieldTypeFloat:
			prop["type"] = "number"
		case FieldTypeBool:
			prop["type"] = "boolean"
		case FieldTypeJSON:
			prop["type"] = "object"
		case FieldTypeClass:
			prop["type"] = "string"
			if len(field.Classes) > 0 {
				prop["enum"] = field.Classes
			}
		default:
			prop["type"] = "string" // Fallback to string
		}

		// Add description if present
		if field.Description != "" {
			prop["description"] = field.Description
		}

		properties[field.Name] = prop

		// Track required fields
		if !field.Optional {
			required = append(required, field.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	// Add description if present
	if s.Description != "" {
		schema["description"] = s.Description
	}

	return schema
}
