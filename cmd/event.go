package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/MH4GF/tq/db"
	"github.com/spf13/cobra"
)

var (
	eventEntity string
	eventID     int64
	eventLimit  int
)

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Query the event log",
}

var eventListCmd = &cobra.Command{
	Use:   "list",
	Short: "List events",
	Long: `List events from the event log. Output is JSON.

Without filters, shows the most recent events (default 50).
Use --entity and --id together to filter events for a specific entity.`,
	Example: `  tq event list
  tq event list --limit 10
  tq event list --entity action --id 5
  tq event list --entity task --id 3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var events []db.Event
		var err error

		if eventEntity != "" && eventID > 0 {
			events, err = database.ListEvents(eventEntity, eventID)
		} else {
			events, err = database.ListRecentEvents(eventLimit)
		}
		if err != nil {
			return fmt.Errorf("list events: %w", err)
		}

		if len(events) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "[]")
			return nil
		}

		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	},
}

func init() {
	eventListCmd.Flags().StringVar(&eventEntity, "entity", "", "Filter by entity type (action, task, project, schedule)")

	eventListCmd.Flags().Int64Var(&eventID, "id", 0, "Filter by entity ID (requires --entity)")
	eventListCmd.Flags().IntVar(&eventLimit, "limit", 50, "Number of recent events to show")
	eventCmd.AddCommand(eventListCmd)
	rootCmd.AddCommand(eventCmd)
}
