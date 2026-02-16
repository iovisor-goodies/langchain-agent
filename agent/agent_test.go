package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rathore/langchain-agent/llm"
	"github.com/rathore/langchain-agent/tools"
)

// MockLLMClient simulates LLM responses for testing
type MockLLMClient struct {
	responses []*llm.Response
	callCount int
	messages  [][]llm.Message // Records all message sets sent
}

func (m *MockLLMClient) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	// Record the messages
	m.messages = append(m.messages, messages)

	if m.callCount >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses (call %d)", m.callCount)
	}

	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

// MockTool is a simple tool for testing
type MockTool struct {
	name        string
	description string
	result      string
	err         error
	callCount   int
	lastParams  map[string]any
}

func (m *MockTool) Name() string        { return m.name }
func (m *MockTool) Description() string { return m.description }
func (m *MockTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{"type": "string"},
		},
	}
}
func (m *MockTool) Call(ctx context.Context, params map[string]any) (string, error) {
	m.callCount++
	m.lastParams = params
	return m.result, m.err
}

// MockStreamingClient wraps MockLLMClient with streaming support
type MockStreamingClient struct {
	MockLLMClient
}

func (m *MockStreamingClient) ChatStream(ctx context.Context, messages []llm.Message, streamFunc func(string)) (*llm.Response, error) {
	resp, err := m.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	// Simulate streaming: stream content if not a tool call
	if len(resp.ToolCalls) == 0 {
		streamFunc(resp.Content)
	}
	return resp, nil
}

func TestAgent_New(t *testing.T) {
	mockClient := &MockLLMClient{}
	mockTool := &MockTool{name: "test", description: "A test tool"}

	agent, err := New(Config{
		Client:  mockClient,
		MaxIter: 5,
		Tools:   []tools.Tool{mockTool},
	})

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agent == nil {
		t.Fatal("New() returned nil agent")
	}
	if agent.maxIter != 5 {
		t.Errorf("maxIter = %d, want 5", agent.maxIter)
	}
	if len(agent.tools) != 1 {
		t.Errorf("tools count = %d, want 1", len(agent.tools))
	}
}

func TestAgent_New_DefaultMaxIter(t *testing.T) {
	mockClient := &MockLLMClient{}

	agent, err := New(Config{
		Client: mockClient,
		// MaxIter not set
	})

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agent.maxIter != 10 {
		t.Errorf("default maxIter = %d, want 10", agent.maxIter)
	}
}

func TestAgent_Run_DirectAnswer(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*llm.Response{
			{
				Content:  "The answer is 42.",
				IsFinish: true,
			},
		},
	}

	agent, _ := New(Config{Client: mockClient})

	result, err := agent.Run(context.Background(), "What is the answer?")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result != "The answer is 42." {
		t.Errorf("Run() = %q, want %q", result, "The answer is 42.")
	}
	if mockClient.callCount != 1 {
		t.Errorf("LLM call count = %d, want 1", mockClient.callCount)
	}
}

func TestAgent_Run_SingleToolCall(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*llm.Response{
			{
				Content: `{"name": "test", "parameters": {"input": "hello"}}`,
				ToolCalls: []llm.ToolCallParse{
					{Name: "test", Params: map[string]any{"input": "hello"}},
				},
			},
			{
				Content:  "The tool returned: world",
				IsFinish: true,
			},
		},
	}

	mockTool := &MockTool{
		name:   "test",
		result: "world",
	}

	agent, _ := New(Config{
		Client: mockClient,
		Tools:  []tools.Tool{mockTool},
	})

	result, err := agent.Run(context.Background(), "Say hello")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result, "world") {
		t.Errorf("Run() = %q, want to contain 'world'", result)
	}
	if mockTool.callCount != 1 {
		t.Errorf("Tool call count = %d, want 1", mockTool.callCount)
	}
	if mockTool.lastParams["input"] != "hello" {
		t.Errorf("Tool params = %v, want input='hello'", mockTool.lastParams)
	}
}

