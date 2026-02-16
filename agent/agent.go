package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/rathore/langchain-agent/llm"
	"github.com/rathore/langchain-agent/tools"
)

// Agent runs the autonomous agent loop
type Agent struct {
	client       llm.ChatClient
	tools        map[string]tools.Tool
	toolDefs     []llm.ToolDef
	maxIter      int
	history      []llm.Message
	systemPrompt string
}

// Config holds agent configuration
type Config struct {
	Model   string
	MaxIter int
	Tools   []tools.Tool
	Client  llm.ChatClient // Optional: inject custom client (for testing)
}

// New creates a new agent
func New(cfg Config) (*Agent, error) {
	var client llm.ChatClient
	var err error

	if cfg.Client != nil {
		client = cfg.Client
	} else {
		client, err = llm.NewClient(cfg.Model)
		if err != nil {
			return nil, err
		}
	}

	a := &Agent{
		client:  client,
		tools:   make(map[string]tools.Tool),
		maxIter: cfg.MaxIter,
	}

	if a.maxIter == 0 {
		a.maxIter = 10
	}

	// Register tools
	for _, t := range cfg.Tools {
		a.tools[t.Name()] = t
		a.toolDefs = append(a.toolDefs, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}

	a.systemPrompt = llm.BuildSystemPrompt(a.toolDefs)
	return a, nil
}

// Run executes the agent with the given user input
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	// Build messages: system + history + new user input
	messages := []llm.Message{
		{Role: "system", Content: a.systemPrompt},
	}
	messages = append(messages, a.history...)
	messages = append(messages, llm.Message{Role: "user", Content: userInput})

	// Add user message to history
	a.history = append(a.history, llm.Message{Role: "user", Content: userInput})

	// Agent loop
	for i := 0; i < a.maxIter; i++ {
		var resp *llm.Response
		var err error

		if sc, ok := a.client.(llm.StreamingChatClient); ok {
			fmt.Print("\n[Agent] ")
			resp, err = sc.ChatStream(ctx, messages, func(chunk string) {
				fmt.Print(chunk)
			})
			fmt.Println()
		} else {
			resp, err = a.client.Chat(ctx, messages)
			if err == nil {
				fmt.Printf("\n[Agent] %s\n", resp.Content)
			}
		}
		if err != nil {
			return "", fmt.Errorf("agent iteration %d: %w", i, err)
		}

		// Check for tool calls
		if len(resp.ToolCalls) > 0 {
			tc := resp.ToolCalls[0] // Handle one tool call at a time
			fmt.Printf("[Tool Call] %s: %v\n", tc.Name, tc.Params)

			result, err := a.executeTool(ctx, tc)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}
			fmt.Printf("[Tool Result] %s\n", truncate(result, 500))

			// Add assistant's tool call and tool result to messages
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			messages = append(messages, llm.Message{
				Role:    "tool",
				Content: fmt.Sprintf("Tool '%s' returned:\n%s", tc.Name, result),
			})
			continue
		}

		// No tool call - this is the final answer
		if resp.IsFinish || !strings.Contains(resp.Content, "{") {
			// Add final response to history
			a.history = append(a.history, llm.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			return resp.Content, nil
		}

		// Add response to messages and continue
		messages = append(messages, llm.Message{
			Role:    "assistant",
			Content: resp.Content,
		})
	}

	return "", fmt.Errorf("max iterations (%d) reached", a.maxIter)
}

// executeTool runs the specified tool
func (a *Agent) executeTool(ctx context.Context, tc llm.ToolCallParse) (string, error) {
	tool, ok := a.tools[tc.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", tc.Name)
	}
	return tool.Call(ctx, tc.Params)
}

// ClearHistory clears the conversation history
func (a *Agent) ClearHistory() {
	a.history = nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
