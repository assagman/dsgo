package yamlprogram

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFile loads and validates a YAML program spec from a file.
func LoadFile(path string) (*Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml spec %q: %w", path, err)
	}
	return Load(bytes.NewReader(b))
}

// Load loads and validates a YAML program spec.
//
// Decoding is strict: unknown fields are rejected.
func Load(r io.Reader) (*Spec, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)

	var spec Spec
	if err := dec.Decode(&spec); err != nil {
		return nil, fmt.Errorf("decode yaml spec: %w", err)
	}

	Normalize(&spec)
	if err := Validate(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}
