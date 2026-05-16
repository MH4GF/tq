package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
)

// collectBlockedBy turns repeatable --blocked-by-action / --blocked-by-task
// flag values into dependency edges. Shared by `action create` and
// `action update`.
func collectBlockedBy(actionIDs, taskIDs []int64) []db.ActionDep {
	deps := make([]db.ActionDep, 0, len(actionIDs)+len(taskIDs))
	for _, id := range actionIDs {
		deps = append(deps, db.ActionDep{Type: db.DepTypeAction, ID: id})
	}
	for _, id := range taskIDs {
		deps = append(deps, db.ActionDep{Type: db.DepTypeTask, ID: id})
	}
	return deps
}

var actionCmd = &cobra.Command{
	Use:   "action",
	Short: "Create, list, and manage actions (units of work)",
}

var actionUpdateCmd = &cobra.Command{
	Use:   "update <ID>",
	Short: "Update an action",
	Long: `Update an action.

Structural fields (--title, --task, --meta, --work-dir) can only be changed
on pending or failed actions.

--result amends the recorded result and is also allowed on done or cancelled
actions (running/dispatched are in-flight; use 'tq action done'/'fail').

--blocked-by-action / --blocked-by-task (repeatable) append completion
dependencies: the action stays pending until every blocker reaches a
successful terminal state (action=done, task=done/archived). A blocker that
ends failed/cancelled blocks forever — rescue via the dependency-triage skill.
--clear-deps removes all dependencies first, so
'--clear-deps --blocked-by-action <id>' replaces the dependency set.`,
	Example: `  tq action update 1 --title "New title"
  tq action update 2 --task 5
  tq action update 3 --meta '{"key":"value"}'
  tq action update 4 --work-dir /path/to/worktree
  tq action update 5 --work-dir ""
  tq action update 6 --result "outcome: recovered after false-positive failure"
  tq action update 7 --blocked-by-action 12 --blocked-by-task 3
  tq action update 8 --clear-deps
  tq action update 9 --clear-deps --blocked-by-action 20`,
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

		clearDeps, _ := cmd.Flags().GetBool("clear-deps")
		actionDeps, _ := cmd.Flags().GetInt64Slice("blocked-by-action")
		taskDeps, _ := cmd.Flags().GetInt64Slice("blocked-by-task")
		deps := collectBlockedBy(actionDeps, taskDeps)
		depMutation := clearDeps || len(deps) > 0

		if title == nil && taskID == nil && meta == nil && workDir == nil && result == nil && !depMutation {
			return fmt.Errorf("at least one flag (--title, --task, --meta, --work-dir, --result, --blocked-by-action, --blocked-by-task, --clear-deps) is required")
		}

		if depMutation {
			a, err := database.GetAction(id)
			if err != nil {
				return fmt.Errorf("get action #%d: %w", id, err)
			}
			if a.Status != db.ActionStatusPending && a.Status != db.ActionStatusFailed {
				return fmt.Errorf("action #%d has status %q: dependencies can only be changed on pending or failed actions", id, a.Status)
			}
		}

		if title != nil || taskID != nil || meta != nil || workDir != nil || result != nil {
			if err := database.UpdateAction(id, title, taskID, meta, workDir, result); err != nil {
				return err
			}
		}
		if clearDeps {
			if err := database.ClearActionDependencies(id); err != nil {
				return err
			}
		}
		if len(deps) > 0 {
			if err := database.AddActionDependencies(id, deps); err != nil {
				return err
			}
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
		deps, err := database.ListActionDependencies(id)
		if err != nil {
			return fmt.Errorf("list action dependencies: %w", err)
		}
		return WriteJSON(cmd.OutOrStdout(), actionToMap(*action, deps), actionGetJQ, listFields)
	},
}

func init() {
	actionUpdateCmd.Flags().String("title", "", "Action title")
	actionUpdateCmd.Flags().Int64("task", 0, "Task ID")
	actionUpdateCmd.Flags().String("meta", "", `JSON metadata for dispatch control (keys: mode, claude_args)`)
	actionUpdateCmd.Flags().String("work-dir", "", `Working directory override for this action (pass "" to clear)`)
	actionUpdateCmd.Flags().String("result", "", "Amend the recorded result (allowed on pending, failed, done, or cancelled actions)")
	actionUpdateCmd.Flags().Int64Slice("blocked-by-action", nil, "Block until this action reaches done (repeatable; pending/failed actions only)")
	actionUpdateCmd.Flags().Int64Slice("blocked-by-task", nil, "Block until this task reaches done/archived (repeatable; pending/failed actions only)")
	actionUpdateCmd.Flags().Bool("clear-deps", false, "Remove all dependencies first (combine with --blocked-by-* to replace)")

	actionGetCmd.Flags().StringVar(&actionGetJQ, "jq", "", jqFlagUsage(listFields))
	actionCmd.AddCommand(actionGetCmd)
	actionCmd.AddCommand(actionUpdateCmd)
}
