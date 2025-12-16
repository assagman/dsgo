package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultShellTimeout   = 10 * time.Minute
	defaultShellMaxOutput = 256 * 1024
)

// ShellServerConfig configures the built-in MCP shell server.
//
// RootDir is used to constrain working directories and patch application.
// DefaultTimeout is used when callers do not provide timeout_seconds.
// MaxOutputBytes bounds stdout/stderr capture for shell_run.
type ShellServerConfig struct {
	RootDir        string
	DefaultTimeout time.Duration
	MaxOutputBytes int
}

// ShellServer is a built-in MCP server that exposes safe tools:
// - shell_run (whitelisted command runner)
// - apply_patch (git apply for unified diffs)
//
// It implements LocalHandler.
type ShellServer struct {
	rootDir        string
	defaultTimeout time.Duration
	maxOutputBytes int
}

// NewShellServer creates a new ShellServer.
func NewShellServer(cfg ShellServerConfig) (*ShellServer, error) {
	root := cfg.RootDir
	if root == "" {
		var err error
		root, err = findRepoRoot()
		if err != nil {
			return nil, err
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root dir: %w", err)
	}

	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = defaultShellTimeout
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = defaultShellMaxOutput
	}

	return &ShellServer{
		rootDir:        root,
		defaultTimeout: cfg.DefaultTimeout,
		maxOutputBytes: cfg.MaxOutputBytes,
	}, nil
}

func (s *ShellServer) Handle(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID)
	case "tools/list":
		return s.handleToolsList(req.ID)
	case "tools/call":
		return s.handleToolsCall(ctx, req.ID, req.Params)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    ErrCodeMethodNotFound,
				Message: "method not found",
			},
		}, nil
	}
}

func (s *ShellServer) handleInitialize(id any) (*JSONRPCResponse, error) {
	result := MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]any{},
		ServerInfo: map[string]string{
			"name":    "dsgo-shell",
			"version": "0.1.0",
		},
	}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: b}, nil
}

func (s *ShellServer) handleToolsList(id any) (*JSONRPCResponse, error) {
	schemas := []MCPToolSchema{
		{
			Name:        "shell_run",
			Description: "Run a whitelisted shell command in the repository (make/go test/git read-only)",
			InputSchema: MCPInputSchema{
				Type: "object",
				Properties: map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "Command to run (e.g. make, go, git)",
					},
					"args": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"description": "Arguments for the command (e.g. [\"test\", \"./...\"]) ",
					},
					"dir": map[string]any{
						"type":        "string",
						"description": "Working directory relative to repo root",
					},
					"timeout_seconds": map[string]any{
						"type":        "integer",
						"description": "Optional timeout (seconds)",
					},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "apply_patch",
			Description: "Apply a unified diff patch to the repository using git apply",
			InputSchema: MCPInputSchema{
				Type: "object",
				Properties: map[string]any{
					"patch": map[string]any{
						"type":        "string",
						"description": "Unified diff (git apply compatible)",
					},
				},
				Required: []string{"patch"},
			},
		},
	}

	payload := MCPListToolsResult{Tools: schemas}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: b}, nil
}

func (s *ShellServer) handleToolsCall(ctx context.Context, id any, rawParams json.RawMessage) (*JSONRPCResponse, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return s.toolResultError(id, fmt.Sprintf("invalid params: %v", err))
	}

	switch params.Name {
	case "shell_run":
		return s.handleShellRun(ctx, id, params.Arguments)
	case "apply_patch":
		return s.handleApplyPatch(ctx, id, params.Arguments)
	default:
		return s.toolResultError(id, fmt.Sprintf("unknown tool: %s", params.Name))
	}
}

