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
	Long: `Create a new action linked to a task and prompt template.

PROMPT is the prompt template ID. Run "tq prompt list" first to see available prompts
and choose the appropriate one for the task.

Errors if an active action with the same prompt exists for the task. Use --force to override.

--meta passes JSON to the prompt template. For prompts that accept an "instruction" field,
write what a worker needs to start the task: goal, relevant context (files, decisions,
technical details from the current session), and constraints.

If @filepath content is attached in session context, embed the file path and content
in the instruction so the worker can skip file discovery.
If instruction cannot be determined from context, ask the user.`,
	Example: `  # Check available prompts, then create:
  tq prompt list
  tq action create review-pr --task 1 --title "Review PR #42"

  # With instruction metadata for implementation prompts:
  tq action create implement --task 2 --title "Add auth middleware" --meta '{"instruction":"Goal: Add JWT auth middleware\nContext: auth/ has existing helpers, routes defined in cmd/server.go\nConstraints: Apply per-route, not globally"}'`,
	Args: cobra.ExactArgs(1),
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
	addCmd.Flags().StringVar(&addTitle, "title", "", fmt.Sprintf("Action title (required, max %d chars)", maxActionTitleLength))
	addCmd.Flags().Int64Var(&addTask, "task", 0, "Task ID (required, see: tq task list)")
	addCmd.MarkFlagRequired("task")
	addCmd.Flags().StringVar(&addMeta, "meta", "{}", `JSON metadata (e.g. {"key":"value"})`)
	addCmd.Flags().StringVar(&addStatus, "status", "", "Initial status (default: pending)")
	addCmd.Flags().BoolVar(&addForce, "force", false, "Skip duplicate check")
	actionCmd.AddCommand(addCmd)
}
