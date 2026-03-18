package cmd

import (
	"fmt"
	"unicode/utf8"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/prompt"
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
			status = db.ActionStatusPending
		}

		if err := validateMetaJSON(addMeta); err != nil {
			return err
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

		promptsDir := resolvePromptsDir()
		lr, err := prompt.Load(promptsDir, addPrompt)
		if err != nil {
			return fmt.Errorf("load prompt: %w", err)
		}
		if len(lr.DeprecatedPatterns) > 0 {
			created, ferr := dispatch.CreateParseErrorFixAction(database, promptsDir, addPrompt, lr.DeprecatedPatterns)
			if ferr != nil {
				return fmt.Errorf("prompt %q uses deprecated patterns: %v (failed to create fix action: %w)", addPrompt, lr.DeprecatedPatterns, ferr)
			}
			if created {
				return fmt.Errorf("prompt %q uses deprecated patterns: %v — a fix action has been created", addPrompt, lr.DeprecatedPatterns)
			}
			return fmt.Errorf("prompt %q uses deprecated patterns: %v", addPrompt, lr.DeprecatedPatterns)
		}
		tempAction := &db.Action{
			TaskID:   addTask,
			PromptID: addPrompt,
			Metadata: addMeta,
			Status:   status,
		}
		promptData, err := dispatch.BuildPromptData(database, tempAction)
		if err != nil {
			return fmt.Errorf("build prompt data: %w", err)
		}
		if _, err := lr.Prompt.Render(promptData); err != nil {
			return fmt.Errorf("prompt %q requires metadata not provided in --meta: %w", addPrompt, err)
		}

		id, err := database.InsertAction(addTitle, addPrompt, addTask, addMeta, status)
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d created (status: %s)\n", id, status)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addTitle, "title", "", fmt.Sprintf("Action title (required, max %d chars)", maxActionTitleLength))
	addCmd.Flags().Int64Var(&addTask, "task", 0, "Task ID (required, see: tq task list)")
	if err := addCmd.MarkFlagRequired("task"); err != nil {
		panic(err)
	}
	addCmd.Flags().StringVar(&addMeta, "meta", "{}", `JSON metadata (e.g. {"key":"value"})`)
	addCmd.Flags().StringVar(&addStatus, "status", "", "Initial status (default: pending)")
	addCmd.Flags().BoolVar(&addForce, "force", false, "Skip duplicate check")
	actionCmd.AddCommand(addCmd)
}
