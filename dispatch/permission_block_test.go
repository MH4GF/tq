package dispatch

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestCreatePermissionBlockAction(t *testing.T) {
	twoDenials := []PermissionDenial{
		{ToolName: "Bash", Input: map[string]any{"command": "gh api notifications"}},
		{ToolName: "Bash", Input: map[string]any{"command": "gh api -X PATCH /notifications/threads/123"}},
	}
	oneDenial := []PermissionDenial{{ToolName: "Bash", Input: map[string]any{"command": "x"}}}

	type sourceSpec struct {
		metadata string
		status   string
	}
	defaultSource := sourceSpec{metadata: `{}`, status: db.ActionStatusDone}

	tests := []struct {
		name           string
		sources        []sourceSpec
		invocations    []int
		denials        []PermissionDenial
		wantBlockCount int
		check          func(t *testing.T, sources []*db.Action, actions []db.Action, taskID int64)
	}{
		{
			name:           "creates pending interactive follow-up",
			sources:        []sourceSpec{defaultSource},
			invocations:    []int{0},
			denials:        twoDenials,
			wantBlockCount: 1,
			check:          checkInteractiveFollowup,
		},
		{
			name:           "dedupes for same blocked action",
			sources:        []sourceSpec{defaultSource},
			invocations:    []int{0, 0},
			denials:        oneDenial,
			wantBlockCount: 1,
		},
		{
			name:           "creates separate follow-ups for different blocked actions",
			sources:        []sourceSpec{defaultSource, defaultSource},
			invocations:    []int{0, 1},
			denials:        oneDenial,
			wantBlockCount: 2,
		},
		{
			name:           "skips when source action is itself a permission-block follow-up",
			sources:        []sourceSpec{{metadata: `{"is_permission_block":true}`, status: db.ActionStatusRunning}},
			invocations:    []int{0},
			denials:        oneDenial,
			wantBlockCount: 0,
		},
		{
			name:           "noop on empty denials",
			sources:        []sourceSpec{defaultSource},
			invocations:    []int{0},
			denials:        nil,
			wantBlockCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, _ := d.InsertTask(1, "Test task", `{}`, "")

			sources := make([]*db.Action, len(tt.sources))
			for i, spec := range tt.sources {
				id, _ := d.InsertAction(fmt.Sprintf("src%d", i), taskID, spec.metadata, spec.status, nil)
				sources[i], _ = d.GetAction(id)
			}

			for _, idx := range tt.invocations {
				CreatePermissionBlockAction(d, sources[idx], tt.denials)
			}

			actions, _ := d.ListActions("", nil, 0)
			wantTotal := len(tt.sources) + tt.wantBlockCount
			if len(actions) != wantTotal {
				t.Errorf("total actions = %d, want %d", len(actions), wantTotal)
			}
			blockCount := 0
			for _, a := range actions {
				if a.Status == db.ActionStatusPending && hasMetaKey(a.Metadata, MetaKeyIsPermissionBlock) {
					blockCount++
				}
			}
			if blockCount != tt.wantBlockCount {
				t.Errorf("permission-block actions = %d, want %d", blockCount, tt.wantBlockCount)
			}

			if tt.check != nil {
				tt.check(t, sources, actions, taskID)
			}
		})
	}

	t.Run("truncates long denial commands", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
		actionID, _ := d.InsertAction("watch", taskID, `{}`, db.ActionStatusDone, nil)
		action, _ := d.GetAction(actionID)

		longCmd := strings.Repeat("x", 23000)
		denials := []PermissionDenial{
			{ToolName: "Bash", Input: map[string]any{"command": longCmd}},
		}
		CreatePermissionBlockAction(d, action, denials)

		var followup *db.Action
		actions, _ := d.ListActions("", nil, 0)
		for i := range actions {
			if actions[i].ID != actionID {
				a := actions[i]
				followup = &a
			}
		}
		if followup == nil {
			t.Fatal("follow-up action not found")
		}

		var meta map[string]any
		_ = json.Unmarshal([]byte(followup.Metadata), &meta)
		instr, _ := meta[MetaKeyInstruction].(string)

		if len(instr) >= 4096 {
			t.Errorf("instruction length = %d, want < 4096", len(instr))
		}
		marker := fmt.Sprintf("(truncated; see tq action get #%d)", actionID)
		if !strings.Contains(instr, marker) {
			t.Errorf("instruction missing truncation marker %q", marker)
		}
		if !strings.Contains(instr, "Bash: xxx") {
			t.Errorf("instruction missing original denial prefix: %s", instr[:200])
		}
		if !utf8.ValidString(instr) {
			t.Errorf("instruction is not valid UTF-8")
		}
	})

	t.Run("truncates at rune boundary for multibyte input", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
		actionID, _ := d.InsertAction("watch", taskID, `{}`, db.ActionStatusDone, nil)
		action, _ := d.GetAction(actionID)

		longCmd := strings.Repeat("あ", 300)
		denials := []PermissionDenial{
			{ToolName: "Bash", Input: map[string]any{"command": longCmd}},
		}
		CreatePermissionBlockAction(d, action, denials)

		var followup *db.Action
		actions, _ := d.ListActions("", nil, 0)
		for i := range actions {
			if actions[i].ID != actionID {
				a := actions[i]
				followup = &a
			}
		}
		if followup == nil {
			t.Fatal("follow-up action not found")
		}
		var meta map[string]any
		_ = json.Unmarshal([]byte(followup.Metadata), &meta)
		instr, _ := meta[MetaKeyInstruction].(string)
		if !utf8.ValidString(instr) {
			t.Errorf("instruction is not valid UTF-8 after truncation: %q", instr)
		}
	})
}

func checkInteractiveFollowup(t *testing.T, sources []*db.Action, actions []db.Action, taskID int64) {
	t.Helper()
	sourceID := sources[0].ID
	var followup *db.Action
	for i := range actions {
		if actions[i].ID != sourceID {
			a := actions[i]
			followup = &a
			break
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
	expectedTitle := fmt.Sprintf("Investigate permission block in action #%d", sourceID)
	if followup.Title != expectedTitle {
		t.Errorf("title = %q, want %q", followup.Title, expectedTitle)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(followup.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if meta[MetaKeyBlockedActionID] != fmt.Sprintf("%d", sourceID) {
		t.Errorf("blocked_action_id = %v, want %d", meta[MetaKeyBlockedActionID], sourceID)
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
	if !strings.Contains(instr, fmt.Sprintf("action #%d", sourceID)) {
		t.Errorf("instruction missing action ref: %s", instr)
	}
}
