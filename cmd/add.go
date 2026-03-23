package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
)

const maxActionTitleLength = 100

var (
	addTitle  string
	addTask   int64
	addMeta   string
	addStatus string
)

var addCmd = &cobra.Command{
	Use:   "create <instruction>",
	Short: "Create an action",
	Long: `Create a new action linked to a task.

Instruction is provided as a positional argument.
--meta passes JSON metadata. The instruction is automatically merged into metadata.`,
	Example: `  tq action create "/github-pr review this" --task 1 --title "Review PR"

  tq action create "Add JWT auth middleware" --task 2 --title "Add auth middleware"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instruction := args[0]

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

		merged, err := mergeInstruction(addMeta, instruction)
		if err != nil {
			return err
		}

		id, err := database.InsertAction(addTitle, addTask, merged, status)
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "action #%d created (status: %s)\n", id, status)
		if status == db.ActionStatusPending {
			printQueueStatus(w, id)
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
	m[dispatch.MetaKeyInstruction] = instruction
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(data), nil
}

func printQueueStatus(w io.Writer, actionID int64) {
	pendingCount := 0
	if counts, err := database.CountByStatus(); err != nil {
		slog.Error("count by status", "error", err)
	} else {
		pendingCount = counts[db.ActionStatusPending]
	}
	workerRunning, err := database.IsWorkerRunning(dispatch.DefaultStaleThreshold)
	if err != nil {
		slog.Error("check worker status", "error", err)
	}

	if !workerRunning {
		_, _ = fmt.Fprintf(w, "  queue: %d pending — no worker detected\n", pendingCount)
		_, _ = fmt.Fprintf(w, "  [agent hint] ask the user to run 'tq ui', or run 'tq dispatch %d' to execute immediately\n", actionID)
		return
	}
	_, _ = fmt.Fprintf(w, "  queue: %d pending — worker running, will be dispatched automatically\n", pendingCount)
}

func init() {
	addCmd.Flags().StringVar(&addTitle, "title", "", fmt.Sprintf("Action title (required, max %d chars)", maxActionTitleLength))
	addCmd.Flags().Int64Var(&addTask, "task", 0, "Task ID (required, see: tq task list)")
	if err := addCmd.MarkFlagRequired("task"); err != nil {
		panic(err)
	}
	addCmd.Flags().StringVar(&addMeta, "meta", "{}", `JSON metadata (e.g. {"key":"value"})`)
	addCmd.Flags().StringVar(&addStatus, "status", "", "Initial status (default: pending)")
	actionCmd.AddCommand(addCmd)
}
