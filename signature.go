package dsgo

import (
	"fmt"
	"reflect"
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