func (s *ShellServer) handleShellRun(ctx context.Context, id any, args map[string]any) (*JSONRPCResponse, error) {
	cmdName, _ := args["command"].(string)
	if strings.TrimSpace(cmdName) == "" {
		return s.toolResultError(id, "command is required")
	}

	cmdArgs, ok := decodeStringArray(args["args"])
	if !ok {
		return s.toolResultError(id, "args must be an array of strings")
	}
	if strings.Contains(cmdName, " ") {
		parts := strings.Fields(cmdName)
		cmdName = parts[0]
		cmdArgs = append(parts[1:], cmdArgs...)
	}

	dirArg, _ := args["dir"].(string)
	workDir, err := s.resolveWorkDir(dirArg)
	if err != nil {
		return s.toolResultError(id, err.Error())
	}

	timeout := s.defaultTimeout
	if raw, ok := args["timeout_seconds"]; ok {
		sec, err := toInt(raw)
		if err != nil {
			return s.toolResultError(id, "timeout_seconds must be an integer")
		}
		if sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	cmdName, cmdArgs, err = normalizeAndValidateCommand(cmdName, cmdArgs)
	if err != nil {
		return s.toolResultError(id, err.Error())
	}

	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, cmdName, cmdArgs...)
	cmd.Dir = workDir

	var stdoutBuf, stderrBuf limitedBuffer
	stdoutBuf.MaxBytes = s.maxOutputBytes
	stderrBuf.MaxBytes = s.maxOutputBytes
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	errorText := ""
	timedOut := false
	if err != nil {
		exitCode = exitCodeFromErr(err)
		errorText = err.Error()
		timedOut = errors.Is(err, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded)
	}

	result := map[string]any{
		"command":         cmdName,
		"args":            cmdArgs,
		"dir":             workDir,
		"exit_code":       exitCode,
		"stdout":          stdoutBuf.String(),
		"stderr":          stderrBuf.String(),
		"stdout_trunc":    stdoutBuf.Truncated,
		"stderr_trunc":    stderrBuf.Truncated,
		"duration_ms":     duration.Milliseconds(),
		"timeout_seconds": int(timeout.Seconds()),
		"timed_out":       timedOut,
		"error":           errorText,
		"goos":            runtime.GOOS,
		"goarch":          runtime.GOARCH,
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return s.toolResultText(id, string(b))
}

func (s *ShellServer) handleApplyPatch(ctx context.Context, id any, args map[string]any) (*JSONRPCResponse, error) {
	patch, _ := args["patch"].(string)
	if strings.TrimSpace(patch) == "" {
		return s.toolResultError(id, "patch is required")
	}

	if err := validateUnifiedDiffPaths(patch); err != nil {
		return s.toolResultError(id, err.Error())
	}

	cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn", "--")
	cmd.Dir = s.rootDir
	cmd.Stdin = strings.NewReader(patch)

	var stderr limitedBuffer
	stderr.MaxBytes = s.maxOutputBytes
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := "git apply failed"
		if stderr.String() != "" {
			msg += ": " + stderr.String()
		}
		return s.toolResultError(id, msg)
	}

	return s.toolResultText(id, "patch applied")
}

func (s *ShellServer) toolResultText(id any, text string) (*JSONRPCResponse, error) {
	result := MCPCallToolResult{Content: []MCPContent{{Type: "text", Text: text}}}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: b}, nil
}

func (s *ShellServer) toolResultError(id any, msg string) (*JSONRPCResponse, error) {
	result := MCPCallToolResult{Content: []MCPContent{{Type: "text", Text: msg}}, IsError: true}
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: b}, nil
}

func (s *ShellServer) resolveWorkDir(dirArg string) (string, error) {
	if strings.TrimSpace(dirArg) == "" {
		return s.rootDir, nil
	}
	candidate := dirArg
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(s.rootDir, candidate)
	}
	candidate = filepath.Clean(candidate)
	candidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("invalid dir: %w", err)
	}

	if !strings.HasPrefix(candidate, s.rootDir+string(filepath.Separator)) && candidate != s.rootDir {
		return "", fmt.Errorf("dir %q is outside repo root", dirArg)
	}
	return candidate, nil
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if st, err := os.Stat(filepath.Join(dir, ".git")); err == nil && st.IsDir() {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("failed to find repo root from %q", cwd)
		}
		dir = parent
	}
}

type limitedBuffer struct {
	MaxBytes  int
	buf       bytes.Buffer
	Truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.MaxBytes <= 0 {
		return b.buf.Write(p)
	}
	remaining := b.MaxBytes - b.buf.Len()
	if remaining <= 0 {
		b.Truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.Truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) String() string { return b.buf.String() }

func decodeStringArray(v any) ([]string, bool) {
	if v == nil {
		return nil, true
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func toInt(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		i, err := strconv.Atoi(t)
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, errors.New("not an int")
	}
}

func exitCodeFromErr(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return 124
	}
	return 1
}

func normalizeAndValidateCommand(command string, args []string) (string, []string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, fmt.Errorf("command is empty")
	}

	switch command {
	case "make":
		if err := validateMakeArgs(args); err != nil {
			return "", nil, err
		}
		return command, args, nil
	case "go":
		if err := validateGoArgs(args); err != nil {
			return "", nil, err
		}
		return command, args, nil
	case "git":
		if err := validateGitArgs(args); err != nil {
			return "", nil, err
		}
		return command, args, nil
	default:
		return "", nil, fmt.Errorf("command not allowed: %s", command)
	}
}

