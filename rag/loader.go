package rag

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// PageContent represents parsed content from a Confluence HTML page
type PageContent struct {
	Title    string
	FilePath string
	Chunks   []TextChunk
	Images   []ImageRef
}

// TextChunk represents a chunk of text from a page
type TextChunk struct {
	Content string
	Type    string // "heading", "paragraph", "list", "code"
}

// ImageRef represents a reference to an image in the page
type ImageRef struct {
	Src      string // Relative path to image
	Alt      string // Alt text
	FullPath string // Full path to image file
}

// ConfluenceLoader parses Confluence HTML exports
type ConfluenceLoader struct {
	basePath string
}

// NewConfluenceLoader creates a new loader for a Confluence export directory
func NewConfluenceLoader(basePath string) *ConfluenceLoader {
	return &ConfluenceLoader{basePath: basePath}
}

// LoadAll loads all HTML pages from the export
func (l *ConfluenceLoader) LoadAll() ([]PageContent, error) {
	var pages []PageContent

	err := filepath.Walk(l.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-HTML files
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".html") && !strings.HasSuffix(strings.ToLower(path), ".htm") {
			return nil
		}

		page, err := l.LoadPage(path)
		if err != nil {
			// Log error but continue with other pages
			fmt.Printf("Warning: failed to parse %s: %v\n", path, err)
			return nil
		}

		if len(page.Chunks) > 0 || len(page.Images) > 0 {
			pages = append(pages, *page)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return pages, nil
}

// LoadPage loads and parses a single HTML page
func (l *ConfluenceLoader) LoadPage(filePath string) (*PageContent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	doc, err := html.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	page := &PageContent{
		FilePath: filePath,
	}

	// Extract title and content
	l.extractContent(doc, page, filePath)

	return page, nil
}

// extractContent recursively extracts content from HTML nodes
func (l *ConfluenceLoader) extractContent(n *html.Node, page *PageContent, filePath string) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "title":
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				page.Title = strings.TrimSpace(n.FirstChild.Data)
			}

		case "h1", "h2", "h3", "h4", "h5", "h6":
			text := l.extractText(n)
			if text != "" {
				page.Chunks = append(page.Chunks, TextChunk{
					Content: text,
					Type:    "heading",
				})
			}

		case "p":
			text := l.extractText(n)
			if text != "" {
				page.Chunks = append(page.Chunks, TextChunk{
					Content: text,
					Type:    "paragraph",
				})
			}

		case "li":
			text := l.extractText(n)
			if text != "" {
				page.Chunks = append(page.Chunks, TextChunk{
					Content: "- " + text,
					Type:    "list",
				})
			}

		case "pre", "code":
			text := l.extractText(n)
			if text != "" {
				page.Chunks = append(page.Chunks, TextChunk{
					Content: text,
					Type:    "code",
				})
			}

		case "img":
			img := l.extractImage(n, filePath)
			if img != nil {
				page.Images = append(page.Images, *img)
			}
		}
	}

	// Recurse into children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		l.extractContent(c, page, filePath)
	}
}

// extractText extracts all text from a node and its children
func (l *ConfluenceLoader) extractText(n *html.Node) string {
	var text strings.Builder
	l.extractTextRecursive(n, &text)
	result := strings.TrimSpace(text.String())
	// Normalize whitespace
	spaceRe := regexp.MustCompile(`\s+`)
	result = spaceRe.ReplaceAllString(result, " ")
	return result
}

func (l *ConfluenceLoader) extractTextRecursive(n *html.Node, text *strings.Builder) {
	if n.Type == html.TextNode {
		text.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		l.extractTextRecursive(c, text)
	}
}

// extractImage extracts image information from an img tag
func (l *ConfluenceLoader) extractImage(n *html.Node, filePath string) *ImageRef {
	var src, alt string
	for _, attr := range n.Attr {
		switch attr.Key {
		case "src":
			src = attr.Val
		case "alt":
			alt = attr.Val
		}
	}

	if src == "" {
		return nil
	}

	// Skip data URIs and external URLs
	if strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return nil
	}

	// Check if it's an actual image file
	ext := strings.ToLower(filepath.Ext(src))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".svg" {
		return nil
	}

	// Resolve full path relative to the HTML file
	dir := filepath.Dir(filePath)
	fullPath := filepath.Join(dir, src)

	// Verify file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// Try relative to base path
		fullPath = filepath.Join(l.basePath, src)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil
		}
	}

	return &ImageRef{
		Src:      src,
		Alt:      alt,
		FullPath: fullPath,
	}
}

// ChunkText splits text into smaller chunks for embedding
func ChunkText(content string, maxChunkSize int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if len(content) <= maxChunkSize {
		return []string{content}
	}

	var chunks []string
	sentences := splitSentences(content)
	var currentChunk strings.Builder

	for _, sentence := range sentences {
		if currentChunk.Len()+len(sentence) > maxChunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
			currentChunk.Reset()
		}
		currentChunk.WriteString(sentence)
		currentChunk.WriteString(" ")
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

// splitSentences splits text into sentences
func splitSentences(text string) []string {
	// Simple sentence splitting on common delimiters
	re := regexp.MustCompile(`[.!?]+\s+`)
	parts := re.Split(text, -1)

	var sentences []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			sentences = append(sentences, part)
		}
	}
	return sentences
}
