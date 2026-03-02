package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset <action_id>",
	Short: "Reset a failed or waiting_human action to pending",
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
		if action.Status != "failed" && action.Status != "waiting_human" {
			return fmt.Errorf("action #%d is %q, only failed or waiting_human can be reset", id, action.Status)
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
