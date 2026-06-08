package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/project"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/state"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/templates"
	"github.com/pedrobelmino/tui-sdd-open-api/internal/workflow"
	"github.com/spf13/cobra"
)

var designCmd = &cobra.Command{
	Use:   "design [feature-name]",
	Short: "Generate design.md for a feature using local model",
	Long: `Generates design.md by first producing a draft, then asking clarifying
questions interactively. Answer the questions to improve the final design.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := project.RequireRoot()
		if err != nil {
			return err
		}

		feature := args[0]
		svc := workflow.New()
		if !svc.Reachable(context.Background()) {
			return fmt.Errorf("ollama not reachable")
		}

		// Pass 1: generate draft with clarifying questions
		fmt.Fprintln(os.Stderr, "Generating draft design with", svc.Model(), "...")
		draft, usage, err := svc.DesignDraft(context.Background(), ctx.Root, feature, func(chunk string) {
			fmt.Fprint(os.Stdout, chunk)
		})
		fmt.Fprintln(os.Stdout)
		if err != nil {
			return err
		}

		// Check for clarifying questions
		questions := workflow.ExtractQuestions(draft)
		if len(questions) > 0 {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, workflow.FormatQuestionsForDisplay(questions))
			answers := promptMultiline("Your answers")
			if strings.TrimSpace(answers) != "" {
				fmt.Fprintln(os.Stderr, "Generating final design with answers...")
				usage2, err2 := svc.DesignFinalize(context.Background(), ctx.Root, feature, draft, answers, func(chunk string) {
					fmt.Fprint(os.Stdout, chunk)
				})
				fmt.Fprintln(os.Stdout)
				if err2 != nil {
					return err2
				}
				usage.PromptTokens += usage2.PromptTokens
			usage.CompletionTokens += usage2.CompletionTokens
				printOK(fmt.Sprintf("wrote .specs/features/%s/design.md (%d+%d tokens)", feature, usage.PromptTokens, usage.CompletionTokens))
				return nil
			}
		}

		// No clarifying questions: write draft directly as final
		designPath := filepath.Join(ctx.Root, ".specs/features", feature, "design.md")
		if err := os.WriteFile(designPath, []byte(templates.Design(feature, draft)), 0o644); err != nil {
			return err
		}
		statePath := filepath.Join(ctx.Root, ".specs/project/STATE.md")
		_ = state.UpdateCurrentWork(statePath, feature+" — design generated")
		printOK(fmt.Sprintf("wrote %s (%d+%d tokens)", designPath, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(designCmd)
}
