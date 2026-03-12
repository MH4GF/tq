package cmd

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MH4GF/tq/dispatch"
	"github.com/spf13/cobra"
)

func init() {
	actionCmd.AddCommand(cancelCmd)
}

var cancelCmd = &cobra.Command{
	Use:   "cancel ACTION_ID [REASON]",
	Short: "Cancel an action",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}

		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("action #%d not found: %w", id, err)
		}

		if action.Status == "done" || action.Status == "cancelled" {
			return fmt.Errorf("action #%d is %q, cannot cancel", id, action.Status)
		}

		if action.Status == "running" && action.TmuxPane.Valid {
			_ = exec.Command("tmux", "kill-window", "-t", fmt.Sprintf("main:tq-action-%d", id)).Run()
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

		fmt.Fprintf(cmd.OutOrStdout(), "action #%d cancelled\n", id)
		if reason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "reason: %s\n", strings.TrimSpace(reason))
		}
		return nil
	},
}
