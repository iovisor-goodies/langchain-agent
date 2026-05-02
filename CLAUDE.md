# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Portable autonomous agent loop using LangChainGo + Ollama/Gemini. Uses JSON tool calling (not ReAct) for reliability with small models.

## Status

**Working:**
- ✅ Agent loop with tool dispatch
- ✅ SSH tool (remote command execution, ssh-agent + interactive password fallback)
- ✅ Shell tool (local command execution)
- ✅ MCP tool (multiple servers, stdio/SSE/HTTP transport, via mark3labs/mcp-go)
- ✅ Conversation history/memory
- ✅ Tool selection rules in prompt
- ✅ Honest error reporting (no hallucination on failures)
- ✅ Wiki RAG tool (Confluence HTML export with diagram support)
- ✅ Gemini backend (Google AI, via `--backend gemini`, requires `GOOGLE_API_KEY`)

**TODO:**
- ✅ Streaming output
- [ ] Domain knowledge improvements (command patterns)
- [ ] Event-driven automation

## Use Cases

```
"ssh to x@y.z and tell me what platform it is"   # → ssh tool
"ssh to x@y.z and see why pods are failing"       # → ssh tool
"list running processes"                          # → shell tool
"check disk space"                                # → shell tool
"use mcp to list files in /tmp"                   # → mcp tool (requires --mcp)
"use mcp to read the file /tmp/test.txt"          # → mcp tool (requires --mcp)
"search wiki for deployment architecture"         # → wiki tool (requires --wiki)
"what does the network diagram show"              # → wiki tool (requires --wiki)
"what is a container?"                            # → direct answer (no tool)
"is Go faster than Python?"                       # → direct answer (no tool)
```

**Note:** MCP requires explicitly saying "mcp" in the prompt. Tool routing keywords are hardcoded in the system prompt (`llm/ollama.go:BuildSystemPrompt`). The MCP routing line is dynamically generated based on registered MCP tool names (`llm/ollama.go:mcpRoutingLine`).

## Backends

Two LLM backends are supported, selected via `--backend`. Both implement the same `llm.ChatClient` + `llm.StreamingChatClient` interface, so all tools (ssh, shell, mcp, wiki) and the agent loop behave identically across backends.

### Ollama (default, local)

```bash
./langchain-agent                       # default model: llama3.1
./langchain-agent --model llama3.2      # smaller/faster, less reliable for tool calling
```
- Requires Ollama running locally (default `http://localhost:11434`).
- No API key needed.
- llama3.1 is the recommended floor for reliable JSON tool calling.

### Gemini (Google AI, cloud)

```bash
export GOOGLE_API_KEY="$(tr -d '[:space:]' < ~/gemini-api-key.txt)"
./langchain-agent --backend gemini                       # default model: gemini-2.5-flash
./langchain-agent --backend gemini --model gemini-2.5-pro
```
- Requires `GOOGLE_API_KEY` env var. The `langchaingo/llms/googleai` package reads it automatically.
- Get a key from https://aistudio.google.com/apikey.
- Verified working end-to-end: knowledge questions, shell tool dispatch, and SSH tool dispatch (including conversation context across turns).

**Known quirks:**
- `gemini-2.0-flash` returns HTTP 404 with langchaingo v0.1.14 even though it's listed in the API's models endpoint. Use `gemini-2.5-flash` (the project default) or newer.
- API keys auto-expire after ~30 days of inactivity. The error message is `"API key expired. Please renew the API key."` even when the AI Studio dashboard doesn't flag the key as expired.

## Build and Test Commands

