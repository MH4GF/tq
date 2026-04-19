package dispatch

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func hasMetaKey(metadata, key string) bool {
	var m map[string]any
	if err := json.Unmarshal([]byte(metadata), &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func TestCreateInvestigateFailureAction(t *testing.T) {
	tests := []struct {
		name        string
		setupAction func(*db.DB) int64
		wantCount   int
		wantTitle   string
	}{
		{
			name: "creates investigation action on same task",
			setupAction: func(d *db.DB) int64 {
				taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
				actionID, _ := d.InsertAction("my-prompt", taskID, "{}", "failed", nil)
				action, _ := d.GetAction(actionID)
				CreateInvestigateFailureAction(d, action, "worker error: timeout")
				return actionID
			},
			wantCount: 1,
		},
		{
			name: "skips duplicate for same failed action",
			setupAction: func(d *db.DB) int64 {
				taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
				actionID, _ := d.InsertAction("my-prompt", taskID, "{}", "failed", nil)
				action, _ := d.GetAction(actionID)
				CreateInvestigateFailureAction(d, action, "error 1")
				CreateInvestigateFailureAction(d, action, "error 1")
				return actionID
			},
			wantCount: 1,
		},
		{
			name: "creates separate investigations for different failed actions",
			setupAction: func(d *db.DB) int64 {
				taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
				action1ID, _ := d.InsertAction("prompt-a", taskID, "{}", "failed", nil)
				action1, _ := d.GetAction(action1ID)
				action2ID, _ := d.InsertAction("prompt-b", taskID, "{}", "failed", nil)
				action2, _ := d.GetAction(action2ID)
				CreateInvestigateFailureAction(d, action1, "error 1")
				CreateInvestigateFailureAction(d, action2, "error 2")
				return action2ID
			},
			wantCount: 2,
		},
		{
			name: "title includes action ID",
			setupAction: func(d *db.DB) int64 {
				taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
				actionID, _ := d.InsertAction("deploy", taskID, "{}", "failed", nil)
				action, _ := d.GetAction(actionID)
				CreateInvestigateFailureAction(d, action, "deploy failed")
				return actionID
			},
			wantCount: 1,
			wantTitle: "Investigate failure of action #%d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			failedActionID := tt.setupAction(d)

			actions, _ := d.ListActions("", nil, 0)
			investigateCount := 0
			for _, a := range actions {
				if hasMetaKey(a.Metadata, "is_investigate_failure") {
					investigateCount++
				}
			}
			if investigateCount != tt.wantCount {
				t.Errorf("investigate count = %d, want %d", investigateCount, tt.wantCount)
			}

			if tt.wantTitle != "" {
				expectedTitle := fmt.Sprintf(tt.wantTitle, failedActionID)
				if actions[0].Title != expectedTitle {
					t.Errorf("title = %q, want %q", actions[0].Title, expectedTitle)
				}
			}
		})
	}
}

func TestCreateInvestigateFailureAction_SkipsInvestigationItself(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	actionID, _ := d.InsertAction("internal:investigate-failure", taskID, `{"is_investigate_failure":true}`, "failed", nil)
	action, _ := d.GetAction(actionID)

	CreateInvestigateFailureAction(d, action, "investigation itself failed")

	actions, _ := d.ListActions("", nil, 0)
	// Should NOT create a new investigation action to prevent infinite loops
	pendingCount := 0
	for _, a := range actions {
		if hasMetaKey(a.Metadata, "is_investigate_failure") && a.Status == db.ActionStatusPending {
			pendingCount++
		}
	}
	if pendingCount != 0 {
		t.Errorf("expected 0 pending investigate actions (skip self), got %d", pendingCount)
	}
}

func TestCreateInvestigateFailureAction_SkipsAlreadyTerminal(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	actionID, _ := d.InsertAction("watch-notifications", taskID, "{}", "pending", nil)
	// Simulate: action completed via /tq:done, then process was killed by timeout
	d.MarkDone(actionID, "outcome: processed 0 notifications")
	action, _ := d.GetAction(actionID)

	CreateInvestigateFailureAction(d, action, "signal: killed")

	actions, _ := d.ListActions("", nil, 0)
	for _, a := range actions {
		if hasMetaKey(a.Metadata, "is_investigate_failure") {
			t.Error("should not create investigate action for already-terminal action")
		}
	}
}

func TestCreateInvestigateFailureAction_SkipsTimeout(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")

	t.Run("skips for signal killed (scheduled)", func(t *testing.T) {
		meta := `{"schedule_id":"4","instruction":"test"}`
		actionID, _ := d.InsertAction("watch", taskID, meta, "failed", nil)
		action, _ := d.GetAction(actionID)

		CreateInvestigateFailureAction(d, action, "signal: killed")

		actions, _ := d.ListActions("", nil, 0)
		for _, a := range actions {
			if hasMetaKey(a.Metadata, "is_investigate_failure") {
				t.Error("should not create investigate action for scheduled timeout")
			}
		}
	})

	t.Run("skips for signal killed (non-scheduled)", func(t *testing.T) {
		meta := `{"instruction":"test"}`
		actionID, _ := d.InsertAction("deploy", taskID, meta, "failed", nil)
		action, _ := d.GetAction(actionID)

		CreateInvestigateFailureAction(d, action, "signal: killed")

		actions, _ := d.ListActions("", nil, 0)
		for _, a := range actions {
			if hasMetaKey(a.Metadata, "is_investigate_failure") {
				t.Error("should not create investigate action for non-scheduled timeout")
			}
		}
	})

	t.Run("skips for context deadline exceeded", func(t *testing.T) {
		actionID, _ := d.InsertAction("build", taskID, "{}", "failed", nil)
		action, _ := d.GetAction(actionID)

		CreateInvestigateFailureAction(d, action, "context deadline exceeded")

		actions, _ := d.ListActions("", nil, 0)
		for _, a := range actions {
			if hasMetaKey(a.Metadata, "is_investigate_failure") {
				t.Error("should not create investigate action for deadline exceeded")
			}
		}
	})

	t.Run("skips for stale noninteractive timeout", func(t *testing.T) {
		actionID, _ := d.InsertAction("sync", taskID, "{}", "failed", nil)
		action, _ := d.GetAction(actionID)

		CreateInvestigateFailureAction(d, action, "stale: noninteractive action exceeded timeout (20m0s)")

		actions, _ := d.ListActions("", nil, 0)
		for _, a := range actions {
			if hasMetaKey(a.Metadata, "is_investigate_failure") {
				t.Error("should not create investigate action for stale noninteractive timeout")
			}
		}
	})
}

func TestCreateInvestigateFailureAction_DoesNotSkipNonTimeout(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	actionID, _ := d.InsertAction("deploy", taskID, "{}", "failed", nil)
	action, _ := d.GetAction(actionID)

	CreateInvestigateFailureAction(d, action, "API Error: Unable to connect")

	actions, _ := d.ListActions("", nil, 0)
	investigateCount := 0
	for _, a := range actions {
		if hasMetaKey(a.Metadata, "is_investigate_failure") {
			investigateCount++
		}
	}
	if investigateCount != 1 {
		t.Errorf("expected 1 investigate action for non-timeout failure, got %d", investigateCount)
	}
}
