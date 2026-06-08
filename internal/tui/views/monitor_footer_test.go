package views

import (
	"strings"
	"testing"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/system"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/tokens"
)

func TestRenderMonitorFooter_SingleLineSummary(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := RenderMonitorFooter(FooterMonitorData{
		Width: 120, ConfiguredModel: "deepseek-v4-flash",
		Ollama: ollama.Snapshot{
			Reachable: true,
			Running: []ollama.RunningModel{{
				Name: "deepseek-v4-flash", SizeVRAM: 2408937472,
			}},
		},
		Tokens:          tokens.SessionCounter{PromptTokens: 1200, CompletionTokens: 800, RequestCount: 3},
		WindowWidth:     120,
		WindowHeight:    40,
		System: system.Snapshot{
			Available: true,
			CPU:       system.CPUInfo{Utilization: 33, HasBaseline: true},
			Memory:    system.MemoryInfo{TotalMiB: 16000, UsedMiB: 7200},
			Load:      system.LoadAvg{One: 0.82},
		},
	})

	bodyLines := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "│") &&
			!strings.Contains(line, "Monitor") &&
			!strings.Contains(line, "╰") {
			bodyLines++
		}
	}
	if bodyLines != 1 {
		t.Fatalf("expected 1 content line, got %d:\n%s", bodyLines, out)
	}

	for _, want := range []string{
		"deepseek-v4-flash",
		"Win 120×40",
		"Tok 1.2k+800",
		"CPU 33%",
		"MEM 45%",
		"Ld 0.8",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatFooterMonitorLine_UsesSingleLine(t *testing.T) {
	line := formatFooterMonitorLine(FooterMonitorData{
		Width: 100,
		Ollama: ollama.Snapshot{Reachable: true, Running: []ollama.RunningModel{{Name: "phi3:mini"}}},
		Tokens:          tokens.SessionCounter{},
		WindowWidth:     80,
		WindowHeight:    24,
		System: system.Snapshot{Available: true, CPU: system.CPUInfo{HasBaseline: true, Utilization: 5}, Memory: system.MemoryInfo{TotalMiB: 8000, UsedMiB: 2000}, Load: system.LoadAvg{One: 0.1}},
	})
	if strings.Count(line, "\n") > 0 {
		t.Fatalf("expected single line, got: %q", line)
	}
	if !strings.Contains(line, " │ ") {
		t.Fatalf("expected horizontal separators: %q", line)
	}
}
