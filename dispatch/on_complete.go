package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/prompt"
)

// triggerFollowUp creates a follow-up action if the given targetPromptID is non-empty.
func triggerFollowUp(database *db.DB, promptsDir string, action *db.Action, result, targetPromptID, predecessorStatus string) error {
	if targetPromptID == "" {
		return nil
	}

	if !action.TaskID.Valid {
		slog.Warn("follow-up skipped: action has no task_id", "action_id", action.ID, "target", targetPromptID)
		return nil
	}

	taskID := action.TaskID.Int64

	has, err := database.HasActiveAction(taskID, targetPromptID)
	if err != nil {
		return fmt.Errorf("check duplicate: %w", err)
	}
	if has {
		slog.Info("follow-up skipped: active action exists", "action_id", action.ID, "target", targetPromptID)
		return nil
	}

	if _, err := prompt.Load(promptsDir, targetPromptID); err != nil {
		slog.Warn("target prompt not found", "template", targetPromptID)
		return fmt.Errorf("load target prompt %q: %w", targetPromptID, err)
	}

	meta := map[string]any{
		"triggered_by_action_id": action.ID,
		"predecessor_result":     result,
		"predecessor_status":     predecessorStatus,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = database.InsertAction(targetPromptID, targetPromptID, &taskID, string(metaJSON), "pending")
	if err != nil {
		return fmt.Errorf("insert follow-up action: %w", err)
	}

	slog.Info("follow-up triggered", "action_id", action.ID, "target", targetPromptID, "predecessor_status", predecessorStatus)
	return nil
}

// TriggerOnDone creates a follow-up action if the completed action's prompt has on_done configured.
func TriggerOnDone(database *db.DB, promptsDir string, action *db.Action, result string) error {
	tmpl, err := prompt.Load(promptsDir, action.PromptID)
	if err != nil {
		return fmt.Errorf("load source prompt %q: %w", action.PromptID, err)
	}
	return triggerFollowUp(database, promptsDir, action, result, tmpl.Config.OnDone, "done")
}

// TriggerOnCancel creates a follow-up action if the cancelled action's prompt has on_cancel configured.
func TriggerOnCancel(database *db.DB, promptsDir string, action *db.Action, result string) error {
	tmpl, err := prompt.Load(promptsDir, action.PromptID)
	if err != nil {
		return fmt.Errorf("load source prompt %q: %w", action.PromptID, err)
	}
	return triggerFollowUp(database, promptsDir, action, result, tmpl.Config.OnCancel, "cancelled")
}
