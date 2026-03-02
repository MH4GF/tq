package cmd

import (
	"fmt"
	"text/tabwriter"

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

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTemplate\tTask\tStatus\tPriority\tResult")
		for _, a := range actions {
			taskStr := "-"
			if a.TaskID.Valid {
				taskStr = fmt.Sprintf("%d", a.TaskID.Int64)
			}
			result := "-"
			if a.Result.Valid && a.Result.String != "" {
				result = a.Result.String
				if len(result) > 60 {
					result = result[:57] + "..."
				}
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\n",
				a.ID, a.TemplateID, taskStr, a.Status, a.Priority, result)
		}
		return w.Flush()
	},
}

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().Int64Var(&listTask, "task", 0, "Filter by task ID")
	actionCmd.AddCommand(listCmd)
}
