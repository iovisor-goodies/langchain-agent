package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// EdgeTempTool reads CPU temperature on the configured edge target.
// It reads /sys/class/thermal/thermal_zone0/temp, which works on Raspberry Pi
// (4/5 with current kernels), Intel/AMD x86 Linux boxes, and most Linux SBCs.
// The kernel reports the value in millidegrees Celsius.
type EdgeTempTool struct {
	host string
	exec sshExecutor
}

func NewEdgeTempTool(host string) *EdgeTempTool {
	return &EdgeTempTool{host: host, exec: defaultSSHExec}
}

func (t *EdgeTempTool) Name() string { return "edge_temp" }

func (t *EdgeTempTool) Description() string {
	return fmt.Sprintf("Read CPU temperature on the configured edge target (%s) via /sys/class/thermal. Works on Pi and amd64 Linux. No parameters.", t.host)
}

func (t *EdgeTempTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *EdgeTempTool) Call(ctx context.Context, params map[string]any) (string, error) {
	out, err := t.exec(ctx, t.host, "cat /sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(out)
	if n, err := strconv.Atoi(line); err == nil {
		return fmt.Sprintf("%.1f°C", float64(n)/1000.0), nil
	}
	return line, nil
}
