package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/MH4GF/tq/db"
)

// CreatePermissionBlockAction creates a pending interactive follow-up action
// to investigate tool permission denials. Skips if the source action is itself
// a permission-block follow-up, or if an active follow-up already exists.
func CreatePermissionBlockAction(database db.Store, action *db.Action, denials []PermissionDenial) {
	if len(denials) == 0 {
		return
	}

	meta, _ := parseMetadata(action.Metadata)
	if _, ok := meta[MetaKeyIsPermissionBlock]; ok {
		slog.Info("skipping permission-block action for permission-block action itself", "action_id", action.ID)
		return
	}

	blockedActionID := fmt.Sprintf("%d", action.ID)
	has, err := database.HasActiveActionWithMeta(action.TaskID, MetaKeyBlockedActionID, blockedActionID)
	if err != nil {
		slog.Error("check active permission-block action", "error", err)
		return
	}
	if has {
		slog.Info("permission-block action already exists", "blocked_action_id", action.ID)
		return
	}

	var list strings.Builder
	for _, d := range denials {
		list.WriteString("- ")
		list.WriteString(d.Summary())
		list.WriteString("\n")
	}

	instruction := fmt.Sprintf(
		"The following tool calls were permission-blocked in action #%s. Identify the root cause (missing settings.local.json entries, pattern mismatch, worktree settings loading issues, etc.) and fix it.\n\n%s",
		blockedActionID, list.String(),
	)

	newMeta := map[string]any{
		MetaKeyIsPermissionBlock: true,
		MetaKeyBlockedActionID:   blockedActionID,
		MetaKeyInstruction:       instruction,
		MetaKeyMode:              ModeInteractive,
	}
	metaJSON, err := json.Marshal(newMeta)
	if err != nil {
		slog.Error("marshal permission-block metadata", "error", err)
		return
	}

	title := fmt.Sprintf("Investigate permission block in action #%d", action.ID)
	id, err := database.InsertAction(title, action.TaskID, string(metaJSON), db.ActionStatusPending)
	if err != nil {
		slog.Error("create permission-block action", "error", err)
		return
	}

	slog.Info("permission-block action created", "action_id", id, "blocked_action_id", action.ID)
}
