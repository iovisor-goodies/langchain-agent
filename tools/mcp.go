package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClient interface for testability
type MCPClient interface {
	ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	Close() error
}

// MCPTool wraps an MCP server's tools as a single agent tool
type MCPTool struct {
	client    MCPClient
	serverCmd string
	tools     []mcp.Tool
	toolMap   map[string]mcp.Tool
}

// Ensure MCPTool implements Closeable
var _ Closeable = (*MCPTool)(nil)

// NewMCPTool creates a new MCPTool by connecting to an MCP server via stdio
func NewMCPTool(ctx context.Context, command string, args []string) (*MCPTool, error) {
	c, err := client.NewStdioMCPClient(command, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to start MCP server: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "langchain-agent",
		Version: "1.0.0",
	}

	_, err = c.Initialize(ctx, initReq)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("MCP initialize failed: %w", err)
	}

	listResult, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("MCP list tools failed: %w", err)
	}

	toolMap := make(map[string]mcp.Tool, len(listResult.Tools))
	for _, t := range listResult.Tools {
		toolMap[t.Name] = t
	}

	return &MCPTool{
		client:    c,
		serverCmd: command,
		tools:     listResult.Tools,
		toolMap:   toolMap,
	}, nil
}

// newMCPToolFromClient creates an MCPTool from a pre-configured client (for testing)
func newMCPToolFromClient(c MCPClient, tools []mcp.Tool) *MCPTool {
	toolMap := make(map[string]mcp.Tool, len(tools))
	for _, t := range tools {
		toolMap[t.Name] = t
	}
	return &MCPTool{
		client:  c,
		tools:   tools,
		toolMap: toolMap,
	}
}

func (m *MCPTool) Name() string {
	return "mcp"
}

func (m *MCPTool) Description() string {
	if len(m.tools) == 0 {
		return "MCP server tool (no tools discovered)"
	}
	var names []string
	for _, t := range m.tools {
		names = append(names, t.Name)
	}
	return fmt.Sprintf("MCP server tool. Available tools: %s", strings.Join(names, ", "))
}

func (m *MCPTool) Parameters() map[string]any {
	// Build enum list with descriptions for tool_name
	var enumValues []string
	var enumDescs []string
	for _, t := range m.tools {
		enumValues = append(enumValues, t.Name)
		desc := t.Description
		if desc == "" {
			desc = t.Name
		}
		enumDescs = append(enumDescs, fmt.Sprintf("%s: %s", t.Name, desc))
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tool_name": map[string]any{
				"type":        "string",
				"description": "MCP tool to call. Available: " + strings.Join(enumDescs, "; "),
				"enum":        enumValues,
			},
			"arguments": map[string]any{
				"type":        "object",
				"description": "Arguments to pass to the MCP tool",
			},
		},
		"required": []string{"tool_name"},
	}
}

func (m *MCPTool) Call(ctx context.Context, params map[string]any) (string, error) {
	toolName, _ := params["tool_name"].(string)
	if toolName == "" {
		return "", fmt.Errorf("tool_name parameter required")
	}

	if _, ok := m.toolMap[toolName]; !ok {
		var available []string
		for _, t := range m.tools {
			available = append(available, t.Name)
		}
		return "", fmt.Errorf("unknown MCP tool %q, available: %s", toolName, strings.Join(available, ", "))
	}

	// Extract arguments
	var arguments map[string]any
	if args, ok := params["arguments"]; ok {
		if argMap, ok := args.(map[string]any); ok {
			arguments = argMap
		}
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = arguments

	result, err := m.client.CallTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("MCP call %q failed: %w", toolName, err)
	}

	// Extract text content from result
	var parts []string
	for _, content := range result.Content {
		switch c := content.(type) {
		case mcp.TextContent:
			parts = append(parts, c.Text)
		case *mcp.TextContent:
			parts = append(parts, c.Text)
		}
	}

	output := strings.Join(parts, "\n")

	if result.IsError {
		return "", fmt.Errorf("MCP tool %q error: %s", toolName, output)
	}

	if output == "" {
		return "(no output)", nil
	}

	return output, nil
}

func (m *MCPTool) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// ToolCount returns the number of discovered MCP tools
func (m *MCPTool) ToolCount() int {
	return len(m.tools)
}
