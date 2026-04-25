package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
)

var resetCmd = &cobra.Command{
	Use:     "reset <action_id>",
	Short:   "Reset a failed or cancelled action to pending",
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
		if action.Status == db.ActionStatusPending || action.Status == db.ActionStatusDone {
			return fmt.Errorf("action #%d is %q, cannot reset (only failed or cancelled actions can be reset to pending)", id, action.Status)
		}
		if action.Status == db.ActionStatusRunning || action.Status == db.ActionStatusDispatched {
			return fmt.Errorf("action #%d is %q; reset would spawn a duplicate worker. Run 'tq action cancel %d' or 'tq action fail %d' first, then reset", id, action.Status, id, id)
		}
		if err := database.ResetToPending(id); err != nil {
			return fmt.Errorf("reset to pending: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d reset to pending\n", id)
		return nil
	},
}

func init() {
	actionCmd.AddCommand(resetCmd)
}
