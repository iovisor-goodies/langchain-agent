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
