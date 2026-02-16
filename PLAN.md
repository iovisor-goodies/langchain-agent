# Plan: Replace Stubbed MCP Tool with Real MCP Client

## Context

The MCP tool (`tools/mcp.go`) is fully stubbed — it returns hardcoded Kubernetes mock data and never connects to any server. We're replacing it with a real MCP client using the `mark3labs/mcp-go` SDK, tested against `mcp-filesystem-server` (a Go binary that exposes filesystem operations over MCP stdio).

## Approach: Single Wrapper Tool

Keep MCPTool as a single `tools.Tool` that wraps all MCP server tools via `tool_name` + `arguments` parameters. This avoids changes to the agent/tool dispatch and keeps the wiring simple. The MCP server's available tools are discovered dynamically at startup via `ListTools`.

## Steps

### 1. Add dependency
```bash
go get github.com/mark3labs/mcp-go@latest
```

### 2. Rewrite `tools/mcp.go`

- Define `MCPClient` interface (for testability):
  ```go
  type MCPClient interface {
      ListTools(ctx context.Context, req mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
      CallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
      Close() error
  }
  ```

- New `MCPTool` struct with state:
  ```go
  type MCPTool struct {
      client    MCPClient
      serverCmd string
      tools     []mcp.Tool
      toolMap   map[string]mcp.Tool
  }
  ```

- `NewMCPTool(ctx, command, args)` constructor:
  1. `client.NewStdioMCPClient(command, nil, args...)` — spawns server process
  2. `client.Initialize(ctx, ...)` — MCP handshake
  3. `client.ListTools(ctx, ...)` — discover available tools, cache them
  4. Return ready-to-use `*MCPTool`

- `Name()` → `"mcp"` (unchanged)

- `Description()` → dynamically lists discovered tool names

- `Parameters()` → JSON schema with:
  - `tool_name` (string, required, enum of discovered tools with descriptions)
  - `arguments` (object, optional, tool-specific)

- `Call(ctx, params)` →
  1. Extract `tool_name`, validate against discovered tools
  2. Extract `arguments` map
  3. `client.CallTool(ctx, req)` — proxy to MCP server
  4. Extract `TextContent` from result, return as string
  5. Handle `IsError` flag from server

- `Close()` → close the underlying client

### 3. Add `Closeable` interface to `tools/tool.go`

```go
type Closeable interface {
    Close() error
}
```

### 4. Update `main.go`

- Add `--mcp` flag: `flag.String("mcp", "", "MCP server command (e.g., 'mcp-filesystem-server /tmp')")`
- Remove default `&tools.MCPTool{}` from tool list (MCP only when `--mcp` provided)
- When `--mcp` is set: parse command string, call `tools.NewMCPTool(ctx, cmd, args)`, append to tool list
- `defer mcpTool.Close()` for cleanup
- Print discovered tool count on startup

### 5. Update system prompt in `llm/ollama.go`

Change line 194 from:
```
- "mcp", "pods", "kubernetes", "openshift", "namespace" → use "mcp" tool
```
To:
```
- "mcp", file operations on MCP server, MCP tool calls → use "mcp" tool
```

### 6. Rewrite `tools/mcp_test.go` (unit tests)

- Mock `MCPClient` implementation returning controlled data
- Tests: Name, Description (includes tool names), Parameters (enum matches), Call success, Call missing tool_name, Call unknown tool, Call with IsError result, Call with client error, Close

### 7. Add `tools/mcp_integration_test.go` (build tag `integration`)

- Skip if `mcp-filesystem-server` not in PATH
- Create temp dir with test files
- Test `list_directory`, `read_file` through the full MCPTool

### 8. Update docs

- `CLAUDE.md`: move MCP from TODO to Working, add `--mcp` to usage
- `README.md`: update MCP tool description, add `--mcp` example

## Files to Modify

| File | Change |
|------|--------|
| `tools/mcp.go` | Full rewrite (~130 lines) |
| `tools/mcp_test.go` | Full rewrite with mock client (~150 lines) |
| `tools/mcp_integration_test.go` | **New** — integration tests (~50 lines) |
| `tools/tool.go` | Add `Closeable` interface (~4 lines) |
| `main.go` | Add `--mcp` flag, conditional init, defer close |
| `llm/ollama.go` | Update one line in tool routing keywords |
| `CLAUDE.md` | Update status and usage |
| `README.md` | Update MCP section |

## Verification

1. `go build -o langchain-agent .` — compiles
2. `go test ./tools/...` — unit tests pass (mock client)
3. Install server: `go install github.com/mark3labs/mcp-filesystem-server@latest`
4. Integration test: `go test -tags integration ./tools/...`
5. Manual test:
   ```bash
   ./langchain-agent --mcp "mcp-filesystem-server /tmp"
   > list files in /tmp
   > read the file /tmp/test.txt
   ```
