package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, d *db.DB)
		args       []string
		wantErr    string
		wantOut    []string
		wantNotOut []string
		wantMeta   map[string]any
	}{
		{
			name:     "positional arg",
			args:     []string{"action", "create", "/github-pr review this", "--task", "1", "--title", "Review PR"},
			wantOut:  []string{"action #1 created"},
			wantMeta: map[string]any{"instruction": "/github-pr review this"},
		},
		{
			name:    "no instruction",
			args:    []string{"action", "create", "--task", "1", "--title", "test"},
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name:     "instruction merges meta",
			args:     []string{"action", "create", "do something", "--task", "1", "--title", "test", "--meta", `{"mode":"noninteractive"}`},
			wantMeta: map[string]any{"instruction": "do something", "mode": "noninteractive"},
		},
		{
			name:    "missing task",
			args:    []string{"action", "create", "review this", "--title", "review-pr"},
			wantErr: "task",
		},
		{
			name: "unfocused project",
			setup: func(t *testing.T, d *db.DB) {
				t.Helper()
				if err := d.SetDispatchEnabled(1, false); err != nil {
					t.Fatal(err)
				}
			},
			args:       []string{"action", "create", "do something", "--task", "1", "--title", "test"},
			wantOut:    []string{"will not be auto-dispatched"},
			wantNotOut: []string{"will be dispatched automatically"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()
			cmd.SetConfigDir(t.TempDir())
			d.InsertTask(1, "test task", "{}", "")

			if tc.setup != nil {
				tc.setup(t, d)
			}

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			err := root.Execute()
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			out := buf.String()
			for _, want := range tc.wantOut {
				if !contains(out, want) {
					t.Errorf("output = %q, want to contain %q", out, want)
				}
			}
			for _, notWant := range tc.wantNotOut {
				if contains(out, notWant) {
					t.Errorf("output = %q, should not contain %q", out, notWant)
				}
			}

			if len(tc.wantMeta) > 0 {
				a, err := d.GetAction(1)
				if err != nil {
					t.Fatalf("get action: %v", err)
				}
				var meta map[string]any
				if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
					t.Fatalf("parse metadata: %v", err)
				}
				for k, want := range tc.wantMeta {
					if got := meta[k]; got != want {
						t.Errorf("meta[%q] = %v, want %v", k, got, want)
					}
				}
			}
		})
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
		{
			name:    "legacy permission_mode rejected",
			meta:    `{"permission_mode":"plan"}`,
			wantErr: `metadata key "permission_mode" is no longer supported`,
		},
		{
			name:    "legacy worktree rejected",
			meta:    `{"worktree":true}`,
			wantErr: `metadata key "worktree" is no longer supported`,
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
