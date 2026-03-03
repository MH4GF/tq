package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/template"
)

// TriggerOnDone creates a follow-up action if the completed action's template has on_done configured.
func TriggerOnDone(database *db.DB, templatesDir string, action *db.Action, result string) error {
	tmpl, err := template.Load(templatesDir, action.TemplateID)
	if err != nil {
		return fmt.Errorf("load source template %q: %w", action.TemplateID, err)
	}

	if tmpl.Config.OnDone == "" {
		return nil
	}

	if !action.TaskID.Valid {
		slog.Warn("on_done skipped: action has no task_id", "action_id", action.ID, "on_done", tmpl.Config.OnDone)
		return nil
	}

	taskID := action.TaskID.Int64
	onDoneTemplateID := tmpl.Config.OnDone

	has, err := database.HasActiveAction(taskID, onDoneTemplateID)
	if err != nil {
		return fmt.Errorf("check duplicate: %w", err)
	}
	if has {
		slog.Info("on_done skipped: active action exists", "action_id", action.ID, "on_done", onDoneTemplateID)
		return nil
	}

	targetTmpl, err := template.Load(templatesDir, onDoneTemplateID)
	if err != nil {
		slog.Warn("on_done template not found", "template", onDoneTemplateID)
		return fmt.Errorf("load target template %q: %w", onDoneTemplateID, err)
	}

	status := "waiting_human"
	if targetTmpl.Config.Auto {
		status = "pending"
	}

	meta := map[string]any{
		"triggered_by_action_id": action.ID,
		"predecessor_result":     result,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = database.InsertAction(onDoneTemplateID, &taskID, string(metaJSON), status, 0, "on_done")
	if err != nil {
		return fmt.Errorf("insert on_done action: %w", err)
	}

	slog.Info("on_done triggered", "action_id", action.ID, "on_done", onDoneTemplateID, "status", status)
	return nil
}
