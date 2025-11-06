package shared

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/assagman/dsgo"
	"github.com/joho/godotenv"
)

// LoadEnv loads .env.local or .env from the examples directory
// Priority: .env.local (local development) > .env (CI/shared) > existing env vars
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
			loadEnvFiles(examplesDir)
			return
		}

		// If we're already in examples directory
		if filepath.Base(dir) == "examples" {
			loadEnvFiles(dir)
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

// loadEnvFiles tries to load .env.local first, then .env
func loadEnvFiles(dir string) {
	// Try .env.local first (for local development)
	envLocalPath := filepath.Join(dir, ".env.local")
	if err := godotenv.Load(envLocalPath); err == nil {
		return // Successfully loaded .env.local
	}

	// Fall back to .env (for CI or shared config)
	envPath := filepath.Join(dir, ".env")
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("No .env or .env.local file found in %s, using environment variables\n", dir)
	}
}

// GetModel returns the model to use from env vars, or a default
// Priority: EXAMPLES_DEFAULT_MODEL (test matrix) > EXAMPLES_MODEL (manual) > default
func GetModel() string {
	// Check EXAMPLES_DEFAULT_MODEL first (used by test matrix)
	if model := os.Getenv("EXAMPLES_DEFAULT_MODEL"); model != "" {
		return model
	}
	// Check EXAMPLES_MODEL (manual override)
	if model := os.Getenv("EXAMPLES_MODEL"); model != "" {
		return model
	}
	// Default: fast, reliable model for examples
	return "openrouter/google/gemini-2.5-flash"
}

// GetDefaultOptions returns default GenerateOptions for examples
// Can be overridden via environment variables:
// - EXAMPLES_MAX_TOKENS (default: 10000)
// - EXAMPLES_TEMPERATURE (default: 0.7)
func GetDefaultOptions() *dsgo.GenerateOptions {
	opts := dsgo.DefaultGenerateOptions()

	// Max tokens
	if maxTokensStr := os.Getenv("EXAMPLES_MAX_TOKENS"); maxTokensStr != "" {
		if maxTokens, err := strconv.Atoi(maxTokensStr); err == nil && maxTokens > 0 {
			opts.MaxTokens = maxTokens
		}
	} else {
		opts.MaxTokens = 100000 // Default for examples
	}

	// Temperature
	if tempStr := os.Getenv("EXAMPLES_TEMPERATURE"); tempStr != "" {
		if temp, err := strconv.ParseFloat(tempStr, 64); err == nil && temp >= 0 {
			opts.Temperature = temp
		}
	} else {
		opts.Temperature = 0.7 // Default for examples
	}

	return opts
}
