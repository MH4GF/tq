package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestAdd_PositionalArg(t *testing.T) {
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
	root.SetArgs([]string{"action", "create", "/github-pr review this", "--task", "1", "--title", "Review PR"})

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
		t.Fatal("expected error for missing instruction")
	}
	if !contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Errorf("error = %q, want to contain 'accepts 1 arg(s), received 0'", err.Error())
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
	root.SetArgs([]string{"action", "create", "do something", "--task", "1", "--title", "test", "--meta", `{"mode":"noninteractive"}`})

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

func TestAdd_MissingTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "review this", "--title", "review-pr"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --task flag")
	}
	if !contains(err.Error(), "task") {
		t.Errorf("error = %q, want to contain 'task'", err.Error())
	}
}

func TestAdd_UnfocusedProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	if err := d.SetDispatchEnabled(1, false); err != nil {
		t.Fatal(err)
	}
	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "create", "do something", "--task", "1", "--title", "test"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "will not be auto-dispatched") {
		t.Errorf("output = %q, want to contain 'will not be auto-dispatched'", out)
	}
	if contains(out, "will be dispatched automatically") {
		t.Errorf("output = %q, should not contain 'will be dispatched automatically'", out)
	}
}

func TestAdd_InvalidInstruction(t *testing.T) {
	tests := []struct {
		name        string
		instruction string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			d.InsertTask(1, "test task", "{}", "")

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"action", "create", tc.instruction, "--task", "1", "--title", "test"})

			err := root.Execute()
			if err == nil {
				t.Fatal("expected error for invalid instruction")
			}
			if !contains(err.Error(), "instruction must not be empty") {
				t.Errorf("error = %q, want to contain 'instruction must not be empty'", err.Error())
			}
		})
	}
}

func TestAdd_ClaudeArgsValid(t *testing.T) {
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
	root.SetArgs([]string{"action", "create", "do something", "--task", "1", "--title", "test", "--meta", `{"claude_args":["--max-turns","5"]}`})

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
	rawArgs, ok := meta["claude_args"].([]any)
	if !ok {
		t.Fatalf("claude_args not found or wrong type in metadata: %v", meta)
	}
	if len(rawArgs) != 2 || rawArgs[0] != "--max-turns" || rawArgs[1] != "5" {
		t.Errorf("claude_args = %v, want [--max-turns 5]", rawArgs)
	}
}

func TestAdd_ClaudeArgsInvalidType(t *testing.T) {
	tests := []struct {
		name    string
		meta    string
		wantErr string
	}{
		{
			name:    "string instead of array",
			meta:    `{"claude_args":"--max-turns 5"}`,
			wantErr: "claude_args must be a JSON array of strings",
		},
		{
			name:    "array with non-string element",
			meta:    `{"claude_args":["--max-turns",5]}`,
			wantErr: "claude_args[1] must be a string",
		},
		{
			name:    "blocked flag",
			meta:    `{"claude_args":["--output-format","text"]}`,
			wantErr: "claude_args cannot include",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			d.InsertTask(1, "test task", "{}", "")

			root := cmd.GetRootCmd()
			root.SetOut(new(bytes.Buffer))
			root.SetErr(new(bytes.Buffer))
			root.SetArgs([]string{"action", "create", "review this", "--title", "test", "--task", "1", "--meta", tc.meta})

			err := root.Execute()
			if err == nil {
				t.Fatal("expected error for invalid claude_args")
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErr)
			}
		})
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
	root.SetArgs([]string{"action", "create", "review this", "--title", "review-pr", "--task", "1", "--meta", "{invalid}"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON meta")
	}
	if !contains(err.Error(), "invalid JSON for --meta (must be a JSON object)") {
		t.Errorf("error = %q, want to contain 'invalid JSON for --meta (must be a JSON object)'", err.Error())
	}
}
