//go:build ignore

// Generator for dsgo-program.schema.json.
// Run via: go generate ./internal/yamlprogram
package main

import (
	"log"
	"os"

	"github.com/assagman/dsgo/internal/yamlprogram"
)

func main() {
	b, err := yamlprogram.SchemaJSON()
	if err != nil {
		log.Fatalf("failed to generate schema: %v", err)
	}

	outPath := "dsgo-program.schema.json"
	if err := os.WriteFile(outPath, b, 0o644); err != nil {
		log.Fatalf("failed to write schema: %v", err)
	}

	log.Printf("wrote %s (%d bytes)", outPath, len(b))
}
