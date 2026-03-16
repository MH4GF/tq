package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
)

const selfImprovementProjectName = "tq-improvement"
const selfImprovementTaskTitle = "prompt maintenance"
const selfImprovementPromptID = "internal:remove-unknown-frontmatter"

func CreateSelfImprovementAction(database db.Store, promptsDir string, promptID string, unknownFields []string) {
	projectID, err := database.EnsureProject(selfImprovementProjectName)
	if err != nil {
		slog.Error("ensure self-improvement project", "error", err)
		return
	}

	taskID, err := database.EnsureTask(projectID, selfImprovementTaskTitle)
	if err != nil {
		slog.Error("ensure self-improvement task", "error", err)
		return
	}

	if err := database.UpdateTaskWorkDir(taskID, promptsDir); err != nil {
		slog.Error("set self-improvement task work_dir", "error", err)
		return
	}

	has, err := database.HasActiveActionWithMeta(taskID, selfImprovementPromptID, "prompt_id", promptID)
	if err != nil {
		slog.Error("check active self-improvement action", "error", err)
		return
	}
	if has {
		slog.Info("self-improvement action already exists", "prompt_id", promptID)
		return
	}

	meta := map[string]any{
		"prompt_id":      promptID,
		"unknown_fields": unknownFields,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		slog.Error("marshal self-improvement metadata", "error", err)
		return
	}

	title := fmt.Sprintf("Remove unknown frontmatter fields from %q", promptID)
	_, err = database.InsertAction(title, selfImprovementPromptID, taskID, string(metaJSON), db.ActionStatusPending)
	if err != nil {
		slog.Error("create self-improvement action", "error", err)
		return
	}

	slog.Info("self-improvement action created", "prompt_id", promptID, "unknown_fields", unknownFields)
}

const parseErrorFixPromptID = "internal:fix-parse-error"

// CreateParseErrorFixAction creates an action to fix a prompt parse error.
// It skips creation if the failing action itself is a fix-parse-error action (infinite loop prevention)
// or if an active fix action already exists for the same source action.
func CreateParseErrorFixAction(database db.Store, promptsDir string, actionID int64, promptID string, errorMessage string) {
	// Prevent infinite loop: don't create fix actions for fix actions
	if promptID == parseErrorFixPromptID {
		slog.Info("skipping parse error fix for fix-parse-error action", "action_id", actionID)
		return
	}

	projectID, err := database.EnsureProject(selfImprovementProjectName)
	if err != nil {
		slog.Error("ensure self-improvement project", "error", err)
		return
	}

	taskID, err := database.EnsureTask(projectID, selfImprovementTaskTitle)
	if err != nil {
		slog.Error("ensure self-improvement task", "error", err)
		return
	}

	if err := database.UpdateTaskWorkDir(taskID, promptsDir); err != nil {
		slog.Error("set self-improvement task work_dir", "error", err)
		return
	}

	// Deduplicate: skip if an active fix action already exists for this source action
	has, err := database.HasActiveActionWithMeta(taskID, parseErrorFixPromptID, "source_action_id", fmt.Sprintf("%d", actionID))
	if err != nil {
		slog.Error("check active parse-error-fix action", "error", err)
		return
	}
	if has {
		slog.Info("parse-error-fix action already exists", "source_action_id", actionID)
		return
	}

	meta := map[string]any{
		"source_action_id": fmt.Sprintf("%d", actionID),
		"prompt_id":        promptID,
		"error_message":    errorMessage,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		slog.Error("marshal parse-error-fix metadata", "error", err)
		return
	}

	title := fmt.Sprintf("Fix parse error in prompt %q (action #%d)", promptID, actionID)
	_, err = database.InsertAction(title, parseErrorFixPromptID, taskID, string(metaJSON), db.ActionStatusPending)
	if err != nil {
		slog.Error("create parse-error-fix action", "error", err)
		return
	}

	slog.Info("parse-error-fix action created", "source_action_id", actionID, "prompt_id", promptID)
}
