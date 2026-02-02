package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rathore/langchain-agent/rag"
)

// WikiTool searches the indexed Confluence wiki content
type WikiTool struct {
	embeddings *rag.EmbeddingClient
	store      *rag.VectorStore
}

// NewWikiTool creates a new wiki search tool
func NewWikiTool(embeddings *rag.EmbeddingClient, store *rag.VectorStore) *WikiTool {
	return &WikiTool{
		embeddings: embeddings,
		store:      store,
	}
}

func (w *WikiTool) Name() string {
	return "wiki"
}

func (w *WikiTool) Description() string {
	return "Search the Confluence wiki for relevant documentation, diagrams, and architecture information. Use when user asks about internal documentation, architecture diagrams, deployment, or project-specific knowledge."
}

func (w *WikiTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action to perform: 'search' to find relevant content, 'count' to get total indexed documents",
				"enum":        []string{"search", "count"},
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (required for 'search' action)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 5)",
			},
		},
		"required": []string{"action"},
	}
}

func (w *WikiTool) Call(ctx context.Context, params map[string]any) (string, error) {
	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action parameter required")
	}

	switch action {
	case "search":
		return w.search(ctx, params)
	case "count":
		return w.count(ctx)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (w *WikiTool) search(ctx context.Context, params map[string]any) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query parameter required for search action")
	}

	limit := 5
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	// Generate embedding for query
	queryVector, err := w.embeddings.Embed(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to embed query: %w", err)
	}

	// Search vector store
	results, err := w.store.Search(ctx, queryVector, limit)
	if err != nil {
		return "", fmt.Errorf("failed to search: %w", err)
	}

	if len(results) == 0 {
		return "No relevant results found in the wiki.", nil
	}

	// Format results
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d relevant results:\n\n", len(results)))

	for i, doc := range results {
		sourceType := "TEXT"
		if doc.SourceType == "image" {
			sourceType = "DIAGRAM"
		}

		pageTitle := doc.Metadata["page_title"]
		if pageTitle == "" {
			pageTitle = "Unknown Page"
		}

		sb.WriteString(fmt.Sprintf("%d. [%s] %s (score: %.2f)\n", i+1, sourceType, pageTitle, doc.Score))

		if doc.SourceType == "image" && doc.ImagePath != "" {
			sb.WriteString(fmt.Sprintf("   Image: %s\n", doc.ImagePath))
		}

		// Truncate content for display
		content := doc.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("   %s\n\n", content))
	}

	return sb.String(), nil
}

func (w *WikiTool) count(ctx context.Context) (string, error) {
	count, err := w.store.Count(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get count: %w", err)
	}
	return fmt.Sprintf("Wiki index contains %d documents.", count), nil
}
