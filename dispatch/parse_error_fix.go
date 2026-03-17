package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
)

const parseErrorFixPromptID = "internal:fix-deprecated-patterns"

func CreateParseErrorFixAction(database db.Store, promptsDir string, promptID string, deprecatedPatterns []string) {
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

	has, err := database.HasActiveActionWithMeta(taskID, parseErrorFixPromptID, "prompt_id", promptID)
	if err != nil {
		slog.Error("check active parse-error-fix action", "error", err)
		return
	}
	if has {
		slog.Info("parse-error-fix action already exists", "prompt_id", promptID)
		return
	}

	meta := map[string]any{
		"prompt_id":           promptID,
		"deprecated_patterns": deprecatedPatterns,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		slog.Error("marshal parse-error-fix metadata", "error", err)
		return
	}

	title := fmt.Sprintf("Fix deprecated patterns in %q", promptID)
	_, err = database.InsertAction(title, parseErrorFixPromptID, taskID, string(metaJSON), db.ActionStatusPending)
	if err != nil {
		slog.Error("create parse-error-fix action", "error", err)
		return
	}

	slog.Info("parse-error-fix action created", "prompt_id", promptID, "deprecated_patterns", deprecatedPatterns)
}
