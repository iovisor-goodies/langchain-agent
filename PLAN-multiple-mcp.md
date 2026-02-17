# Plan: Support Multiple MCP Servers

## Context

Currently only one `--mcp` flag is supported. To connect to multiple MCP servers simultaneously (e.g., a filesystem server and a Kubernetes server), we need to allow repeating the `--mcp` flag and give each MCPTool a unique name so the agent can distinguish them.

## Design

- Repeated `--mcp` flags, each with an optional label: `--mcp "label:command-or-url"`
- If no label is provided, auto-generate: `mcp` for first, `mcp2`, `mcp3`, etc.
- Label example: `--mcp "fs:mcp-filesystem-server /tmp"` → tool name `mcp_fs`
- System prompt MCP routing line becomes dynamic based on registered MCP tool names

## Files to Modify

### 1. `main.go` — Repeated `--mcp` flag support

Add a custom `stringSlice` flag type to collect multiple `--mcp` values:

```go
type stringSlice []string
func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(val string) error { *s = append(*s, val); return nil }
```

Replace `flag.String("mcp", ...)` with:
```go
var mcpSpecs stringSlice
flag.Var(&mcpSpecs, "mcp", "MCP server (repeatable). Format: [label:]command-or-url")
```

Loop over `mcpSpecs`, parse optional `label:` prefix, create each MCPTool:
```go
for i, spec := range mcpSpecs {
    label, target := parseMCPSpec(spec, i)
    // create tool via NewMCPTool or NewMCPToolFromURL based on target
    // pass label to constructor so Name() returns it
}
```

Add `parseMCPSpec(spec string, index int) (label, target string)` helper:
- If spec contains `:` where the part before `:` doesn't look like a URL scheme → split on first `:` as `label:target`, tool name = `mcp_<label>`
- Otherwise: `mcp` for index 0, `mcp2` for index 1, etc.

### 2. `tools/mcp.go` — Configurable tool name

Add a `name` field to `MCPTool` struct. Update `Name()` to return it.

Update `initMCPTool` signature to accept the name:
```go
func initMCPTool(ctx context.Context, c *client.Client, name, label string) (*MCPTool, error)
```

Update `NewMCPTool` and `NewMCPToolFromURL` to accept `name string` parameter:
```go
func NewMCPTool(ctx context.Context, name, command string, args []string) (*MCPTool, error)
func NewMCPToolFromURL(ctx context.Context, name, serverURL string) (*MCPTool, error)
```

### 3. `llm/ollama.go` — Dynamic MCP routing in system prompt

Replace the hardcoded MCP line:
```
- "mcp", file operations on MCP server, MCP tool calls → use "mcp" tool
```

With a dynamic line built from tool names. `BuildSystemPrompt` already receives `[]ToolDef` — scan for tools whose name starts with `mcp` and generate the routing line listing them all:
```
- "mcp", MCP tool calls → use "mcp_fs" or "mcp_k8s" tool (check descriptions for available tools)
```

If no MCP tools present, omit the line entirely (as with wiki today).

## Usage

```bash
# Single server (backward compatible)
./langchain-agent --mcp "mcp-filesystem-server /tmp"

# Multiple servers with auto-naming
./langchain-agent --mcp "mcp-filesystem-server /tmp" --mcp "http://k8s:8080/sse"

# Multiple servers with labels
./langchain-agent --mcp "fs:mcp-filesystem-server /tmp" --mcp "k8s:http://k8s:8080/sse"
```

## Verification

1. `go build -o langchain-agent .`
2. `go test ./...` — all existing tests pass
3. Single `--mcp` still works as before
4. Multiple `--mcp` flags create separate tools with unique names
5. System prompt lists all MCP tools in routing section

## Future: Phase 2 — Improved MCP tool descriptions

Currently the LLM distinguishes MCP servers only by their discovered sub-tool names (e.g. `read_file`, `list_pods`). This works when sub-tool names are self-explanatory but could be ambiguous with multiple servers.

**Improvement:** Incorporate the user-provided label into `Description()` so the LLM gets an extra hint about each server's purpose:
- `"MCP server (fs). Available tools: read_file, list_directory"`
- `"MCP server (k8s). Available tools: list_pods, get_logs"`

This only requires a small change to `MCPTool.Description()` to use the stored name/label.
