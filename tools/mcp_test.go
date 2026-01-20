package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPTool_Name(t *testing.T) {
	tool := &MCPTool{}
	if got := tool.Name(); got != "mcp" {
		t.Errorf("Name() = %q, want %q", got, "mcp")
	}
}

func TestMCPTool_Description(t *testing.T) {
	tool := &MCPTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
	// Should mention key actions
	for _, action := range []string{"get_pods", "describe_pod", "get_logs"} {
		if !strings.Contains(desc, action) {
			t.Errorf("Description() should mention %q", action)
		}
	}
}

func TestMCPTool_Parameters(t *testing.T) {
	tool := &MCPTool{}
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Errorf("Parameters type = %v, want 'object'", params["type"])
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Parameters should have 'required' array")
	}

	// server and action are required
	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r] = true
	}
	if !requiredMap["server"] || !requiredMap["action"] {
		t.Errorf("required = %v, want to include 'server' and 'action'", required)
	}
}

func TestMCPTool_Call_GetPods(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"server":    "test.example.com",
		"action":    "get_pods",
		"namespace": "myns",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	// Parse result as JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Check namespace is returned
	if data["namespace"] != "myns" {
		t.Errorf("namespace = %v, want 'myns'", data["namespace"])
	}

	// Check pods array exists
	pods, ok := data["pods"].([]any)
	if !ok {
		t.Fatal("Result should contain 'pods' array")
	}
	if len(pods) == 0 {
		t.Error("pods array should not be empty")
	}

	// Check first pod has expected fields
	pod := pods[0].(map[string]any)
	for _, field := range []string{"name", "status", "ready"} {
		if _, ok := pod[field]; !ok {
			t.Errorf("pod should have '%s' field", field)
		}
	}
}

func TestMCPTool_Call_DefaultNamespace(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"server": "test.example.com",
		"action": "get_pods",
		// namespace not specified
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Should default to "default" namespace
	if data["namespace"] != "default" {
		t.Errorf("namespace = %v, want 'default'", data["namespace"])
	}
}

func TestMCPTool_Call_DescribePod(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"server":    "test.example.com",
		"action":    "describe_pod",
		"namespace": "myns",
		"resource":  "my-pod-123",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Check pod details
	if data["name"] != "my-pod-123" {
		t.Errorf("name = %v, want 'my-pod-123'", data["name"])
	}
	if _, ok := data["status"]; !ok {
		t.Error("Result should contain 'status'")
	}
	if _, ok := data["containers"]; !ok {
		t.Error("Result should contain 'containers'")
	}
}

func TestMCPTool_Call_DescribePod_MissingResource(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	_, err := tool.Call(ctx, map[string]any{
		"server": "test.example.com",
		"action": "describe_pod",
		// resource not specified
	})

	if err == nil {
		t.Error("describe_pod without resource should return error")
	}
}

func TestMCPTool_Call_GetLogs(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"server":   "test.example.com",
		"action":   "get_logs",
		"resource": "my-pod",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	// Logs should contain typical log format
	if !strings.Contains(result, "ERROR") && !strings.Contains(result, "INFO") {
		t.Error("Logs should contain log level markers")
	}
}

func TestMCPTool_Call_GetLogs_MissingResource(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	_, err := tool.Call(ctx, map[string]any{
		"server": "test.example.com",
		"action": "get_logs",
		// resource not specified
	})

	if err == nil {
		t.Error("get_logs without resource should return error")
	}
}

func TestMCPTool_Call_GetEvents(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"server":    "test.example.com",
		"action":    "get_events",
		"namespace": "myns",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	events, ok := data["events"].([]any)
	if !ok {
		t.Fatal("Result should contain 'events' array")
	}
	if len(events) == 0 {
		t.Error("events array should not be empty")
	}

	// Check event structure
	event := events[0].(map[string]any)
	for _, field := range []string{"type", "reason", "message"} {
		if _, ok := event[field]; !ok {
			t.Errorf("event should have '%s' field", field)
		}
	}
}

func TestMCPTool_Call_GetDeployments(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	result, err := tool.Call(ctx, map[string]any{
		"server": "test.example.com",
		"action": "get_deployments",
	})

	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	deployments, ok := data["deployments"].([]any)
	if !ok {
		t.Fatal("Result should contain 'deployments' array")
	}
	if len(deployments) == 0 {
		t.Error("deployments array should not be empty")
	}
}

func TestMCPTool_Call_UnknownAction(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	_, err := tool.Call(ctx, map[string]any{
		"server": "test.example.com",
		"action": "unknown_action",
	})

	if err == nil {
		t.Error("unknown action should return error")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("error = %v, want to contain 'unknown action'", err)
	}
}

func TestMCPTool_Call_MissingServer(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	_, err := tool.Call(ctx, map[string]any{
		"action": "get_pods",
	})

	if err == nil {
		t.Error("missing server should return error")
	}
}

func TestMCPTool_Call_MissingAction(t *testing.T) {
	tool := &MCPTool{}
	ctx := context.Background()

	_, err := tool.Call(ctx, map[string]any{
		"server": "test.example.com",
	})

	if err == nil {
		t.Error("missing action should return error")
	}
}
