# LangChain Agent

Portable autonomous agent loop using [LangChainGo](https://github.com/tmc/langchaingo) with **Ollama** (local or remote) or **Gemini** (Google AI) for LLM inference. Uses JSON tool calling (not ReAct) for reliable tool use with smaller models.

## Features

- **JSON tool calling** (not ReAct) — reliable with smaller models
- **Two LLM backends** — Ollama (local *or* remote via `--ollama-url`) and Gemini (Google AI), selected with `--backend`
- **Multi-hop agent loop** — chains tool calls to answer a request, then summarizes
- **Streaming output** — tokens render as the model generates them
- **SSH tool** — execute commands on remote hosts (ssh-agent → keys → interactive password fallback)
- **Shell tool** — execute local commands
- **MCP tool** — connect to one or more MCP servers via stdio / SSE / streamable-HTTP
- **Wiki RAG tool** — semantic search over Confluence HTML exports, with diagram understanding
- **Edge sensor tools** — `edge_temp` / `edge_gpio` operate a remote Linux box (Pi, NUC, mini-PC) over SSH
- **HTTP webhook** — `POST /webhook` runs the agent, for event-driven use alongside the REPL
- **Conversation memory** — maintains context until cleared
- **Honest error reporting** — no hallucination on failures

## Quick Start

```bash
# Build
go build -o langchain-agent .

# Run with Ollama (default backend). Default model is qwen2.5:32b (needs a GPU).
ollama pull qwen2.5:32b
./langchain-agent

# On a modest box, use a smaller model instead:
ollama pull llama3.1
./langchain-agent --model llama3.1
```

## Usage

```
> ssh to admin@192.168.1.10 and tell me what platform it is
[Tool Call] ssh: map[command:uname -a host:admin@192.168.1.10]
[Tool Result] Linux server 5.15.0-generic x86_64 GNU/Linux
[Answer] The server is running Linux (kernel 5.15.0) on x86_64 architecture.

> check disk usage there
[Tool Call] ssh: map[command:df -h host:admin@192.168.1.10]
...
```

REPL commands: `/help`, `/clear` (clear history), `/exit` (or `/quit`).

## Backends

### Ollama (default)

```bash
./langchain-agent                                              # default model: qwen2.5:32b
./langchain-agent --model llama3.1                             # smaller, reliable floor for tool calling
./langchain-agent --ollama-url http://big-tower.local:11434   # remote Ollama host (e.g. a GPU tower)
```

- Requires an Ollama server (default `http://localhost:11434`).
- `--ollama-url` points at a remote host; also honors `$OLLAMA_HOST` when the flag is unset.
- The model must expose the `tools` capability for reliable JSON tool calling — `qwen2.5:32b` and `llama3.1` do; `llama3.2` (3B) works but is less reliable.

### Gemini (Google AI, cloud)

```bash
export GOOGLE_API_KEY=your-key-here
./langchain-agent --backend gemini                     # default model: gemini-2.5-flash
./langchain-agent --backend gemini --model gemini-2.5-pro
```

- Requires `GOOGLE_API_KEY` (read automatically by langchaingo). Get one at https://aistudio.google.com/apikey.
- Use `gemini-2.5-flash` or newer (`gemini-2.0-flash` 404s with langchaingo v0.1.14).

## Options

```bash
./langchain-agent --backend gemini                     # Use Gemini instead of Ollama
./langchain-agent --model llama3.1                     # Choose a model
./langchain-agent --ollama-url http://host:11434       # Remote Ollama server
./langchain-agent --max-iter 5                         # Limit agent iterations
./langchain-agent --wiki ~/wiki/                       # Enable wiki RAG tool
./langchain-agent --wiki ~/wiki/ --index-only          # Index wiki only, then exit
./langchain-agent --qdrant http://localhost:6333       # Custom Qdrant URL
./langchain-agent --mcp "mcp-filesystem-server /tmp"   # Enable an MCP server (repeatable)
./langchain-agent --edge eagle@192.168.1.63            # Enable edge_temp / edge_gpio tools
./langchain-agent --webhook-port 8090                  # Start HTTP webhook listener
```

## Tool Routing

The agent uses keyword matching in the system prompt to decide which tool to use. Routing lines for MCP and edge tools are generated dynamically based on which tools are actually registered.

| Prompt pattern | Tool | Examples |
|---|---|---|
| "ssh to", "connect to", user@host, IP address | **ssh** | "ssh to root@10.0.0.1 and check uptime" |
| Local operations, run commands, check local files | **shell** | "list running processes", "what's my hostname" |
| "mcp", MCP tool calls | **mcp** | "use mcp to list files in /tmp" |
| "wiki", "confluence", "documentation", "diagram" | **wiki** | "search wiki for deployment architecture" |
| "cpu temp", "temperature" on the edge box | **edge_temp** | "what is the cpu temperature on the pi" |
| "gpio", "pin", "read pin", "set pin" | **edge_gpio** | "read gpio pin 17" |
| Knowledge questions, explanations, opinions | *direct answer* | "what is a container?", "is Go faster than Python?" |

**Note:** MCP requires explicitly saying "mcp" in the prompt. Edge tools require `--edge`; wiki requires `--wiki`.

## MCP Servers

The `--mcp` flag is repeatable and supports labels and multiple transports:

```bash
./langchain-agent --mcp "mcp-filesystem-server /tmp"       # stdio; tool name "mcp"
./langchain-agent --mcp "fs:mcp-filesystem-server /tmp"    # labeled → tool name "mcp_fs"
./langchain-agent --mcp "http://localhost:8080"            # streamable HTTP transport
./langchain-agent --mcp "http://localhost:8080/sse"        # SSE transport (URL ending in /sse)
./langchain-agent --mcp "cmd-a ..." --mcp "http://host-b"  # multiple servers at once
```

Unlabeled servers auto-name as `mcp`, `mcp2`, `mcp3`, ...

## Edge Sensor Tools

First-class tools that operate a remote Linux box over SSH (Raspberry Pi, NUC, mini-PC, x86 thin client — not Pi-specific). The agent runs on your workstation; the edge box is set once via `--edge user@host`.

- `edge_temp` — CPU temperature via `/sys/class/thermal/thermal_zone0/temp` → `XX.X°C`
- `edge_gpio` — read/write GPIO lines via libgpiod (`gpioget` / `gpioset`; default `gpiochip0`, Pi 5 uses `gpiochip4`)

Because they're multi-hop verbs, the agent can chain them — e.g. read `edge_temp`, decide against a threshold, then flip a pin with `edge_gpio`.

> **Scope:** this is a workstation agent that *operates* edge devices remotely. Running the agent *on* the Pi (ARM64-resident) is a separate project, out of scope here.

## HTTP Webhook

Run the agent from external events alongside the REPL:

```bash
./langchain-agent --ollama-url http://big-tower.local:11434 --webhook-port 8090 < /dev/null &

curl -s http://localhost:8090/health                                    # → OK
curl -s -X POST http://localhost:8090/webhook \
     -H 'Content-Type: application/json' \
     -d '{"prompt":"what is the cpu temp on the pi"}' | jq .
# → {"answer":"..."}
```

- `POST /webhook` — body `{"prompt": "..."}` → `{"answer": "..."}` (or `{"error": "..."}`)
- `GET /health` — liveness probe
- REPL and webhook share one agent, serialized by a mutex. Closing stdin (`< /dev/null`) runs it headless.

## Wiki RAG

Search Confluence HTML exports with semantic search and diagram understanding. See [docs/confluence-import.md](docs/confluence-import.md) for import instructions.

### Prerequisites

```bash
ollama pull nomic-embed-text   # embeddings
ollama pull llava              # image/diagram description
docker run -d -p 6333:6333 qdrant/qdrant   # Qdrant vector store
```

### Usage

```bash
./langchain-agent --wiki ~/wiki/confluence-export/

> search wiki for deployment architecture
> what does the network diagram show
```

The wiki tool parses Confluence HTML, extracts text (headings, paragraphs, lists, code), uses LLaVA to describe diagrams, stores embeddings in Qdrant, and returns relevant chunks and diagram descriptions.

## Architecture

```
langchain-agent/
├── main.go              # REPL entry point + flag wiring
├── agent/
│   ├── agent.go         # Agent loop (tool dispatch, history, mutex)
│   └── agent_test.go    # Tests with mock LLM
├── llm/
│   ├── ollama.go        # Ollama client, JSON tool-call parsing, prompt building
│   ├── gemini.go        # Gemini client (Google AI)
│   └── ollama_test.go   # Parsing tests
├── webhook/
│   └── server.go        # HTTP webhook listener (POST /webhook, GET /health)
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
    ├── mcp.go           # MCP client (via mcp-go SDK)
    ├── wiki.go          # Wiki RAG search
    ├── edge_helper.go   # Shared SSH executor for edge_* tools
    ├── edge_temp.go     # CPU temp via /sys/class/thermal
    └── edge_gpio.go     # GPIO read/write via libgpiod
```

### How It Works

1. User input (REPL or webhook) → Agent builds messages (system prompt + history + input)
2. Send to the configured backend (Ollama or Gemini)
3. LLM returns a JSON tool call or a final answer
4. If tool call → execute tool, append result, loop back to step 2
5. If final answer → return to user

The agent maintains context across turns, so follow-ups ("try grep vmx instead") apply to the same host/task.

## Requirements

- Go 1.21+
- **Ollama backend:** [Ollama](https://ollama.com/) (local or remote) with a `tools`-capable model — `qwen2.5:32b` (default, needs a GPU) or `llama3.1`
- **Gemini backend:** `GOOGLE_API_KEY` env var
- **Wiki RAG:** `nomic-embed-text` + `llava` models and Qdrant (Docker)

## SSH Authentication

Standard SSH auth chain: ssh-agent → key files (`~/.ssh/id_rsa`, `~/.ssh/id_ed25519`) → interactive password prompt.

## Testing

```bash
go test ./...
```

## License

MIT
