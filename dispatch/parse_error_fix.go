package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
)

const parseErrorFixPromptID = "internal:fix-deprecated-patterns"

func CreateParseErrorFixAction(database db.Store, promptsDir, promptID string, deprecatedPatterns []string) (bool, error) {
	projectID, err := database.EnsureProject(selfImprovementProjectName)
	if err != nil {
		return false, fmt.Errorf("ensure self-improvement project: %w", err)
	}

	taskID, err := database.EnsureTask(projectID, selfImprovementTaskTitle)
	if err != nil {
		return false, fmt.Errorf("ensure self-improvement task: %w", err)
	}

	if err := database.UpdateTaskWorkDir(taskID, promptsDir); err != nil {
		return false, fmt.Errorf("set self-improvement task work_dir: %w", err)
	}

	has, err := database.HasActiveActionWithMeta(taskID, parseErrorFixPromptID, "prompt_id", promptID)
	if err != nil {
		return false, fmt.Errorf("check active parse-error-fix action: %w", err)
	}
	if has {
		slog.Info("parse-error-fix action already exists", "prompt_id", promptID)
		return false, nil
	}

	meta := map[string]any{
		"prompt_id":           promptID,
		"deprecated_patterns": deprecatedPatterns,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return false, fmt.Errorf("marshal parse-error-fix metadata: %w", err)
	}

	title := fmt.Sprintf("Fix deprecated patterns in %q", promptID)
	id, err := database.InsertAction(title, parseErrorFixPromptID, taskID, string(metaJSON), db.ActionStatusPending)
	if err != nil {
		return false, fmt.Errorf("create parse-error-fix action: %w", err)
	}

	slog.Info("parse-error-fix action created", "action_id", id, "prompt_id", promptID, "deprecated_patterns", deprecatedPatterns)
	return true, nil
}
