package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

// GeminiClient wraps the Google AI LLM with the ChatClient interface.
type GeminiClient struct {
	llm   *googleai.GoogleAI
	model string
}

// Ensure GeminiClient implements both interfaces.
var _ ChatClient = (*GeminiClient)(nil)
var _ StreamingChatClient = (*GeminiClient)(nil)

// NewGeminiClient creates a new Google AI (Gemini) client.
// API key is read from GOOGLE_API_KEY env var automatically.
func NewGeminiClient(model string) (*GeminiClient, error) {
	ctx := context.Background()
	llm, err := googleai.New(ctx, googleai.WithDefaultModel(model))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}
	return &GeminiClient{llm: llm, model: model}, nil
}

// Close closes the underlying Google AI client.
func (c *GeminiClient) Close() error {
	return c.llm.Close()
}

// Chat sends messages to Gemini and returns the response.
func (c *GeminiClient) Chat(ctx context.Context, messages []Message) (*Response, error) {
	llmMessages := convertMessages(messages)

	resp, err := c.llm.GenerateContent(ctx, llmMessages)
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from gemini")
	}

	content := resp.Choices[0].Content
	return parseResponse(content), nil
}

// ChatStream sends messages to Gemini and streams text responses in real-time.
func (c *GeminiClient) ChatStream(ctx context.Context, messages []Message, streamFunc func(chunk string)) (*Response, error) {
	llmMessages := convertMessages(messages)

	var buf strings.Builder
	streaming := false
	jsonMode := false

	resp, err := c.llm.GenerateContent(ctx, llmMessages,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			buf.Write(chunk)

			if !streaming && !jsonMode {
				trimmed := strings.TrimSpace(buf.String())
				if len(trimmed) > 0 {
					if trimmed[0] == '{' {
						jsonMode = true
					} else {
						streaming = true
						streamFunc(buf.String())
					}
				}
			} else if streaming {
				streamFunc(string(chunk))
			}

			return nil
		}))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from gemini")
	}

	content := resp.Choices[0].Content
	return parseResponse(content), nil
}
