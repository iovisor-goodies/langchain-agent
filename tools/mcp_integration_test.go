//go:build integration

package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMCPTool_Integration_FilesystemServer(t *testing.T) {
	// Skip if mcp-filesystem-server is not available
	serverPath, err := exec.LookPath("mcp-filesystem-server")
	if err != nil {
		t.Skip("mcp-filesystem-server not in PATH; install with: go install github.com/mark3labs/mcp-filesystem-server@latest")
	}

	// Create temp dir with test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello from MCP"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to MCP filesystem server
	tool, err := NewMCPTool(ctx, "mcp", serverPath, []string{tmpDir})
	if err != nil {
		t.Fatalf("NewMCPTool() error = %v", err)
	}
	defer tool.Close()

	if tool.ToolCount() == 0 {
		t.Fatal("expected at least one tool from filesystem server")
	}

	t.Logf("Discovered %d tools: %s", tool.ToolCount(), tool.Description())

	// Test list_directory
	t.Run("list_directory", func(t *testing.T) {
		result, err := tool.Call(ctx, map[string]any{
			"tool_name": "list_directory",
			"arguments": map[string]any{"path": tmpDir},
		})
		if err != nil {
			t.Fatalf("Call(list_directory) error = %v", err)
		}
		if !strings.Contains(result, "hello.txt") {
			t.Errorf("list_directory result should contain hello.txt, got: %s", result)
		}
	})

	// Test read_file
	t.Run("read_file", func(t *testing.T) {
		result, err := tool.Call(ctx, map[string]any{
			"tool_name": "read_file",
			"arguments": map[string]any{"path": testFile},
		})
		if err != nil {
			t.Fatalf("Call(read_file) error = %v", err)
		}
		if !strings.Contains(result, "hello from MCP") {
			t.Errorf("read_file result should contain file content, got: %s", result)
		}
	})
}
