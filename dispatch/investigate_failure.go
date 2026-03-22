package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MH4GF/tq/db"
)

// CreateInvestigateFailureAction creates a pending action to investigate why
// an action failed. It uses instruction-only mode with metadata for dedup.
// To prevent infinite loops, investigation actions are not created for failures
// of investigation actions themselves.
func CreateInvestigateFailureAction(database db.Store, action *db.Action, failureResult string) {
	meta, _ := parseMetadata(action.Metadata)
	if _, ok := meta[MetaKeyIsInvestigation]; ok {
		slog.Info("skipping investigate-failure for investigation action itself", "action_id", action.ID)
		return
	}

	failedActionID := fmt.Sprintf("%d", action.ID)
	has, err := database.HasActiveActionWithMeta(action.TaskID, MetaKeyFailedActionID, failedActionID)
	if err != nil {
		slog.Error("check active investigate-failure action", "error", err)
		return
	}
	if has {
		slog.Info("investigate-failure action already exists", "failed_action_id", action.ID)
		return
	}

	instruction := fmt.Sprintf("Investigate why action #%s failed.\n\nFailure result:\n%s\n\nSteps:\n1. Run `tq action list --task $TQ_TASK_ID` to review action history\n2. Check logs and context for the failed action\n3. Determine root cause and create a fix action if needed\n4. Mark this action done with findings", failedActionID, failureResult)
	newMeta := map[string]any{
		MetaKeyIsInvestigation: true,
		MetaKeyFailedActionID:  failedActionID,
		"failure_result":       failureResult,
		MetaKeyInstruction:     instruction,
	}
	metaJSON, err := json.Marshal(newMeta)
	if err != nil {
		slog.Error("marshal investigate-failure metadata", "error", err)
		return
	}

	title := fmt.Sprintf("Investigate failure of action #%d", action.ID)
	id, err := database.InsertAction(title, action.TaskID, string(metaJSON), db.ActionStatusPending)
	if err != nil {
		slog.Error("create investigate-failure action", "error", err)
		return
	}

	slog.Info("investigate-failure action created", "action_id", id, "failed_action_id", action.ID)
}