func validateMakeArgs(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-f" || strings.HasPrefix(arg, "-f") || arg == "--file" || strings.HasPrefix(arg, "--file") {
			return fmt.Errorf("make file flags are not allowed")
		}
		if arg == "-C" || strings.HasPrefix(arg, "-C") || arg == "--directory" || strings.HasPrefix(arg, "--directory") {
			return fmt.Errorf("make directory flags are not allowed")
		}
		if strings.Contains(arg, "=") {
			return fmt.Errorf("make variable assignments are not allowed")
		}
	}
	return nil
}

func validateGoArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("go subcommand is required")
	}
	if args[0] != "test" {
		return fmt.Errorf("go only allows 'test'")
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-c" || strings.HasPrefix(arg, "-c="):
			return fmt.Errorf("go test -c is not allowed")
		case arg == "-o" || strings.HasPrefix(arg, "-o="):
			return fmt.Errorf("go test output flags are not allowed")
		case arg == "-coverprofile" || strings.HasPrefix(arg, "-coverprofile="):
			return fmt.Errorf("go test coverprofile is not allowed")
		case arg == "-cpuprofile" || strings.HasPrefix(arg, "-cpuprofile="):
			return fmt.Errorf("go test cpuprofile is not allowed")
		case arg == "-memprofile" || strings.HasPrefix(arg, "-memprofile="):
			return fmt.Errorf("go test memprofile is not allowed")
		case arg == "-trace" || strings.HasPrefix(arg, "-trace="):
			return fmt.Errorf("go test trace is not allowed")
		}
	}
	return nil
}

func validateGitArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("git subcommand is required")
	}

	// Allow a small set of safe global options, then require a read-only subcommand.
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			args = args[i+1:]
			break
		}

		if !strings.HasPrefix(arg, "-") {
			args = args[i:]
			break
		}

		switch {
		case arg == "--no-pager":
			continue
		case arg == "-c" || strings.HasPrefix(arg, "-c") || arg == "--config":
			return fmt.Errorf("git config options are not allowed")
		case arg == "-C" || strings.HasPrefix(arg, "-C"):
			return fmt.Errorf("git -C is not allowed")
		case arg == "--git-dir" || strings.HasPrefix(arg, "--git-dir"):
			return fmt.Errorf("git-dir overrides are not allowed")
		case arg == "--work-tree" || strings.HasPrefix(arg, "--work-tree"):
			return fmt.Errorf("work-tree overrides are not allowed")
		default:
			return fmt.Errorf("git global option not allowed: %s", arg)
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("git subcommand missing")
	}

	sub := args[0]
	allowed := map[string]bool{
		"status":    true,
		"diff":      true,
		"log":       true,
		"show":      true,
		"ls-files":  true,
		"rev-parse": true,
	}
	if !allowed[sub] {
		return fmt.Errorf("git subcommand not allowed: %s", sub)
	}
	return nil
}

func validateUnifiedDiffPaths(patch string) error {
	// Validate common diff headers.
	// Allow /dev/null and a/ b/ prefixes.
	lines := strings.Split(patch, "\n")
	for _, line := range lines {
		var path string
		switch {
		case strings.HasPrefix(line, "diff --git "):
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// diff --git a/foo b/foo
				if err := validateDiffPath(parts[2]); err != nil {
					return err
				}
				if err := validateDiffPath(parts[3]); err != nil {
					return err
				}
			}
			continue
		case strings.HasPrefix(line, "--- "):
			path = strings.TrimSpace(strings.TrimPrefix(line, "--- "))
		case strings.HasPrefix(line, "+++ "):
			path = strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
		default:
			continue
		}
		if path == "/dev/null" {
			continue
		}
		if err := validateDiffPath(path); err != nil {
			return err
		}
	}
	return nil
}

func validateDiffPath(p string) error {
	if p == "" {
		return nil
	}
	if strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/") {
		p = p[2:]
	}
	p = filepath.Clean(p)
	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute paths are not allowed in patches")
	}
	if strings.HasPrefix(p, ".."+string(filepath.Separator)) || p == ".." {
		return fmt.Errorf("parent traversal paths are not allowed in patches")
	}
	if strings.Contains(p, ".."+string(filepath.Separator)) {
		return fmt.Errorf("parent traversal paths are not allowed in patches")
	}
	return nil
}
