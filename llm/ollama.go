package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// ChatClient interface for LLM interactions (allows mocking in tests)
type ChatClient interface {
	Chat(ctx context.Context, messages []Message) (*Response, error)
}

// Client wraps the Ollama LLM with tool calling support
type Client struct {
	llm   *ollama.LLM
	model string
}

// StreamingChatClient extends ChatClient with streaming support
type StreamingChatClient interface {
	ChatClient
	ChatStream(ctx context.Context, messages []Message, streamFunc func(chunk string)) (*Response, error)
}

// Ensure Client implements both interfaces
var _ ChatClient = (*Client)(nil)
var _ StreamingChatClient = (*Client)(nil)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // system, user, assistant, tool
	Content string `json:"content"`
}

// Response from the LLM
type Response struct {
	Content   string          // Text response
	ToolCalls []ToolCallParse // Parsed tool calls, if any
	IsFinish  bool            // True if this is a final answer
}

// ToolCallParse represents a parsed tool call
type ToolCallParse struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"parameters"`
}

// NewClient creates a new Ollama client
func NewClient(model string) (*Client, error) {
	llm, err := ollama.New(ollama.WithModel(model))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama client: %w", err)
	}
	return &Client{llm: llm, model: model}, nil
}

// Chat sends messages to the LLM and returns the response
func (c *Client) Chat(ctx context.Context, messages []Message) (*Response, error) {
	// Convert to langchaingo message format
	var llmMessages []llms.MessageContent
	for _, msg := range messages {
		var role llms.ChatMessageType
		switch msg.Role {
		case "system":
			role = llms.ChatMessageTypeSystem
		case "user":
			role = llms.ChatMessageTypeHuman
		case "assistant":
			role = llms.ChatMessageTypeAI
		case "tool":
			role = llms.ChatMessageTypeHuman // Tool results go as human messages
		default:
			role = llms.ChatMessageTypeHuman
		}
		llmMessages = append(llmMessages, llms.MessageContent{
			Role:  role,
			Parts: []llms.ContentPart{llms.TextContent{Text: msg.Content}},
		})
	}

	// Call the LLM
	resp, err := c.llm.GenerateContent(ctx, llmMessages)
	if err != nil {
		return nil, fmt.Errorf("llm generate failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from llm")
	}

	content := resp.Choices[0].Content
	return c.parseResponse(content), nil
}

// ChatStream sends messages to the LLM and streams text responses in real-time.
// Tool call responses (starting with '{') are buffered silently.
func (c *Client) ChatStream(ctx context.Context, messages []Message, streamFunc func(chunk string)) (*Response, error) {
	// Convert to langchaingo message format
	var llmMessages []llms.MessageContent
	for _, msg := range messages {
		var role llms.ChatMessageType
		switch msg.Role {
		case "system":
			role = llms.ChatMessageTypeSystem
		case "user":
			role = llms.ChatMessageTypeHuman
		case "assistant":
			role = llms.ChatMessageTypeAI
		case "tool":
			role = llms.ChatMessageTypeHuman
		default:
			role = llms.ChatMessageTypeHuman
		}
		llmMessages = append(llmMessages, llms.MessageContent{
			Role:  role,
			Parts: []llms.ContentPart{llms.TextContent{Text: msg.Content}},
		})
	}

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
		return nil, fmt.Errorf("llm generate failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from llm")
	}

	content := resp.Choices[0].Content
	return c.parseResponse(content), nil
}

// parseResponse extracts tool calls or final answer from LLM response
func (c *Client) parseResponse(content string) *Response {
	resp := &Response{Content: content}

	// Try to find JSON tool call in the response
	// Look for patterns like {"name": "...", "parameters": {...}}
	// or {"tool": "...", "parameters": {...}}

	content = strings.TrimSpace(content)

	// Check if response contains a tool call JSON
	if idx := strings.Index(content, "{"); idx != -1 {
		jsonPart := content[idx:]
		// Find the matching closing brace
		if endIdx := findMatchingBrace(jsonPart); endIdx != -1 {
			jsonStr := jsonPart[:endIdx+1]
			var toolCall struct {
				Name       string         `json:"name"`
				Tool       string         `json:"tool"`
				Parameters map[string]any `json:"parameters"`
				Params     map[string]any `json:"params"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &toolCall); err == nil {
				name := toolCall.Name
				if name == "" {
					name = toolCall.Tool
				}
				params := toolCall.Parameters
				if params == nil {
					params = toolCall.Params
				}
				if name != "" {
					resp.ToolCalls = append(resp.ToolCalls, ToolCallParse{
						Name:   name,
						Params: params,
					})
					// Truncate content to just the tool call JSON,
					// discarding any hallucinated output after it
					resp.Content = strings.TrimSpace(content[:idx+endIdx+1])
					return resp
				}
			}
		}
	}

	// Check for explicit final answer markers
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "final answer:") ||
		strings.Contains(lowerContent, "answer:") ||
		!strings.Contains(content, "{") {
		resp.IsFinish = true
	}

	return resp
}

// findMatchingBrace finds the index of the matching closing brace
func findMatchingBrace(s string) int {
	if len(s) == 0 || s[0] != '{' {
		return -1
	}
	depth := 0
	inString := false
	escape := false
	for i, ch := range s {
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// BuildSystemPrompt creates the system prompt with tool definitions
func BuildSystemPrompt(tools []ToolDef) string {
	var sb strings.Builder
	sb.WriteString(`You are an autonomous agent that uses tools to complete tasks.

RESPONSE FORMAT:
- To call a tool: respond with ONLY a JSON object: {"name": "tool_name", "parameters": {...}}
- To give final answer: respond with plain text (no JSON)

WHEN TO USE TOOLS:
- "ssh to", "connect to", user@host, remote server, IP address → use "ssh" tool
- Local machine operations, run commands, check files → use "shell" tool
- "mcp", file operations on MCP server, MCP tool calls → use "mcp" tool
- "wiki", "confluence", "documentation", "diagram", "architecture" → use "wiki" tool

WHEN NOT TO USE TOOLS (answer directly from your knowledge):
- General knowledge questions (math, science, history, concepts)
- Explanations, definitions, "what is", "how does X work"
- Opinions, comparisons, "which is better", "is X easier than Y"
- Programming questions, code explanations, best practices
- Anything you can answer from knowledge without running commands

CONTEXT RULES:
- Maintain context from previous messages until user says "clear"
- If user gives a correction or follow-up, apply it to the SAME host/target from previous messages
- Example: if you just used ssh to host X and user says "try grep vmx instead", use ssh to host X again

CRITICAL RULES:
- NEVER fabricate system/command output - if you run a tool, report real results
- If a command fails or returns empty, report exactly what happened
- For knowledge questions, use your own knowledge - no tools needed
- If unsure about facts, say so

Available tools:
`)

	for _, tool := range tools {
		toolJSON, _ := json.MarshalIndent(tool, "", "  ")
		sb.WriteString("\n")
		sb.Write(toolJSON)
		sb.WriteString("\n")
	}

	sb.WriteString(`
Process:
1. Can I answer this from my knowledge? → answer directly (no tools)
2. Do I need to run a command or check a system? → use appropriate tool
3. If tool result is useful, provide final answer
4. If tool result is empty/error, report honestly or try alternative
`)
	return sb.String()
}

// ToolDef defines a tool for the system prompt
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}
