package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	addPrompt string
	addTask   int64
	addMeta   string
	addSource string
	addStatus string
	addForce  bool
)

var addCmd = &cobra.Command{
	Use:   "create <PROMPT>",
	Short: "Create an action",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addPrompt = args[0]

		if addTask <= 0 {
			return fmt.Errorf("--task flag is required")
		}

		status := addStatus
		if status == "" {
			status = "pending"
		}

		taskIDPtr := &addTask

		if !addForce {
			dup, err := database.HasActiveAction(addTask, addPrompt)
			if err != nil {
				return fmt.Errorf("check duplicates: %w", err)
			}
			if dup {
				return fmt.Errorf("blocked: active action already exists for task %d prompt %s (use --force to override)", addTask, addPrompt)
			}
		}

		id, err := database.InsertAction(addPrompt, taskIDPtr, addMeta, status, addSource)
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "action #%d created (status: %s)\n", id, status)
		return nil
	},
}

func init() {
	addCmd.Flags().Int64Var(&addTask, "task", 0, "Task ID")
	addCmd.Flags().StringVar(&addMeta, "meta", "{}", "Metadata JSON")
	addCmd.Flags().StringVar(&addSource, "source", "human", "Source")
	addCmd.Flags().StringVar(&addStatus, "status", "", "Override status (pending|done|running|failed|waiting_human)")
	addCmd.Flags().BoolVar(&addForce, "force", false, "Skip duplicate check")
	actionCmd.AddCommand(addCmd)
}
