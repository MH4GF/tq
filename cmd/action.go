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

var actionUpdateCmd = &cobra.Command{
	Use:   "update <ID>",
	Short: "Update an action",
	Example: `  tq action update 1 --title "New title"
  tq action update 2 --task 5
  tq action update 3 --meta '{"key":"value"}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}

		var title, meta *string
		var taskID *int64

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			title = &v
		}
		if cmd.Flags().Changed("task") {
			v, _ := cmd.Flags().GetInt64("task")
			taskID = &v
		}
		if cmd.Flags().Changed("meta") {
			v, _ := cmd.Flags().GetString("meta")
			if err := validateMetaJSON(v); err != nil {
				return err
			}
			meta = &v
		}

		if title == nil && taskID == nil && meta == nil {
			return fmt.Errorf("at least one flag (--title, --task, --meta) is required")
		}

		if err := database.UpdateAction(id, title, taskID, meta); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d updated\n", id)
		return nil
	},
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
	actionUpdateCmd.Flags().String("title", "", "Action title")
	actionUpdateCmd.Flags().Int64("task", 0, "Task ID")
	actionUpdateCmd.Flags().String("meta", "", `JSON metadata for dispatch control (keys: mode, permission_mode, worktree)`)

	actionCmd.AddCommand(actionGetCmd)
	actionCmd.AddCommand(actionUpdateCmd)
}
