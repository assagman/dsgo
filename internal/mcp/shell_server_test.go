package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShellServer_ToolsList(t *testing.T) {
	server, err := NewShellServer(ShellServerConfig{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewShellServer() error: %v", err)
	}

	resp, err := server.Handle(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tools/list"})
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if resp == nil || resp.Result == nil {
		t.Fatalf("expected result")
	}
	var list MCPListToolsResult
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		t.Fatalf("unmarshal tools/list: %v", err)
	}
	if len(list.Tools) < 2 {
		t.Fatalf("expected at least 2 tools, got %d", len(list.Tools))
	}
}

func TestShellServer_ShellRun_DeniesMakeFileFlag(t *testing.T) {
	server, err := NewShellServer(ShellServerConfig{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewShellServer() error: %v", err)
	}

	params, _ := json.Marshal(map[string]any{
		"name": "shell_run",
		"arguments": map[string]any{
			"command": "make",
			"args":    []any{"-f", "Makefile"},
		},
	})
	resp, err := server.Handle(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	var result MCPCallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected isError")
	}
}

func TestShellServer_ApplyPatch_RejectsTraversal(t *testing.T) {
	server, err := NewShellServer(ShellServerConfig{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewShellServer() error: %v", err)
	}

	patch := "diff --git a/../x b/../x\nnew file mode 100644\nindex 0000000..e69de29\n--- /dev/null\n+++ b/../x\n@@ -0,0 +1 @@\n+hi\n"
	params, _ := json.Marshal(map[string]any{
		"name": "apply_patch",
		"arguments": map[string]any{
			"patch": patch,
		},
	})
	resp, err := server.Handle(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	var result MCPCallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected isError")
	}
}

func TestShellServer_ApplyPatch_AllowsNewFileInGitRepo(t *testing.T) {
	root := t.TempDir()

	if err := runGit(t, root, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	server, err := NewShellServer(ShellServerConfig{RootDir: root})
	if err != nil {
		t.Fatalf("NewShellServer() error: %v", err)
	}

	patch := "diff --git a/newfile.txt b/newfile.txt\nnew file mode 100644\nindex 0000000..9daeafb\n--- /dev/null\n+++ b/newfile.txt\n@@ -0,0 +1 @@\n+hello\n"
	params, _ := json.Marshal(map[string]any{
		"name": "apply_patch",
		"arguments": map[string]any{
			"patch": patch,
		},
	})
	resp, err := server.Handle(context.Background(), &JSONRPCRequest{JSONRPC: "2.0", ID: "1", Method: "tools/call", Params: params})
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	var result MCPCallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].Text)
	}

	if _, err := os.Stat(filepath.Join(root, "newfile.txt")); err != nil {
		t.Fatalf("expected newfile.txt to exist: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := execCommand("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v failed: %v: %s", args, err, string(out))
	}
	return nil
}

// execCommand exists to avoid linter complaints about shadowing.
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
