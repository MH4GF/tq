package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func writeTestPrompt(t *testing.T, dir, name, content string) {
	t.Helper()
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0o755)
	if err := os.WriteFile(filepath.Join(promptsDir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAdd_PromptFlag(t *testing.T) {
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

	taskID, _ := d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--prompt", "review-pr", "--title", "review-pr", "--task", "1"})

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
	if !contains(out, "no worker detected") {
		t.Errorf("output = %q, want to contain 'no worker detected'", out)
	}
	if !contains(out, "queue: 1 pending") {
		t.Errorf("output = %q, want to contain 'queue: 1 pending'", out)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.PromptID != "review-pr" {
		t.Errorf("prompt_id = %q, want %q", a.PromptID, "review-pr")
	}
	if a.TaskID != taskID {
		t.Errorf("task_id = %d, want %d", a.TaskID, taskID)
	}
}

func TestAdd_InstructionOnly(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--task", "1", "--title", "Review PR", "--instruction", "/github-pr review this"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 created") {
		t.Errorf("output = %q, want to contain 'action #1 created'", out)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.PromptID != "" {
		t.Errorf("prompt_id = %q, want empty", a.PromptID)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if meta["instruction"] != "/github-pr review this" {
		t.Errorf("instruction = %v, want %q", meta["instruction"], "/github-pr review this")
	}
}

func TestAdd_PromptAndInstruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)
	writeTestPrompt(t, tqDir, "implement", `---
description: Implement
---
Instruction: {{index .Action.Meta "instruction"}}
`)

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--prompt", "implement", "--task", "1", "--title", "Add auth", "--instruction", "Add JWT middleware"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.PromptID != "implement" {
		t.Errorf("prompt_id = %q, want %q", a.PromptID, "implement")
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if meta["instruction"] != "Add JWT middleware" {
		t.Errorf("instruction = %v, want %q", meta["instruction"], "Add JWT middleware")
	}
}

func TestAdd_NoPromptNoInstruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--task", "1", "--title", "test"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --prompt and --instruction")
	}
	if !contains(err.Error(), "at least one of --prompt or --instruction") {
		t.Errorf("error = %q, want to contain 'at least one of --prompt or --instruction'", err.Error())
	}
}

func TestAdd_InstructionMergesMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--task", "1", "--title", "test", "--instruction", "do something", "--meta", `{"mode":"noninteractive"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if meta["instruction"] != "do something" {
		t.Errorf("instruction = %v, want %q", meta["instruction"], "do something")
	}
	if meta["mode"] != "noninteractive" {
		t.Errorf("mode = %v, want %q", meta["mode"], "noninteractive")
	}
}

func TestAdd_PositionalArgRejected(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "create", "review-pr", "--task", "1", "--title", "test"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for positional argument")
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

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertAction("review-pr", "review-pr", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--prompt", "review-pr", "--title", "review-pr", "--task", "1"})

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
	if !contains(err.Error(), "action #") {
		t.Errorf("error = %q, want to contain 'action #'", err.Error())
	}
	if !contains(err.Error(), "tq action cancel") {
		t.Errorf("error = %q, want to contain 'tq action cancel'", err.Error())
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

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertAction("review-pr", "review-pr", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--prompt", "review-pr", "--title", "review-pr", "--task", "1", "--force"})

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
	root.SetArgs([]string{"action", "create", "--prompt", "review-pr", "--title", "review-pr"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --task flag")
	}
	if !contains(err.Error(), "task") {
		t.Errorf("error = %q, want to contain 'task'", err.Error())
	}
}

func TestAdd_MissingMetaKey(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)
	writeTestPrompt(t, tqDir, "implement", `---
description: Implement
---
Instruction: {{index .Action.Meta "instruction"}}
`)

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--prompt", "implement", "--title", "impl task", "--task", "1"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing meta key")
	}
	if !contains(err.Error(), "requires metadata not provided in --meta") {
		t.Errorf("error = %q, want to contain 'requires metadata not provided in --meta'", err.Error())
	}
	if !contains(err.Error(), `missing metadata key "instruction"`) {
		t.Errorf("error = %q, want to contain 'missing metadata key \"instruction\"'", err.Error())
	}
}

func TestAdd_InvalidMeta(t *testing.T) {
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

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "create", "--prompt", "review-pr", "--title", "review-pr", "--task", "1", "--meta", "{invalid}"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON meta")
	}
	if !contains(err.Error(), "invalid JSON for --meta (must be a JSON object)") {
		t.Errorf("error = %q, want to contain 'invalid JSON for --meta (must be a JSON object)'", err.Error())
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
	root.SetArgs([]string{"action", "create", "--task", "1", "--title", "test"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing prompt and instruction")
	}
}
