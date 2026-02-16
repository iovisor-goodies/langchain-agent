# LangChain Agent

Autonomous agent loop using [LangChainGo](https://github.com/tmc/langchaingo) + [Ollama](https://ollama.com/) for local LLM inference.

## Features

- **JSON tool calling** (not ReAct) - reliable with smaller models
- **SSH tool** - execute commands on remote hosts
- **Shell tool** - execute local commands
- **MCP tool** - connect to any MCP server via stdio (e.g., filesystem, Kubernetes)
- **Wiki RAG tool** - search Confluence wiki exports with diagram support
- **Conversation memory** - maintains context until cleared
- **Honest error reporting** - no hallucination on failures

## Quick Start

```bash
# Build
go build -o langchain-agent .

# Run (requires Ollama with llama3.1)
ollama pull llama3.1
./langchain-agent
```

## Usage

```
> ssh to admin@192.168.1.10 and tell me what platform it is
[Agent] {"name": "ssh", "parameters": {"host": "admin@192.168.1.10", "command": "uname -a"}}
[Tool Call] ssh: map[command:uname -a host:admin@192.168.1.10]
[Tool Result] Linux server 5.15.0-generic x86_64 GNU/Linux
[Answer] The server is running Linux (kernel 5.15.0) on x86_64 architecture.

> check disk usage there
[Agent] {"name": "ssh", "parameters": {"host": "admin@192.168.1.10", "command": "df -h"}}
...
```

Commands:
- `clear` - clear conversation history
- `quit` / `exit` - exit

## Options

```bash
./langchain-agent -model llama3.2    # Use smaller/faster model
./langchain-agent -max-iter 5        # Limit agent iterations
./langchain-agent --wiki ~/wiki/     # Enable wiki RAG tool
./langchain-agent --wiki ~/wiki/ --index-only  # Index wiki only
./langchain-agent --qdrant http://localhost:6333  # Custom Qdrant URL
./langchain-agent --mcp "mcp-filesystem-server /tmp"  # Enable MCP tool
```

## Wiki RAG

Search Confluence wiki exports with semantic search and diagram understanding.

See [docs/confluence-import.md](docs/confluence-import.md) for detailed import instructions.

### Prerequisites

```bash
# Pull required Ollama models
ollama pull nomic-embed-text   # For embeddings
ollama pull llava              # For image/diagram description

# Run Qdrant vector store
docker run -d -p 6333:6333 qdrant/qdrant
```

### Usage

```bash
# Export your Confluence space as HTML, then:
./langchain-agent --wiki ~/wiki/confluence-export/

> search wiki for deployment architecture
> what does the network diagram show
```

The wiki tool:
- Parses Confluence HTML exports
- Extracts text (headings, paragraphs, lists, code)
- Uses LLaVA to describe diagrams/images
- Stores embeddings in Qdrant for semantic search
- Returns relevant text chunks and diagram descriptions

## Architecture

```
langchain-agent/
├── main.go              # REPL entry point
├── agent/
│   ├── agent.go         # Agent loop (tool dispatch, history)
│   └── agent_test.go    # Tests with mock LLM
├── llm/
│   ├── ollama.go        # Ollama client, JSON parsing
│   └── ollama_test.go   # Parsing tests
├── rag/
│   ├── embeddings.go    # Ollama embeddings (nomic-embed-text)
│   ├── store.go         # Qdrant vector store
│   ├── loader.go        # Confluence HTML parser
│   ├── vision.go        # LLaVA image description
│   └── indexer.go       # Wiki indexing pipeline
└── tools/
    ├── tool.go          # Tool interface
    ├── ssh.go           # Remote execution
    ├── shell.go         # Local execution
    ├── mcp.go           # MCP client (real, via mcp-go SDK)
    └── wiki.go          # Wiki RAG search
```

### How It Works

1. User input → Agent builds messages (system prompt + history + input)
2. Send to Ollama LLM
3. LLM returns JSON tool call or final answer
4. If tool call → execute tool, append result, loop back to step 2
5. If final answer → return to user

The agent maintains context across turns. Follow-up questions ("try grep vmx instead") apply to the same host/task.

## Requirements

- Go 1.21+
- [Ollama](https://ollama.com/) running locally
- `llama3.1` model (recommended) or `llama3.2`
- For wiki RAG: `nomic-embed-text`, `llava` models, and Qdrant (Docker)

## SSH Authentication

Uses standard SSH authentication:
- ssh-agent
- `~/.ssh/id_rsa`, `~/.ssh/id_ed25519`
- Keys configured via `ssh-copy-id`

## Testing

```bash
go test ./...
```

## License

MIT
