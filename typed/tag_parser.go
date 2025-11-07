package typed

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/assagman/dsgo/core"
)

// FieldInfo contains parsed information from a struct field
type FieldInfo struct {
	Name         string
	Type         core.FieldType
	Description  string
	Optional     bool
	Classes      []string
	ClassAliases map[string]string
	IsInput      bool
	IsOutput     bool
}

// ParseStructTags parses dsgo tags from a struct type and returns field information
// Tag format: `dsgo:"input|output[,optional][,desc=...][,enum=val1|val2|val3][,alias:short=long]"`
func ParseStructTags(structType reflect.Type) ([]FieldInfo, error) {
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct type, got %s", structType.Kind())
	}

	var fields []FieldInfo

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("dsgo")
		if tag == "" {
			continue // Skip fields without dsgo tag
		}

		info, err := parseFieldTag(field.Name, field.Type, tag)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		fields = append(fields, info)
	}

	return fields, nil
}

// parseFieldTag parses a single field's dsgo tag
func parseFieldTag(fieldName string, fieldType reflect.Type, tag string) (FieldInfo, error) {
	info := FieldInfo{
		Name:         fieldName,
		ClassAliases: make(map[string]string),
	}

	parts := splitTag(tag)
	if len(parts) == 0 {
		return info, fmt.Errorf("empty tag")
	}

	// First part must be "input" or "output"
	switch parts[0] {
	case "input":
		info.IsInput = true
	case "output":
		info.IsOutput = true
	default:
		return info, fmt.Errorf("tag must start with 'input' or 'output', got '%s'", parts[0])
	}

	// Parse remaining parts
	for _, part := range parts[1:] {
		if part == "optional" {
			info.Optional = true
			continue
		}

		if strings.HasPrefix(part, "desc=") {
			info.Description = strings.TrimPrefix(part, "desc=")
			continue
		}

		if strings.HasPrefix(part, "enum=") {
			enumStr := strings.TrimPrefix(part, "enum=")
			info.Classes = strings.Split(enumStr, "|")
			continue
		}

		if strings.HasPrefix(part, "alias:") {
			aliasStr := strings.TrimPrefix(part, "alias:")
			aliasParts := strings.Split(aliasStr, "=")
			if len(aliasParts) == 2 {
				info.ClassAliases[aliasParts[0]] = aliasParts[1]
			}
			continue
		}

		// Unknown option, ignore for forward compatibility
	}

	// Infer DSGo field type from Go type
	info.Type = inferFieldType(fieldType, info.Classes)

	return info, nil
}

// inferFieldType maps Go types to DSGo field types
func inferFieldType(goType reflect.Type, classes []string) core.FieldType {
	// If enum is specified, it's a class type
	if len(classes) > 0 {
		return core.FieldTypeClass
	}

	switch goType.Kind() {
	case reflect.String:
		return core.FieldTypeString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return core.FieldTypeInt
	case reflect.Float32, reflect.Float64:
		return core.FieldTypeFloat
	case reflect.Bool:
		return core.FieldTypeBool
	case reflect.Map, reflect.Slice, reflect.Struct:
		return core.FieldTypeJSON
	default:
		return core.FieldTypeString // Default fallback
	}
}

// splitTag splits a tag string by commas, respecting quoted values
func splitTag(tag string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for _, ch := range tag {
		switch ch {
		case ',':
			if !inQuotes {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(ch)
			}
		case '"', '\'':
			inQuotes = !inQuotes
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
