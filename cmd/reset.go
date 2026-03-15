package cmd

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/MH4GF/tq/dispatch"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:     "reset <action_id>",
	Short:   "Reset a failed or running action to pending",
	Example: `  tq action reset 7`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}
		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("get action: %w", err)
		}
		if action.Status == "pending" || action.Status == "done" {
			return fmt.Errorf("action #%d is %q, cannot reset (only running, failed, or cancelled actions can be reset to pending)", id, action.Status)
		}
		if action.Status == "running" && action.TmuxPane.Valid {
			_ = exec.Command("tmux", "kill-window", "-t", "main:"+dispatch.WindowName(id)).Run()
		}
		if err := database.ResetToPending(id); err != nil {
			return fmt.Errorf("reset to pending: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "action #%d reset to pending\n", id)
		return nil
	},
}

func init() {
	actionCmd.AddCommand(resetCmd)
}
