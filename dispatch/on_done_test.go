package dispatch

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func writeOnDoneTemplate(t *testing.T, dir, name string, onDone string) {
	t.Helper()
	content := fmt.Sprintf(`---
description: %s
on_done: %s
---
Do %s.
`, name, onDone, name)
	os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644)
}

func TestTriggerOnDone_NoOnDone(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeOnDoneTemplate(t, promptsDir, "check-pr", "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, promptsDir, action, `{"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 1 {
		t.Errorf("expected 1 action (no follow-up), got %d", len(actions))
	}
}

func TestTriggerOnDone_NoTaskID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeOnDoneTemplate(t, promptsDir, "check-pr", "review")
	writeOnDoneTemplate(t, promptsDir, "review", "")

	actionID, _ := d.InsertAction("check-pr", nil, "{}", "done")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, promptsDir, action, `{"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 1 {
		t.Errorf("expected 1 action (skipped), got %d", len(actions))
	}
}

func TestTriggerOnDone_AutoTarget(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeOnDoneTemplate(t, promptsDir, "check-pr", "review")
	writeOnDoneTemplate(t, promptsDir, "review", "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done")
	action, _ := d.GetAction(actionID)

	result := `{"status":"merged"}`
	err := TriggerOnDone(d, promptsDir, action, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	followUp := actions[1]
	if followUp.PromptID != "review" {
		t.Errorf("template_id = %q, want %q", followUp.PromptID, "review")
	}
	if followUp.Status != "pending" {
		t.Errorf("status = %q, want %q", followUp.Status, "pending")
	}
	if !followUp.TaskID.Valid || followUp.TaskID.Int64 != taskID {
		t.Errorf("task_id = %v, want %d", followUp.TaskID, taskID)
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
}

func TestTriggerOnDone_DuplicateSkipped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeOnDoneTemplate(t, promptsDir, "check-pr", "review")
	writeOnDoneTemplate(t, promptsDir, "review", "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	d.InsertAction("review", &taskID, "{}", "pending")

	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, promptsDir, action, `{"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Errorf("expected 2 actions (no new one), got %d", len(actions))
	}
}

func TestTriggerOnDone_PendingSkipped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeOnDoneTemplate(t, promptsDir, "check-pr", "review")
	writeOnDoneTemplate(t, promptsDir, "review", "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	d.InsertAction("review", &taskID, "{}", "pending")

	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, promptsDir, action, `{"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Errorf("expected 2 actions (pending blocks creation), got %d", len(actions))
	}
}

func TestTriggerOnDone_TargetTemplateNotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeOnDoneTemplate(t, promptsDir, "check-pr", "nonexistent")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done")
	action := &db.Action{
		ID:         actionID,
		PromptID: "check-pr",
		TaskID:     sql.NullInt64{Int64: taskID, Valid: true},
	}

	err := TriggerOnDone(d, promptsDir, action, `{"ok":true}`)
	if err == nil {
		t.Fatal("expected error for missing target template")
	}
}
