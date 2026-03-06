package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	listStatus string
	listTask   int64
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List actions",
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
			fmt.Fprintln(cmd.OutOrStdout(), "no actions found")
			return nil
		}

		rows := make([]map[string]any, len(actions))
		for i, a := range actions {
			row := map[string]any{
				"id":          a.ID,
				"prompt_id": a.PromptID,
				"metadata":    a.Metadata,
				"status":      a.Status,
				"source":      a.Source,
				"created_at":  a.CreatedAt,
			}
			if a.TaskID.Valid {
				row["task_id"] = a.TaskID.Int64
			} else {
				row["task_id"] = nil
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
				row["started_at"] = a.StartedAt.String
			} else {
				row["started_at"] = nil
			}
			if a.CompletedAt.Valid {
				row["completed_at"] = a.CompletedAt.String
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
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().Int64Var(&listTask, "task", 0, "Filter by task ID")
	actionCmd.AddCommand(listCmd)
}
