# Plan: Confluence Wiki RAG with Diagrams

**Status: COMPLETED**

## Goal
Add RAG capability to query Confluence wiki content including diagrams.

## Implementation

See `rag/` directory for implementation:
- `embeddings.go` - Ollama embeddings client (nomic-embed-text)
- `store.go` - Qdrant vector store wrapper
- `loader.go` - Confluence HTML parser
- `vision.go` - LLaVA image description with caching
- `indexer.go` - Wiki indexing orchestration

See `tools/wiki.go` for the wiki search tool.

## Usage

```bash
# Prerequisites
ollama pull nomic-embed-text
ollama pull llava
docker run -d -p 6333:6333 qdrant/qdrant

# Index and run
./langchain-agent --wiki ~/wiki/confluence-export/

# Index only
./langchain-agent --wiki ~/wiki/confluence-export/ --index-only

# Query
> search wiki for deployment architecture
> what does the network diagram show
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              INDEXING (one-time)                            │
└─────────────────────────────────────────────────────────────────────────────┘

  Confluence HTML Export
       │
       ├── index.html, page1.html, page2.html...
       └── images/diagram1.png, diagram2.png...
       │
       ▼
  ┌──────────────────┐
  │  HTML Parser     │
  │  (loader.go)     │
  └────────┬─────────┘
           │
           ├─────────────────────────────────┐
           │                                 │
           ▼                                 ▼
  ┌─────────────────┐               ┌─────────────────┐
  │  Text Chunks    │               │  Images/Diagrams│
  │  (paragraphs,   │               │  (PNG, JPG)     │
  │   headers)      │               └────────┬────────┘
  └────────┬────────┘                        │
           │                                 ▼
           │                        ┌─────────────────┐
           │                        │  LLaVA Model    │
           │                        │  (describe      │
           │                        │   diagram)      │
           │                        └────────┬────────┘
           │                                 │
           ▼                                 ▼
  ┌─────────────────┐               ┌─────────────────┐
  │  nomic-embed    │               │  Image          │
  │  (text → vec)   │               │  Descriptions   │
  └────────┬────────┘               └────────┬────────┘
           │                                 │
           │                        ┌────────┴────────┐
           │                        │  nomic-embed    │
           │                        │  (desc → vec)   │
           │                        └────────┬────────┘
           │                                 │
           └─────────────┬───────────────────┘
                         │
                         ▼
                ┌─────────────────┐
                │     Qdrant      │
                │   (Docker)      │
                │   :6333         │
                └─────────────────┘


┌─────────────────────────────────────────────────────────────────────────────┐
│                              QUERYING (runtime)                             │
└─────────────────────────────────────────────────────────────────────────────┘

  User: "show me the network architecture diagram"
       │
       ▼
  ┌─────────┐      ┌──────────────┐      ┌─────────────────┐
  │  Agent  │ ───▶ │  Wiki Tool   │ ───▶ │  Embed query    │
  │  Loop   │      │  (wiki.go)   │      │  via Ollama     │
  └─────────┘      └──────────────┘      └────────┬────────┘
       ▲                                          │
       │                                          ▼
       │                                 ┌─────────────────┐
       │                                 │ Similarity      │
       │                                 │ Search Qdrant   │
       │                                 └────────┬────────┘
       │                                          │
       │           ┌──────────────┐               │
       └───────────│ Results:     │◀──────────────┘
                   │ - Text chunks│
                   │ - Diagram    │
                   │   descriptions
                   └──────────────┘
```

## Completed Phases

- [x] Phase 1: HTML Loader - Parse Confluence HTML, extract text, find images
- [x] Phase 2: Image Description - LLaVA integration with JSON caching
- [x] Phase 3: Embeddings + Storage - nomic-embed-text + Qdrant
- [x] Phase 4: Wiki Tool - search action with metadata
- [x] Phase 5: Integration - --wiki flag, indexing on startup

## How It Works (Under the Hood)

### Indexing Pipeline

