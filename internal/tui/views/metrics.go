package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/tokens"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/ui"
)

const (
	tokensPanelTitle   = "Session Tokens"
	metadataPanelTitle = "Metadata"
)

// MetricsData is the subset of RootModel state needed to render the Metrics view.
type MetricsData struct {
	Width           int
	Height          int
	WindowWidth     int
	WindowHeight    int
	Tokens          tokens.SessionCounter
	ConfiguredModel string
}

// RenderMetrics renders the Metrics view with session token and metadata panels.
func RenderMetrics(m MetricsData) string {
	width := m.Width
	if width < 1 {
		width = 80
	}
	height := m.Height
	if height < 1 {
		height = 20
	}

	tokenH := 8
	metaH := height - tokenH - 1
	if metaH < 5 {
		metaH = 5
	}

	tokensBox := ui.Panel(tokensPanelTitle, renderTokensContent(m.Tokens), width, tokenH)
	metaBox := ui.Panel(metadataPanelTitle, renderMetadataContent(m), width, metaH)

	return tokensBox + "\n\n" + metaBox
}

func renderTokensContent(counter tokens.SessionCounter) string {
	lines := []string{
		fmt.Sprintf("Prompt:     %s", formatCommaInt(counter.PromptTokens)),
		fmt.Sprintf("Completion: %s", formatCommaInt(counter.CompletionTokens)),
		fmt.Sprintf("Total:      %s", formatCommaInt(counter.TotalTokens)),
		fmt.Sprintf("Requests:   %d", counter.RequestCount),
	}
	if counter.RequestCount == 0 {
		lines = append(lines, "no requests yet")
	} else if counter.LastRequest != nil {
		last := counter.LastRequest
		lines = append(lines, fmt.Sprintf(
			"Last:       %s prompt + %s completion @ %s",
			formatCommaInt(last.PromptTokens),
			formatCommaInt(last.CompletionTokens),
			last.At.Format("15:04:05"),
		))
	}
	return strings.Join(lines, "\n")
}

func renderMetadataContent(m MetricsData) string {
	return fmt.Sprintf("Terminal size: %d × %d\nConfigured model: %s",
		m.WindowWidth, m.WindowHeight, m.ConfiguredModel)
}

func formatCommaInt(n int) string {
	if n < 0 {
		return strconv.Itoa(n)
	}

	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}

	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}
