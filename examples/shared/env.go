package shared

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnv loads .env.local from the examples directory
func LoadEnv() {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Println("Could not get working directory:", err)
		return
	}

	// Try to find examples directory
	dir := cwd
	for {
		// Check if we're in or have an examples directory
		examplesDir := filepath.Join(dir, "examples")
		if stat, err := os.Stat(examplesDir); err == nil && stat.IsDir() {
			envPath := filepath.Join(examplesDir, ".env.local")
			if err := godotenv.Load(envPath); err != nil {
				log.Printf("No .env.local file found at %s, using environment variables\n", envPath)
			}
			return
		}

		// If we're already in examples directory
		if filepath.Base(dir) == "examples" {
			envPath := filepath.Join(dir, ".env.local")
			if err := godotenv.Load(envPath); err != nil {
				log.Printf("No .env.local file found at %s, using environment variables\n", envPath)
			}
			return
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding examples
			log.Println("Could not find examples directory, using environment variables")
			return
		}
		dir = parent
	}
}

// GetModel returns the model to use from EXAMPLES_MODEL env var, or a default
func GetModel() string {
	model := os.Getenv("EXAMPLES_MODEL")
	if model == "" {
		return "openai/gpt-4o-mini"
	}
	return model
}
