package yamlprogram

//go:generate go run gen_schema.go

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// SchemaJSON returns a JSON Schema (draft 2020-12) for the YAML program spec.
//
// Editors can use this to provide autocomplete, docs, and validation.
// For VS Code YAML extension, you can associate this schema with your pipeline YAML.
func SchemaJSON() ([]byte, error) {
	r := &jsonschema.Reflector{}
	s := r.Reflect(&Spec{})

	// Helpful defaults for editors.
	if s.ID == "" {
		s.ID = "https://github.com/assagman/dsgo/internal/yamlprogram.schema.json"
	}
	if s.Title == "" {
		s.Title = "DSGo YAML Program Spec"
	}

	// Fix Duration: accepts string ("30m", "5s") or integer (seconds).
	// The reflector doesn't understand custom UnmarshalYAML.
	if defs := s.Definitions; defs != nil {
		if dur, ok := defs["Duration"]; ok {
			dur.Type = "integer"
			dur.Description = "Duration in seconds"
			dur.Properties = nil
			dur.AdditionalProperties = nil
			dur.Required = nil
		}

		// Fix FieldSpec: accepts string shorthand ("string") or object with type/desc/values.
		if fs, ok := defs["FieldSpec"]; ok {
			// Save the original object schema.
			objSchema := &jsonschema.Schema{
				Type:                 "object",
				Properties:           fs.Properties,
				AdditionalProperties: fs.AdditionalProperties,
				Required:             fs.Required,
			}
			// Clear and set oneOf.
			fs.Type = ""
			fs.Properties = nil
			fs.AdditionalProperties = nil
			fs.Required = nil
			fs.OneOf = []*jsonschema.Schema{
				{Type: "string", Description: "Shorthand: field type (string, int, float, bool, json, image, datetime, enum)"},
				objSchema,
			}
		}
	}

	return json.MarshalIndent(s, "", "  ")
}
