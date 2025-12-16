package main

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/assagman/dsgo"
)

// MCPClientRegistry holds initialized MCP clients
type MCPClientRegistry struct {
	clients map[string]*dsgo.MCPClient
	specs   map[string]MCPSpec
}

// NewMCPClientRegistry creates MCP clients from YAML specs
func NewMCPClientRegistry(ctx context.Context, specs map[string]MCPSpec, timeouts TimeoutSettings) (*MCPClientRegistry, error) {
	registry := &MCPClientRegistry{
		clients: make(map[string]*dsgo.MCPClient),
		specs:   specs,
	}

	for name, spec := range specs {
		client, err := createMCPClient(ctx, name, spec, timeouts)
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP client '%s': %w", name, err)
		}
		registry.clients[name] = client
	}

	return registry, nil
}

// GetSpec returns the spec for an MCP client
func (r *MCPClientRegistry) GetSpec(name string) (MCPSpec, bool) {
	spec, exists := r.specs[name]
	return spec, exists
}

// Clients returns all client names
func (r *MCPClientRegistry) Clients() []string {
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	return names
}

// Get returns an MCP client by name
func (r *MCPClientRegistry) Get(name string) (*dsgo.MCPClient, error) {
	client, exists := r.clients[name]
	if !exists {
		return nil, fmt.Errorf("MCP client not found: %s", name)
	}
	return client, nil
}

// createMCPClient creates an MCP client from a spec.
//
// We build clients via explicit transports so the YAML runner can override
// timeouts without relying on environment variables.
func createMCPClient(ctx context.Context, name string, spec MCPSpec, timeouts TimeoutSettings) (*dsgo.MCPClient, error) {
	apiKey := spec.APIKey
	if apiKey == "" {
		apiKey = getAPIKeyFromEnv(spec.Type)
	}

	httpTimeout := timeouts.MCPHTTP.Duration
	postTimeout := timeouts.MCPSSEPost.Duration
	waitTimeout := timeouts.MCPSSEWait.Duration

	var transport dsgo.MCPTransport
	switch spec.Type {
	case "exa":
		transport = dsgo.NewMCPHTTPTransportWithTimeout("https://mcp.exa.ai/mcp", apiKey, httpTimeout)
	case "jina":
		transport = dsgo.NewMCPSSETransportWithTimeouts("https://mcp.jina.ai/sse", apiKey, postTimeout, waitTimeout)
	case "tavily":
		baseURL, err := url.Parse("https://mcp.tavily.com/mcp")
		if err != nil {
			return nil, fmt.Errorf("failed to parse Tavily MCP URL: %w", err)
		}
		q := baseURL.Query()
		q.Set("tavilyApiKey", apiKey)
		baseURL.RawQuery = q.Encode()

		// Tavily uses the query param for auth, not headers.
		transport = dsgo.NewMCPHTTPTransportWithTimeout(baseURL.String(), "", httpTimeout)
	case "shell":
		projectRoot, err := findProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}
		shellServer, err := dsgo.NewMCPShellServer(dsgo.MCPShellServerConfig{RootDir: projectRoot})
		if err != nil {
			return nil, fmt.Errorf("failed to create shell MCP server: %w", err)
		}
		transport, err = dsgo.NewMCPLocalTransport(shellServer)
		if err != nil {
			return nil, fmt.Errorf("failed to create local transport: %w", err)
		}
	case "filesystem":
		projectRoot, err := findProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}
		allowedDir := projectRoot
		if len(spec.AllowedDirs) > 0 {
			allowedDir = spec.AllowedDirs[0] // Use first directory as primary
		}
		client, err := dsgo.NewMCPFilesystemClient(allowedDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create filesystem MCP client: %w", err)
		}
		if err := client.Initialize(ctx); err != nil {
			return nil, fmt.Errorf("failed to initialize filesystem MCP client: %w", err)
		}
		return client, nil
	case "custom":
		transport = dsgo.NewMCPHTTPTransportWithTimeout(spec.URL, apiKey, httpTimeout)
	default:
		return nil, fmt.Errorf("unsupported MCP type: %s", spec.Type)
	}

	client, err := dsgo.NewMCPClient(dsgo.MCPClientConfig{Transport: transport})
	if err != nil {
		return nil, err
	}

	if err := client.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return client, nil
}

// getAPIKeyFromEnv returns the API key from environment variable based on MCP type
func getAPIKeyFromEnv(mcpType string) string {
	envVars := map[string]string{
		"exa":    "EXA_API_KEY",
		"jina":   "JINA_API_KEY",
		"tavily": "TAVILY_API_KEY",
	}

	if envVar, exists := envVars[mcpType]; exists {
		return os.Getenv(envVar)
	}
	return ""
}

