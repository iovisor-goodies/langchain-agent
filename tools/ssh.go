package tools

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// SSHTool executes commands on remote hosts via SSH
type SSHTool struct{}

func (s *SSHTool) Name() string {
	return "ssh"
}

func (s *SSHTool) Description() string {
	return "Execute a command on a REMOTE host via SSH. ALWAYS use this when user says 'ssh to', provides user@host, or mentions a remote server/IP address."
}

func (s *SSHTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host": map[string]any{
				"type":        "string",
				"description": "The remote host in format user@hostname or just hostname (uses current user)",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "The command to execute on the remote host",
			},
		},
		"required": []string{"host", "command"},
	}
}

func (s *SSHTool) Call(ctx context.Context, params map[string]any) (string, error) {
	hostParam, ok := params["host"].(string)
	if !ok {
		return "", fmt.Errorf("host parameter required")
	}
	command, ok := params["command"].(string)
	if !ok {
		return "", fmt.Errorf("command parameter required")
	}

	// Parse user@host format
	user, host := parseHost(hostParam)

	// Add default port if not specified
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	// Try key-based auth first, fall back to interactive password prompt
	client, err := s.dialWithAuth(user, host)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", host, err)
	}
	defer client.Close()

	// Create session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Run command
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}

	// Provide clear context about what happened
	if err != nil {
		if output == "" {
			output = "(command produced no output)\n"
		}
		output += fmt.Sprintf("Command exited with status: %v (note: grep returns status 1 when no matches found, which is not an error)", err)
	} else if output == "" {
		output = "(command succeeded but produced no output)"
	}

	return output, nil
}

// dialWithAuth tries key-based auth first, then falls back to interactive password prompt
func (s *SSHTool) dialWithAuth(user, host string) (*ssh.Client, error) {
	// Try key-based auth methods first (ssh-agent + key files)
	keyMethods := getKeyAuthMethods()
	if len(keyMethods) > 0 {
		config := &ssh.ClientConfig{
			User:            user,
			Auth:            keyMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		client, err := ssh.Dial("tcp", host, config)
		if err == nil {
			return client, nil
		}
	}

	// Key auth failed or unavailable — prompt for password
	fmt.Printf("Password for %s@%s: ", user, strings.TrimSuffix(host, ":22"))
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // newline after password input
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
			ssh.KeyboardInteractive(
				func(user, instruction string, questions []string, echos []bool) ([]string, error) {
					answers := make([]string, len(questions))
					for i := range questions {
						answers[i] = password
					}
					return answers, nil
				}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return ssh.Dial("tcp", host, config)
}

// parseHost extracts user and host from user@host format
func parseHost(hostStr string) (user, host string) {
	if idx := strings.Index(hostStr, "@"); idx != -1 {
		return hostStr[:idx], hostStr[idx+1:]
	}
	// Default to current user
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "root"
	}
	return currentUser, hostStr
}

// getKeyAuthMethods returns key-based SSH auth methods (ssh-agent + key files)
func getKeyAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// Try ssh-agent first
	if agentConn := os.Getenv("SSH_AUTH_SOCK"); agentConn != "" {
		conn, err := net.Dial("unix", agentConn)
		if err == nil {
			agentClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	// Try default key files
	home, _ := os.UserHomeDir()
	keyFiles := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}

	for _, keyFile := range keyFiles {
		key, err := os.ReadFile(keyFile)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	return methods
}
