package cmd

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
)

func init() {
	actionCmd.AddCommand(cancelCmd)
}

var cancelCmd = &cobra.Command{
	Use:   "cancel ACTION_ID [REASON]",
	Short: "Cancel a pending, running, or failed action",
	Long: `Cancel an action. Cannot cancel actions already done or cancelled.

REASON serves as feedback for improving classification logic. Before cancelling,
review the task's action history (tq action list --task <id>) to understand how
this action was created. Then record why it was unnecessary and how the
classification could be improved to avoid similar unnecessary actions.`,
	Example: `  tq action cancel 5
  tq action cancel 5 "Duplicate: review-pr #58 already running for same task.
  classify-next-action should check for active actions with same prompt before creating new ones."`,
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

		if action.Status == db.ActionStatusDone || action.Status == db.ActionStatusCancelled {
			return fmt.Errorf("action #%d is already %q, cannot cancel (only pending, running, or failed actions can be cancelled)", id, action.Status)
		}

		if action.Status == db.ActionStatusRunning && action.TmuxPane.Valid {
			_ = exec.Command("tmux", "kill-window", "-t", "main:"+dispatch.WindowName(id)).Run()
		}

		reason := ""
		if len(args) > 1 {
			reason = args[1]
		}

		if err := database.MarkCancelled(id, reason); err != nil {
			return fmt.Errorf("mark cancelled: %w", err)
		}

		promptsDir := resolvePromptsDir()
		if err := dispatch.TriggerOnCancel(database, promptsDir, action, reason); err != nil {
			slog.Warn("on_cancel trigger failed", "action_id", id, "error", err)
		}

		if reason != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d cancelled (reason: %s)\n", id, strings.TrimSpace(reason))
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d cancelled\n", id)
		}
		return nil
	},
}