// ToolRegistry holds resolved DSGo tools (custom tools only, not MCP)
type ToolRegistry struct {
	tools       map[string]dsgo.Tool
	mcpRegistry *MCPClientRegistry
}

// NewToolRegistry creates tools from YAML specs (custom tools only)
func NewToolRegistry(ctx context.Context, specs map[string]ToolSpec, mcpRegistry *MCPClientRegistry) (*ToolRegistry, error) {
	registry := &ToolRegistry{
		tools:       make(map[string]dsgo.Tool),
		mcpRegistry: mcpRegistry,
	}

	for name, spec := range specs {
		tool, err := createTool(name, spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create tool '%s': %w", name, err)
		}
		registry.tools[name] = tool
	}

	return registry, nil
}

// Get returns a tool by name
func (r *ToolRegistry) Get(name string) (dsgo.Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return dsgo.Tool{}, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

// GetMultiple returns multiple tools by name
func (r *ToolRegistry) GetMultiple(names []string) ([]dsgo.Tool, error) {
	result := make([]dsgo.Tool, 0, len(names))
	for _, name := range names {
		tool, err := r.Get(name)
		if err != nil {
			return nil, err
		}
		result = append(result, tool)
	}
	return result, nil
}

// GetMCPTools returns tools from an MCP client, filtered by the tool list.
// If toolFilters contains "*", all tools from that MCP client are returned.
func (r *ToolRegistry) GetMCPTools(mcpName string, toolFilters []string) ([]dsgo.Tool, error) {
	if r.mcpRegistry == nil {
		return nil, fmt.Errorf("no MCP registry available")
	}

	client, err := r.mcpRegistry.Get(mcpName)
	if err != nil {
		return nil, err
	}

	allTools := client.GetTools()

	// Check for wildcard
	for _, filter := range toolFilters {
		if filter == "*" {
			return allTools, nil
		}
	}

	// Filter to specific tools
	filterSet := make(map[string]bool)
	for _, name := range toolFilters {
		filterSet[name] = true
	}

	result := make([]dsgo.Tool, 0, len(toolFilters))
	for _, tool := range allTools {
		if filterSet[tool.Name] {
			result = append(result, tool)
		}
	}

	return result, nil
}

// GetAllMCPToolsForModule resolves all MCP tools configured for a module
func (r *ToolRegistry) GetAllMCPToolsForModule(mcpConfigs map[string]ModuleMCPSpec) ([]dsgo.Tool, error) {
	var result []dsgo.Tool
	for mcpName, mcpSpec := range mcpConfigs {
		tools, err := r.GetMCPTools(mcpName, mcpSpec.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to get tools from MCP '%s': %w", mcpName, err)
		}
		result = append(result, tools...)
	}
	return result, nil
}

// createTool creates a DSGo tool from a spec (custom tools only, not MCP)
func createTool(name string, spec ToolSpec) (dsgo.Tool, error) {
	switch spec.Type {
	case "filesystem":
		return createFilesystemTool(name, spec)
	case "function":
		return createFunctionTool(name, spec)
	default:
		return dsgo.Tool{}, fmt.Errorf("unsupported tool type: %s (valid: filesystem, function)", spec.Type)
	}
}

// createFilesystemTool creates a filesystem tool
func createFilesystemTool(name string, spec ToolSpec) (dsgo.Tool, error) {
	toolName := spec.Name
	if toolName == "" {
		toolName = name
	}

	switch toolName {
	case "list_files":
		return newListFilesTool(), nil
	case "read_file":
		return newReadFileTool(), nil
	case "search_files":
		return newSearchFilesTool(), nil
	default:
		return dsgo.Tool{}, fmt.Errorf("unknown filesystem tool: %s", toolName)
	}
}

// findProjectRoot finds the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return os.Getwd()
		}
		dir = parent
	}
}

// validatePath ensures path is within project root
func validatePath(projectRoot, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}
	path = filepath.Clean(path)

	if path != projectRoot && !strings.HasPrefix(path, projectRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %s is outside project root", path)
	}
	return path, nil
}

// newListFilesTool creates a tool for listing files
func newListFilesTool() dsgo.Tool {
	listFiles := func(ctx context.Context, args map[string]any) (any, error) {
		projectRoot, err := findProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}

		directory := projectRoot
		if dirArg, ok := args["directory"].(string); ok && dirArg != "" {
			directory, err = validatePath(projectRoot, dirArg)
			if err != nil {
				return nil, err
			}
		}

		depth := 3
		if depthVal, ok := args["depth"].(float64); ok {
			depth = int(depthVal)
		}

		var files []string
		err = filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

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
		AddParameter("directory", "string", "The directory path to list (relative to project root)", false).
		AddParameter("depth", "int", "Maximum depth to traverse (default: 3)", false)
}

