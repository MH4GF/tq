package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <keyword>",
	Short: "Search tasks and actions",
	Long:  `Full-text search across task titles, task metadata, action titles, action results, and action metadata. Output is JSON.`,
	Example: `  tq search "login bug"
  tq search deploy`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := database.Search(args[0])
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		if len(results) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "[]")
			return nil
		}

		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	},
}
