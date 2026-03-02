package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MH4GF/tq/view"
	"github.com/spf13/cobra"
)

var (
	viewDate   string
	viewInject bool
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View tasks and actions as markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := view.Generate(database)
		if err != nil {
			return fmt.Errorf("generate view: %w", err)
		}

		if !viewInject {
			if content == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "no tasks")
			} else {
				fmt.Fprint(cmd.OutOrStdout(), content)
			}
			return nil
		}

		date := time.Now().Format("2006-01-02")
		if viewDate != "" {
			date = viewDate
		}

		worksRoot := filepath.Dir(tqDirResolved)
		dailyPath := filepath.Join(worksRoot, "daily", date+".md")

		if _, err := os.Stat(dailyPath); os.IsNotExist(err) {
			return fmt.Errorf("daily note not found: %s", dailyPath)
		}

		if err := view.Inject(dailyPath, content); err != nil {
			return fmt.Errorf("inject: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "injected into %s\n", dailyPath)
		return nil
	},
}

func init() {
	viewCmd.Flags().StringVar(&viewDate, "date", "", "Date (YYYY-MM-DD), defaults to today")
	viewCmd.Flags().BoolVar(&viewInject, "inject", false, "Inject into daily note file")
}
