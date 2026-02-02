package rag

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChunkText(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		maxChunkSize  int
		expectedCount int
	}{
		{
			name:          "short text",
			content:       "This is a short text.",
			maxChunkSize:  100,
			expectedCount: 1,
		},
		{
			name:          "long text splits",
			content:       "First sentence. Second sentence. Third sentence. Fourth sentence. Fifth sentence.",
			maxChunkSize:  40,
			expectedCount: 3,
		},
		{
			name:          "empty text",
			content:       "",
			maxChunkSize:  100,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := ChunkText(tt.content, tt.maxChunkSize)
			if len(chunks) != tt.expectedCount {
				t.Errorf("ChunkText() = %d chunks, want %d", len(chunks), tt.expectedCount)
			}
		})
	}
}

func TestConfluenceLoader(t *testing.T) {
	// Create a temporary directory with test HTML files
	tmpDir, err := os.MkdirTemp("", "confluence-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test HTML file
	testHTML := `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Main Heading</h1>
<p>This is a test paragraph with some content.</p>
<h2>Sub Heading</h2>
<ul>
<li>List item 1</li>
<li>List item 2</li>
</ul>
<pre>Some code here</pre>
</body>
</html>`

	htmlPath := filepath.Join(tmpDir, "test.html")
	if err := os.WriteFile(htmlPath, []byte(testHTML), 0644); err != nil {
		t.Fatalf("Failed to write test HTML: %v", err)
	}

	loader := NewConfluenceLoader(tmpDir)

	// Test LoadPage
	page, err := loader.LoadPage(htmlPath)
	if err != nil {
		t.Fatalf("LoadPage() error = %v", err)
	}

	if page.Title != "Test Page" {
		t.Errorf("Title = %q, want %q", page.Title, "Test Page")
	}

	if len(page.Chunks) < 4 {
		t.Errorf("Expected at least 4 chunks, got %d", len(page.Chunks))
	}

	// Verify chunk types
	foundHeading := false
	foundParagraph := false
	foundList := false
	foundCode := false

	for _, chunk := range page.Chunks {
		switch chunk.Type {
		case "heading":
			foundHeading = true
		case "paragraph":
			foundParagraph = true
		case "list":
			foundList = true
		case "code":
			foundCode = true
		}
	}

	if !foundHeading {
		t.Error("Expected to find heading chunk")
	}
	if !foundParagraph {
		t.Error("Expected to find paragraph chunk")
	}
	if !foundList {
		t.Error("Expected to find list chunk")
	}
	if !foundCode {
		t.Error("Expected to find code chunk")
	}

	// Test LoadAll
	pages, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(pages) != 1 {
		t.Errorf("LoadAll() = %d pages, want 1", len(pages))
	}
}

func TestImageExtraction(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "confluence-img-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a dummy image file
	imgPath := filepath.Join(tmpDir, "diagram.png")
	if err := os.WriteFile(imgPath, []byte("fake png"), 0644); err != nil {
		t.Fatalf("Failed to write dummy image: %v", err)
	}

	// Create HTML referencing the image
	testHTML := `<!DOCTYPE html>
<html>
<head><title>Page With Image</title></head>
<body>
<h1>Architecture</h1>
<p>Here is the diagram:</p>
<img src="diagram.png" alt="Architecture Diagram">
</body>
</html>`

	htmlPath := filepath.Join(tmpDir, "page.html")
	if err := os.WriteFile(htmlPath, []byte(testHTML), 0644); err != nil {
		t.Fatalf("Failed to write test HTML: %v", err)
	}

	loader := NewConfluenceLoader(tmpDir)
	page, err := loader.LoadPage(htmlPath)
	if err != nil {
		t.Fatalf("LoadPage() error = %v", err)
	}

	if len(page.Images) != 1 {
		t.Fatalf("Expected 1 image, got %d", len(page.Images))
	}

	img := page.Images[0]
	if img.Src != "diagram.png" {
		t.Errorf("Image src = %q, want %q", img.Src, "diagram.png")
	}
	if img.Alt != "Architecture Diagram" {
		t.Errorf("Image alt = %q, want %q", img.Alt, "Architecture Diagram")
	}
}
