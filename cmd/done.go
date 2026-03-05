package cmd

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/MH4GF/tq/dispatch"
	"github.com/spf13/cobra"
)

func init() {
	actionCmd.AddCommand(doneCmd)
}

var doneCmd = &cobra.Command{
	Use:   "done ACTION_ID [RESULT]",
	Short: "Mark action as done",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}

		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("action #%d not found: %w", id, err)
		}

		result := ""
		if len(args) > 1 {
			result = args[1]
		}

		if err := database.MarkDone(id, result); err != nil {
			return fmt.Errorf("mark done: %w", err)
		}

		projectWorkDir := getProjectWorkDir(action)
		templatesDir := resolveTemplatesDir(projectWorkDir)
		if err := dispatch.TriggerOnDone(database, templatesDir, action, result); err != nil {
			slog.Warn("on_done trigger failed", "action_id", id, "error", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "action #%d done\n", id)
		return nil
	},
}
