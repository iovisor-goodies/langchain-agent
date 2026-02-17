package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// mockMCPClient implements MCPClient for testing
type mockMCPClient struct {
	callToolFn func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
	closed     bool
}

func (m *mockMCPClient) ListTools(ctx context.Context, req mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return nil, nil // not used in unit tests; tools are injected directly
}

func (m *mockMCPClient) CallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if m.callToolFn != nil {
		return m.callToolFn(ctx, req)
	}
	return &mcp.CallToolResult{}, nil
}

func (m *mockMCPClient) Close() error {
	m.closed = true
	return nil
}

func testTools() []mcp.Tool {
	return []mcp.Tool{
		{Name: "read_file", Description: "Read a file from disk"},
		{Name: "list_directory", Description: "List directory contents"},
	}
}

func TestMCPTool_Name(t *testing.T) {
	tool := newMCPToolFromClient(&mockMCPClient{}, "", testTools())
	if got := tool.Name(); got != "mcp" {
		t.Errorf("Name() = %q, want %q", got, "mcp")
	}
}

func TestMCPTool_Description(t *testing.T) {
	tool := newMCPToolFromClient(&mockMCPClient{}, "", testTools())
	desc := tool.Description()
	for _, name := range []string{"read_file", "list_directory"} {
		if !strings.Contains(desc, name) {
			t.Errorf("Description() should contain %q, got %q", name, desc)
		}
	}
}

func TestMCPTool_Description_NoTools(t *testing.T) {
	tool := newMCPToolFromClient(&mockMCPClient{}, "", nil)
	desc := tool.Description()
	if !strings.Contains(desc, "no tools") {
		t.Errorf("Description() should indicate no tools, got %q", desc)
	}
}

func TestMCPTool_Parameters(t *testing.T) {
	tool := newMCPToolFromClient(&mockMCPClient{}, "", testTools())
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("Parameters type = %v, want 'object'", params["type"])
	}

	required, ok := params["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "tool_name" {
		t.Errorf("required = %v, want [tool_name]", required)
	}

	props := params["properties"].(map[string]any)
	toolNameProp := props["tool_name"].(map[string]any)
	enumValues := toolNameProp["enum"].([]string)
	if len(enumValues) != 2 || enumValues[0] != "read_file" || enumValues[1] != "list_directory" {
		t.Errorf("enum = %v, want [read_file, list_directory]", enumValues)
	}
}

func TestMCPTool_Call_Success(t *testing.T) {
	mock := &mockMCPClient{
		callToolFn: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if req.Params.Name != "read_file" {
				t.Errorf("called tool %q, want read_file", req.Params.Name)
			}
			args := req.Params.Arguments.(map[string]any)
			if args["path"] != "/tmp/test.txt" {
				t.Errorf("path arg = %v, want /tmp/test.txt", args["path"])
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Type: "text", Text: "hello world"},
				},
			}, nil
		},
	}

	tool := newMCPToolFromClient(mock, "", testTools())
	result, err := tool.Call(context.Background(), map[string]any{
		"tool_name": "read_file",
		"arguments": map[string]any{"path": "/tmp/test.txt"},
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result != "hello world" {
		t.Errorf("Call() = %q, want %q", result, "hello world")
	}
}

func TestMCPTool_Call_MissingToolName(t *testing.T) {
	tool := newMCPToolFromClient(&mockMCPClient{}, "", testTools())
	_, err := tool.Call(context.Background(), map[string]any{})

	if err == nil {
		t.Fatal("Call() should return error for missing tool_name")
	}
	if !strings.Contains(err.Error(), "tool_name") {
		t.Errorf("error = %v, want to mention tool_name", err)
	}
}

func TestMCPTool_Call_UnknownTool(t *testing.T) {
	tool := newMCPToolFromClient(&mockMCPClient{}, "", testTools())
	_, err := tool.Call(context.Background(), map[string]any{
		"tool_name": "nonexistent",
	})

	if err == nil {
		t.Fatal("Call() should return error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown MCP tool") {
		t.Errorf("error = %v, want to contain 'unknown MCP tool'", err)
	}
}

func TestMCPTool_Call_IsError(t *testing.T) {
	mock := &mockMCPClient{
		callToolFn: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Type: "text", Text: "permission denied"},
				},
				IsError: true,
			}, nil
		},
	}

	tool := newMCPToolFromClient(mock, "", testTools())
	_, err := tool.Call(context.Background(), map[string]any{
		"tool_name": "read_file",
	})

	if err == nil {
		t.Fatal("Call() should return error when IsError is true")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %v, want to contain 'permission denied'", err)
	}
}

func TestMCPTool_Call_ClientError(t *testing.T) {
	mock := &mockMCPClient{
		callToolFn: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, fmt.Errorf("connection lost")
		},
	}

	tool := newMCPToolFromClient(mock, "", testTools())
	_, err := tool.Call(context.Background(), map[string]any{
		"tool_name": "read_file",
	})

	if err == nil {
		t.Fatal("Call() should return error on client error")
	}
	if !strings.Contains(err.Error(), "connection lost") {
		t.Errorf("error = %v, want to contain 'connection lost'", err)
	}
}

func TestMCPTool_Call_NoOutput(t *testing.T) {
	mock := &mockMCPClient{
		callToolFn: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	}

	tool := newMCPToolFromClient(mock, "", testTools())
	result, err := tool.Call(context.Background(), map[string]any{
		"tool_name": "read_file",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result != "(no output)" {
		t.Errorf("Call() = %q, want %q", result, "(no output)")
	}
}

func TestMCPTool_Close(t *testing.T) {
	mock := &mockMCPClient{}
	tool := newMCPToolFromClient(mock, "", testTools())

	if err := tool.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !mock.closed {
		t.Error("Close() should close the underlying client")
	}
}

func TestMCPTool_ToolCount(t *testing.T) {
	tool := newMCPToolFromClient(&mockMCPClient{}, "", testTools())
	if got := tool.ToolCount(); got != 2 {
		t.Errorf("ToolCount() = %d, want 2", got)
	}
}
