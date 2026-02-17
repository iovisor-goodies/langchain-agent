# Plan: Test RAG Pipeline with Sample Data

## Context

The RAG system (Confluence HTML → embeddings → Qdrant → semantic search) is implemented but untested with real data. User has Qdrant running and Ollama models pulled (`nomic-embed-text`, `llava`), but no Confluence export yet. We'll create realistic sample HTML data and run the full pipeline end-to-end.

## Step 1: Create sample Confluence HTML export

Create `testdata/wiki/` with 3-4 HTML pages mimicking Confluence export structure:

- `testdata/wiki/deployment-guide.html` — deployment procedures, K8s concepts
- `testdata/wiki/network-architecture.html` — network topology, firewall rules
- `testdata/wiki/troubleshooting.html` — common issues and fixes
- `testdata/wiki/images/` — a simple PNG diagram (can be a placeholder)

Each HTML file uses Confluence-style markup (`<h1>`, `<p>`, `<pre>`, `<li>`, `<img>`) so the loader parses them correctly.

## Step 2: Index the sample wiki

```bash
go build -o langchain-agent .
./langchain-agent --wiki testdata/wiki/ --index-only
```

Verify:
- Pages are loaded and chunked
- Embeddings are generated (nomic-embed-text)
- Images are described (llava) if present
- Documents are upserted to Qdrant collection `confluence_wiki`

## Step 3: Run the agent with wiki tool enabled

```bash
./langchain-agent --wiki testdata/wiki/
```

Test queries:
- `"search wiki for deployment steps"` → should return deployment guide chunks
- `"what does the network diagram show"` → should return image description
- `"search wiki for troubleshooting pods"` → should return troubleshooting content

## Step 4: Verify Qdrant directly

```bash
curl http://localhost:6333/collections/confluence_wiki
```

Check document count, collection status.

## Files Created

| File | Purpose |
|------|---------|
| `testdata/wiki/deployment-guide.html` | K8s deployment procedures |
| `testdata/wiki/network-architecture.html` | Network topology docs |
| `testdata/wiki/troubleshooting.html` | Common issue fixes |
| `testdata/wiki/images/diagram.png` | Placeholder diagram |

No existing code changes needed — this is purely creating test fixtures and running the pipeline.

## Results

- **Indexing**: 3 pages → 62 document chunks stored in Qdrant (768-dim cosine vectors)
- **Image processing**: LLaVA described diagram.png, stored as [DIAGRAM] document
- **Qdrant**: Collection `confluence_wiki` green, 62 points
- **Query: deployment steps** → Deployment guide chunks (score 0.70)
- **Query: troubleshooting pods** → Troubleshooting guide chunks (score 0.73)
- **Query: network diagram** → LLaVA-described diagram (score 0.62)
- **Multi-turn**: Second query without "search wiki" keyword correctly routed through wiki tool using conversation context
