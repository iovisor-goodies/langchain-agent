# Project Plan: LangChain Agent

Portable autonomous agent loop using LangChainGo + Ollama. Uses JSON tool calling (not ReAct) for reliability with small models.

## Completed

### 1. Core Agent Loop
- Agent loop with tool dispatch (`agent/agent.go`)
- JSON tool call parsing from LLM responses (`llm/ollama.go`)
- System prompt with tool definitions and routing rules
- Conversation history/memory across turns
- Max iteration guard to prevent infinite loops
- `ChatClient` interface for mocking in tests

### 2. SSH Tool
- Remote command execution via `golang.org/x/crypto/ssh`
- Host/command parameters parsed from LLM tool calls
- Auth chain: ssh-agent → key files (~/.ssh/id_{rsa,ed25519,ecdsa}) → interactive password prompt
- Password fallback uses `golang.org/x/term` for hidden terminal input
- Supports both `ssh.Password` and `ssh.KeyboardInteractive` methods

### 3. Shell Tool
- Local command execution
- Routed via keywords: "run command", "check files", local operations

### 4. Wiki RAG Tool
- Confluence HTML export parser (`rag/loader.go`)
- LLaVA image description for diagrams (`rag/vision.go`)
- Ollama embeddings via nomic-embed-text (`rag/embeddings.go`)
- Qdrant vector store (`rag/store.go`)
- Wiki indexing orchestration (`rag/indexer.go`)
- `--wiki` flag to enable, `--index-only` to index and exit
- Keywords: "wiki", "confluence", "documentation", "diagram", "architecture"

### 5. MCP Tool (Real Client)
- Replaced stubbed MCP with real client via `mark3labs/mcp-go` SDK
- Stdio transport, spawns server process
- Dynamic tool discovery via `ListTools` at startup
- Single wrapper tool (`tool_name` + `arguments` parameters)
- `Closeable` interface for cleanup
- `--mcp` flag, e.g. `--mcp "mcp-filesystem-server /tmp"`
- Unit tests with mock `MCPClient`, integration tests with build tag
- Keywords: "mcp", file operations on MCP server

### 6. System Prompt & Tool Routing
- Explicit routing rules prevent wrong tool choice
- Keywords hardcoded in `llm/ollama.go:BuildSystemPrompt`
- Knowledge questions answered directly (no tool)

### 7. Hallucination Fix
- LLM was fabricating tool results after JSON tool calls
- `parseResponse()` now truncates content to just the tool call JSON
- Discards any hallucinated output after the closing brace

### 8. Streaming Output
- `StreamingChatClient` interface extends `ChatClient` (backward compatible)
- `ChatStream()` on `Client` uses `llms.WithStreamingFunc`
- Buffer-then-decide: first non-whitespace `{` → buffer silently (tool call), otherwise stream tokens
- `Agent.Run()` type-asserts for streaming; falls back to `Chat()` for non-streaming clients
- Transparent to REPL — no `main.go` changes needed

### 9. Slash Commands
- `/help`, `/clear`, `/exit` as REPL commands handled in `main.go`
- Backward compatible: bare `quit`, `exit`, `clear` still work
- Startup banner updated to mention `/help`

## TODO

- [ ] Domain knowledge improvements (command patterns)
- [ ] Event-driven automation
