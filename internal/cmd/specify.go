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

var specifyBrief string

var specifyCmd = &cobra.Command{
	Use:   "specify [feature-name]",
	Short: "Generate spec.md for a feature using local model",
	Long: `Generates spec.md by first producing a draft, then asking clarifying
questions interactively. Answer the questions to improve the final spec.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := project.RequireRoot()
		if err != nil {
			return err
		}

		feature := args[0]

		brief := specifyBrief
		if brief == "" {
			brief = promptMultiline("Describe the feature")
		}
		if brief == "" {
			return fmt.Errorf("feature description required")
		}

		svc := workflow.New()
		if !svc.Reachable(context.Background()) {
			return fmt.Errorf("ollama not reachable")
		}

		// Pass 1: generate draft with clarifying questions
		fmt.Fprintln(os.Stderr, "Generating draft spec with", svc.Model(), "...")
		draft, usage, err := svc.SpecifyDraft(context.Background(), ctx.Root, feature, brief, func(chunk string) {
			fmt.Fprint(os.Stdout, chunk)
		})
		fmt.Fprintln(os.Stdout)
		if err != nil {
			return err
		}

		// Check for clarifying questions — ask user if found
		questions := workflow.ExtractQuestions(draft)
		if len(questions) > 0 {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, workflow.FormatQuestionsForDisplay(questions))
			answers := promptMultiline("Your answers")
			if strings.TrimSpace(answers) != "" {
				fmt.Fprintln(os.Stderr, "Generating final spec with answers...")
				usage2, err2 := svc.SpecifyFinalize(context.Background(), ctx.Root, feature, brief, draft, answers, func(chunk string) {
					fmt.Fprint(os.Stdout, chunk)
				})
				fmt.Fprintln(os.Stdout)
				if err2 != nil {
					return err2
				}
				usage.PromptTokens += usage2.PromptTokens
				usage.CompletionTokens += usage2.CompletionTokens
				printOK(fmt.Sprintf("wrote .specs/features/%s/spec.md (%d+%d tokens)", feature, usage.PromptTokens, usage.CompletionTokens))
				return nil
			}
		}

		// No clarifying questions (or user skipped): write draft directly as final
		featureDir := filepath.Join(ctx.Root, ".specs/features", feature)
		if err := os.MkdirAll(featureDir, 0o755); err != nil {
			return err
		}
		specPath := filepath.Join(featureDir, "spec.md")
		if err := os.WriteFile(specPath, []byte(templates.Spec(feature, draft)), 0o644); err != nil {
			return err
		}
		statePath := filepath.Join(ctx.Root, ".specs/project/STATE.md")
		_ = state.UpdateCurrentWork(statePath, feature+" — spec generated")
		printOK(fmt.Sprintf("wrote %s (%d+%d tokens)", specPath, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	specifyCmd.Flags().StringVar(&specifyBrief, "brief", "", "feature description (skip prompt)")
	rootCmd.AddCommand(specifyCmd)
}
