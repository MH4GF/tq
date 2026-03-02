package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show queue summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		counts, err := database.CountByStatus()
		if err != nil {
			return fmt.Errorf("count actions: %w", err)
		}

		total := 0
		for _, v := range counts {
			total += v
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Queue Status:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  total:         %d\n", total)
		for _, s := range []string{"pending", "running", "done", "failed", "waiting_human"} {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-14s %d\n", s+":", counts[s])
		}
		return nil
	},
}
