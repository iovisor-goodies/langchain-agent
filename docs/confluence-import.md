# How to Import a Confluence Space

## Step 1: Export from Confluence

1. Go to your Confluence space
2. Click **Space Settings** (gear icon) → **Content Tools** → **Export**
3. Select **HTML** format
4. Choose:
   - "Normal Export" (includes all pages)
   - Check "Include comments" if desired
5. Click **Export**
6. Download the ZIP file when ready

## Step 2: Extract the Export

```bash
# Create a directory for your wiki
mkdir -p ~/wiki/my-space

# Extract the ZIP
unzip Confluence-export-*.zip -d ~/wiki/my-space/
```

The structure will look like:
```
~/wiki/my-space/
├── index.html
├── Page-Name_12345.html
├── Another-Page_67890.html
├── attachments/
│   └── 12345/
│       ├── diagram.png
│       └── screenshot.jpg
└── images/
    └── icons/...
```

## Step 3: Start Prerequisites

```bash
# Start Qdrant (using podman)
podman run -d --name qdrant --network host docker.io/qdrant/qdrant

# Ensure Ollama models are available
ollama pull nomic-embed-text
ollama pull llava
```

## Step 4: Index and Run

```bash
# Option A: Index only (useful for large wikis)
./langchain-agent --wiki ~/wiki/my-space/ --index-only

# Option B: Index and start agent
./langchain-agent --wiki ~/wiki/my-space/
```

## Step 5: Query

```
> search wiki for deployment process
> what does the architecture diagram show
> find documentation about authentication
```

## Notes

- **First run takes time**: LLaVA processes each image (cached for subsequent runs)
- **Re-indexing**: Each run with `--wiki` re-indexes from scratch
- **Image cache**: Stored in `~/wiki/my-space/.vision_cache.json`
- **Large spaces**: May take several minutes to index (depends on image count)

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "Qdrant not running" | `podman start qdrant` |
| "nomic-embed-text not found" | `ollama pull nomic-embed-text` |
| "llava not found" | `ollama pull llava` |
| Slow indexing | Normal for first run; images are cached |
