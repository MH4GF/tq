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

func writeOnDoneTemplate(t *testing.T, dir, name string, auto bool, onDone string) {
	t.Helper()
	content := fmt.Sprintf(`---
description: %s
auto: %v
on_done: %s
---
Do %s.
`, name, auto, onDone, name)
	os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644)
}

func TestTriggerOnDone_NoOnDone(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeOnDoneTemplate(t, templatesDir, "check-pr", true, "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done", 0, "test")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, templatesDir, action, `{"ok":true}`)
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
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeOnDoneTemplate(t, templatesDir, "check-pr", true, "review")
	writeOnDoneTemplate(t, templatesDir, "review", true, "")

	actionID, _ := d.InsertAction("check-pr", nil, "{}", "done", 0, "test")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, templatesDir, action, `{"ok":true}`)
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
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeOnDoneTemplate(t, templatesDir, "check-pr", true, "review")
	writeOnDoneTemplate(t, templatesDir, "review", true, "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done", 0, "test")
	action, _ := d.GetAction(actionID)

	result := `{"status":"merged"}`
	err := TriggerOnDone(d, templatesDir, action, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	followUp := actions[1]
	if followUp.TemplateID != "review" {
		t.Errorf("template_id = %q, want %q", followUp.TemplateID, "review")
	}
	if followUp.Status != "pending" {
		t.Errorf("status = %q, want %q", followUp.Status, "pending")
	}
	if followUp.Source != "on_done" {
		t.Errorf("source = %q, want %q", followUp.Source, "on_done")
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

func TestTriggerOnDone_NonAutoTarget(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeOnDoneTemplate(t, templatesDir, "check-pr", true, "review")
	writeOnDoneTemplate(t, templatesDir, "review", false, "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done", 0, "test")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, templatesDir, action, `{"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	if actions[1].Status != "waiting_human" {
		t.Errorf("status = %q, want %q", actions[1].Status, "waiting_human")
	}
}

func TestTriggerOnDone_DuplicateSkipped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeOnDoneTemplate(t, templatesDir, "check-pr", true, "review")
	writeOnDoneTemplate(t, templatesDir, "review", true, "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	d.InsertAction("review", &taskID, "{}", "pending", 0, "on_done")

	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done", 0, "test")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, templatesDir, action, `{"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Errorf("expected 2 actions (no new one), got %d", len(actions))
	}
}

func TestTriggerOnDone_WaitingHumanSkipped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeOnDoneTemplate(t, templatesDir, "check-pr", true, "review")
	writeOnDoneTemplate(t, templatesDir, "review", true, "")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	d.InsertAction("review", &taskID, "{}", "waiting_human", 0, "on_done")

	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done", 0, "test")
	action, _ := d.GetAction(actionID)

	err := TriggerOnDone(d, templatesDir, action, `{"ok":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Errorf("expected 2 actions (waiting_human blocks creation), got %d", len(actions))
	}
}

func TestTriggerOnDone_TargetTemplateNotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeOnDoneTemplate(t, templatesDir, "check-pr", true, "nonexistent")

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	actionID, _ := d.InsertAction("check-pr", &taskID, "{}", "done", 0, "test")
	action := &db.Action{
		ID:         actionID,
		TemplateID: "check-pr",
		TaskID:     sql.NullInt64{Int64: taskID, Valid: true},
	}

	err := TriggerOnDone(d, templatesDir, action, `{"ok":true}`)
	if err == nil {
		t.Fatal("expected error for missing target template")
	}
}
