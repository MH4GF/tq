package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use:   "action",
	Short: "Create, list, and manage actions (units of work)",
}

var actionGetCmd = &cobra.Command{
	Use:   "get <ID>",
	Short: "Get an action by ID (JSON output)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("get action: %w", err)
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(actionToMap(*action))
	},
}

func init() {
	actionCmd.AddCommand(actionGetCmd)
}
