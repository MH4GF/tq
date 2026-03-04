package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func writeTestTemplate(t *testing.T, dir, name, content string) {
	t.Helper()
	templatesDir := filepath.Join(dir, "templates")
	os.MkdirAll(templatesDir, 0755)
	if err := os.WriteFile(filepath.Join(templatesDir, name+".md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAdd(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetTQDir(tqDir)
	writeTestTemplate(t, tqDir, "review-pr", `---
description: Review PR
auto: true
---
Review this PR.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "review-pr", "--task", "1", "--priority", "5"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 created") {
		t.Errorf("output = %q, want to contain 'action #1 created'", out)
	}
	if !contains(out, "status: pending") {
		t.Errorf("output = %q, want to contain 'status: pending'", out)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.TemplateID != "review-pr" {
		t.Errorf("template_id = %q, want %q", a.TemplateID, "review-pr")
	}
	if !a.TaskID.Valid || a.TaskID.Int64 != taskID {
		t.Errorf("task_id = %v, want %d", a.TaskID, taskID)
	}
	if a.Priority != 5 {
		t.Errorf("priority = %d, want 5", a.Priority)
	}
}

func TestAdd_AutoFalse(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetTQDir(tqDir)
	writeTestTemplate(t, tqDir, "manual-task", `---
description: Manual task
auto: false
---
Manual only.
`)

	d.InsertTask(1, "test task", "", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "manual-task", "--task", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "status: waiting_human") {
		t.Errorf("output = %q, want to contain 'status: waiting_human'", out)
	}
}

func TestAdd_DuplicateBlocked(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetTQDir(tqDir)
	writeTestTemplate(t, tqDir, "review-pr", `---
description: Review PR
auto: true
---
Review.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", 0, "auto")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "review-pr", "--task", "1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for duplicate action")
	}
	if !contains(err.Error(), "blocked") {
		t.Errorf("error = %q, want to contain 'blocked'", err.Error())
	}
	if !contains(err.Error(), "--force") {
		t.Errorf("error = %q, want to contain '--force'", err.Error())
	}
}

func TestAdd_DuplicateWaitingHumanBlocked(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetTQDir(tqDir)
	writeTestTemplate(t, tqDir, "implement", `---
description: Implement
auto: true
---
Implement.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	d.InsertAction("implement", &taskID, "{}", "waiting_human", 0, "auto")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "implement", "--task", "1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for duplicate waiting_human action")
	}
	if !contains(err.Error(), "blocked") {
		t.Errorf("error = %q, want to contain 'blocked'", err.Error())
	}
}

func TestAdd_DuplicateForce(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetTQDir(tqDir)
	writeTestTemplate(t, tqDir, "review-pr", `---
description: Review PR
auto: true
---
Review.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", 0, "auto")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "review-pr", "--task", "1", "--force"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #2 created") {
		t.Errorf("output = %q, want to contain 'action #2 created'", out)
	}
}

func TestAdd_MissingTemplate(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "create"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing template argument")
	}
}
