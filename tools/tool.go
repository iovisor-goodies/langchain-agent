package tools

import "context"

// Tool defines the interface for agent tools
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any // JSON schema for parameters
	Call(ctx context.Context, params map[string]any) (string, error)
}

// ToolCall represents a parsed tool call from the LLM
type ToolCall struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"parameters"`
}

// Closeable is implemented by tools that hold resources needing cleanup
type Closeable interface {
	Close() error
}

// ToolResult holds the result of a tool execution
type ToolResult struct {
	Tool   string
	Result string
	Error  error
}
