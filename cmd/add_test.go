package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		name             string
		dispatchDisabled bool
		taskStatus       string
		args             []string
		wantErr          string
		wantOut          []string
		wantNotOut       []string
		wantMeta         map[string]any
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
			name:             "unfocused project",
			dispatchDisabled: true,
			args:             []string{"action", "create", "do something", "--task", "1", "--title", "test"},
			wantOut:          []string{"will not be auto-dispatched"},
			wantNotOut:       []string{"will be dispatched automatically"},
		},
		{
			name:       "parent task done",
			taskStatus: "done",
			args:       []string{"action", "create", "x", "--task", "1", "--title", "t"},
			wantErr:    "status=done",
		},
		{
			name:       "parent task archived",
			taskStatus: "archived",
			args:       []string{"action", "create", "x", "--task", "1", "--title", "t"},
			wantErr:    "status=archived",
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

			if tc.taskStatus != "" {
				if err := d.UpdateTask(1, tc.taskStatus, ""); err != nil {
					t.Fatal(err)
				}
			}

			if tc.dispatchDisabled {
				if err := d.SetDispatchEnabled(1, false); err != nil {
					t.Fatal(err)
				}
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

func TestAdd_WorkDir(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantWorkDir string
	}{
		{
			name:        "flag omitted defaults to empty",
			args:        []string{"action", "create", "x", "--task", "1", "--title", "t"},
			wantWorkDir: "",
		},
		{
			name:        "absolute path stored",
			args:        []string{"action", "create", "x", "--task", "1", "--title", "t", "--work-dir", "/tmp/wt"},
			wantWorkDir: "/tmp/wt",
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

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			if err := root.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			a, err := d.GetAction(1)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			if a.WorkDir != tc.wantWorkDir {
				t.Errorf("action.work_dir = %q, want %q", a.WorkDir, tc.wantWorkDir)
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

func TestAdd_InvalidMode(t *testing.T) {
	tests := []struct {
		name        string
		meta        string
		wantErrSubs []string
	}{
		{
			name: "claude permission-mode auto",
			meta: `{"mode":"auto"}`,
			wantErrSubs: []string{
				"must be one of: interactive, noninteractive, remote, experimental_bg",
				`got "auto"`,
				"claude_args",
				"--permission-mode",
			},
		},
		{
			name:        "claude permission-mode plan",
			meta:        `{"mode":"plan"}`,
			wantErrSubs: []string{`got "plan"`, "claude_args"},
		},
		{
			name:        "case sensitive Interactive",
			meta:        `{"mode":"Interactive"}`,
			wantErrSubs: []string{`got "Interactive"`},
		},
		{
			name:        "non-string mode",
			meta:        `{"mode":123}`,
			wantErrSubs: []string{`"mode" must be a string`},
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
				t.Fatal("expected error for invalid mode, got nil")
			}
			for _, sub := range tc.wantErrSubs {
				if !contains(err.Error(), sub) {
					t.Errorf("error = %q, want to contain %q", err.Error(), sub)
				}
			}

			if _, gerr := d.GetAction(1); gerr == nil {
				t.Error("action should NOT have been created when mode validation fails")
			}
		})
	}
}

func TestAdd_AutoStampExecutor(t *testing.T) {
	tests := []struct {
		name         string
		envRemote    string
		args         []string
		wantExecutor string // "" means key must be absent
	}{
		{
			name:         "status=running + REMOTE=true → executor=cloud",
			envRemote:    "true",
			args:         []string{"action", "create", "x", "--task", "1", "--title", "t", "--status", "running"},
			wantExecutor: dispatch.ExecutorCloud,
		},
		{
			name:         "status=pending + REMOTE=true → executor unset (creation env != execution env)",
			envRemote:    "true",
			args:         []string{"action", "create", "x", "--task", "1", "--title", "t"},
			wantExecutor: "",
		},
		{
			name:         "status=running + REMOTE unset → executor unset",
			envRemote:    "",
			args:         []string{"action", "create", "x", "--task", "1", "--title", "t", "--status", "running"},
			wantExecutor: "",
		},
		{
			name:         "status=running + REMOTE=true + explicit executor=local → preserved",
			envRemote:    "true",
			args:         []string{"action", "create", "x", "--task", "1", "--title", "t", "--status", "running", "--meta", `{"executor":"local"}`},
			wantExecutor: dispatch.ExecutorLocal,
		},
		{
			name:         "status=running + REMOTE=false → executor unset",
			envRemote:    "false",
			args:         []string{"action", "create", "x", "--task", "1", "--title", "t", "--status", "running"},
			wantExecutor: "",
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
			t.Setenv("CLAUDE_CODE_REMOTE", tc.envRemote)

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v (output: %s)", err, buf.String())
			}

			a, err := d.GetAction(1)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			var meta map[string]any
			if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
				t.Fatalf("parse metadata: %v", err)
			}
			got, _ := meta[dispatch.MetaKeyExecutor].(string)
			if got != tc.wantExecutor {
				t.Errorf("metadata.executor = %q, want %q", got, tc.wantExecutor)
			}
		})
	}
}

func TestAdd_ApplyDefaultMode(t *testing.T) {
	tests := []struct {
		name         string
		globalMode   string // "" = leave settings unset
		meta         string // "" = no --meta flag
		wantMode     string // "" = metadata.mode key must be absent
		wantErrSub   string // non-empty = expect error, action not created
		wantPreserve map[string]string
	}{
		{
			name:     "global unset, no meta → mode absent",
			wantMode: "",
		},
		{
			name:       "global set, no meta → stamped",
			globalMode: "experimental_bg",
			wantMode:   "experimental_bg",
		},
		{
			name:       "explicit meta mode overrides global",
			globalMode: "experimental_bg",
			meta:       `{"mode":"interactive"}`,
			wantMode:   "interactive",
		},
		{
			name:         "stamp preserves other metadata keys",
			globalMode:   "noninteractive",
			meta:         `{"claude_args":["--effort","high"]}`,
			wantMode:     "noninteractive",
			wantPreserve: map[string]string{"claude_args": `["high"]`},
		},
		{
			name:       "invalid configured mode fails create",
			globalMode: "bogus",
			wantErrSub: `configured default mode "bogus" is invalid`,
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
			if tc.globalMode != "" {
				if err := d.SetSetting(db.SettingDefaultMode, tc.globalMode); err != nil {
					t.Fatalf("seed setting: %v", err)
				}
			}

			args := []string{"action", "create", "x", "--task", "1", "--title", "t"}
			if tc.meta != "" {
				args = append(args, "--meta", tc.meta)
			}
			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(args)

			err := root.Execute()
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrSub)
				}
				if _, gerr := d.GetAction(1); gerr == nil {
					t.Error("action should NOT have been created on invalid configured mode")
				}
				return
			}
			if err != nil {
				t.Fatalf("execute: %v (output: %s)", err, buf.String())
			}

			a, err := d.GetAction(1)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			var meta map[string]any
			if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
				t.Fatalf("parse metadata: %v", err)
			}
			got, _ := meta[dispatch.MetaKeyMode].(string)
			if got != tc.wantMode {
				t.Errorf("metadata.mode = %q, want %q", got, tc.wantMode)
			}
			if tc.wantPreserve != nil {
				if _, ok := meta["claude_args"]; !ok {
					t.Errorf("claude_args dropped during default-mode stamp; metadata=%s", a.Metadata)
				}
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
