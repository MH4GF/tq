package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/prompt"
)

// TriggerOnDone creates a follow-up action if the completed action's template has on_done configured.
func TriggerOnDone(database *db.DB, promptsDir string, action *db.Action, result string) error {
	tmpl, err := prompt.Load(promptsDir, action.PromptID)
	if err != nil {
		return fmt.Errorf("load source prompt %q: %w", action.PromptID, err)
	}

	if tmpl.Config.OnDone == "" {
		return nil
	}

	if !action.TaskID.Valid {
		slog.Warn("on_done skipped: action has no task_id", "action_id", action.ID, "on_done", tmpl.Config.OnDone)
		return nil
	}

	taskID := action.TaskID.Int64
	onDonePromptID := tmpl.Config.OnDone

	has, err := database.HasActiveAction(taskID, onDonePromptID)
	if err != nil {
		return fmt.Errorf("check duplicate: %w", err)
	}
	if has {
		slog.Info("on_done skipped: active action exists", "action_id", action.ID, "on_done", onDonePromptID)
		return nil
	}

	if _, err := prompt.Load(promptsDir, onDonePromptID); err != nil {
		slog.Warn("on_done prompt not found", "template", onDonePromptID)
		return fmt.Errorf("load target prompt %q: %w", onDonePromptID, err)
	}

	status := "pending"

	meta := map[string]any{
		"triggered_by_action_id": action.ID,
		"predecessor_result":     result,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = database.InsertAction(onDonePromptID, &taskID, string(metaJSON), status, "on_done")
	if err != nil {
		return fmt.Errorf("insert on_done action: %w", err)
	}

	slog.Info("on_done triggered", "action_id", action.ID, "on_done", onDonePromptID, "status", status)
	return nil
}
