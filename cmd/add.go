package cmd

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/prompt"
)

const maxActionTitleLength = 100

var (
	addPrompt      string
	addInstruction string
	addTitle       string
	addTask        int64
	addMeta        string
	addStatus      string
	addForce       bool
)

var addCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an action",
	Long: `Create a new action linked to a task. Provide a prompt template and/or direct instruction.

  --prompt: specify a prompt template ID
  --instruction: provide a direct instruction (e.g., "/github-pr review this")

  At least one of --prompt or --instruction is required.
  When both are given, instruction is available in template as {{index .Action.Meta "instruction"}}.

Errors if an active action with the same prompt exists for the task. Use --force to override.

--meta passes JSON to the prompt template. For prompts that accept an "instruction" field,
write what a worker needs to start the task: goal, relevant context (files, decisions,
technical details from the current session), and constraints.

If @filepath content is attached in session context, embed the file path and content
in the instruction so the worker can skip file discovery.
If instruction cannot be determined from context, ask the user.`,
	Example: `  # With prompt template
  tq action create --prompt review-pr --task 1 --title "Review PR #42"

  # With direct instruction (no template needed)
  tq action create --task 1 --title "Review PR" --instruction "/github-pr review this"

  # Both (instruction available in template)
  tq action create --prompt implement --task 1 --title "Add auth" --instruction "Add JWT middleware"

  # With instruction metadata for implementation prompts:
  tq action create --prompt implement --task 2 --title "Add auth middleware" --meta '{"instruction":"Goal: Add JWT auth middleware"}'`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if addPrompt == "" && addInstruction == "" {
			return fmt.Errorf("at least one of --prompt or --instruction is required")
		}

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

		meta := addMeta
		if addInstruction != "" {
			merged, err := mergeInstruction(meta, addInstruction)
			if err != nil {
				return err
			}
			meta = merged
		}

		isInteractive := false
		if addPrompt != "" {
			if !addForce {
				existing, err := database.GetActiveAction(addTask, addPrompt)
				if err != nil {
					return fmt.Errorf("check duplicates: %w", err)
				}
				if existing != nil {
					return fmt.Errorf("blocked: active action exists for task %d prompt %s\n  → action #%d %q (status: %s, created: %s)\n  hint: cancel it first: tq action cancel %d\n  hint: or create a second action with --force (both will be dispatched)",
						addTask, addPrompt, existing.ID, existing.Title, existing.Status, existing.CreatedAt, existing.ID)
				}
			}

			promptsDir := resolvePromptsDir()
			lr, err := prompt.Load(promptsDir, addPrompt)
			if err != nil {
				return fmt.Errorf("load prompt: %w", err)
			}
			isInteractive = lr.Prompt.Config.IsInteractive()
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
				Metadata: meta,
				Status:   status,
			}
			promptData, err := dispatch.BuildPromptData(database, tempAction)
			if err != nil {
				return fmt.Errorf("build prompt data: %w", err)
			}
			if _, err := lr.Prompt.Render(promptData); err != nil {
				return fmt.Errorf("prompt %q requires metadata not provided in --meta: %w", addPrompt, err)
			}
		}

		id, err := database.InsertAction(addTitle, addPrompt, addTask, meta, status)
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "action #%d created (status: %s)\n", id, status)
		if status == db.ActionStatusPending {
			printQueueStatus(w, id, isInteractive)
		}
		return nil
	},
}

func mergeInstruction(metaJSON, instruction string) (string, error) {
	m := make(map[string]any)
	if metaJSON != "" && metaJSON != "{}" {
		if err := json.Unmarshal([]byte(metaJSON), &m); err != nil {
			return "", fmt.Errorf("parse metadata for instruction merge: %w", err)
		}
	}
	m["instruction"] = instruction
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(data), nil
}

func printQueueStatus(w io.Writer, actionID int64, isInteractive bool) {
	pendingCount := 0
	if counts, err := database.CountByStatus(); err != nil {
		slog.Error("count by status", "error", err)
	} else {
		pendingCount = counts[db.ActionStatusPending]
	}
	maxInteractive := dispatch.DefaultMaxInteractive
	workerRunning := false
	if mi, err := database.GetWorkerMaxInteractive(dispatch.DefaultStaleThreshold); err == nil {
		workerRunning = true
		maxInteractive = mi
	} else if !errors.Is(err, sql.ErrNoRows) {
		slog.Error("check worker status", "error", err)
	}

	if !workerRunning {
		_, _ = fmt.Fprintf(w, "  queue: %d pending — no worker detected\n", pendingCount)
		_, _ = fmt.Fprintf(w, "  [agent hint] ask the user to run 'tq ui', or run 'tq dispatch %d' to execute immediately\n", actionID)
		return
	}
	if !isInteractive {
		_, _ = fmt.Fprintf(w, "  queue: %d pending — worker running, will be dispatched automatically\n", pendingCount)
		return
	}
	runningInteractive, err := database.CountRunningInteractive()
	if err != nil {
		slog.Error("count running interactive", "error", err)
	}
	if runningInteractive >= maxInteractive {
		_, _ = fmt.Fprintf(w, "  queue: %d pending — worker running, but interactive slots full (%d/%d)\n",
			pendingCount, runningInteractive, maxInteractive)
		_, _ = fmt.Fprintf(w, "  [agent hint] ask the user before running 'tq dispatch %d' to execute immediately\n", actionID)
	} else {
		_, _ = fmt.Fprintf(w, "  queue: %d pending — worker running, will be dispatched automatically (interactive: %d/%d)\n",
			pendingCount, runningInteractive, maxInteractive)
	}
}

func init() {
	addCmd.Flags().StringVarP(&addPrompt, "prompt", "p", "", "Prompt template ID (see: tq prompt list)")
	addCmd.Flags().StringVarP(&addInstruction, "instruction", "i", "", "Direct instruction text")
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
