package tools

import (
	"context"
)

// sshExecutor is the function signature used by edge_* tools to run a command
// on the configured edge target over SSH. Tests inject a fake to avoid
// actually opening an SSH connection.
type sshExecutor func(ctx context.Context, host, cmd string) (string, error)

// defaultSSHExec runs cmd on host using the existing SSHTool.
func defaultSSHExec(ctx context.Context, host, cmd string) (string, error) {
	t := &SSHTool{}
	return t.Call(ctx, map[string]any{"host": host, "command": cmd})
}
