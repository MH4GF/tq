package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
)

var (
	listStatus string
	listTask   int64
	listLimit  int
)

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

		if len(actions) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "[]")
			return nil
		}

		rows := make([]map[string]any, len(actions))
		for i, a := range actions {
			rows[i] = actionToMap(a)
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	},
}

func actionToMap(a db.Action) map[string]any {
	row := map[string]any{
		"id":         a.ID,
		"title":      a.Title,
		"prompt_id":  a.PromptID,
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
	if a.CompletedAt.Valid {
		row["completed_at"] = db.FormatLocal(a.CompletedAt.String)
	} else {
		row["completed_at"] = nil
	}
	return row
}

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (pending, running, done, failed, cancelled)")
	listCmd.Flags().Int64Var(&listTask, "task", 0, "Filter by task ID (see: tq task list)")
	listCmd.Flags().IntVar(&listLimit, "limit", 0, "Limit number of results (0 = no limit)")
	actionCmd.AddCommand(listCmd)
}
