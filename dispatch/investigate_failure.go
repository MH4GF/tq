package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
)

const investigateFailurePromptID = "internal:investigate-failure"

// CreateInvestigateFailureAction creates a pending action to investigate why
// an action failed. It follows the same pattern as CreateSelfImprovementAction:
// automatically creating an investigation action on the same task, with
// duplicate prevention based on the failed action ID.
// To prevent infinite loops, investigation actions are not created for failures
// of investigation actions themselves.
func CreateInvestigateFailureAction(database db.Store, action *db.Action, failureResult string) {
	if action.PromptID == investigateFailurePromptID {
		slog.Info("skipping investigate-failure for investigation action itself", "action_id", action.ID)
		return
	}

	has, err := database.HasActiveActionWithMeta(action.TaskID, investigateFailurePromptID, "failed_action_id", fmt.Sprintf("%d", action.ID))
	if err != nil {
		slog.Error("check active investigate-failure action", "error", err)
		return
	}
	if has {
		slog.Info("investigate-failure action already exists", "failed_action_id", action.ID)
		return
	}

	failedActionID := fmt.Sprintf("%d", action.ID)
	meta := map[string]any{
		"failed_action_id": failedActionID,
		"failed_prompt_id": action.PromptID,
		"failure_result":   failureResult,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		slog.Error("marshal investigate-failure metadata", "error", err)
		return
	}

	title := fmt.Sprintf("Investigate failure of action #%d (%s)", action.ID, action.PromptID)
	_, err = database.InsertAction(title, investigateFailurePromptID, action.TaskID, string(metaJSON), db.ActionStatusPending)
	if err != nil {
		slog.Error("create investigate-failure action", "error", err)
		return
	}

	slog.Info("investigate-failure action created", "failed_action_id", action.ID, "prompt_id", action.PromptID)
}
