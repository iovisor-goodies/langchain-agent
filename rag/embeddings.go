package rag

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/ollama"
)

// EmbeddingClient generates text embeddings using Ollama
type EmbeddingClient struct {
	embedder embeddings.Embedder
	model    string
}

// NewEmbeddingClient creates a new embedding client using Ollama
func NewEmbeddingClient(model string) (*EmbeddingClient, error) {
	llm, err := ollama.New(ollama.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama client: %w", err)
	}

	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	return &EmbeddingClient{
		embedder: embedder,
		model:    model,
	}, nil
}

// Embed generates an embedding for a single text
func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := c.embedder.EmbedDocuments(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("failed to embed text: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return vectors[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (c *EmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors, err := c.embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("failed to embed texts: %w", err)
	}
	return vectors, nil
}
