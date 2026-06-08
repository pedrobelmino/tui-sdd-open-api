package views

import (
	"strings"
	"testing"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/tokens"
)

func TestRenderMonitorStrip_RunningModelAndMetadata(t *testing.T) {
	out := RenderMonitorStrip(MonitorData{
		Width: 100,
		Ollama: ollama.Snapshot{
			Reachable: true,
			Running: []ollama.RunningModel{{
				Name: "qwen2.5-coder:latest", SizeVRAM: 2408937472, ContextLength: 4096,
			}},
		},
		Tokens:          tokens.SessionCounter{PromptTokens: 100, CompletionTokens: 50, RequestCount: 2},
		WindowWidth:     120,
		WindowHeight:    40,
		ConfiguredModel: "deepseek-v4-flash",
	})

	for _, want := range []string{
		"Monitor",
		"qwen2.5-coder:latest",
		"120×40",
		"deepseek-v4-flash",
		"100 prompt + 50 completion",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMonitorStrip_Offline(t *testing.T) {
	out := RenderMonitorStrip(MonitorData{
		Width:  80,
		Ollama: ollama.Snapshot{Reachable: false},
	})
	if !strings.Contains(out, "offline") {
		t.Fatalf("expected offline message: %s", out)
	}
}
