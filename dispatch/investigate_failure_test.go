package dispatch

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestCreateInvestigateFailureAction(t *testing.T) {
	t.Run("creates investigation action on same task", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
		actionID, _ := d.InsertAction("my-prompt", "my-prompt", taskID, "{}", "failed")
		action, _ := d.GetAction(actionID)

		CreateInvestigateFailureAction(d, action, "worker error: timeout")

		actions, _ := d.ListActions("", nil)
		if len(actions) != 2 {
			t.Fatalf("expected 2 actions, got %d", len(actions))
		}

		investigate := actions[1]
		if investigate.PromptID != "internal:investigate-failure" {
			t.Errorf("prompt_id = %q, want internal:investigate-failure", investigate.PromptID)
		}
		if investigate.Status != "pending" {
			t.Errorf("status = %q, want pending", investigate.Status)
		}
		if investigate.TaskID != taskID {
			t.Errorf("task_id = %d, want %d", investigate.TaskID, taskID)
		}

		var meta map[string]any
		if err := json.Unmarshal([]byte(investigate.Metadata), &meta); err != nil {
			t.Fatalf("parse metadata: %v", err)
		}
		if meta["failed_action_id"] != fmt.Sprintf("%d", actionID) {
			t.Errorf("failed_action_id = %v, want %d", meta["failed_action_id"], actionID)
		}
		if meta["failed_prompt_id"] != "my-prompt" {
			t.Errorf("failed_prompt_id = %v, want my-prompt", meta["failed_prompt_id"])
		}
		if meta["failure_result"] != "worker error: timeout" {
			t.Errorf("failure_result = %v, want 'worker error: timeout'", meta["failure_result"])
		}
	})

	t.Run("skips duplicate for same failed action", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
		actionID, _ := d.InsertAction("my-prompt", "my-prompt", taskID, "{}", "failed")
		action, _ := d.GetAction(actionID)

		// Create first investigation
		CreateInvestigateFailureAction(d, action, "error 1")
		// Try to create a duplicate
		CreateInvestigateFailureAction(d, action, "error 1")

		actions, _ := d.ListActions("", nil)
		investigateCount := 0
		for _, a := range actions {
			if a.PromptID == "internal:investigate-failure" {
				investigateCount++
			}
		}
		if investigateCount != 1 {
			t.Errorf("expected 1 investigate action, got %d", investigateCount)
		}
	})

	t.Run("creates separate investigations for different failed actions", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
		action1ID, _ := d.InsertAction("prompt-a", "prompt-a", taskID, "{}", "failed")
		action1, _ := d.GetAction(action1ID)
		action2ID, _ := d.InsertAction("prompt-b", "prompt-b", taskID, "{}", "failed")
		action2, _ := d.GetAction(action2ID)

		CreateInvestigateFailureAction(d, action1, "error 1")
		CreateInvestigateFailureAction(d, action2, "error 2")

		actions, _ := d.ListActions("", nil)
		investigateCount := 0
		for _, a := range actions {
			if a.PromptID == "internal:investigate-failure" {
				investigateCount++
			}
		}
		if investigateCount != 2 {
			t.Errorf("expected 2 investigate actions, got %d", investigateCount)
		}
	})

	t.Run("title includes action ID and prompt ID", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)

		taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
		actionID, _ := d.InsertAction("deploy", "deploy", taskID, "{}", "failed")
		action, _ := d.GetAction(actionID)

		CreateInvestigateFailureAction(d, action, "deploy failed")

		actions, _ := d.ListActions("", nil)
		investigate := actions[1]
		expectedTitle := fmt.Sprintf("Investigate failure of action #%d (deploy)", actionID)
		if investigate.Title != expectedTitle {
			t.Errorf("title = %q, want %q", investigate.Title, expectedTitle)
		}
	})
}

func TestCreateInvestigateFailureAction_SkipsInvestigateFailurePrompt(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	actionID, _ := d.InsertAction("internal:investigate-failure", "internal:investigate-failure", taskID, "{}", "failed")
	action, _ := d.GetAction(actionID)

	CreateInvestigateFailureAction(d, action, "investigation itself failed")

	actions, _ := d.ListActions("", nil)
	// Should NOT create a new investigation action to prevent infinite loops
	pendingCount := 0
	for _, a := range actions {
		if a.PromptID == "internal:investigate-failure" && a.Status == db.ActionStatusPending {
			pendingCount++
		}
	}
	if pendingCount != 0 {
		t.Errorf("expected 0 pending investigate actions (skip self), got %d", pendingCount)
	}
}
