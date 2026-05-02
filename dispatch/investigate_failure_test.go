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
		setupAction func(db.Store) int64
		wantCount   int
		wantTitle   string
	}{
		{
			name: "creates investigation action on same task",
			setupAction: func(d db.Store) int64 {
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
			setupAction: func(d db.Store) int64 {
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
			setupAction: func(d db.Store) int64 {
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
			setupAction: func(d db.Store) int64 {
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
				if hasMetaKey(a.Metadata, MetaKeyIsInvestigation) {
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

func TestCreateInvestigateFailureAction_SkipBehavior(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		meta           string
		status         string
		markDone       bool
		markDoneResult string
		failureResult  string
		wantCount      int
	}{
		{
			name:          "skips investigation itself (prevents infinite loop)",
			prompt:        "internal:investigate-failure",
			meta:          `{"is_investigate_failure":true}`,
			status:        "failed",
			failureResult: "investigation itself failed",
			wantCount:     0,
		},
		{
			name:           "skips already-terminal action killed after MarkDone",
			prompt:         "watch-notifications",
			meta:           `{}`,
			status:         "pending",
			markDone:       true,
			markDoneResult: "outcome: processed 0 notifications",
			failureResult:  "signal: killed",
			wantCount:      0,
		},
		{
			name:          "skips signal killed (scheduled)",
			prompt:        "test",
			meta:          `{"schedule_id":"4","instruction":"test"}`,
			status:        "failed",
			failureResult: "signal: killed",
			wantCount:     0,
		},
		{
			name:          "skips signal killed (non-scheduled)",
			prompt:        "test",
			meta:          `{"instruction":"test"}`,
			status:        "failed",
			failureResult: "signal: killed",
			wantCount:     0,
		},
		{
			name:          "skips context deadline exceeded",
			prompt:        "test",
			meta:          `{}`,
			status:        "failed",
			failureResult: "context deadline exceeded",
			wantCount:     0,
		},
		{
			name:          "skips stale noninteractive timeout",
			prompt:        "test",
			meta:          `{}`,
			status:        "failed",
			failureResult: "stale: noninteractive action exceeded timeout (20m0s)",
			wantCount:     0,
		},
		{
			name:          "does not skip non-timeout failure",
			prompt:        "deploy",
			meta:          `{}`,
			status:        "failed",
			failureResult: "API Error: Unable to connect",
			wantCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
			actionID, _ := d.InsertAction(tt.prompt, taskID, tt.meta, tt.status, nil)
			if tt.markDone {
				d.MarkDone(actionID, tt.markDoneResult)
			}
			action, _ := d.GetAction(actionID)

			CreateInvestigateFailureAction(d, action, tt.failureResult)

			actions, _ := d.ListActions("", nil, 0)
			count := 0
			for _, a := range actions {
				if hasMetaKey(a.Metadata, MetaKeyIsInvestigation) && a.Status == db.ActionStatusPending {
					count++
				}
			}
			if count != tt.wantCount {
				t.Errorf("pending investigate count = %d, want %d", count, tt.wantCount)
			}
		})
	}
}
