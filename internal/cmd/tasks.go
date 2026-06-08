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

var tasksCmd = &cobra.Command{
	Use:   "tasks [feature-name]",
	Short: "Break a feature into atomic tasks using local model",
	Long: `Generates tasks.md by first producing a draft, then asking clarifying
questions interactively. Answer the questions to improve the final tasks.`,
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
		fmt.Fprintln(os.Stderr, "Generating draft tasks with", svc.Model(), "...")
		draft, usage, err := svc.TasksDraft(context.Background(), ctx.Root, feature, func(chunk string) {
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
				fmt.Fprintln(os.Stderr, "Generating final tasks with answers...")
				usage2, err2 := svc.TasksFinalize(context.Background(), ctx.Root, feature, draft, answers, func(chunk string) {
					fmt.Fprint(os.Stdout, chunk)
				})
				fmt.Fprintln(os.Stdout)
				if err2 != nil {
					return err2
				}
				usage.PromptTokens += usage2.PromptTokens
				usage.CompletionTokens += usage2.CompletionTokens
				printOK(fmt.Sprintf("wrote .specs/features/%s/tasks.md (%d+%d tokens)", feature, usage.PromptTokens, usage.CompletionTokens))
				return nil
			}
		}

		// No clarifying questions: write draft directly as final
		tasksPath := filepath.Join(ctx.Root, ".specs/features", feature, "tasks.md")
		content := templates.Tasks(draft)
		if err := os.WriteFile(tasksPath, []byte(content), 0o644); err != nil {
			return err
		}
		statePath := filepath.Join(ctx.Root, ".specs/project/STATE.md")
		_ = state.UpdateCurrentWork(statePath, feature+" — tasks generated")
		printOK(fmt.Sprintf("wrote %s (%d+%d tokens)", tasksPath, usage.PromptTokens, usage.CompletionTokens))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tasksCmd)
}
