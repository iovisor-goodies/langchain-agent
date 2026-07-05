package tools

import (
	"context"
	"fmt"
	"strings"
)

// EdgeGPIOTool reads or writes a GPIO line on the configured edge target
// using libgpiod's gpioget/gpioset CLIs. Works on any Linux box with
// libgpiod-tools installed and an addressable gpiochip (Raspberry Pi 4/5,
// many SBCs). Will fail cleanly on hosts without GPIO hardware.
type EdgeGPIOTool struct {
	host string
	exec sshExecutor
}

func NewEdgeGPIOTool(host string) *EdgeGPIOTool {
	return &EdgeGPIOTool{host: host, exec: defaultSSHExec}
}

func (t *EdgeGPIOTool) Name() string { return "edge_gpio" }

func (t *EdgeGPIOTool) Description() string {
	return fmt.Sprintf("Read or write a GPIO line on the configured edge target (%s) via libgpiod (gpioget/gpioset). Defaults to gpiochip0; on Pi 5 use chip='gpiochip4'.", t.host)
}

func (t *EdgeGPIOTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pin": map[string]any{
				"type":        "integer",
				"description": "GPIO line offset within the chip (e.g. 17 for BCM17 on Pi 4 gpiochip0)",
			},
			"action": map[string]any{
				"type":        "string",
				"description": "'read' to read the line, 'write' to set it",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "Required when action='write': 'high' or 'low'",
			},
			"chip": map[string]any{
				"type":        "string",
				"description": "Optional gpiochip name. Default: gpiochip0. Use gpiochip4 for Pi 5.",
			},
		},
		"required": []string{"pin", "action"},
	}
}

func (t *EdgeGPIOTool) Call(ctx context.Context, params map[string]any) (string, error) {
	pin, ok := pinAsInt(params["pin"])
	if !ok {
		return "", fmt.Errorf("pin parameter required (integer)")
	}
	action, ok := params["action"].(string)
	if !ok {
		return "", fmt.Errorf("action parameter required ('read' or 'write')")
	}
	chip, _ := params["chip"].(string)
	if chip == "" {
		chip = "gpiochip0"
	}

	switch action {
	case "read":
		out, err := t.exec(ctx, t.host, fmt.Sprintf("gpioget %s %d", chip, pin))
		if err != nil {
			return "", err
		}
		return "level=" + strings.TrimSpace(out), nil
	case "write":
		v, ok := params["value"].(string)
		if !ok {
			return "", fmt.Errorf("value parameter required when action='write'")
		}
		var bit string
		switch strings.ToLower(v) {
		case "high", "1", "on":
			bit = "1"
		case "low", "0", "off":
			bit = "0"
		default:
			return "", fmt.Errorf("value must be 'high' or 'low' (got %q)", v)
		}
		_, err := t.exec(ctx, t.host, fmt.Sprintf("gpioset %s %d=%s", chip, pin, bit))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("ok: %s line %d set %s", chip, pin, v), nil
	default:
		return "", fmt.Errorf("action must be 'read' or 'write' (got %q)", action)
	}
}

// pinAsInt accepts either a JSON number (float64) or a string.
func pinAsInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case string:
		var n int
		if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
			return n, true
		}
	}
	return 0, false
}
