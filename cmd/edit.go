package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var editStatus string

var editCmd = &cobra.Command{
	Use:   "edit <action_id>",
	Short: "Edit action fields (e.g. status)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}
		if _, err := database.GetAction(id); err != nil {
			return fmt.Errorf("get action: %w", err)
		}
		if editStatus != "" {
			if err := database.UpdateActionStatus(id, editStatus); err != nil {
				return fmt.Errorf("update status: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "action #%d updated (status: %s)\n", id, editStatus)
		}
		return nil
	},
}

func init() {
	editCmd.Flags().StringVar(&editStatus, "status", "", "New status (pending|running|failed|waiting_human)")
	actionCmd.AddCommand(editCmd)
}
