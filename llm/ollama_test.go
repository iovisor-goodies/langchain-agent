package llm

import (
	"strings"
	"testing"
)

func TestParseResponse_ValidToolCall(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name       string
		content    string
		wantTool   string
		wantParams map[string]any
	}{
		{
			name:     "simple tool call",
			content:  `{"name": "shell", "parameters": {"command": "ls -la"}}`,
			wantTool: "shell",
			wantParams: map[string]any{
				"command": "ls -la",
			},
		},
		{
			name:     "tool call with tool key",
			content:  `{"tool": "ssh", "parameters": {"host": "user@host", "command": "uname"}}`,
			wantTool: "ssh",
			wantParams: map[string]any{
				"host":    "user@host",
				"command": "uname",
			},
		},
		{
			name:     "tool call with params key",
			content:  `{"name": "mcp", "params": {"server": "test.com", "action": "get_pods"}}`,
			wantTool: "mcp",
			wantParams: map[string]any{
				"server": "test.com",
				"action": "get_pods",
			},
		},
		{
			name:     "tool call with surrounding text",
			content:  `I need to check the system. {"name": "shell", "parameters": {"command": "whoami"}} Let me do that.`,
			wantTool: "shell",
			wantParams: map[string]any{
				"command": "whoami",
			},
		},
		{
			name:     "tool call with newlines",
			content:  "Let me execute:\n{\"name\": \"shell\", \"parameters\": {\"command\": \"pwd\"}}",
			wantTool: "shell",
			wantParams: map[string]any{
				"command": "pwd",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.parseResponse(tt.content)

			if len(resp.ToolCalls) == 0 {
				t.Fatal("expected tool call, got none")
			}

			tc := resp.ToolCalls[0]
			if tc.Name != tt.wantTool {
				t.Errorf("tool name = %q, want %q", tc.Name, tt.wantTool)
			}

			for key, want := range tt.wantParams {
				got, ok := tc.Params[key]
				if !ok {
					t.Errorf("missing param %q", key)
					continue
				}
				if got != want {
					t.Errorf("param[%q] = %v, want %v", key, got, want)
				}
			}
		})
	}
}

func TestParseResponse_FinalAnswer(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "plain text answer",
			content: "The system is running Linux on x86_64 architecture.",
		},
		{
			name:    "answer with Final Answer marker",
			content: "Final Answer: The file contains 42 lines.",
		},
		{
			name:    "answer with Answer marker",
			content: "Answer: The server is healthy and running.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.parseResponse(tt.content)

			if len(resp.ToolCalls) > 0 {
				t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
			}
			if !resp.IsFinish {
				t.Error("expected IsFinish=true for final answer")
			}
			if resp.Content != tt.content {
				t.Errorf("content = %q, want %q", resp.Content, tt.content)
			}
		})
	}
}

func TestParseResponse_MalformedJSON(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "incomplete JSON",
			content: `{"name": "shell", "parameters": {"command": "ls"`,
		},
		{
			name:    "JSON without name",
			content: `{"parameters": {"command": "ls"}}`,
		},
		{
			name:    "empty JSON object",
			content: `{}`,
		},
		{
			name:    "JSON array at top level",
			content: `["item1", "item2"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.parseResponse(tt.content)

			// Should not crash, should treat as final answer or no tool call
			if len(resp.ToolCalls) > 0 && resp.ToolCalls[0].Name != "" {
				t.Errorf("malformed JSON should not produce valid tool call, got %+v", resp.ToolCalls[0])
			}
		})
	}
}

func TestFindMatchingBrace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "simple object",
			input: `{"key": "value"}`,
			want:  15,
		},
		{
			name:  "nested object",
			input: `{"outer": {"inner": "value"}}`,
			want:  28,
		},
		{
			name:  "with string containing braces",
			input: `{"key": "value with { and }"}`,
			want:  28,
		},
		{
			name:  "with escaped quotes",
			input: `{"key": "value with \"escaped\""}`,
			want:  32,
		},
		{
			name:  "no opening brace",
			input: `key: value}`,
			want:  -1,
		},
		{
			name:  "unmatched brace",
			input: `{"key": "value"`,
			want:  -1,
		},
		{
			name:  "empty string",
			input: ``,
			want:  -1,
		},
		{
			name:  "deeply nested",
			input: `{"a": {"b": {"c": {"d": "e"}}}}`,
			want:  30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMatchingBrace(tt.input)
			if got != tt.want {
				t.Errorf("findMatchingBrace(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	tools := []ToolDef{
		{
			Name:        "shell",
			Description: "Execute local commands",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "ssh",
			Description: "Execute remote commands via SSH",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"host":    map[string]any{"type": "string"},
					"command": map[string]any{"type": "string"},
				},
			},
		},
	}

	prompt := BuildSystemPrompt(tools)

	// Check that prompt contains key instructions
	expectations := []string{
		"autonomous agent",
		"JSON",
		"tool_name",
		"parameters",
		"final answer",
		"shell",
		"ssh",
		"Execute local commands",
		"Execute remote commands",
	}

	for _, exp := range expectations {
		if !strings.Contains(prompt, exp) {
			t.Errorf("prompt should contain %q", exp)
		}
	}
}

func TestBuildSystemPrompt_EmptyTools(t *testing.T) {
	prompt := BuildSystemPrompt(nil)

	// Should still be a valid prompt
	if prompt == "" {
		t.Error("prompt should not be empty even with no tools")
	}
	if !strings.Contains(prompt, "agent") {
		t.Error("prompt should mention agent")
	}
}

func TestToolDef_JSONMarshal(t *testing.T) {
	tool := ToolDef{
		Name:        "test",
		Description: "A test tool",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "The input value",
				},
			},
			"required": []string{"input"},
		},
	}

	prompt := BuildSystemPrompt([]ToolDef{tool})

	// Verify the tool definition appears in the prompt
	if !strings.Contains(prompt, `"name": "test"`) {
		t.Error("prompt should contain tool name")
	}
	if !strings.Contains(prompt, `"description": "A test tool"`) {
		t.Error("prompt should contain tool description")
	}
}
