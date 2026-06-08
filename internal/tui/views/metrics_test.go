package views

import (
	"strings"
	"testing"
	"time"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/tokens"
)

const testMetricsWidth = 80

func TestMetrics_NoRequestsYet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderMetrics(MetricsData{
		Width:           testMetricsWidth,
		Height:          16,
		Tokens:          tokens.SessionCounter{},
		WindowWidth:     120,
		WindowHeight:    40,
		ConfiguredModel: "deepseek-v4-flash",
	})

	for _, want := range []string{
		"Session Tokens",
		"Prompt:     0",
		"Completion: 0",
		"Total:      0",
		"no requests yet",
		"Metadata",
		"120 × 40",
		"deepseek-v4-flash",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestMetrics_TokenPanelWithCounts(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var counter tokens.SessionCounter
	counter.Add(1240, 890)

	got := RenderMetrics(MetricsData{
		Width:           testMetricsWidth,
		Height:          16,
		Tokens:          counter,
		ConfiguredModel: "deepseek-v4-flash",
	})

	for _, want := range []string{
		"Prompt:     1,240",
		"Completion: 890",
		"Total:      2,130",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output:\n%s", want, got)
		}
	}
	if strings.Contains(got, "no requests yet") {
		t.Fatalf("did not expect %q when requests exist:\n%s", "no requests yet", got)
	}
}

func TestMetrics_Snapshot80Cols(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var counter tokens.SessionCounter
	counter.Add(1240, 890)

	got := RenderMetrics(MetricsData{
		Width:           testMetricsWidth,
		Height:          16,
		Tokens:          counter,
		WindowWidth:     120,
		WindowHeight:    40,
		ConfiguredModel: "deepseek-v4-flash",
	})

	if !strings.Contains(got, "Session Tokens") {
		t.Fatal("expected Session Tokens panel")
	}
	if !strings.Contains(got, "Metadata") {
		t.Fatal("expected Metadata panel")
	}
	if !strings.Contains(got, "1,240") {
		t.Fatal("expected formatted prompt tokens")
	}
	if !strings.Contains(got, "120 × 40") {
		t.Fatal("expected window dimensions")
	}
	if !strings.Contains(got, "deepseek-v4-flash") {
		t.Fatal("expected configured model name")
	}
}

func TestMetrics_LastRequestShown(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var counter tokens.SessionCounter
	now := time.Now()
	counter.PromptTokens = 5000
	counter.CompletionTokens = 3000
	counter.RequestCount = 1
	counter.LastRequest = &tokens.UsageSnapshot{
		PromptTokens:     2000,
		CompletionTokens: 1500,
		At:               now,
	}

	got := RenderMetrics(MetricsData{
		Width:           testMetricsWidth,
		Height:          16,
		Tokens:          counter,
		ConfiguredModel: "deepseek-v4-flash",
	})

	if !strings.Contains(got, now.Format("15:04:05")) {
		t.Fatalf("expected last request timestamp in output:\n%s", got)
	}
	if !strings.Contains(got, "2,000") || !strings.Contains(got, "1,500") {
		t.Fatalf("expected last request token counts in output:\n%s", got)
	}
}

func TestFormatCommaInt(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{100, "100"},
		{1000, "1,000"},
		{1000000, "1,000,000"},
		{123456789, "123,456,789"},
		{-100, "-100"},
	}
	for _, c := range cases {
		got := formatCommaInt(c.n)
		if got != c.want {
			t.Fatalf("formatCommaInt(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
