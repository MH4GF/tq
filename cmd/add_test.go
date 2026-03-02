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
	root.SetArgs([]string{"action", "create", "--template", "review-pr", "--task", "1", "--priority", "5"})

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
	root.SetArgs([]string{"action", "create", "--template", "manual-task", "--task", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "status: waiting_human") {
		t.Errorf("output = %q, want to contain 'status: waiting_human'", out)
	}
}

func TestAdd_Duplicate(t *testing.T) {
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
	root.SetArgs([]string{"action", "create", "--template", "review-pr", "--task", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "warning") {
		t.Errorf("output = %q, want to contain duplicate warning", out)
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
	root.SetArgs([]string{"action", "create", "--task", "1"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --template flag")
	}
}