```bash
go build -o langchain-agent .
./langchain-agent                                    # Run with default model (llama3.1)
./langchain-agent -model llama3.2                    # Use smaller/faster model (less reliable)
GOOGLE_API_KEY=... ./langchain-agent --backend gemini               # Gemini (default: gemini-2.5-flash)
GOOGLE_API_KEY=... ./langchain-agent --backend gemini --model gemini-2.5-pro  # Gemini with specific model
./langchain-agent --wiki ~/wiki/     # Enable wiki RAG (requires Qdrant)
./langchain-agent --wiki ~/wiki/ --index-only  # Index only, then exit
./langchain-agent --mcp "mcp-filesystem-server /tmp"      # Single MCP server (stdio)
./langchain-agent --mcp "fs:mcp-filesystem-server /tmp"   # Labeled MCP server → tool "mcp_fs"
./langchain-agent --mcp "mcp-filesystem-server /tmp" --mcp "http://localhost:8080"  # Multiple servers
./langchain-agent --mcp "http://localhost:8080/sse"        # SSE transport (URL ending in /sse)
./langchain-agent --mcp "http://localhost:8080"            # Streamable HTTP transport

go test ./...                        # Run all tests
go test -v ./agent/...               # Agent loop tests (with mock LLM)
go test -v ./llm/...                 # JSON parsing tests
go test -v ./tools/...               # Tool unit tests
go test -v ./rag/...                 # RAG loader tests
go test -tags integration -v ./tools/...  # MCP integration tests (needs mcp-filesystem-server)
```

## Architecture

```
langchain-agent/
├── main.go              # REPL entry point
├── agent/
│   ├── agent.go         # Agent loop (tool dispatch, history)
│   └── agent_test.go    # Tests with mock LLM client
├── llm/
│   ├── ollama.go        # Ollama client, JSON tool call parsing, shared helpers
│   ├── gemini.go        # Gemini client (Google AI)
│   └── ollama_test.go   # Parsing tests
├── rag/
│   ├── embeddings.go    # Ollama embeddings client (nomic-embed-text)
│   ├── store.go         # Qdrant vector store wrapper
│   ├── loader.go        # Confluence HTML parser
│   ├── vision.go        # LLaVA image description
│   ├── indexer.go       # Wiki indexing orchestration
│   └── loader_test.go   # Loader tests
└── tools/
    ├── tool.go          # Tool interface
    ├── ssh.go           # SSH remote execution
    ├── shell.go         # Local shell execution
    ├── mcp.go           # MCP client (real, via mcp-go SDK)
    ├── wiki.go          # Wiki RAG search tool
    └── *_test.go        # Tool tests
```

**Key design decisions:**
- JSON tool calling in system prompt (not ReAct) - reliable with llama3.2
- Explicit tool selection rules in prompt to prevent wrong tool choice
- Clear error messages distinguish "no output" from "command failed"
- `llm.ChatClient` interface allows mocking in tests
- SSH auth: tries ssh-agent → key files → interactive password prompt (like `ssh` itself)
- MCP: repeatable `--mcp` flag supports multiple servers with label syntax and auto-naming:
  - `label:command args` → tool name `mcp_<label>` (stdio transport)
  - `http://...` → Streamable HTTP transport; `http://.../sse` → SSE transport
  - No label: first server = `mcp`, subsequent = `mcp2`, `mcp3`, etc.
  - System prompt MCP routing line is dynamically built from registered tool names

## Research Findings

LangChainGo doesn't have first-class Ollama native tool calling in agent framework. Options:
1. **Manual JSON approach** (current) - embed tool schemas in prompt, parse JSON responses
2. ReAct agents - unreliable with small models
3. OpenAIFunctionsAgent - designed for OpenAI, not Ollama

## Key Packages Used

- `github.com/tmc/langchaingo/llms/ollama` - Ollama LLM integration
- `github.com/tmc/langchaingo/llms/googleai` - Google AI (Gemini) integration
- `github.com/tmc/langchaingo/embeddings` - Text embeddings
- `github.com/mark3labs/mcp-go` - MCP client (stdio, SSE, Streamable HTTP transport)
- `golang.org/x/crypto/ssh` - SSH client
- `golang.org/x/term` - Terminal password input (hidden)
- `golang.org/x/net/html` - HTML parsing for Confluence export

## Git

- Do NOT add "Co-Authored-By: Claude" to commits
