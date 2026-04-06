package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

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

	// Skip if the action has already completed successfully or been cancelled
	// (race condition: action completed via /tq:done but process was killed
	// by timeout afterward). We do NOT skip for "failed" status, since that
	// is the expected state when this function is called legitimately.
	fresh, err := database.GetAction(action.ID)
	if err == nil && (fresh.Status == db.ActionStatusDone || fresh.Status == db.ActionStatusCancelled) {
		slog.Info("skipping investigate-failure for already terminal action", "action_id", action.ID, "status", fresh.Status)
		return
	}

	// Skip investigate for actions that failed due to timeout/kill.
	// Timeouts are self-explanatory and don't need investigation.
	if isTimeoutFailure(failureResult) {
		slog.Info("skipping investigate-failure for timeout", "action_id", action.ID)
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

	instruction := fmt.Sprintf("Investigate why action #%s failed.\n\nFailure result:\n%s\n\nSteps:\n1. Check logs and context for the failed action\n2. Determine root cause and create a fix action if needed\n3. Mark this action done with findings", failedActionID, failureResult)
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

func isTimeoutFailure(result string) bool {
	return strings.Contains(result, "signal: killed") ||
		strings.Contains(result, "context deadline exceeded") ||
		strings.Contains(result, "stale: noninteractive action exceeded timeout")
}