// newReadFileTool creates a tool for reading files
func newReadFileTool() dsgo.Tool {
	readFile := func(ctx context.Context, args map[string]any) (any, error) {
		projectRoot, err := findProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}

		filepathArg, ok := args["filepath"].(string)
		if !ok || filepathArg == "" {
			return nil, fmt.Errorf("filepath parameter is required")
		}

		filepathArg, err = validatePath(projectRoot, filepathArg)
		if err != nil {
			return nil, err
		}

		content, err := os.ReadFile(filepathArg)
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		totalBytes := len(content)
		totalLines := strings.Count(string(content), "\n")
		if totalBytes > 0 && content[totalBytes-1] != '\n' {
			totalLines++
		}

		contentStr := string(content)
		truncated := false
		if len(contentStr) > 10000 {
			contentStr = contentStr[:10000]
			truncated = true
			truncatedLines := strings.Count(contentStr, "\n")
			contentStr += fmt.Sprintf("\n... [truncated: showing %d/%d bytes, ~%d/%d lines]",
				10000, totalBytes, truncatedLines, totalLines)
		}

		return map[string]any{
			"content":     contentStr,
			"filepath":    filepathArg,
			"size_bytes":  totalBytes,
			"total_lines": totalLines,
			"truncated":   truncated,
		}, nil
	}

	return *dsgo.NewTool("read_file", "Read the content of a specific file", readFile).
		AddParameter("filepath", "string", "The path to the file to read (relative to project root)", true)
}