func TestAgent_Run_MultipleToolCalls(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*llm.Response{
			{
				Content: `{"name": "tool1", "parameters": {}}`,
				ToolCalls: []llm.ToolCallParse{
					{Name: "tool1", Params: map[string]any{}},
				},
			},
			{
				Content: `{"name": "tool2", "parameters": {}}`,
				ToolCalls: []llm.ToolCallParse{
					{Name: "tool2", Params: map[string]any{}},
				},
			},
			{
				Content:  "Done with both tools.",
				IsFinish: true,
			},
		},
	}

	tool1 := &MockTool{name: "tool1", result: "result1"}
	tool2 := &MockTool{name: "tool2", result: "result2"}

	agent, _ := New(Config{
		Client: mockClient,
		Tools:  []tools.Tool{tool1, tool2},
	})

	result, err := agent.Run(context.Background(), "Use both tools")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if tool1.callCount != 1 {
		t.Errorf("tool1 call count = %d, want 1", tool1.callCount)
	}
	if tool2.callCount != 1 {
		t.Errorf("tool2 call count = %d, want 1", tool2.callCount)
	}
	if mockClient.callCount != 3 {
		t.Errorf("LLM call count = %d, want 3", mockClient.callCount)
	}
	if !strings.Contains(result, "Done") {
		t.Errorf("Run() = %q, want to contain 'Done'", result)
	}
}

func TestAgent_Run_ToolError(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*llm.Response{
			{
				Content: `{"name": "failing", "parameters": {}}`,
				ToolCalls: []llm.ToolCallParse{
					{Name: "failing", Params: map[string]any{}},
				},
			},
			{
				Content:  "The tool failed, but I handled it.",
				IsFinish: true,
			},
		},
	}

	failingTool := &MockTool{
		name: "failing",
		err:  fmt.Errorf("tool exploded"),
	}

	agent, _ := New(Config{
		Client: mockClient,
		Tools:  []tools.Tool{failingTool},
	})

	result, err := agent.Run(context.Background(), "Use the failing tool")

	// Agent should handle tool errors gracefully
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (tool errors should be handled)", err)
	}
	if failingTool.callCount != 1 {
		t.Errorf("Tool call count = %d, want 1", failingTool.callCount)
	}

	// Check that error was passed to LLM in second call
	if len(mockClient.messages) < 2 {
		t.Fatal("Expected at least 2 LLM calls")
	}
	secondCallMessages := mockClient.messages[1]
	lastMsg := secondCallMessages[len(secondCallMessages)-1]
	if !strings.Contains(lastMsg.Content, "Error") {
		t.Errorf("Error should be passed to LLM, got: %s", lastMsg.Content)
	}

	if !strings.Contains(result, "handled") {
		t.Errorf("Run() = %q, want to contain 'handled'", result)
	}
}

func TestAgent_Run_UnknownTool(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*llm.Response{
			{
				Content: `{"name": "nonexistent", "parameters": {}}`,
				ToolCalls: []llm.ToolCallParse{
					{Name: "nonexistent", Params: map[string]any{}},
				},
			},
			{
				Content:  "I tried an unknown tool.",
				IsFinish: true,
			},
		},
	}

	agent, _ := New(Config{
		Client: mockClient,
		Tools:  []tools.Tool{}, // No tools registered
	})

	result, err := agent.Run(context.Background(), "Use unknown tool")

	// Should handle gracefully
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// The "unknown tool" error should be sent back to LLM
	if len(mockClient.messages) >= 2 {
		lastMsg := mockClient.messages[1][len(mockClient.messages[1])-1]
		if !strings.Contains(lastMsg.Content, "unknown tool") {
			t.Errorf("Unknown tool error should be passed to LLM")
		}
	}
	if !strings.Contains(result, "unknown") {
		t.Errorf("Run() = %q, want to contain 'unknown'", result)
	}
}

