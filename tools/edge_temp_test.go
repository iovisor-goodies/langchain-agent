package tools

import (
	"context"
	"testing"
)

func TestEdgeTempTool_ParsesMillidegrees(t *testing.T) {
	tool := &EdgeTempTool{
		host: "eagle@host",
		exec: func(ctx context.Context, host, cmd string) (string, error) {
			if host != "eagle@host" {
				t.Errorf("host = %q, want eagle@host", host)
			}
			if cmd != "cat /sys/class/thermal/thermal_zone0/temp" {
				t.Errorf("cmd = %q, want thermal_zone0 read", cmd)
			}
			return "43800\n", nil
		},
	}

	got, err := tool.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	want := "43.8°C"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEdgeTempTool_ReturnsRawWhenUnparseable(t *testing.T) {
	tool := &EdgeTempTool{
		host: "h",
		exec: func(ctx context.Context, host, cmd string) (string, error) {
			return "weird output", nil
		},
	}
	got, err := tool.Call(context.Background(), nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got != "weird output" {
		t.Errorf("got %q, want raw line", got)
	}
}
