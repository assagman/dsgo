package typed

import (
	"fmt"
	"reflect"

	"github.com/assagman/dsgo"
)

// StructToSignature converts a struct type with dsgo tags to a Signature
func StructToSignature(structType reflect.Type, description string) (*dsgo.Signature, error) {
	fields, err := ParseStructTags(structType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse struct tags: %w", err)
	}

	sig := dsgo.NewSignature(description)

	for _, field := range fields {
		f := dsgo.Field{
			Name:         field.Name,
			Type:         field.Type,
			Description:  field.Description,
			Optional:     field.Optional,
			Classes:      field.Classes,
			ClassAliases: field.ClassAliases,
		}

		if field.IsInput {
			sig.InputFields = append(sig.InputFields, f)
		}
		if field.IsOutput {
			sig.OutputFields = append(sig.OutputFields, f)
		}
	}

	return sig, nil
}

// StructToMap converts a struct instance to a map[string]any for use with dsgo modules
func StructToMap(v any) (map[string]any, error) {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	// Handle pointers
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", val.Kind())
	}

	result := make(map[string]any)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Only include fields with dsgo tags
		tag := field.Tag.Get("dsgo")
		if tag == "" {
			continue
		}

		result[field.Name] = val.Field(i).Interface()
	}

	return result, nil
}

// MapToStruct populates a struct from a map[string]any
func MapToStruct(m map[string]any, target any) error {
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer to struct")
	}

	val = val.Elem()
	typ := val.Type()

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct, got pointer to %s", val.Kind())
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Only populate fields with dsgo tags
		tag := field.Tag.Get("dsgo")
		if tag == "" {
			continue
		}

		value, exists := m[field.Name]
		if !exists {
			continue // Skip missing fields
		}

		if value == nil {
			continue // Skip nil values
		}

		// Set the field value
		fieldVal := val.Field(i)
		if !fieldVal.CanSet() {
			continue
		}

		// Convert value to correct type
		convertedVal := reflect.ValueOf(value)
		if convertedVal.Type().AssignableTo(fieldVal.Type()) {
			fieldVal.Set(convertedVal)
		} else if convertedVal.Type().ConvertibleTo(fieldVal.Type()) {
			fieldVal.Set(convertedVal.Convert(fieldVal.Type()))
		} else {
			return fmt.Errorf("cannot assign %s to field %s of type %s", convertedVal.Type(), field.Name, fieldVal.Type())
		}
	}

	return nil
}