func TestAgent_Run_MaxIterations(t *testing.T) {
	// LLM keeps calling tools forever
	infiniteResponses := make([]*llm.Response, 100)
	for i := range infiniteResponses {
		infiniteResponses[i] = &llm.Response{
			Content: `{"name": "loop", "parameters": {}}`,
			ToolCalls: []llm.ToolCallParse{
				{Name: "loop", Params: map[string]any{}},
			},
		}
	}

	mockClient := &MockLLMClient{responses: infiniteResponses}
	loopTool := &MockTool{name: "loop", result: "looping"}

	agent, _ := New(Config{
		Client:  mockClient,
		MaxIter: 3,
		Tools:   []tools.Tool{loopTool},
	})

	_, err := agent.Run(context.Background(), "Loop forever")

	if err == nil {
		t.Error("Run() should return error when max iterations reached")
	}
	if !strings.Contains(err.Error(), "max iterations") {
		t.Errorf("error = %v, want to contain 'max iterations'", err)
	}
	if loopTool.callCount != 3 {
		t.Errorf("Tool call count = %d, want 3 (maxIter)", loopTool.callCount)
	}
}

func TestAgent_ClearHistory(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*llm.Response{
			{Content: "First response", IsFinish: true},
			{Content: "Second response", IsFinish: true},
		},
	}

	agent, _ := New(Config{Client: mockClient})

	// First query
	agent.Run(context.Background(), "First query")

	// History should have messages
	if len(agent.history) == 0 {
		t.Error("History should not be empty after query")
	}

	// Clear history
	agent.ClearHistory()

	if len(agent.history) != 0 {
		t.Errorf("History length = %d after clear, want 0", len(agent.history))
	}
}

func TestAgent_History_Accumulates(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*llm.Response{
			{Content: "Response 1", IsFinish: true},
			{Content: "Response 2", IsFinish: true},
		},
	}

	agent, _ := New(Config{Client: mockClient})

	agent.Run(context.Background(), "Query 1")
	agent.Run(context.Background(), "Query 2")

	// Should have 4 messages: user1, assistant1, user2, assistant2
	if len(agent.history) != 4 {
		t.Errorf("History length = %d, want 4", len(agent.history))
	}

	// Second LLM call should include history from first query
	if len(mockClient.messages) < 2 {
		t.Fatal("Expected 2 LLM calls")
	}
	secondCallMsgs := mockClient.messages[1]
	// Should have: system + history(user1, assistant1) + user2
	if len(secondCallMsgs) < 4 {
		t.Errorf("Second call message count = %d, want at least 4", len(secondCallMsgs))
	}
}

func TestAgent_Run_Streaming(t *testing.T) {
	mockClient := &MockStreamingClient{
		MockLLMClient: MockLLMClient{
			responses: []*llm.Response{
				{
					Content:  "Streaming answer about containers.",
					IsFinish: true,
				},
			},
		},
	}

	agent, _ := New(Config{Client: mockClient})

	result, err := agent.Run(context.Background(), "What is a container?")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result != "Streaming answer about containers." {
		t.Errorf("Run() = %q, want %q", result, "Streaming answer about containers.")
	}
	if mockClient.callCount != 1 {
		t.Errorf("LLM call count = %d, want 1", mockClient.callCount)
	}
}

func TestAgent_Run_StreamingToolCall(t *testing.T) {
	mockClient := &MockStreamingClient{
		MockLLMClient: MockLLMClient{
			responses: []*llm.Response{
				{
					Content: `{"name": "test", "parameters": {"input": "hello"}}`,
					ToolCalls: []llm.ToolCallParse{
						{Name: "test", Params: map[string]any{"input": "hello"}},
					},
				},
				{
					Content:  "The tool returned: world",
					IsFinish: true,
				},
			},
		},
	}

	mockTool := &MockTool{
		name:   "test",
		result: "world",
	}

	agent, _ := New(Config{
		Client: mockClient,
		Tools:  []tools.Tool{mockTool},
	})

	result, err := agent.Run(context.Background(), "Say hello")

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result, "world") {
		t.Errorf("Run() = %q, want to contain 'world'", result)
	}
	if mockTool.callCount != 1 {
		t.Errorf("Tool call count = %d, want 1", mockTool.callCount)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"ab", 1, "a..."},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
