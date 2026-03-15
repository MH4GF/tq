package cmd

import "github.com/spf13/cobra"

var actionCmd = &cobra.Command{
	Use:   "action",
	Short: "Create, list, and manage actions (units of work)",
}
