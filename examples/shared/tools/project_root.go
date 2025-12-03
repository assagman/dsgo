package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindProjectRoot locates the project root by searching for dsgo.go file.
// It searches upward from the current directory until it finds dsgo.go.
// Returns an error if dsgo.go is not found (should never happen in valid examples).
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Search upward from current directory
	for {
		dsgoPath := filepath.Join(dir, "dsgo.go")
		if _, err := os.Stat(dsgoPath); err == nil {
			// Found dsgo.go, return this directory as project root
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding dsgo.go
			return "", fmt.Errorf("project root not found: dsgo.go file not located in any parent directory")
		}
		dir = parent
	}
}

// validatePath ensures that the requested path is within the project root.
// Returns the absolute, validated path or an error if the path is outside project bounds.
func validatePath(projectRoot, requestedPath string) (string, error) {
	// Convert to absolute path
	if !filepath.IsAbs(requestedPath) {
		requestedPath = filepath.Join(projectRoot, requestedPath)
	}

	// Clean the path to resolve any .. or . components
	requestedPath = filepath.Clean(requestedPath)

	// Ensure the path is within project root
	relPath, err := filepath.Rel(projectRoot, requestedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check if the relative path starts with ".." (indicating it's outside project root)
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." {
		return "", fmt.Errorf("access denied: path '%s' is outside project root", requestedPath)
	}

	return requestedPath, nil
}
