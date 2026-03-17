package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/MH4GF/tq/db"
	"github.com/spf13/cobra"
)

var (
	listStatus string
	listTask   int64
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List actions",
	Long: `List actions, optionally filtered by status and/or task ID. Output is JSON.`,
	Example: `  tq action list
  tq action list --status pending
  tq action list --task 3
  tq action list --task 1 --status running`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var taskIDPtr *int64
		if listTask > 0 {
			taskIDPtr = &listTask
		}

		actions, err := database.ListActions(listStatus, taskIDPtr)
		if err != nil {
			return fmt.Errorf("list actions: %w", err)
		}

		if len(actions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
			return nil
		}

		rows := make([]map[string]any, len(actions))
		for i, a := range actions {
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

			rows[i] = row
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	},
}

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (pending, running, done, failed, cancelled)")
	listCmd.Flags().Int64Var(&listTask, "task", 0, "Filter by task ID (see: tq task list)")
	actionCmd.AddCommand(listCmd)
}
