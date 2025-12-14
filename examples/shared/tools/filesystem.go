package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/assagman/dsgo"
)

// NewListFilesTool creates a tool for listing files and directories in a given path up to a specified depth.
// The tool automatically detects the project root and constrains all operations to within project boundaries.
func NewListFilesTool() dsgo.Tool {
	listFiles := func(ctx context.Context, args map[string]any) (any, error) {
		fmt.Printf("🛠️  Tool Call: list_files %v\n", args)

		// Get project root
		projectRoot, err := FindProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}

		// Get directory parameter (defaults to project root)
		directory := projectRoot
		if dirArg, ok := args["directory"].(string); ok && dirArg != "" {
			directory, err = validatePath(projectRoot, dirArg)
			if err != nil {
				return nil, err
			}
		}

		// Get depth parameter (defaults to 3)
		depth := 3
		if depthVal, ok := args["depth"].(float64); ok {
			depth = int(depthVal)
		}

		var files []string
		err = filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Calculate relative depth
			relPath, err := filepath.Rel(projectRoot, path)
			if err != nil {
				return err
			}
			currentDepth := len(strings.Split(relPath, string(os.PathSeparator)))

			if currentDepth > depth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking directory: %w", err)
		}

		return map[string]any{
			"files":     files,
			"directory": directory,
		}, nil
	}

	return *dsgo.NewTool("list_files", "List files and directories in a given path up to a specified depth", listFiles).
		AddParameter("directory", "string", "The directory path to list (relative to project root, defaults to project root)", false).
		AddParameter("depth", "int", "Maximum depth to traverse (default: 3)", false)
}

// NewReadFileTool creates a tool for reading the content of a specific file.
// The tool automatically detects the project root and ensures files are within project boundaries.
func NewReadFileTool() dsgo.Tool {
	readFile := func(ctx context.Context, args map[string]any) (any, error) {
		fmt.Printf("🛠️  Tool Call: read_file %v\n", args)

		// Get project root
		projectRoot, err := FindProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}

		// Get filepath parameter (required)
		filepathArg, ok := args["filepath"].(string)
		if !ok || filepathArg == "" {
			return nil, fmt.Errorf("filepath parameter is required")
		}

		// Validate and sanitize the file path
		filepathArg, err = validatePath(projectRoot, filepathArg)
		if err != nil {
			return nil, err
		}

		// Read file content
		content, err := os.ReadFile(filepathArg)
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		// Calculate file metadata
		totalBytes := len(content)
		totalLines := strings.Count(string(content), "\n")
		if totalBytes > 0 && content[totalBytes-1] != '\n' {
			totalLines++
		}

		// Limit content size to prevent overwhelming the LM
		contentStr := string(content)
		truncated := false
		if len(contentStr) > 10000 { // 10KB limit
			contentStr = contentStr[:10000]
			truncated = true
			truncatedLines := strings.Count(contentStr, "\n")
			contentStr += fmt.Sprintf("\n... [truncated: showing %d/%d bytes, ~%d/%d lines]",
				10000, totalBytes, truncatedLines, totalLines)
		}

		result := map[string]any{
			"content":     contentStr,
			"filepath":    filepathArg,
			"size_bytes":  totalBytes,
			"total_lines": totalLines,
			"truncated":   truncated,
		}
		return result, nil
	}

	return *dsgo.NewTool("read_file", "Read the content of a specific file", readFile).
		AddParameter("filepath", "string", "The path to the file to read (relative to project root)", true)
}

// NewSearchFilesTool creates a tool for searching files matching a glob pattern.
// The tool automatically detects the project root and constrains searches to within project boundaries.
func NewSearchFilesTool() dsgo.Tool {
	searchFiles := func(ctx context.Context, args map[string]any) (any, error) {
		fmt.Printf("🛠️  Tool Call: search_files %v\n", args)

		// Get project root
		projectRoot, err := FindProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}

		// Get directory parameter (defaults to project root)
		directory := projectRoot
		if dirArg, ok := args["directory"].(string); ok && dirArg != "" {
			directory, err = validatePath(projectRoot, dirArg)
			if err != nil {
				return nil, err
			}
		}

		// Get pattern parameter (required)
		pattern, ok := args["pattern"].(string)
		if !ok || pattern == "" {
			return nil, fmt.Errorf("pattern parameter is required")
		}

		// Construct the full pattern
		fullPattern := filepath.Join(directory, pattern)

		// Search for files matching the pattern
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("error searching files: %w", err)
		}

		// Convert to relative paths for cleaner output
		relativeMatches := make([]string, len(matches))
		for i, match := range matches {
			relativePath, err := filepath.Rel(projectRoot, match)
			if err != nil {
				// Fallback to absolute path if relative conversion fails
				relativeMatches[i] = match
			} else {
				relativeMatches[i] = relativePath
			}
		}

		return map[string]any{
			"files":     relativeMatches,
			"directory": directory,
			"pattern":   pattern,
		}, nil
	}

	return *dsgo.NewTool("search_files", "Search for files matching a glob pattern", searchFiles).
		AddParameter("directory", "string", "The directory to search in (relative to project root, defaults to project root)", false).
		AddParameter("pattern", "string", "Glob pattern to match (e.g., *.go, **/*.txt)", true)
}

// GetAllFilesystemTools returns all available filesystem tools as a slice.
// This is the recommended way to get all filesystem tools for use in examples.
func GetAllFilesystemTools() []dsgo.Tool {
	return []dsgo.Tool{
		NewListFilesTool(),
		NewReadFileTool(),
		NewSearchFilesTool(),
	}
}
