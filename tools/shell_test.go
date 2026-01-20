package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestShellTool_Name(t *testing.T) {
	tool := &ShellTool{}
	if got := tool.Name(); got != "shell" {
		t.Errorf("Name() = %q, want %q", got, "shell")
	}
}

func TestShellTool_Description(t *testing.T) {
	tool := &ShellTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
	if !strings.Contains(strings.ToLower(desc), "local") {
		t.Error("Description() should mention 'local'")
	}
}

func TestShellTool_Parameters(t *testing.T) {
	tool := &ShellTool{}
	params := tool.Parameters()

	// Check it's an object type
	if params["type"] != "object" {
		t.Errorf("Parameters type = %v, want 'object'", params["type"])
	}

	// Check required fields
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Parameters should have 'required' array")
	}
	if len(required) != 1 || required[0] != "command" {
		t.Errorf("required = %v, want ['command']", required)
	}
}

func TestShellTool_Call_SimpleCommand(t *testing.T) {
	tool := &ShellTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"command": "echo hello",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("Call() = %q, want to contain 'hello'", result)
	}
}

func TestShellTool_Call_MultipleCommands(t *testing.T) {
	tool := &ShellTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"command": "echo one && echo two",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if !strings.Contains(result, "one") || !strings.Contains(result, "two") {
		t.Errorf("Call() = %q, want to contain 'one' and 'two'", result)
	}
}

func TestShellTool_Call_CapturesStderr(t *testing.T) {
	tool := &ShellTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"command": "echo error >&2",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if !strings.Contains(result, "STDERR") {
		t.Errorf("Call() = %q, want to contain 'STDERR'", result)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("Call() = %q, want to contain 'error'", result)
	}
}

func TestShellTool_Call_HandlesExitError(t *testing.T) {
	tool := &ShellTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"command": "exit 1",
	})

	// Should not return error, but include exit info in result
	if err != nil {
		t.Fatalf("Call() error = %v, want nil (error in result)", err)
	}
	if !strings.Contains(result, "exited with status") {
		t.Errorf("Call() = %q, want to contain 'exited with status'", result)
	}
}

func TestShellTool_Call_Timeout(t *testing.T) {
	tool := &ShellTool{Timeout: 100 * time.Millisecond}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"command": "sleep 10",
	})

	if err != nil {
		t.Fatalf("Call() error = %v, want nil (timeout in result)", err)
	}
	if !strings.Contains(result, "timed out") {
		t.Errorf("Call() = %q, want to contain 'timed out'", result)
	}
}

func TestShellTool_Call_MissingCommand(t *testing.T) {
	tool := &ShellTool{}
	ctx := context.Background()

	_, err := tool.Call(ctx, map[string]any{})

	if err == nil {
		t.Error("Call() with no command should return error")
	}
}

func TestShellTool_Call_EmptyCommand(t *testing.T) {
	tool := &ShellTool{}
	ctx := context.Background()

	_, err := tool.Call(ctx, map[string]any{
		"command": "",
	})

	if err == nil {
		t.Error("Call() with empty command should return error")
	}
}

func TestShellTool_Call_EnvironmentVariables(t *testing.T) {
	tool := &ShellTool{}
	ctx := context.Background()

	// Test that env vars work
	result, err := tool.Call(ctx, map[string]any{
		"command": "echo $HOME",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	// HOME should be set and non-empty
	result = strings.TrimSpace(result)
	if result == "" || result == "$HOME" {
		t.Errorf("Call() = %q, expected HOME to be expanded", result)
	}
}
