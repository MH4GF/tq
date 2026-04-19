package cmd

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
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
	addAfter  string
)

var addCmd = &cobra.Command{
	Use:   "create <instruction>",
	Short: "Create an action",
	Long: `Create a new action linked to a task.

Instruction is provided as a positional argument.
--meta passes JSON metadata. The instruction is automatically merged into metadata.

Metadata keys for dispatch control:
  mode         Execution mode: "interactive" (default), "noninteractive", "remote"
  claude_args  Additional CLI arguments for claude (JSON array of strings,
               e.g. ["--permission-mode","plan","--worktree","--max-turns","5"])`,
	Example: `  tq action create "/github-pr review this" --task 1 --title "Review PR"
  tq action create "Add JWT auth middleware" --task 2 --title "Add auth middleware"
  tq action create "/review" --task 3 --title "Plan review" --meta '{"claude_args":["--permission-mode","plan","--worktree"]}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instruction := args[0]

		if strings.TrimSpace(instruction) == "" {
			return fmt.Errorf("instruction must not be empty")
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

		var dispatchAfter *string
		if addAfter != "" {
			t, err := time.ParseInLocation("2006-01-02 15:04", addAfter, time.Local)
			if err != nil {
				return fmt.Errorf("invalid --after format (expected YYYY-MM-DD HH:MM): %w", err)
			}
			if !t.After(time.Now()) {
				return fmt.Errorf("--after must be in the future (got %s)", addAfter)
			}
			s := t.UTC().Format(db.TimeLayout)
			dispatchAfter = &s
		}

		merged, err := mergeInstruction(addMeta, instruction)
		if err != nil {
			return err
		}

		id, err := database.InsertAction(addTitle, addTask, merged, status, dispatchAfter)
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		w := cmd.OutOrStdout()
		if dispatchAfter != nil {
			_, _ = fmt.Fprintf(w, "action #%d created (status: %s, dispatch after: %s)\n", id, status, db.FormatLocal(*dispatchAfter))
		} else {
			_, _ = fmt.Fprintf(w, "action #%d created (status: %s)\n", id, status)
		}
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
	pc, err := database.CountPendingByDispatch()
	if err != nil {
		slog.Error("count pending by dispatch", "error", err)
	}
	pendingLabel := pc.Label()

	dispatchEnabled, err := database.IsActionDispatchEnabled(actionID)
	if err != nil {
		slog.Error("check action dispatch enabled", "error", err)
	}
	if err == nil && !dispatchEnabled {
		_, _ = fmt.Fprintf(w, "  queue: %s\n", pendingLabel)
		_, _ = fmt.Fprintf(w, "  [agent hint] project is unfocused — action will not be auto-dispatched. run 'tq action dispatch %d' to execute manually?\n", actionID)
		return
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
		_, _ = fmt.Fprintf(w, "  queue: %s — no worker detected\n", pendingLabel)
		_, _ = fmt.Fprintf(w, "  [agent hint] ask the user to run 'tq ui', or run 'tq action dispatch %d' to execute immediately\n", actionID)
		return
	}
	runningInteractive, err := database.CountRunningInteractive()
	if err != nil {
		slog.Error("count running interactive", "error", err)
	}
	if runningInteractive >= maxInteractive {
		_, _ = fmt.Fprintf(w, "  queue: %s — worker running, but interactive slots full (%d/%d)\n",
			pendingLabel, runningInteractive, maxInteractive)
		_, _ = fmt.Fprintf(w, "  [agent hint] ask the user before running 'tq action dispatch %d' to execute immediately\n", actionID)
	} else {
		_, _ = fmt.Fprintf(w, "  queue: %s — worker running, will be dispatched automatically (interactive: %d/%d)\n",
			pendingLabel, runningInteractive, maxInteractive)
	}
}

func init() {
	addCmd.Flags().StringVar(&addTitle, "title", "", fmt.Sprintf("Action title (required, max %d chars)", maxActionTitleLength))
	addCmd.Flags().Int64Var(&addTask, "task", 0, "Task ID (required, see: tq task list)")
	if err := addCmd.MarkFlagRequired("task"); err != nil {
		panic(err)
	}
	addCmd.Flags().StringVar(&addMeta, "meta", "{}", `JSON metadata for dispatch control (keys: mode, claude_args)`)
	addCmd.Flags().StringVar(&addStatus, "status", "", "Initial status (default: pending)")
	addCmd.Flags().StringVar(&addAfter, "after", "", "Dispatch after this time (YYYY-MM-DD HH:MM, local timezone)")
	actionCmd.AddCommand(addCmd)
}
