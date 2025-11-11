// Package env provides functionality for loading environment variables from .env files.
// It supports loading .env and .env.local files with the same precedence as godotenv.
package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load loads environment variables from the specified .env file.
// It mimics the behavior of github.com/joho/godotenv.Load().
func Load(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle export prefix (bash style)
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(line[7:])
		}

		// Split on first = sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Only set if not already set
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("failed to set environment variable %s: %w", key, err)
			}
		}
	}

	return scanner.Err()
}

// LoadFiles loads environment variables from .env and .env.local files.
// It searches for files in the current directory and walks up the directory tree.
// .env.local takes precedence over .env files.
func LoadFiles() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Search for .env.local and .env files
	envLocalPath := findEnvFile(cwd, ".env.local")
	envPath := findEnvFile(cwd, ".env")

	// Load .env.local first (higher precedence)
	// This ensures .env.local values override .env values
	if envLocalPath != "" {
		if err := Load(envLocalPath); err != nil {
			return fmt.Errorf("failed to load .env.local file: %w", err)
		}
	}

	// Load .env second (lower precedence, only fills in missing values)
	if envPath != "" {
		if err := Load(envPath); err != nil {
			return fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	return nil
}

// findEnvFile searches for an environment file starting from the given directory
// and walking up the directory tree.
func findEnvFile(startDir, filename string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Also check current directory
	candidate := filepath.Join(startDir, filename)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	return ""
}

// AutoLoad automatically loads environment variables from .env files.
// It checks DSGO_ENV_FILE_PATH first, then falls back to searching for
// .env.local and .env files in the current directory and parent directories.
// If no files are found, it returns nil (no error).
// Already-set environment variables are never overwritten.
func AutoLoad() error {
	// Check DSGO_ENV_FILE_PATH first
	if customPath := os.Getenv("DSGO_ENV_FILE_PATH"); customPath != "" {
		if err := Load(customPath); err != nil {
			// Only return error if file explicitly specified but can't be loaded
			return fmt.Errorf("failed to load DSGO_ENV_FILE_PATH (%s): %w", customPath, err)
		}
		return nil
	}

	// Fall back to automatic discovery
	if err := LoadFiles(); err != nil {
		// LoadFiles only returns errors for actual load failures, not missing files
		return err
	}

	return nil
}