```
┌─────────────────────────────────────────────────────────────────────────┐
│  1. HTML PARSING (rag/loader.go)                                        │
└─────────────────────────────────────────────────────────────────────────┘
     │
     │  Walks wiki directory, parses each .html file
     │  using golang.org/x/net/html
     │
     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Extracts:                                                              │
│  • <h1>-<h6> → heading chunks                                           │
│  • <p> → paragraph chunks                                               │
│  • <li> → list item chunks                                              │
│  • <pre>/<code> → code chunks                                           │
│  • <img src="..."> → image references                                   │
└─────────────────────────────────────────────────────────────────────────┘
     │
     ├──────────────────────────────────┐
     │                                  │
     ▼                                  ▼
┌──────────────────────┐    ┌──────────────────────────────────────────┐
│  Text chunks         │    │  Images (PNG, JPG, etc.)                 │
│  (split if >500      │    │                                          │
│   chars)             │    │  Sent to LLaVA via Ollama API:           │
└──────────┬───────────┘    │  POST /api/generate                      │
           │                │  {"model": "llava",                      │
           │                │   "images": ["base64..."],               │
           │                │   "prompt": "Describe this diagram..."}  │
           │                └──────────────────┬───────────────────────┘
           │                                   │
           │                                   ▼
           │                ┌──────────────────────────────────────────┐
           │                │  LLaVA output (cached in .vision_cache): │
           │                │  "This is an architecture diagram of a   │
           │                │   web application system. It shows..."   │
           │                └──────────────────┬───────────────────────┘
           │                                   │
           └───────────────┬───────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  2. EMBEDDING (rag/embeddings.go)                                       │
│                                                                         │
│  For each chunk, call Ollama:                                           │
│  POST http://localhost:11434/api/embeddings                             │
│  {"model": "nomic-embed-text", "prompt": "chunk text..."}               │
│                                                                         │
│  Returns 768-dimensional float32 vector per chunk                       │
│  e.g., [-0.156, 0.712, -3.567, 0.843, ...]                              │
│                                                                         │
│  Text with similar meaning → vectors close together in 768D space       │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  3. STORAGE (rag/store.go → Qdrant)                                     │
│                                                                         │
│  PUT http://localhost:6333/collections/confluence_wiki/points           │
│  {                                                                      │
│    "points": [                                                          │
│      {                                                                  │
│        "id": "uuid-v5-hash",                                            │
│        "vector": [0.1, 0.2, ...],  // 768 floats                        │
│        "payload": {                                                     │
│          "content": "The Acme Platform...",                             │
│          "source_type": "text",      // or "image"                      │
│          "page_title": "Architecture",                                  │
│          "image_path": "/path/to/img.png"  // if image                  │
│        }                                                                │
│      }                                                                  │
│    ]                                                                    │
│  }                                                                      │
│                                                                         │
│  Qdrant stores vectors in HNSW index (Hierarchical Navigable            │
│  Small World graph) for fast approximate nearest neighbor search        │
└─────────────────────────────────────────────────────────────────────────┘
```

### Query Pipeline

```
┌─────────────────────────────────────────────────────────────────────────┐
│  User: "search wiki for architecture diagram"                           │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  1. LLM decides to use wiki tool (tools/wiki.go)                        │
│     {"name": "wiki", "parameters": {"action": "search",                 │
│                                     "query": "architecture diagram"}}   │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  2. EMBED QUERY                                                         │
│     Same nomic-embed-text model → 768D vector for query                 │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  3. VECTOR SIMILARITY SEARCH                                            │
│     POST http://localhost:6333/collections/confluence_wiki/points/search│
│     {"vector": [query embedding], "limit": 5}                           │
│                                                                         │
│     Qdrant computes cosine similarity between query vector and all      │
│     stored vectors, returns top matches                                 │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  4. RESULTS                                                             │
│     Score 0.689: [IMAGE] "This is an architecture diagram..."           │
│     Score 0.571: [TEXT] "three-tier microservices architecture..."      │
│     Score 0.550: [TEXT] "modern, scalable e-commerce solution..."       │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  5. LLM synthesizes final answer from retrieved context                 │
└─────────────────────────────────────────────────────────────────────────┘
```

### Why It Works: Semantic Similarity

The key insight is **semantic similarity via embeddings**:

1. "architecture diagram" and "This is an architecture diagram of a web application" have similar *meaning*
2. `nomic-embed-text` encodes this meaning into 768-dimensional vectors
3. Vectors for semantically similar text are geometrically close (high cosine similarity)
4. This is why the image description (generated by LLaVA) matches diagram queries - even though the user never typed those exact words

### API Endpoints Used

| Service | Endpoint | Purpose |
|---------|----------|---------|
| Ollama | `POST /api/embeddings` | Generate text embeddings |
| Ollama | `POST /api/generate` | LLaVA image description |
| Qdrant | `PUT /collections/{name}` | Create collection |
| Qdrant | `PUT /collections/{name}/points` | Store vectors |
| Qdrant | `POST /collections/{name}/points/search` | Similarity search |

### Test Results

Tested with sample wiki (3 HTML pages + 1 diagram):

| Query | Top Result Type | Score |
|-------|----------------|-------|
| "system architecture" | IMAGE (LLaVA desc) | 0.627 |
| "deployment kubernetes" | TEXT | 0.714 |
| "diagram" | IMAGE (LLaVA desc) | 0.689 |
