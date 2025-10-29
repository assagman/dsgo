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
	Name        string
	Type        FieldType
	Description string
	Optional    bool
	Classes     []string // For class/enum types
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

// BuildPrompt constructs a prompt from the signature
func (s *Signature) BuildPrompt(inputs map[string]any) (string, error) {
	var prompt strings.Builder

	// Add description
	if s.Description != "" {
		prompt.WriteString(s.Description)
		prompt.WriteString("\n\n")
	}

	// Add input fields
	if len(s.InputFields) > 0 {
		prompt.WriteString("--- Inputs ---\n")
		for _, field := range s.InputFields {
			value, exists := inputs[field.Name]
			if !exists {
				return "", fmt.Errorf("missing required input field: %s", field.Name)
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("%s (%s): %v\n", field.Name, field.Description, value))
			} else {
				prompt.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value))
			}
		}
		prompt.WriteString("\n")
	}

	// Add output format specification
	if len(s.OutputFields) > 0 {
		prompt.WriteString("--- Required Output Format ---\n")
		prompt.WriteString("Respond with a JSON object containing the following fields:\n")
		for _, field := range s.OutputFields {
			optional := ""
			if field.Optional {
				optional = " (optional)"
			}
			classInfo := ""
			if field.Type == FieldTypeClass && len(field.Classes) > 0 {
				classInfo = fmt.Sprintf(" - MUST be exactly one of: %s", strings.Join(field.Classes, ", "))
			}
			if field.Description != "" {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s: %s\n", field.Name, field.Type, optional, classInfo, field.Description))
			} else {
				prompt.WriteString(fmt.Sprintf("- %s (%s)%s%s\n", field.Name, field.Type, optional, classInfo))
			}
		}
		prompt.WriteString("\nIMPORTANT: Return ONLY a valid JSON object. Do not include any code blocks, explanations, or additional text.\n")
	}

	return prompt.String(), nil
}

// ValidateInputs validates that all required inputs are present
func (s *Signature) ValidateInputs(inputs map[string]any) error {
	for _, field := range s.InputFields {
		if _, exists := inputs[field.Name]; !exists {
			return fmt.Errorf("missing required input field: %s", field.Name)
		}
	}
	return nil
}

// GetOutputField returns the output field with the given name, or nil if not found
func (s *Signature) GetOutputField(name string) *Field {
	for _, field := range s.OutputFields {
		if field.Name == name {
			return &field
		}
	}
	return nil
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
			valid := false
			for _, class := range field.Classes {
				if valueStr == class {
					valid = true
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

func (s *Signature) validateFieldType(field Field, value any) error {
	if value == nil {
		if field.Optional {
			return nil
		}
		return fmt.Errorf("field %s cannot be nil", field.Name)
	}

	switch field.Type {
	case FieldTypeString, FieldTypeClass, FieldTypeImage, FieldTypeDatetime:
		if reflect.TypeOf(value).Kind() != reflect.String {
			return fmt.Errorf("field %s expected string, got %T", field.Name, value)
		}
	case FieldTypeInt:
		kind := reflect.TypeOf(value).Kind()
		if kind != reflect.Int && kind != reflect.Int64 && kind != reflect.Int32 && kind != reflect.Float64 {
			return fmt.Errorf("field %s expected int, got %T", field.Name, value)
		}
	case FieldTypeFloat:
		kind := reflect.TypeOf(value).Kind()
		if kind != reflect.Float64 && kind != reflect.Float32 && kind != reflect.Int && kind != reflect.Int64 {
			return fmt.Errorf("field %s expected float, got %T", field.Name, value)
		}
	case FieldTypeBool:
		if reflect.TypeOf(value).Kind() != reflect.Bool {
			return fmt.Errorf("field %s expected bool, got %T", field.Name, value)
		}
	}
	return nil
}
