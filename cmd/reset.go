package cmd

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset <action_id>",
	Short: "Reset a failed or running action to pending",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}
		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("get action: %w", err)
		}
		if action.Status != "failed" && action.Status != "running" {
			return fmt.Errorf("action #%d is %q, only failed or running can be reset", id, action.Status)
		}
		if action.Status == "running" && action.TmuxPane.Valid {
			_ = exec.Command("tmux", "kill-window", "-t", fmt.Sprintf("main:tq-action-%d", id)).Run()
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
