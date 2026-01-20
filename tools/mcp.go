package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// MCPTool is a stub for MCP server interactions
// TODO: Implement real MCP client protocol
type MCPTool struct{}

func (m *MCPTool) Name() string {
	return "mcp"
}

func (m *MCPTool) Description() string {
	return "Query an MCP server for Kubernetes/OpenShift operations. Actions: get_pods, describe_pod, get_logs, get_events, get_deployments"
}

func (m *MCPTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"server": map[string]any{
				"type":        "string",
				"description": "MCP server hostname (e.g., test.my.domain)",
			},
			"action": map[string]any{
				"type":        "string",
				"description": "Action to perform: get_pods, describe_pod, get_logs, get_events, get_deployments",
			},
			"namespace": map[string]any{
				"type":        "string",
				"description": "Kubernetes namespace (optional, defaults to 'default')",
			},
			"resource": map[string]any{
				"type":        "string",
				"description": "Resource name (e.g., pod name) for describe/logs actions",
			},
		},
		"required": []string{"server", "action"},
	}
}

func (m *MCPTool) Call(ctx context.Context, params map[string]any) (string, error) {
	server, _ := params["server"].(string)
	action, _ := params["action"].(string)
	namespace, _ := params["namespace"].(string)
	resource, _ := params["resource"].(string)

	if server == "" {
		return "", fmt.Errorf("server parameter required")
	}
	if action == "" {
		return "", fmt.Errorf("action parameter required")
	}
	if namespace == "" {
		namespace = "default"
	}

	// STUB: Return mock data for testing
	// TODO: Implement real MCP client protocol
	return m.mockResponse(server, action, namespace, resource)
}

func (m *MCPTool) mockResponse(server, action, namespace, resource string) (string, error) {
	switch action {
	case "get_pods":
		return m.mockGetPods(namespace)
	case "describe_pod":
		return m.mockDescribePod(namespace, resource)
	case "get_logs":
		return m.mockGetLogs(namespace, resource)
	case "get_events":
		return m.mockGetEvents(namespace)
	case "get_deployments":
		return m.mockGetDeployments(namespace)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (m *MCPTool) mockGetPods(namespace string) (string, error) {
	pods := []map[string]any{
		{
			"name":    "api-server-7d8f9b6c5-xk2mn",
			"status":  "Running",
			"ready":   "1/1",
			"restarts": 0,
			"age":     "2d",
		},
		{
			"name":    "worker-5c4d3b2a1-pq9rs",
			"status":  "CrashLoopBackOff",
			"ready":   "0/1",
			"restarts": 15,
			"age":     "1d",
		},
		{
			"name":    "database-6e5f4d3c2-lm8no",
			"status":  "Running",
			"ready":   "1/1",
			"restarts": 0,
			"age":     "5d",
		},
	}
	result, _ := json.MarshalIndent(map[string]any{
		"namespace": namespace,
		"pods":      pods,
	}, "", "  ")
	return string(result), nil
}

func (m *MCPTool) mockDescribePod(namespace, podName string) (string, error) {
	if podName == "" {
		return "", fmt.Errorf("resource (pod name) required for describe_pod")
	}
	desc := map[string]any{
		"name":      podName,
		"namespace": namespace,
		"status": map[string]any{
			"phase":   "CrashLoopBackOff",
			"reason":  "Error",
			"message": "Back-off 5m0s restarting failed container",
		},
		"containers": []map[string]any{
			{
				"name":         "main",
				"image":        "myapp:latest",
				"state":        "Waiting",
				"reason":       "CrashLoopBackOff",
				"restartCount": 15,
				"lastState": map[string]any{
					"exitCode": 1,
					"reason":   "Error",
				},
			},
		},
		"events": []map[string]any{
			{"type": "Warning", "reason": "BackOff", "message": "Back-off restarting failed container"},
			{"type": "Warning", "reason": "Failed", "message": "Error: container exited with code 1"},
		},
	}
	result, _ := json.MarshalIndent(desc, "", "  ")
	return string(result), nil
}

func (m *MCPTool) mockGetLogs(namespace, podName string) (string, error) {
	if podName == "" {
		return "", fmt.Errorf("resource (pod name) required for get_logs")
	}
	return `2024-01-15T10:23:45Z [ERROR] Failed to connect to database: connection refused
2024-01-15T10:23:45Z [ERROR] Retrying in 5 seconds...
2024-01-15T10:23:50Z [ERROR] Failed to connect to database: connection refused
2024-01-15T10:23:50Z [FATAL] Max retries exceeded, exiting
`, nil
}

func (m *MCPTool) mockGetEvents(namespace string) (string, error) {
	events := []map[string]any{
		{
			"type":    "Warning",
			"reason":  "BackOff",
			"object":  "pod/worker-5c4d3b2a1-pq9rs",
			"message": "Back-off restarting failed container",
			"age":     "5m",
		},
		{
			"type":    "Normal",
			"reason":  "Pulled",
			"object":  "pod/api-server-7d8f9b6c5-xk2mn",
			"message": "Successfully pulled image",
			"age":     "2d",
		},
	}
	result, _ := json.MarshalIndent(map[string]any{
		"namespace": namespace,
		"events":    events,
	}, "", "  ")
	return string(result), nil
}

func (m *MCPTool) mockGetDeployments(namespace string) (string, error) {
	deployments := []map[string]any{
		{
			"name":      "api-server",
			"ready":     "1/1",
			"upToDate":  1,
			"available": 1,
			"age":       "10d",
		},
		{
			"name":      "worker",
			"ready":     "0/1",
			"upToDate":  1,
			"available": 0,
			"age":       "10d",
		},
	}
	result, _ := json.MarshalIndent(map[string]any{
		"namespace":   namespace,
		"deployments": deployments,
	}, "", "  ")
	return string(result), nil
}
