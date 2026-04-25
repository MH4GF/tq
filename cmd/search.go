package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	searchJQ        string
	searchProjectID int64
	searchFields    = []string{"entity_type", "entity_id", "task_id", "project_id", "field", "snippet", "status", "created_at"}
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "Search tasks and actions",
	Long:  `Full-text search across task titles, task metadata, task status change reasons, action titles, action results, and action metadata. Output is JSON.`,
	Example: `  tq search "login bug"
  tq search deploy`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := database.Search(args[0], searchProjectID)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		return WriteJSON(cmd.OutOrStdout(), results, searchJQ, searchFields)
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchJQ, "jq", "", jqFlagUsage(searchFields))
	searchCmd.Flags().Int64Var(&searchProjectID, "project", 0, "Filter by project ID")
}
