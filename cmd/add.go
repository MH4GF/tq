package cmd

import (
	"fmt"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

const maxActionTitleLength = 100

var (
	addPrompt string
	addTitle  string
	addTask   int64
	addMeta   string
	addStatus string
	addForce  bool
)

var addCmd = &cobra.Command{
	Use:   "create <PROMPT>",
	Short: "Create an action",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addPrompt = args[0]

		if addTitle == "" {
			return fmt.Errorf("--title is required: set a concise, descriptive title (max %d chars) that explains what this action does", maxActionTitleLength)
		}
		if utf8.RuneCountInString(addTitle) > maxActionTitleLength {
			return fmt.Errorf("--title must be %d characters or less (got %d)", maxActionTitleLength, utf8.RuneCountInString(addTitle))
		}

		if addTask <= 0 {
			return fmt.Errorf("--task must be a positive integer")
		}

		status := addStatus
		if status == "" {
			status = "pending"
		}

		if !addForce {
			dup, err := database.HasActiveAction(addTask, addPrompt)
			if err != nil {
				return fmt.Errorf("check duplicates: %w", err)
			}
			if dup {
				return fmt.Errorf("blocked: active action already exists for task %d prompt %s (use --force to override)", addTask, addPrompt)
			}
		}

		id, err := database.InsertAction(addTitle, addPrompt, addTask, addMeta, status)
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "action #%d created (status: %s)\n", id, status)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addTitle, "title", "", fmt.Sprintf("Concise title describing the action (max %d chars)", maxActionTitleLength))
	addCmd.Flags().Int64Var(&addTask, "task", 0, "Task ID (required)")
	addCmd.MarkFlagRequired("task")
	addCmd.Flags().StringVar(&addMeta, "meta", "{}", "Metadata JSON")
	addCmd.Flags().StringVar(&addStatus, "status", "", "Override status (pending|running|done|failed|cancelled)")
	addCmd.Flags().BoolVar(&addForce, "force", false, "Skip duplicate check")
	actionCmd.AddCommand(addCmd)
}
