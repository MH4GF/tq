package dispatch

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestCreatePermissionBlockAction(t *testing.T) {
	t.Run("creates pending interactive follow-up", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
		actionID, _ := d.InsertAction("watch", taskID, `{"instruction":"x","mode":"noninteractive"}`, db.ActionStatusDone, nil)
		action, _ := d.GetAction(actionID)

		denials := []PermissionDenial{
			{ToolName: "Bash", Input: map[string]any{"command": "gh api notifications"}},
			{ToolName: "Bash", Input: map[string]any{"command": "gh api -X PATCH /notifications/threads/123"}},
		}
		CreatePermissionBlockAction(d, action, denials)

		actions, _ := d.ListActions("", nil, 0)
		if len(actions) != 2 {
			t.Fatalf("expected 2 actions, got %d", len(actions))
		}

		var followup *db.Action
		for i := range actions {
			if actions[i].ID != actionID {
				a := actions[i]
				followup = &a
			}
		}
		if followup == nil {
			t.Fatal("follow-up action not found")
		}

		if followup.Status != db.ActionStatusPending {
			t.Errorf("status = %q, want pending", followup.Status)
		}
		if followup.TaskID != taskID {
			t.Errorf("task_id = %d, want %d", followup.TaskID, taskID)
		}
		expectedTitle := fmt.Sprintf("Investigate permission block in action #%d", actionID)
		if followup.Title != expectedTitle {
			t.Errorf("title = %q, want %q", followup.Title, expectedTitle)
		}

		var meta map[string]any
		if err := json.Unmarshal([]byte(followup.Metadata), &meta); err != nil {
			t.Fatalf("parse metadata: %v", err)
		}
		if meta[MetaKeyBlockedActionID] != fmt.Sprintf("%d", actionID) {
			t.Errorf("blocked_action_id = %v, want %d", meta[MetaKeyBlockedActionID], actionID)
		}
		if meta[MetaKeyIsPermissionBlock] != true {
			t.Errorf("is_permission_block = %v, want true", meta[MetaKeyIsPermissionBlock])
		}
		if meta[MetaKeyMode] != ModeInteractive {
			t.Errorf("mode = %v, want %q", meta[MetaKeyMode], ModeInteractive)
		}
		instr, _ := meta[MetaKeyInstruction].(string)
		if !strings.Contains(instr, "Bash: gh api notifications") {
			t.Errorf("instruction missing first denial: %s", instr)
		}
		if !strings.Contains(instr, "Bash: gh api -X PATCH /notifications/threads/123") {
			t.Errorf("instruction missing second denial: %s", instr)
		}
		if !strings.Contains(instr, fmt.Sprintf("action #%d", actionID)) {
			t.Errorf("instruction missing action ref: %s", instr)
		}
	})

	t.Run("dedupes for same blocked action", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
		actionID, _ := d.InsertAction("watch", taskID, `{}`, db.ActionStatusDone, nil)
		action, _ := d.GetAction(actionID)

		denials := []PermissionDenial{{ToolName: "Bash", Input: map[string]any{"command": "x"}}}
		CreatePermissionBlockAction(d, action, denials)
		CreatePermissionBlockAction(d, action, denials)

		actions, _ := d.ListActions("", nil, 0)
		count := 0
		for _, a := range actions {
			if hasMetaKey(a.Metadata, MetaKeyIsPermissionBlock) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected 1 permission-block action, got %d", count)
		}
	})

	t.Run("creates separate follow-ups for different blocked actions", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
		id1, _ := d.InsertAction("a", taskID, `{}`, db.ActionStatusDone, nil)
		id2, _ := d.InsertAction("b", taskID, `{}`, db.ActionStatusDone, nil)
		a1, _ := d.GetAction(id1)
		a2, _ := d.GetAction(id2)

		denials := []PermissionDenial{{ToolName: "Bash", Input: map[string]any{"command": "x"}}}
		CreatePermissionBlockAction(d, a1, denials)
		CreatePermissionBlockAction(d, a2, denials)

		actions, _ := d.ListActions("", nil, 0)
		count := 0
		for _, a := range actions {
			if hasMetaKey(a.Metadata, MetaKeyIsPermissionBlock) {
				count++
			}
		}
		if count != 2 {
			t.Errorf("expected 2 permission-block actions, got %d", count)
		}
	})

	t.Run("skips when source action is itself a permission-block follow-up", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
		actionID, _ := d.InsertAction("self", taskID, `{"is_permission_block":true}`, db.ActionStatusRunning, nil)
		action, _ := d.GetAction(actionID)

		denials := []PermissionDenial{{ToolName: "Bash", Input: map[string]any{"command": "x"}}}
		CreatePermissionBlockAction(d, action, denials)

		actions, _ := d.ListActions("", nil, 0)
		pendingCount := 0
		for _, a := range actions {
			if a.Status == db.ActionStatusPending && hasMetaKey(a.Metadata, MetaKeyIsPermissionBlock) {
				pendingCount++
			}
		}
		if pendingCount != 0 {
			t.Errorf("expected 0 pending permission-block actions, got %d", pendingCount)
		}
	})

	t.Run("noop on empty denials", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
		actionID, _ := d.InsertAction("a", taskID, `{}`, db.ActionStatusDone, nil)
		action, _ := d.GetAction(actionID)

		CreatePermissionBlockAction(d, action, nil)

		actions, _ := d.ListActions("", nil, 0)
		if len(actions) != 1 {
			t.Errorf("expected 1 action (no follow-up), got %d", len(actions))
		}
	})
}
