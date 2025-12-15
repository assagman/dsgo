package main

import (
	"fmt"

	"github.com/assagman/dsgo"
)

// ConvertSignature converts a YAML SignatureSpec to a DSGo Signature
func ConvertSignature(name string, spec SignatureSpec) (*dsgo.Signature, error) {
	sig := dsgo.NewSignature(spec.Description)

	// Add input fields
	for _, field := range spec.Inputs {
		fieldType, err := parseFieldType(field.Type)
		if err != nil {
			return nil, fmt.Errorf("input field '%s': %w", field.Name, err)
		}

		if field.Optional {
			sig.AddOptionalInput(field.Name, fieldType, field.Description)
		} else {
			sig.AddInput(field.Name, fieldType, field.Description)
		}
	}

	// Add output fields
	for _, field := range spec.Outputs {
		fieldType, err := parseFieldType(field.Type)
		if err != nil {
			return nil, fmt.Errorf("output field '%s': %w", field.Name, err)
		}

		if field.Type == "class" && len(field.Classes) > 0 {
			sig.AddClassOutput(field.Name, field.Classes, field.Description)
		} else if field.Optional {
			sig.AddOptionalOutput(field.Name, fieldType, field.Description)
		} else {
			sig.AddOutput(field.Name, fieldType, field.Description)
		}
	}

	return sig, nil
}

// parseFieldType converts a string type to DSGo FieldType
func parseFieldType(typeStr string) (dsgo.FieldType, error) {
	switch typeStr {
	case "string":
		return dsgo.FieldTypeString, nil
	case "int":
		return dsgo.FieldTypeInt, nil
	case "float":
		return dsgo.FieldTypeFloat, nil
	case "bool":
		return dsgo.FieldTypeBool, nil
	case "json":
		return dsgo.FieldTypeJSON, nil
	case "class":
		return dsgo.FieldTypeClass, nil
	case "image":
		return dsgo.FieldTypeImage, nil
	case "datetime":
		return dsgo.FieldTypeDatetime, nil
	default:
		return "", fmt.Errorf("unknown field type: %s", typeStr)
	}
}

// SignatureRegistry holds converted signatures
type SignatureRegistry struct {
	signatures map[string]*dsgo.Signature
}

// NewSignatureRegistry creates a new registry from YAML specs
func NewSignatureRegistry(specs map[string]SignatureSpec) (*SignatureRegistry, error) {
	registry := &SignatureRegistry{
		signatures: make(map[string]*dsgo.Signature),
	}

	for name, spec := range specs {
		sig, err := ConvertSignature(name, spec)
		if err != nil {
			return nil, fmt.Errorf("failed to convert signature '%s': %w", name, err)
		}
		registry.signatures[name] = sig
	}

	return registry, nil
}

// Get returns a signature by name
func (r *SignatureRegistry) Get(name string) (*dsgo.Signature, error) {
	sig, exists := r.signatures[name]
	if !exists {
		return nil, fmt.Errorf("signature not found: %s", name)
	}
	return sig, nil
}
