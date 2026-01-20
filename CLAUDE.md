# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Portable autonomous agent loop using LangChainGo + Ollama. Uses JSON tool calling (not ReAct) for reliability with small models.

## Status

**Working:**
- ✅ Agent loop with tool dispatch
- ✅ SSH tool (remote command execution)
- ✅ Shell tool (local command execution)
- ✅ MCP tool (stubbed with mock data)
- ✅ Conversation history/memory
- ✅ Tool selection rules in prompt
- ✅ Honest error reporting (no hallucination on failures)

**TODO:**
- [ ] Real MCP client implementation
- [ ] Streaming output
- [ ] Domain knowledge improvements (command patterns)
- [ ] Event-driven automation

## Use Cases

```
"ssh to x@y.z and tell me what platform it is"
"ssh to x@y.z and see why pods are failing"
"use MCP server on test.my.domain and see what pods are failing in openshift namespace"
```

## Build and Test Commands

```bash
go build -o langchain-agent .
./langchain-agent                    # Run with default model (llama3.1)
./langchain-agent -model llama3.2    # Use smaller/faster model (less reliable)

go test ./...                        # Run all tests
go test -v ./agent/...               # Agent loop tests (with mock LLM)
go test -v ./llm/...                 # JSON parsing tests
go test -v ./tools/...               # Tool unit tests
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
└── tools/
    ├── tool.go          # Tool interface
    ├── ssh.go           # SSH remote execution
    ├── shell.go         # Local shell execution
    ├── mcp.go           # MCP client (stubbed)
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
- `golang.org/x/crypto/ssh` - SSH client

## Git

- Do NOT add "Co-Authored-By: Claude" to commits
