package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func writeTestPrompt(t *testing.T, dir, name, content string) {
	t.Helper()
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	if err := os.WriteFile(filepath.Join(promptsDir, name+".md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAdd(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)
	writeTestPrompt(t, tqDir, "review-pr", `---
description: Review PR
---
Review this PR.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "review-pr", "--task", "1"})

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
	if a.PromptID != "review-pr" {
		t.Errorf("prompt_id = %q, want %q", a.PromptID, "review-pr")
	}
	if !a.TaskID.Valid || a.TaskID.Int64 != taskID {
		t.Errorf("task_id = %v, want %d", a.TaskID, taskID)
	}
}

func TestAdd_DuplicateBlocked(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)
	writeTestPrompt(t, tqDir, "review-pr", `---
description: Review PR
---
Review.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", "auto")

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
	cmd.SetConfigDir(tqDir)
	writeTestPrompt(t, tqDir, "implement", `---
description: Implement
---
Implement.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	d.InsertAction("implement", &taskID, "{}", "waiting_human", "auto")

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
	cmd.SetConfigDir(tqDir)
	writeTestPrompt(t, tqDir, "review-pr", `---
description: Review PR
---
Review.
`)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", "auto")

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

func TestAdd_MissingTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)
	writeTestPrompt(t, tqDir, "review-pr", `---
description: Review PR
---
Review.
`)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "review-pr"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --task flag")
	}
	if !contains(err.Error(), "--task flag is required") {
		t.Errorf("error = %q, want to contain '--task flag is required'", err.Error())
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
		t.Fatal("expected error for missing prompt argument")
	}
}
