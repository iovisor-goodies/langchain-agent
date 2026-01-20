package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ShellTool executes local shell commands
type ShellTool struct {
	Timeout time.Duration
}

func (s *ShellTool) Name() string {
	return "shell"
}

func (s *ShellTool) Description() string {
	return "Execute a command on the LOCAL machine only. Do NOT use for remote hosts - use ssh tool instead."
}

func (s *ShellTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute locally",
			},
		},
		"required": []string{"command"},
	}
}

func (s *ShellTool) Call(ctx context.Context, params map[string]any) (string, error) {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("command parameter required")
	}

	timeout := s.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "STDERR:\n" + stderr.String()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output + "\nError: command timed out", nil
		}
		if output == "" {
			output = "(command produced no output)\n"
		}
		return output + fmt.Sprintf("Command exited with status: %v (note: grep returns status 1 when no matches found, which is not an error)", err), nil
	}

	if output == "" {
		return "(command succeeded but produced no output)", nil
	}
	return output, nil
}
