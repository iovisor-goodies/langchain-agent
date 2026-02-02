package rag

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// VisionClient generates descriptions for images using LLaVA
type VisionClient struct {
	llm       *ollama.LLM
	model     string
	cacheFile string
	cache     map[string]string
}

// NewVisionClient creates a new vision client using Ollama LLaVA
func NewVisionClient(model string, cacheFile string) (*VisionClient, error) {
	llm, err := ollama.New(ollama.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama client: %w", err)
	}

	client := &VisionClient{
		llm:       llm,
		model:     model,
		cacheFile: cacheFile,
		cache:     make(map[string]string),
	}

	// Load cache if exists
	if cacheFile != "" {
		client.loadCache()
	}

	return client, nil
}

// DescribeImage generates a text description for an image
func (c *VisionClient) DescribeImage(ctx context.Context, imagePath string) (string, error) {
	// Check cache first
	absPath, _ := filepath.Abs(imagePath)
	if desc, ok := c.cache[absPath]; ok {
		return desc, nil
	}

	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image: %w", err)
	}

	// Encode as base64
	b64Image := base64.StdEncoding.EncodeToString(imageData)

	// Determine MIME type
	ext := strings.ToLower(filepath.Ext(imagePath))
	mimeType := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	}

	// Create prompt for image description
	prompt := `Describe this diagram or image in detail. Focus on:
1. What type of diagram/image it is (architecture diagram, flowchart, screenshot, etc.)
2. The main components or elements shown
3. The relationships or connections between components
4. Any text or labels visible
5. The overall purpose or what it's trying to communicate

Provide a clear, comprehensive description that would allow someone to understand the image without seeing it.`

	// Create message with image
	content := []llms.ContentPart{
		llms.BinaryPart(mimeType, imageData),
		llms.TextPart(prompt),
	}

	// Send to LLM
	resp, err := c.llm.GenerateContent(ctx, []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: content,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from vision model")
	}

	description := resp.Choices[0].Content

	// Cache the result
	c.cache[absPath] = description
	c.saveCache()

	// Also return base64 for reference (not used in embedding, just for debugging)
	_ = b64Image

	return description, nil
}

// loadCache loads the description cache from file
func (c *VisionClient) loadCache() {
	if c.cacheFile == "" {
		return
	}

	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		return // File doesn't exist yet
	}

	json.Unmarshal(data, &c.cache)
}

// saveCache saves the description cache to file
func (c *VisionClient) saveCache() {
	if c.cacheFile == "" {
		return
	}

	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(c.cacheFile, data, 0644)
}

// ClearCache clears the description cache
func (c *VisionClient) ClearCache() {
	c.cache = make(map[string]string)
	if c.cacheFile != "" {
		os.Remove(c.cacheFile)
	}
}