// newSearchFilesTool creates a tool for searching files by glob pattern
func newSearchFilesTool() dsgo.Tool {
	searchFiles := func(ctx context.Context, args map[string]any) (any, error) {
		projectRoot, err := findProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find project root: %w", err)
		}

		directory := projectRoot
		if dirArg, ok := args["directory"].(string); ok && dirArg != "" {
			directory, err = validatePath(projectRoot, dirArg)
			if err != nil {
				return nil, err
			}
		}

		pattern, ok := args["pattern"].(string)
		if !ok || pattern == "" {
			return nil, fmt.Errorf("pattern parameter is required")
		}

		fullPattern := filepath.Join(directory, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("error searching files: %w", err)
		}

		relativeMatches := make([]string, len(matches))
		for i, match := range matches {
			relativePath, err := filepath.Rel(projectRoot, match)
			if err != nil {
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

	return *dsgo.NewTool("search_files", "Search for files matching a glob pattern (standard Go match, no ** recursion)", searchFiles).
		AddParameter("directory", "string", "The directory to search in (relative to project root)", false).
		AddParameter("pattern", "string", "Glob pattern to match (e.g., *.go)", true)
}

// createFunctionTool creates a native Go function tool
func createFunctionTool(name string, spec ToolSpec) (dsgo.Tool, error) {
	toolName := spec.Name
	if toolName == "" {
		toolName = name
	}

	switch toolName {
	case "current_datetime":
		return newCurrentDateTimeTool(), nil
	case "calculate":
		return newCalculateTool(), nil
	case "random_number":
		return newRandomNumberTool(), nil
	case "string_length":
		return newStringLengthTool(), nil
	case "word_count":
		return newWordCountTool(), nil
	case "environment_info":
		return newEnvironmentInfoTool(), nil
	default:
		return dsgo.Tool{}, fmt.Errorf("unknown function tool: %s", toolName)
	}
}

// newCurrentDateTimeTool creates a tool that returns current date and time
func newCurrentDateTimeTool() dsgo.Tool {
	getCurrentDateTime := func(ctx context.Context, args map[string]any) (any, error) {
		now := time.Now()

		format := "2006-01-02 15:04:05"
		if formatArg, ok := args["format"].(string); ok && formatArg != "" {
			format = formatArg
		}

		timezone := "Local"
		if tzArg, ok := args["timezone"].(string); ok && tzArg != "" {
			loc, err := time.LoadLocation(tzArg)
			if err != nil {
				return nil, fmt.Errorf("invalid timezone: %w", err)
			}
			now = now.In(loc)
			timezone = tzArg
		}

		return map[string]any{
			"datetime":    now.Format(format),
			"unix":        now.Unix(),
			"timezone":    timezone,
			"day_of_week": now.Weekday().String(),
			"iso8601":     now.Format(time.RFC3339),
		}, nil
	}

	return *dsgo.NewTool("current_datetime", "Get the current date and time", getCurrentDateTime).
		AddParameter("format", "string", "Date format (Go style, e.g., '2006-01-02')", false).
		AddParameter("timezone", "string", "Timezone (e.g., 'America/New_York', 'UTC')", false)
}

// newCalculateTool creates a simple calculator tool
func newCalculateTool() dsgo.Tool {
	calculate := func(ctx context.Context, args map[string]any) (any, error) {
		a, ok := args["a"].(float64)
		if !ok {
			return nil, fmt.Errorf("parameter 'a' is required and must be a number")
		}

		b, ok := args["b"].(float64)
		if !ok {
			return nil, fmt.Errorf("parameter 'b' is required and must be a number")
		}

		op, ok := args["operation"].(string)
		if !ok {
			op = "add"
		}

		var result float64
		switch op {
		case "add", "+":
			result = a + b
		case "subtract", "-":
			result = a - b
		case "multiply", "*":
			result = a * b
		case "divide", "/":
			if b == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			result = a / b
		default:
			return nil, fmt.Errorf("unknown operation: %s (valid: add, subtract, multiply, divide)", op)
		}

		return map[string]any{
			"result":    result,
			"operation": op,
			"a":         a,
			"b":         b,
		}, nil
	}

	return *dsgo.NewTool("calculate", "Perform basic arithmetic calculations", calculate).
		AddParameter("a", "number", "First operand", true).
		AddParameter("b", "number", "Second operand", true).
		AddParameter("operation", "string", "Operation: add, subtract, multiply, divide", false)
}

// newRandomNumberTool creates a random number generator tool
func newRandomNumberTool() dsgo.Tool {
	randomNumber := func(ctx context.Context, args map[string]any) (any, error) {
		minVal := 1.0
		maxVal := 100.0

		if min, ok := args["min"].(float64); ok {
			minVal = min
		}
		if max, ok := args["max"].(float64); ok {
			maxVal = max
		}

		if minVal >= maxVal {
			return nil, fmt.Errorf("min must be less than max")
		}

		result := minVal + rand.Float64()*(maxVal-minVal)

		return map[string]any{
			"number": int(result),
			"min":    int(minVal),
			"max":    int(maxVal),
		}, nil
	}

	return *dsgo.NewTool("random_number", "Generate a random number within a range", randomNumber).
		AddParameter("min", "number", "Minimum value (default: 1)", false).
		AddParameter("max", "number", "Maximum value (default: 100)", false)
}

// newStringLengthTool creates a tool that returns string length
func newStringLengthTool() dsgo.Tool {
	stringLength := func(ctx context.Context, args map[string]any) (any, error) {
		text, ok := args["text"].(string)
		if !ok {
			return nil, fmt.Errorf("parameter 'text' is required")
		}

		return map[string]any{
			"length":     len(text),
			"rune_count": len([]rune(text)),
			"word_count": len(strings.Fields(text)),
			"line_count": strings.Count(text, "\n") + 1,
			"is_empty":   len(strings.TrimSpace(text)) == 0,
		}, nil
	}

	return *dsgo.NewTool("string_length", "Get length and character count of a string", stringLength).
		AddParameter("text", "string", "The text to analyze", true)
}

// newWordCountTool creates a tool that counts words
func newWordCountTool() dsgo.Tool {
	wordCount := func(ctx context.Context, args map[string]any) (any, error) {
		text, ok := args["text"].(string)
		if !ok {
			return nil, fmt.Errorf("parameter 'text' is required")
		}

		words := strings.Fields(text)
		wordFreq := make(map[string]int)
		for _, word := range words {
			word = strings.ToLower(strings.Trim(word, ".,!?;:\"'()[]{}"))
			if word != "" {
				wordFreq[word]++
			}
		}

		return map[string]any{
			"total_words":  len(words),
			"unique_words": len(wordFreq),
			"characters":   len(text),
			"average_word_length": func() float64 {
				if len(words) == 0 {
					return 0
				}
				total := 0
				for _, w := range words {
					total += len(w)
				}
				return float64(total) / float64(len(words))
			}(),
		}, nil
	}

	return *dsgo.NewTool("word_count", "Count words and analyze text statistics", wordCount).
		AddParameter("text", "string", "The text to analyze", true)
}

// newEnvironmentInfoTool creates a tool that returns environment information
func newEnvironmentInfoTool() dsgo.Tool {
	environmentInfo := func(ctx context.Context, args map[string]any) (any, error) {
		cwd, _ := os.Getwd()
		hostname, _ := os.Hostname()

		info := map[string]any{
			"os":          runtime.GOOS,
			"arch":        runtime.GOARCH,
			"hostname":    hostname,
			"working_dir": cwd,
			"go_version":  runtime.Version(),
			"num_cpu":     runtime.NumCPU(),
			"user":        os.Getenv("USER"),
			"home":        os.Getenv("HOME"),
		}

		if varName, ok := args["var"].(string); ok && varName != "" {
			info["requested_var"] = os.Getenv(varName)
		}

		return info, nil
	}

	return *dsgo.NewTool("environment_info", "Get information about the current environment", environmentInfo).
		AddParameter("var", "string", "Optional: specific environment variable to retrieve", false)
}
