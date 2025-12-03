# Shared Tools for DSGo Examples

This package contains reusable tools that can be used across all DSGo examples. The tools are designed with security and ease of use in mind.

## Design Principles

1. **Project Root Detection**: Tools automatically detect the project root by finding the `dsgo.go` file
2. **Security by Default**: All operations are constrained to the project root and its subdirectories
3. **Zero Configuration**: No need to specify base directories or project paths
4. **Consistent Interface**: All tools follow the same parameter and response patterns

## Available Tools

### Filesystem Tools (`tools/filesystem.go`)

#### `list_files`
Lists files and directories in a given path up to a specified depth.

**Parameters:**
- `directory` (string, optional): Directory path to list (relative to project root, defaults to project root)
- `depth` (int, optional): Maximum depth to traverse (default: 3)

**Returns:**
```json
{
  "files": ["file1.go", "dir1/", "dir2/file2.txt"],
  "directory": "/absolute/path/to/directory"
}
```

**Example Usage:**
```go
// List all files in project root with default depth
tools := tools.GetAllFilesystemTools()

// List files in specific directory with custom depth
// LM would call: list_files({"directory": "internal/core", "depth": 2})
```

#### `read_file`
Reads the content of a specific file.

**Parameters:**
- `filepath` (string, required): Path to the file to read (relative to project root)

**Returns:**
```json
{
  "content": "file content here...",
  "filepath": "/absolute/path/to/file"
}
```

**Security Notes:**
- Content is limited to 10KB to prevent overwhelming the LM
- Files outside project root are automatically rejected

**Example Usage:**
```go
// Read a specific file
// LM would call: read_file({"filepath": "go.mod"})
```

#### `search_files`
Searches for files matching a glob pattern.

**Parameters:**
- `directory` (string, optional): Directory to search in (relative to project root, defaults to project root)
- `pattern` (string, required): Glob pattern to match (e.g., `*.go`, `**/*.txt`)

**Returns:**
```json
{
  "files": ["file1.go", "dir1/file2.go"],
  "directory": "/absolute/path/to/directory",
  "pattern": "*.go"
}
```

**Example Usage:**
```go
// Find all Go files in project
// LM would call: search_files({"pattern": "*.go"})

// Find all test files in specific directory
// LM would call: search_files({"directory": "internal/core", "pattern": "*_test.go"})
```

## How to Use

### Import the Package
```go
import "github.com/assagman/dsgo/examples/shared/tools"
```

### Get All Filesystem Tools
```go
// Get all available filesystem tools
fsTools := tools.GetAllFilesystemTools()

// Use with ReAct module
react := dsgo.NewReAct(signature, lm, dsgo.WithTools(fsTools))
```

### Get Individual Tools
```go
// Get specific tools only
listTool := tools.NewListFilesTool()
readTool := tools.NewReadFileTool()
searchTool := tools.NewSearchFilesTool()

customTools := []dsgo.Tool{listTool, readTool}
```

## Security Features

### Project Root Enforcement
- All operations are automatically constrained to the project root
- Project root is detected by finding the `dsgo.go` file
- Attempts to access files outside the project root are rejected with clear error messages

### Path Validation
- User-provided paths are validated and sanitized
- Relative paths are resolved against the project root
- Path traversal attacks (e.g., `../../../etc/passwd`) are prevented

### Symlink Protection
- Symlinks are resolved to their final destinations
- Symlinks pointing outside the project root are rejected
- This prevents symlink escape attacks

## Error Handling

The tools provide clear, descriptive error messages:

- **Project root not found**: "project root not found: dsgo.go file not located in any parent directory"
- **Access denied**: "access denied: path '/etc/passwd' is outside project root"
- **Missing parameters**: "filepath parameter is required"
- **File not found**: "error reading file: open /path/to/file: no such file or directory"

## Implementation Details

### Project Root Detection
The `FindProjectRoot()` function:
1. Gets the current working directory
2. Searches upward through parent directories
3. Looks for `dsgo.go` file in each directory
4. Returns the first directory containing `dsgo.go`

### Path Validation
The `validatePath()` function:
1. Converts relative paths to absolute paths
2. Resolves `.` and `..` components
3. Ensures the final path is within project root
4. Returns the validated absolute path

## Future Extensions

This shared tools package is designed to be extensible. Future tool categories might include:

- **Git Tools**: `git_status`, `git_diff`, `git_log`
- **Network Tools**: `http_get`, `http_post` (with proper security constraints)
- **Build Tools**: `run_tests`, `build_project`
- **Analysis Tools**: `code_metrics`, `dependency_graph`

## Contributing

When adding new tools:

1. Follow the same security principles (project root enforcement)
2. Use consistent parameter naming and response formats
3. Provide clear error messages
4. Add comprehensive documentation
5. Include usage examples
6. Test with various edge cases

## Examples

See the `examples/package_analysis/` directory for a complete example of how to use these shared tools in practice.
