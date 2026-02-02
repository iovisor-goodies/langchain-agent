package rag

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

// IndexerConfig holds configuration for the indexer
type IndexerConfig struct {
	WikiPath       string // Path to Confluence HTML export
	QdrantURL      string // Qdrant server URL
	CollectionName string // Qdrant collection name
	EmbedModel     string // Embedding model (e.g., nomic-embed-text)
	VisionModel    string // Vision model (e.g., llava)
	VectorSize     int    // Vector dimensions
	ChunkSize      int    // Max chunk size for text
}

// DefaultConfig returns default indexer configuration
func DefaultConfig() IndexerConfig {
	return IndexerConfig{
		QdrantURL:      "http://localhost:6333",
		CollectionName: "confluence_wiki",
		EmbedModel:     "nomic-embed-text",
		VisionModel:    "llava",
		VectorSize:     768, // nomic-embed-text dimension
		ChunkSize:      500,
	}
}

// Indexer handles indexing Confluence content into the vector store
type Indexer struct {
	config     IndexerConfig
	embeddings *EmbeddingClient
	vision     *VisionClient
	store      *VectorStore
	loader     *ConfluenceLoader
}

// NewIndexer creates a new indexer
func NewIndexer(config IndexerConfig) (*Indexer, error) {
	embeddings, err := NewEmbeddingClient(config.EmbedModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding client: %w", err)
	}

	cacheFile := filepath.Join(config.WikiPath, ".vision_cache.json")
	vision, err := NewVisionClient(config.VisionModel, cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create vision client: %w", err)
	}

	store := NewVectorStore(config.QdrantURL, config.CollectionName)
	loader := NewConfluenceLoader(config.WikiPath)

	return &Indexer{
		config:     config,
		embeddings: embeddings,
		vision:     vision,
		store:      store,
		loader:     loader,
	}, nil
}

// Index performs full re-indexing of the wiki content
func (idx *Indexer) Index(ctx context.Context) error {
	fmt.Println("Loading Confluence HTML export...")

	// Load all pages
	pages, err := idx.loader.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load pages: %w", err)
	}

	fmt.Printf("Found %d pages to index\n", len(pages))

	// Delete and recreate collection
	fmt.Println("Resetting vector store...")
	if err := idx.store.DeleteCollection(ctx); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	if err := idx.store.EnsureCollection(ctx, idx.config.VectorSize); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	// Process each page
	var allDocs []Document
	docCount := 0

	for i, page := range pages {
		fmt.Printf("Processing page %d/%d: %s\n", i+1, len(pages), page.Title)

		// Process text chunks
		for _, chunk := range page.Chunks {
			// Split into smaller chunks if needed
			textChunks := ChunkText(chunk.Content, idx.config.ChunkSize)
			for _, text := range textChunks {
				if len(text) < 20 {
					continue // Skip very short chunks
				}

				docID := generateDocID(page.FilePath, text)
				allDocs = append(allDocs, Document{
					ID:         docID,
					Content:    text,
					SourceType: "text",
					Metadata: map[string]string{
						"page_title": page.Title,
						"file_path":  page.FilePath,
						"chunk_type": chunk.Type,
					},
				})
				docCount++
			}
		}

		// Process images with vision model
		for _, img := range page.Images {
			fmt.Printf("  Describing image: %s\n", filepath.Base(img.FullPath))

			description, err := idx.vision.DescribeImage(ctx, img.FullPath)
			if err != nil {
				fmt.Printf("  Warning: failed to describe image %s: %v\n", img.FullPath, err)
				continue
			}

			docID := generateDocID(img.FullPath, "image")
			allDocs = append(allDocs, Document{
				ID:         docID,
				Content:    description,
				SourceType: "image",
				ImagePath:  img.FullPath,
				Metadata: map[string]string{
					"page_title": page.Title,
					"file_path":  page.FilePath,
					"image_alt":  img.Alt,
				},
			})
			docCount++
		}
	}

	fmt.Printf("Generated %d document chunks, generating embeddings...\n", docCount)

	// Generate embeddings in batches
	batchSize := 10
	for i := 0; i < len(allDocs); i += batchSize {
		end := i + batchSize
		if end > len(allDocs) {
			end = len(allDocs)
		}

		batch := allDocs[i:end]
		texts := make([]string, len(batch))
		for j, doc := range batch {
			texts[j] = doc.Content
		}

		vectors, err := idx.embeddings.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("failed to embed batch: %w", err)
		}

		for j := range batch {
			allDocs[i+j].Vector = vectors[j]
		}

		fmt.Printf("Embedded %d/%d documents\n", end, len(allDocs))
	}

	// Upsert all documents
	fmt.Println("Storing documents in vector store...")
	if err := idx.store.Upsert(ctx, allDocs); err != nil {
		return fmt.Errorf("failed to upsert documents: %w", err)
	}

	fmt.Printf("Indexing complete! %d documents indexed.\n", len(allDocs))
	return nil
}

// GetStore returns the vector store for querying
func (idx *Indexer) GetStore() *VectorStore {
	return idx.store
}

// GetEmbeddings returns the embedding client for querying
func (idx *Indexer) GetEmbeddings() *EmbeddingClient {
	return idx.embeddings
}

// generateDocID creates a unique ID for a document (UUID v5)
func generateDocID(path, content string) string {
	// Use a fixed namespace UUID for wiki documents
	namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // URL namespace
	return uuid.NewSHA1(namespace, []byte(path+content)).String()
}
