package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Document represents a document in the vector store
type Document struct {
	ID         string            `json:"id"`
	Content    string            `json:"content"`
	Vector     []float32         `json:"vector,omitempty"`
	Metadata   map[string]string `json:"metadata"`
	Score      float32           `json:"score,omitempty"`
	SourceType string            `json:"source_type"` // "text" or "image"
	ImagePath  string            `json:"image_path,omitempty"`
}

// VectorStore wraps Qdrant for storing and querying embeddings
type VectorStore struct {
	baseURL        string
	collectionName string
	client         *http.Client
}

// NewVectorStore creates a new Qdrant vector store client
func NewVectorStore(baseURL, collectionName string) *VectorStore {
	return &VectorStore{
		baseURL:        baseURL,
		collectionName: collectionName,
		client:         &http.Client{},
	}
}

// EnsureCollection creates the collection if it doesn't exist
func (s *VectorStore) EnsureCollection(ctx context.Context, vectorSize int) error {
	// Check if collection exists
	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.collectionName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil // Collection exists
	}

	// Create collection
	createReq := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	body, _ := json.Marshal(createReq)

	req, err = http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: %s", string(respBody))
	}

	return nil
}

// DeleteCollection deletes the collection (for re-indexing)
func (s *VectorStore) DeleteCollection(ctx context.Context) error {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.collectionName)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	defer resp.Body.Close()

	// 404 is fine - collection didn't exist
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete collection: %s", string(respBody))
	}

	return nil
}

// Upsert adds or updates documents in the store
func (s *VectorStore) Upsert(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	points := make([]map[string]any, len(docs))
	for i, doc := range docs {
		payload := map[string]any{
			"content":     doc.Content,
			"source_type": doc.SourceType,
		}
		for k, v := range doc.Metadata {
			payload[k] = v
		}
		if doc.ImagePath != "" {
			payload["image_path"] = doc.ImagePath
		}

		points[i] = map[string]any{
			"id":      doc.ID,
			"vector":  doc.Vector,
			"payload": payload,
		}
	}

	upsertReq := map[string]any{
		"points": points,
	}
	body, _ := json.Marshal(upsertReq)

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", s.baseURL, s.collectionName)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upsert points: %s", string(respBody))
	}

	return nil
}

// Search finds similar documents
func (s *VectorStore) Search(ctx context.Context, queryVector []float32, limit int) ([]Document, error) {
	searchReq := map[string]any{
		"vector":       queryVector,
		"limit":        limit,
		"with_payload": true,
	}
	body, _ := json.Marshal(searchReq)

	url := fmt.Sprintf("%s/collections/%s/points/search", s.baseURL, s.collectionName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: %s", string(respBody))
	}

	var result struct {
		Result []struct {
			ID      any            `json:"id"`
			Score   float32        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	docs := make([]Document, len(result.Result))
	for i, r := range result.Result {
		doc := Document{
			Score: r.Score,
		}

		// Handle ID which can be string or int
		switch id := r.ID.(type) {
		case string:
			doc.ID = id
		case float64:
			doc.ID = fmt.Sprintf("%d", int(id))
		}

		if content, ok := r.Payload["content"].(string); ok {
			doc.Content = content
		}
		if sourceType, ok := r.Payload["source_type"].(string); ok {
			doc.SourceType = sourceType
		}
		if imagePath, ok := r.Payload["image_path"].(string); ok {
			doc.ImagePath = imagePath
		}

		doc.Metadata = make(map[string]string)
		for k, v := range r.Payload {
			if k != "content" && k != "source_type" && k != "image_path" {
				if str, ok := v.(string); ok {
					doc.Metadata[k] = str
				}
			}
		}

		docs[i] = doc
	}

	return docs, nil
}

// Count returns the number of documents in the collection
func (s *VectorStore) Count(ctx context.Context) (int, error) {
	url := fmt.Sprintf("%s/collections/%s", s.baseURL, s.collectionName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get collection info: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			PointsCount int `json:"points_count"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Result.PointsCount, nil
}
