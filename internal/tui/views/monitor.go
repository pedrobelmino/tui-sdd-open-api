package views

import (
	"fmt"
	"strings"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/tokens"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/ui"
)

const monitorPanelTitle = "Monitor"

// MonitorData is the live Ollama/token/window snapshot for the status strip.
type MonitorData struct {
	Width           int
	WindowWidth     int
	WindowHeight    int
	Ollama          ollama.Snapshot
	Tokens          tokens.SessionCounter
	ConfiguredModel string
	Provider        string
}

// RenderMonitorStrip renders a compact always-visible model + metadata status panel.
func RenderMonitorStrip(d MonitorData) string {
	width := d.Width
	if width < 1 {
		width = 80
	}
	return ui.Panel(monitorPanelTitle, monitorContent(d), width, 5)
}

func monitorContent(d MonitorData) string {
	return strings.Join([]string{
		formatModelLine(d.Ollama, d.ConfiguredModel, d.Provider),
		formatMetadataLine(d),
		formatTokensLine(d.Tokens),
	}, "\n")
}

func formatModelLine(snap ollama.Snapshot, configured, provider string) string {
	if provider == "openapi" {
		return fmt.Sprintf("Provider: Open API (remote) · %s", configured)
	}
	if !snap.Reachable {
		return "Model: offline — Ollama unreachable"
	}
	if len(snap.Running) == 0 {
		if configured != "" {
			return "Model: · " + configured + " — not loaded in VRAM"
		}
		return "Model: idle — no model loaded in VRAM"
	}
	r := snap.Running[0]
	vram := formatBytes(r.SizeVRAM)
	ctx := ""
	if r.ContextLength > 0 {
		ctx = fmt.Sprintf(" │ ctx %d", r.ContextLength)
	}
	return fmt.Sprintf("Model: ● %s │ VRAM %s%s", r.Name, vram, ctx)
}

func formatMetadataLine(d MonitorData) string {
	return fmt.Sprintf("Window: %d×%d │ Model: %s",
		d.WindowWidth, d.WindowHeight, d.ConfiguredModel)
}

func formatTokensLine(counter tokens.SessionCounter) string {
	if counter.RequestCount == 0 {
		return "Tokens: —"
	}
	return fmt.Sprintf("Tokens: %s prompt + %s completion (%d requests)",
		formatCommaInt(counter.PromptTokens),
		formatCommaInt(counter.CompletionTokens),
		counter.RequestCount)
}