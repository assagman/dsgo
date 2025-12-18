package yamlprogram

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

	return json.MarshalIndent(s, "", "  ")
}
