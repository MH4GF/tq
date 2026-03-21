package dispatch

import (
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestTriggerOnDone(t *testing.T) {
	tests := []struct {
		name           string
		onDone         string
		existingActive bool // pre-insert an active action for the target prompt
		wantFollowUp   bool
		wantErr        bool
		wantStatus     string // expected predecessor_status in metadata
	}{
		{
			name:         "no on_done configured",
			onDone:       "",
			wantFollowUp: false,
		},
		{
			name:         "creates follow-up",
			onDone:       "review",
			wantFollowUp: true,
			wantStatus:   db.ActionStatusDone,
		},
		{
			name:           "duplicate skipped",
			onDone:         "review",
			existingActive: true,
			wantFollowUp:   false,
		},
		{
			name:    "target template not found",
			onDone:  "nonexistent",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			promptsDir := setupPromptsDir(t)

			writeTestPromptFull(t, promptsDir, "check-pr", "interactive", tc.onDone, "")
			if tc.onDone != "" && tc.onDone != "nonexistent" {
				writeTestPromptFull(t, promptsDir, tc.onDone, "interactive", "", "")
			}

			taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")

			if tc.existingActive {
				d.InsertAction(tc.onDone, tc.onDone, taskID, "{}", db.ActionStatusPending)
			}

			actionID, _ := d.InsertAction("check-pr", "check-pr", taskID, "{}", db.ActionStatusDone)
			action, _ := d.GetAction(actionID)

			result := `{"status":"merged"}`
			err := TriggerOnDone(d, promptsDir, action, result)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			actions, _ := d.ListActions("", nil)
			expectedCount := 1
			if tc.existingActive {
				expectedCount++
			}
			if tc.wantFollowUp {
				expectedCount++
			}
			if len(actions) != expectedCount {
				t.Fatalf("expected %d actions, got %d", expectedCount, len(actions))
			}

			if tc.wantFollowUp {
				followUp := actions[len(actions)-1]
				if followUp.PromptID != tc.onDone {
					t.Errorf("prompt_id = %q, want %q", followUp.PromptID, tc.onDone)
				}
				if followUp.Status != db.ActionStatusPending {
					t.Errorf("status = %q, want %q", followUp.Status, db.ActionStatusPending)
				}

				var meta map[string]any
				if err := json.Unmarshal([]byte(followUp.Metadata), &meta); err != nil {
					t.Fatalf("parse metadata: %v", err)
				}
				if meta["predecessor_result"] != result {
					t.Errorf("predecessor_result = %v, want %q", meta["predecessor_result"], result)
				}
				if int64(meta["triggered_by_action_id"].(float64)) != actionID {
					t.Errorf("triggered_by_action_id = %v, want %d", meta["triggered_by_action_id"], actionID)
				}
				if meta["predecessor_status"] != tc.wantStatus {
					t.Errorf("predecessor_status = %v, want %q", meta["predecessor_status"], tc.wantStatus)
				}
				if meta["instruction"] != result {
					t.Errorf("instruction = %v, want %q", meta["instruction"], result)
				}
			}
		})
	}
}

func TestTriggerOnDone_NoPromptID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	promptsDir := setupPromptsDir(t)

	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	actionID, _ := d.InsertAction("instruction-only", "", taskID, `{"instruction":"do something"}`, db.ActionStatusDone)
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, promptsDir, action, "done result")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 1 {
		t.Errorf("expected 1 action (no follow-up), got %d", len(actions))
	}
}

func TestTriggerOnCancel_NoPromptID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	promptsDir := setupPromptsDir(t)

	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	actionID, _ := d.InsertAction("instruction-only", "", taskID, `{"instruction":"do something"}`, db.ActionStatusCancelled)
	action, _ := d.GetAction(actionID)

	err := TriggerOnCancel(d, promptsDir, action, "cancelled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 1 {
		t.Errorf("expected 1 action (no follow-up), got %d", len(actions))
	}
}

func TestTriggerOnCancel(t *testing.T) {
	tests := []struct {
		name          string
		onCancel      string
		wantFollowUp bool
		wantStatus    string
	}{
		{
			name:          "no on_cancel configured",
			onCancel:      "",
			wantFollowUp: false,
		},
		{
			name:          "creates follow-up",
			onCancel:      "improve",
			wantFollowUp: true,
			wantStatus:    db.ActionStatusCancelled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			promptsDir := setupPromptsDir(t)

			writeTestPromptFull(t, promptsDir, "check-pr", "interactive", "", tc.onCancel)
			if tc.onCancel != "" {
				writeTestPromptFull(t, promptsDir, tc.onCancel, "interactive", "", "")
			}

			taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
			actionID, _ := d.InsertAction("check-pr", "check-pr", taskID, "{}", db.ActionStatusCancelled)
			action, _ := d.GetAction(actionID)

			reason := "cancelled with feedback"
			err := TriggerOnCancel(d, promptsDir, action, reason)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			actions, _ := d.ListActions("", nil)
			expectedCount := 1
			if tc.wantFollowUp {
				expectedCount++
			}
			if len(actions) != expectedCount {
				t.Fatalf("expected %d actions, got %d", expectedCount, len(actions))
			}

			if tc.wantFollowUp {
				followUp := actions[1]
				if followUp.PromptID != tc.onCancel {
					t.Errorf("prompt_id = %q, want %q", followUp.PromptID, tc.onCancel)
				}
				if followUp.Status != db.ActionStatusPending {
					t.Errorf("status = %q, want %q", followUp.Status, db.ActionStatusPending)
				}

				var meta map[string]any
				if err := json.Unmarshal([]byte(followUp.Metadata), &meta); err != nil {
					t.Fatalf("parse metadata: %v", err)
				}
				if meta["predecessor_status"] != tc.wantStatus {
					t.Errorf("predecessor_status = %v, want %q", meta["predecessor_status"], tc.wantStatus)
				}
				if meta["predecessor_result"] != reason {
					t.Errorf("predecessor_result = %v, want %q", meta["predecessor_result"], reason)
				}
				if int64(meta["triggered_by_action_id"].(float64)) != actionID {
					t.Errorf("triggered_by_action_id = %v, want %d", meta["triggered_by_action_id"], actionID)
				}
			}
		})
	}
}
