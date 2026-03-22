package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

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

	var meta map[string]any
	if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if meta["instruction"] != "/github-pr review this" {
		t.Errorf("instruction = %v, want %q", meta["instruction"], "/github-pr review this")
	}
}

func TestAdd_NoInstruction(t *testing.T) {
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
		t.Fatal("expected error for missing --instruction")
	}
	if !contains(err.Error(), "--instruction is required") {
		t.Errorf("error = %q, want to contain '--instruction is required'", err.Error())
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

func TestAdd_MissingTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "--title", "review-pr", "--instruction", "review this"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --task flag")
	}
	if !contains(err.Error(), "task") {
		t.Errorf("error = %q, want to contain 'task'", err.Error())
	}
}

func TestAdd_InvalidMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "create", "--title", "review-pr", "--task", "1", "--instruction", "review this", "--meta", "{invalid}"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON meta")
	}
	if !contains(err.Error(), "invalid JSON for --meta (must be a JSON object)") {
		t.Errorf("error = %q, want to contain 'invalid JSON for --meta (must be a JSON object)'", err.Error())
	}
}
