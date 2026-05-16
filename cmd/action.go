package cmd

import (
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
  tq action update 3 --meta '{"key":"value"}'
  tq action update 4 --work-dir /path/to/worktree
  tq action update 5 --work-dir ""
  tq action update 6 --result "outcome: recovered after false-positive failure"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}

		var title, meta, workDir, result *string
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
		if cmd.Flags().Changed("work-dir") {
			v, _ := cmd.Flags().GetString("work-dir")
			workDir = &v
		}
		if cmd.Flags().Changed("result") {
			v, _ := cmd.Flags().GetString("result")
			result = &v
		}

		if title == nil && taskID == nil && meta == nil && workDir == nil && result == nil {
			return fmt.Errorf("at least one flag (--title, --task, --meta, --work-dir, --result) is required")
		}

		if err := database.UpdateAction(id, title, taskID, meta, workDir, result); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d updated\n", id)
		return nil
	},
}

var actionGetJQ string

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
		return WriteJSON(cmd.OutOrStdout(), actionToMap(*action), actionGetJQ, listFields)
	},
}

func init() {
	actionUpdateCmd.Flags().String("title", "", "Action title")
	actionUpdateCmd.Flags().Int64("task", 0, "Task ID")
	actionUpdateCmd.Flags().String("meta", "", `JSON metadata for dispatch control (keys: mode, claude_args)`)
	actionUpdateCmd.Flags().String("work-dir", "", `Working directory override for this action (pass "" to clear)`)
	actionUpdateCmd.Flags().String("result", "", "Amend the recorded result (allowed on pending, failed, done, or cancelled actions)")

	actionGetCmd.Flags().StringVar(&actionGetJQ, "jq", "", jqFlagUsage(listFields))
	actionCmd.AddCommand(actionGetCmd)
	actionCmd.AddCommand(actionUpdateCmd)
}
