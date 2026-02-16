# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Portable autonomous agent loop using LangChainGo + Ollama. Uses JSON tool calling (not ReAct) for reliability with small models.

## Status

**Working:**
- ✅ Agent loop with tool dispatch
- ✅ SSH tool (remote command execution)
- ✅ Shell tool (local command execution)
- ✅ MCP tool (real client via mark3labs/mcp-go, stdio transport)
- ✅ Conversation history/memory
- ✅ Tool selection rules in prompt
- ✅ Honest error reporting (no hallucination on failures)
- ✅ Wiki RAG tool (Confluence HTML export with diagram support)

**TODO:**
- [ ] Streaming output
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

**Note:** MCP requires explicitly saying "mcp" in the prompt. Tool routing keywords are hardcoded in the system prompt (`llm/ollama.go:BuildSystemPrompt`).

## Build and Test Commands

```bash
go build -o langchain-agent .
./langchain-agent                    # Run with default model (llama3.1)
./langchain-agent -model llama3.2    # Use smaller/faster model (less reliable)
./langchain-agent --wiki ~/wiki/     # Enable wiki RAG (requires Qdrant)
./langchain-agent --wiki ~/wiki/ --index-only  # Index only, then exit
./langchain-agent --mcp "mcp-filesystem-server /tmp"  # Enable MCP tool

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
│   ├── ollama.go        # Ollama client, JSON tool call parsing
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

## Research Findings

LangChainGo doesn't have first-class Ollama native tool calling in agent framework. Options:
1. **Manual JSON approach** (current) - embed tool schemas in prompt, parse JSON responses
2. ReAct agents - unreliable with small models
3. OpenAIFunctionsAgent - designed for OpenAI, not Ollama

## Key Packages Used

- `github.com/tmc/langchaingo/llms/ollama` - Ollama LLM integration
- `github.com/tmc/langchaingo/embeddings` - Text embeddings
- `github.com/mark3labs/mcp-go` - MCP client (stdio transport)
- `golang.org/x/crypto/ssh` - SSH client
- `golang.org/x/net/html` - HTML parsing for Confluence export

## Git

- Do NOT add "Co-Authored-By: Claude" to commits
