package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
)

var (
	listStatus string
	listTask   int64
	listLimit  int
	listJQ     string
)

var listFields = []string{"id", "title", "task_id", "metadata", "status", "result", "session_id", "dispatch_after", "started_at", "completed_at", "created_at"}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List actions",
	Long:  `List actions, optionally filtered by status and/or task ID. Output is JSON.`,
	Example: `  tq action list
  tq action list --status pending
  tq action list --task 3
  tq action list --task 1 --status running`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var taskIDPtr *int64
		if listTask > 0 {
			taskIDPtr = &listTask
		}

		actions, err := database.ListActions(listStatus, taskIDPtr, listLimit)
		if err != nil {
			return fmt.Errorf("list actions: %w", err)
		}

		rows := make([]map[string]any, len(actions))
		for i, a := range actions {
			rows[i] = actionToMap(a)
		}
		return WriteJSON(cmd.OutOrStdout(), rows, listJQ, listFields)
	},
}

func actionToMap(a db.Action) map[string]any {
	row := map[string]any{
		"id":         a.ID,
		"title":      a.Title,
		"task_id":    a.TaskID,
		"metadata":   a.Metadata,
		"status":     a.Status,
		"created_at": db.FormatLocal(a.CreatedAt),
	}
	if a.Result.Valid {
		row["result"] = a.Result.String
	} else {
		row["result"] = nil
	}
	if a.SessionID.Valid {
		row["session_id"] = a.SessionID.String
	} else {
		row["session_id"] = nil
	}
	if a.StartedAt.Valid {
		row["started_at"] = db.FormatLocal(a.StartedAt.String)
	} else {
		row["started_at"] = nil
	}
	if a.DispatchAfter.Valid {
		row["dispatch_after"] = db.FormatLocal(a.DispatchAfter.String)
	} else {
		row["dispatch_after"] = nil
	}
	if a.CompletedAt.Valid {
		row["completed_at"] = db.FormatLocal(a.CompletedAt.String)
	} else {
		row["completed_at"] = nil
	}
	return row
}

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (pending, running, dispatched, done, failed, cancelled)")
	listCmd.Flags().Int64Var(&listTask, "task", 0, "Filter by task ID (see: tq task list)")
	listCmd.Flags().IntVar(&listLimit, "limit", 0, "Limit number of results (0 = no limit)")
	listCmd.Flags().StringVar(&listJQ, "jq", "", jqFlagUsage(listFields))
	actionCmd.AddCommand(listCmd)
}
