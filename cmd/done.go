package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/dispatch"
)

func init() {
	actionCmd.AddCommand(doneCmd)
}

var doneCmd = &cobra.Command{
	Use:   "done ACTION_ID [RESULT]",
	Short: "Mark action as done",
	Long: `Mark an action as done, optionally recording a result summary.
Can be called on any non-terminal action (pending or running).
Triggers on_done hooks defined in the prompt template.

RESULT is free-form text read by future workers to understand past work.
Structure it with these sections (omit any that don't apply):

  outcome:    What changed — concrete deliverables, not process steps
  decisions:  What was decided and why — include rejected alternatives
  artifacts:  PR numbers, file paths, commit SHAs, URLs
  remaining:  Unfinished work, known issues, follow-up needed

Do NOT describe process ("I ran grep, then read the file…").
Session logs already capture that.`,
	Example: `  tq action done 5

  tq action done 5 'outcome: Added JWT auth middleware
  decisions: Tried Redis session store, reverted due to p99 latency (+40ms); fell back to signed cookies
  artifacts: PR #142, auth/middleware.go, auth/token.go
  remaining: API docs not yet updated for new token format'`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}

		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("action #%d not found (see: tq action list): %w", id, err)
		}

		result := ""
		if len(args) > 1 {
			result = args[1]
		}

		if err := database.MarkDone(id, result); err != nil {
			return fmt.Errorf("mark done: %w", err)
		}

		if task, err := database.GetTask(action.TaskID); err != nil {
			slog.Warn("failed to get task for work_dir sync", "task_id", action.TaskID, "error", err)
		} else if pwd, err := os.Getwd(); err != nil {
			slog.Warn("failed to get working directory", "error", err)
		} else if task.WorkDir != pwd {
			if err := database.UpdateTaskWorkDir(task.ID, pwd); err != nil {
				slog.Warn("failed to update task work_dir", "task_id", task.ID, "error", err)
			} else {
				slog.Info("updated task work_dir", "task_id", task.ID, "work_dir", pwd)
			}
		}

		promptsDir := resolvePromptsDir()
		if err := dispatch.TriggerOnDone(database, promptsDir, action, result); err != nil {
			slog.Warn("on_done trigger failed", "action_id", id, "error", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d done\n", id)
		return nil
	},
}
