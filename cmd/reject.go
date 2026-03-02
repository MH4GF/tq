package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var rejectCmd = &cobra.Command{
	Use:   "reject <action_id>",
	Short: "Reject a waiting_human action (mark as failed)",
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
		if action.Status != "waiting_human" {
			return fmt.Errorf("action #%d is %q, not waiting_human", id, action.Status)
		}
		if err := database.MarkFailed(id, "rejected by human"); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "action #%d rejected (failed)\n", id)
		return nil
	},
}

func init() {
	actionCmd.AddCommand(rejectCmd)
}
