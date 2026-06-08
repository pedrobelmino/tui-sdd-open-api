package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/openapi"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/gpu"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/ollama"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/project"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check Ollama, GPU, model, and .specs/ health",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadCfg()
		ok := true

		fmt.Fprintln(os.Stderr, "tsll doctor")
		fmt.Fprintln(os.Stderr, stringsRepeat("─", 40))

		if cfg.Provider == "openapi" {
			key := cfg.APIKey
			if key == "" {
				key = os.Getenv("TSLL_API_KEY")
			}
			if key == "" {
				fmt.Fprintf(os.Stderr, "✗ Open API provider selected but no API key set (TSLL_API_KEY env or config)\n")
				ok = false
			} else {
				dc := openapi.NewClient(key, cfg.APIHost, cfg.Model)
				if dc.Reachable(context.Background()) {
					printOK("Open API reachable")
					printOK("model " + cfg.Model + " configured")
				} else {
					fmt.Fprintf(os.Stderr, "✗ Open API unreachable — check key: %s…\n", safeTruncate(key, 8))
					ok = false
				}
			}
		} else {
			client := ollama.NewClient(cfg.OllamaHost)
			if client.Reachable(context.Background()) {
				printOK("Ollama reachable at " + cfg.OllamaHost)
				snap := ollama.FetchSnapshot(context.Background(), client, cfg.Model)
				if snap.ModelMissing {
					fmt.Fprintf(os.Stderr, "✗ model %s not pulled — run: ollama pull %s\n", cfg.Model, cfg.Model)
					ok = false
				} else {
					printOK("model " + cfg.Model + " available")
				}
			} else {
				fmt.Fprintf(os.Stderr, "✗ Ollama unreachable at %s\n", cfg.OllamaHost)
				ok = false
			}
		}

		gpuSnap, _ := gpu.Query(context.Background())
		if gpuSnap.Available && len(gpuSnap.Devices) > 0 {
			d := gpuSnap.Devices[0]
			printOK(fmt.Sprintf("GPU [%s] %s — util %.0f%% VRAM %d/%d MiB",
				gpuSnap.Vendor, d.Name, d.Utilization, d.MemoryUsed, d.MemoryTotal))
		} else {
			fmt.Fprintf(os.Stderr, "⚠ GPU metrics unavailable (%s)\n", gpuSnap.Error)
		}

		if ctx, err := project.RequireRoot(); err == nil {
			printOK(".specs/project at " + ctx.Root)
		} else {
			fmt.Fprintln(os.Stderr, "⚠ not inside a tsll project (tsll init)")
		}

		fmt.Fprintf(os.Stderr, "\nConfig: model=%s gpu_prefer=%s\n", cfg.Model, cfg.GPUPrefer)
		if !ok {
			return fmt.Errorf("doctor found issues")
		}
		return nil
	},
}

func stringsRepeat(s string, n int) string {
	r := ""
	for i := 0; i < n; i++ {
		r += s
	}
	return r
}

func safeTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
