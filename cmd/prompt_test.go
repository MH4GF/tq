package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestPromptList_JSON(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tmpDir := t.TempDir()
	cmd.SetConfigDir(tmpDir)

	writeTestPrompt(t, tmpDir, "review-pr", `---
description: Review a PR
mode: noninteractive
on_done: done
---
Body.
`)
	writeTestPrompt(t, tmpDir, "implement", `---
description: Implement a feature
mode: interactive
---
Body.
`)

	// Run from a dir with no project prompts
	origDir, _ := os.Getwd()
	noProjectDir := t.TempDir()
	os.Chdir(noProjectDir)
	defer os.Chdir(origDir)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"prompt", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Sorted by ID: implement < review-pr
	if rows[0]["id"] != "implement" {
		t.Errorf("first row id = %v, want %q", rows[0]["id"], "implement")
	}
	if rows[0]["description"] != "Implement a feature" {
		t.Errorf("description = %v, want %q", rows[0]["description"], "Implement a feature")
	}
	if rows[0]["scope"] != "user" {
		t.Errorf("scope = %v, want %q", rows[0]["scope"], "user")
	}
	if rows[1]["id"] != "review-pr" {
		t.Errorf("second row id = %v, want %q", rows[1]["id"], "review-pr")
	}
	if rows[1]["mode"] != "noninteractive" {
		t.Errorf("mode = %v, want %q", rows[1]["mode"], "noninteractive")
	}
	if rows[1]["on_done"] != "done" {
		t.Errorf("on_done = %v, want %q", rows[1]["on_done"], "done")
	}
}

func TestPromptList_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tmpDir := t.TempDir()
	cmd.SetConfigDir(tmpDir)

	// Empty prompts dir
	os.MkdirAll(filepath.Join(tmpDir, "prompts"), 0o755)

	origDir, _ := os.Getwd()
	noProjectDir := t.TempDir()
	os.Chdir(noProjectDir)
	defer os.Chdir(origDir)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"prompt", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "no prompts found") {
		t.Errorf("output = %q, want 'no prompts found'", out)
	}
}

func TestPromptList_ProjectOverridesUser(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tmpDir := t.TempDir()
	cmd.SetConfigDir(tmpDir)

	writeTestPrompt(t, tmpDir, "review-pr", `---
description: User review
mode: interactive
---
Body.
`)

	// Create project dir with project-scope prompts
	projectDir := t.TempDir()
	writeTestPrompt(t, projectDir, "review-pr", `---
description: Project review
mode: noninteractive
---
Body.
`)

	origDir, _ := os.Getwd()
	os.Chdir(projectDir)
	defer os.Chdir(origDir)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"prompt", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if rows[0]["description"] != "Project review" {
		t.Errorf("description = %v, want %q (project should override user)", rows[0]["description"], "Project review")
	}
	if rows[0]["scope"] != "project" {
		t.Errorf("scope = %v, want %q", rows[0]["scope"], "project")
	}
}
