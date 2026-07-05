package tools

import (
	"context"
	"strings"
	"testing"
)

func TestEdgeGPIOTool_Read(t *testing.T) {
	var gotCmd string
	tool := &EdgeGPIOTool{
		host: "h",
		exec: func(ctx context.Context, host, cmd string) (string, error) {
			gotCmd = cmd
			return "1\n", nil
		},
	}

	got, err := tool.Call(context.Background(), map[string]any{
		"pin":    float64(17),
		"action": "read",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if gotCmd != "gpioget gpiochip0 17" {
		t.Errorf("cmd = %q", gotCmd)
	}
	if got != "level=1" {
		t.Errorf("got %q, want level=1", got)
	}
}

func TestEdgeGPIOTool_ReadCustomChip(t *testing.T) {
	var gotCmd string
	tool := &EdgeGPIOTool{
		host: "h",
		exec: func(ctx context.Context, host, cmd string) (string, error) {
			gotCmd = cmd
			return "0\n", nil
		},
	}

	_, err := tool.Call(context.Background(), map[string]any{
		"pin":    float64(17),
		"action": "read",
		"chip":   "gpiochip4",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if gotCmd != "gpioget gpiochip4 17" {
		t.Errorf("cmd = %q, want pi5 chip", gotCmd)
	}
}

func TestEdgeGPIOTool_WriteHigh(t *testing.T) {
	var gotCmd string
	tool := &EdgeGPIOTool{
		host: "h",
		exec: func(ctx context.Context, host, cmd string) (string, error) {
			gotCmd = cmd
			return "", nil
		},
	}

	got, err := tool.Call(context.Background(), map[string]any{
		"pin":    float64(22),
		"action": "write",
		"value":  "high",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if gotCmd != "gpioset gpiochip0 22=1" {
		t.Errorf("cmd = %q", gotCmd)
	}
	if !strings.Contains(got, "ok") {
		t.Errorf("got %q, want ok", got)
	}
}

func TestEdgeGPIOTool_WriteLow(t *testing.T) {
	var gotCmd string
	tool := &EdgeGPIOTool{
		host: "h",
		exec: func(ctx context.Context, host, cmd string) (string, error) {
			gotCmd = cmd
			return "", nil
		},
	}

	_, err := tool.Call(context.Background(), map[string]any{
		"pin":    float64(22),
		"action": "write",
		"value":  "low",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if gotCmd != "gpioset gpiochip0 22=0" {
		t.Errorf("cmd = %q", gotCmd)
	}
}

func TestEdgeGPIOTool_BadAction(t *testing.T) {
	tool := &EdgeGPIOTool{host: "h", exec: func(ctx context.Context, host, cmd string) (string, error) { return "", nil }}
	_, err := tool.Call(context.Background(), map[string]any{"pin": float64(1), "action": "toggle"})
	if err == nil {
		t.Error("expected error for bad action")
	}
}

func TestEdgeGPIOTool_BadValue(t *testing.T) {
	tool := &EdgeGPIOTool{host: "h", exec: func(ctx context.Context, host, cmd string) (string, error) { return "", nil }}
	_, err := tool.Call(context.Background(), map[string]any{"pin": float64(1), "action": "write", "value": "wibble"})
	if err == nil {
		t.Error("expected error for bad value")
	}
}

func TestEdgeGPIOTool_PinAsString(t *testing.T) {
	var gotCmd string
	tool := &EdgeGPIOTool{
		host: "h",
		exec: func(ctx context.Context, host, cmd string) (string, error) {
			gotCmd = cmd
			return "0\n", nil
		},
	}
	_, err := tool.Call(context.Background(), map[string]any{"pin": "5", "action": "read"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if gotCmd != "gpioget gpiochip0 5" {
		t.Errorf("cmd = %q", gotCmd)
	}
}
