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

func CreateSelfImprovementAction(database db.Store, promptID string, unknownFields []string) {
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
