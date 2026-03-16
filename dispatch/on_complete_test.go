package dispatch

import (
	"encoding/json"
	"testing"

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
			wantStatus:   "done",
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

			writeTestPromptFull(t, promptsDir, "check-pr", "interactive", tc.onDone, "", "")
			if tc.onDone != "" && tc.onDone != "nonexistent" {
				writeTestPromptFull(t, promptsDir, tc.onDone, "interactive", "", "", "")
			}

			taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")

			if tc.existingActive {
				d.InsertAction(tc.onDone, tc.onDone, taskID, "{}", "pending")
			}

			actionID, _ := d.InsertAction("check-pr", "check-pr", taskID, "{}", "done")
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
				if followUp.Status != "pending" {
					t.Errorf("status = %q, want %q", followUp.Status, "pending")
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
			}
		})
	}
}

func TestTriggerOnFail(t *testing.T) {
	tests := []struct {
		name           string
		onFail         string
		existingActive bool
		wantFollowUp   bool
		wantErr        bool
		wantStatus     string
	}{
		{
			name:         "no on_fail configured",
			onFail:       "",
			wantFollowUp: false,
		},
		{
			name:         "creates follow-up",
			onFail:       "investigate",
			wantFollowUp: true,
			wantStatus:   "failed",
		},
		{
			name:           "duplicate skipped",
			onFail:         "investigate",
			existingActive: true,
			wantFollowUp:   false,
		},
		{
			name:    "target template not found",
			onFail:  "nonexistent",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			promptsDir := setupPromptsDir(t)

			writeTestPromptFull(t, promptsDir, "failing-action", "noninteractive", "", "", tc.onFail)
			if tc.onFail != "" && tc.onFail != "nonexistent" {
				writeTestPromptFull(t, promptsDir, tc.onFail, "noninteractive", "", "", "")
			}

			taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")

			if tc.existingActive {
				d.InsertAction(tc.onFail, tc.onFail, taskID, "{}", "pending")
			}

			actionID, _ := d.InsertAction("failing-action", "failing-action", taskID, "{}", "failed")
			action, _ := d.GetAction(actionID)

			result := "worker error: context deadline exceeded"
			err := TriggerOnFail(d, promptsDir, action, result)

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
				if followUp.PromptID != tc.onFail {
					t.Errorf("prompt_id = %q, want %q", followUp.PromptID, tc.onFail)
				}
				if followUp.Status != "pending" {
					t.Errorf("status = %q, want %q", followUp.Status, "pending")
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
			}
		})
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
			wantStatus:    "cancelled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			promptsDir := setupPromptsDir(t)

			writeTestPromptFull(t, promptsDir, "check-pr", "interactive", "", tc.onCancel, "")
			if tc.onCancel != "" {
				writeTestPromptFull(t, promptsDir, tc.onCancel, "interactive", "", "", "")
			}

			taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
			actionID, _ := d.InsertAction("check-pr", "check-pr", taskID, "{}", "cancelled")
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
				if followUp.Status != "pending" {
					t.Errorf("status = %q, want %q", followUp.Status, "pending")
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
