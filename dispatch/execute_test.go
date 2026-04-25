package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestExecuteAction(t *testing.T) {
	tests := []struct {
		name              string
		instruction       string // empty → "do the task"
		promptMode        string
		claudeArgs        []string
		workerResult      string
		workerErr         error
		workerDenials     []PermissionDenial
		sessionID         string // non-empty → install mockSessionLogChecker
		beforeInteractive func(*db.Action) error
		wantMode          string
		wantStatus        string
		wantErrType       string // "" | "failed" | "deferred"
		wantErrSubstr     string
		wantWorkerCount   int
		wantFollowUps     int
		wantSessionID     string
	}{
		{
			name:            "noninteractive success",
			promptMode:      "noninteractive",
			workerResult:    `{"ok":true}`,
			wantMode:        ModeNonInteractive,
			wantStatus:      db.ActionStatusDone,
			wantWorkerCount: 1,
		},
		{
			name:            "noninteractive worker failure",
			promptMode:      "noninteractive",
			workerErr:       context.DeadlineExceeded,
			wantStatus:      db.ActionStatusFailed,
			wantErrType:     "failed",
			wantWorkerCount: 1,
		},
		{
			name:       "interactive deferred by BeforeInteractive",
			promptMode: "interactive",
			beforeInteractive: func(_ *db.Action) error {
				return ErrInteractiveDeferred
			},
			wantStatus:      db.ActionStatusPending,
			wantErrType:     "deferred",
			wantWorkerCount: 0,
		},
		{
			name:            "remote success",
			promptMode:      "remote",
			workerResult:    "remote:session=https://example.com/session/abc",
			wantMode:        ModeRemote,
			wantStatus:      db.ActionStatusDispatched,
			wantWorkerCount: 1,
		},
		{
			name:            "instruction only defaults to interactive",
			instruction:     "/github-pr review this",
			workerResult:    "interactive:done",
			wantMode:        ModeInteractive,
			wantStatus:      db.ActionStatusRunning,
			wantWorkerCount: 1,
		},
		{
			name:            "instruction with mode override",
			instruction:     "do something",
			promptMode:      "noninteractive",
			workerResult:    `{"ok":true}`,
			wantMode:        ModeNonInteractive,
			wantStatus:      db.ActionStatusDone,
			wantWorkerCount: 1,
		},
		{
			name:         "noninteractive denials create follow-up",
			promptMode:   "noninteractive",
			workerResult: "ok",
			workerDenials: []PermissionDenial{
				{ToolName: "Bash", Input: map[string]any{"command": "gh api notifications"}},
			},
			wantMode:        ModeNonInteractive,
			wantStatus:      db.ActionStatusDone,
			wantWorkerCount: 1,
			wantFollowUps:   1,
		},
		{
			name:            "noninteractive no denials no follow-up",
			promptMode:      "noninteractive",
			workerResult:    "ok",
			workerDenials:   []PermissionDenial{},
			wantMode:        ModeNonInteractive,
			wantStatus:      db.ActionStatusDone,
			wantWorkerCount: 1,
			wantFollowUps:   0,
		},
		{
			name:            "valid claude_args",
			promptMode:      "noninteractive",
			claudeArgs:      []string{"--max-turns", "5", "--model", "opus"},
			workerResult:    `{"ok":true}`,
			wantMode:        ModeNonInteractive,
			wantStatus:      db.ActionStatusDone,
			wantWorkerCount: 1,
		},
		{
			name:            "blocked claude_args fail before invoking worker",
			promptMode:      "noninteractive",
			claudeArgs:      []string{"--output-format", "text"},
			wantStatus:      db.ActionStatusFailed,
			wantErrSubstr:   "claude_args cannot include",
			wantWorkerCount: 0,
		},
		{
			name:            "noninteractive saves claude_session_id on success",
			promptMode:      "noninteractive",
			workerResult:    `{"ok":true}`,
			sessionID:       "sess-noninteractive",
			wantMode:        ModeNonInteractive,
			wantStatus:      db.ActionStatusDone,
			wantWorkerCount: 1,
			wantSessionID:   "sess-noninteractive",
		},
		{
			name:            "noninteractive saves claude_session_id on failure",
			promptMode:      "noninteractive",
			workerErr:       context.DeadlineExceeded,
			sessionID:       "sess-failed",
			wantStatus:      db.ActionStatusFailed,
			wantErrType:     "failed",
			wantWorkerCount: 1,
			wantSessionID:   "sess-failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			instruction := tc.instruction
			if instruction == "" {
				instruction = "do the task"
			}
			metaMap := map[string]any{"instruction": instruction}
			if tc.promptMode != "" {
				metaMap["mode"] = tc.promptMode
			}
			if len(tc.claudeArgs) > 0 {
				metaMap["claude_args"] = tc.claudeArgs
			}
			meta, _ := json.Marshal(metaMap)

			taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
			d.InsertAction("test", taskID, string(meta), db.ActionStatusPending, nil)

			action, _ := d.NextPending(context.Background())

			worker := &countingWorker{
				result:  tc.workerResult,
				err:     tc.workerErr,
				denials: tc.workerDenials,
			}
			workerFunc := func() Worker { return worker }

			cfg := DispatchConfig{
				DB:                 d,
				NonInteractiveFunc: workerFunc,
				InteractiveFunc:    workerFunc,
				RemoteFunc:         workerFunc,
			}
			if tc.sessionID != "" {
				cfg.SessionLogChecker = &mockSessionLogChecker{active: true, sessionID: tc.sessionID}
			}

			result, err := ExecuteAction(context.Background(), ExecuteParams{
				DispatchConfig:    cfg,
				BeforeInteractive: tc.beforeInteractive,
			}, action)

			switch tc.wantErrType {
			case "failed":
				var af *ActionFailedError
				if !errors.As(err, &af) {
					t.Fatalf("expected ActionFailedError, got %v", err)
				}
			case "deferred":
				if !errors.Is(err, ErrInteractiveDeferred) {
					t.Fatalf("expected ErrInteractiveDeferred, got %v", err)
				}
			default:
				if tc.wantErrSubstr == "" {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if result.Mode != tc.wantMode {
						t.Errorf("mode = %q, want %q", result.Mode, tc.wantMode)
					}
				}
			}

			if tc.wantErrSubstr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr)) {
				t.Errorf("error = %v, want substring %q", err, tc.wantErrSubstr)
			}

			if worker.count != tc.wantWorkerCount {
				t.Errorf("worker.count = %d, want %d", worker.count, tc.wantWorkerCount)
			}

			a, _ := d.GetAction(action.ID)
			if a.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", a.Status, tc.wantStatus)
			}

			actions, _ := d.ListActions("", nil, 0)
			var followups int
			for _, x := range actions {
				if hasMetaKey(x.Metadata, MetaKeyIsPermissionBlock) && x.Status == db.ActionStatusPending {
					followups++
				}
			}
			if followups != tc.wantFollowUps {
				t.Errorf("permission-block follow-ups = %d, want %d", followups, tc.wantFollowUps)
			}

			if tc.wantSessionID != "" {
				var m map[string]any
				_ = json.Unmarshal([]byte(a.Metadata), &m)
				if m["claude_session_id"] != tc.wantSessionID {
					t.Errorf("claude_session_id = %v, want %q", m["claude_session_id"], tc.wantSessionID)
				}
			}
		})
	}
}

func TestWrapInstruction(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		wantDone    bool
		wantHistory bool
	}{
		{
			name:        "interactive includes /tq:done and /tq:failed",
			mode:        ModeInteractive,
			wantDone:    true,
			wantHistory: true,
		},
		{
			name:        "noninteractive includes /tq:done and /tq:failed",
			mode:        ModeNonInteractive,
			wantDone:    true,
			wantHistory: true,
		},
		{
			name:        "remote excludes /tq:done and /tq:failed",
			mode:        ModeRemote,
			wantDone:    false,
			wantHistory: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapInstruction("Fix the bug", 99, 42, tc.mode)
			if !strings.Contains(got, "action #99") {
				t.Error("should contain action ID")
			}
			if !strings.Contains(got, "task #42") {
				t.Error("should contain task ID")
			}
			if tc.wantHistory && !strings.Contains(got, "tq action list --task 42") {
				t.Error("should contain postamble with tq action list and task ID")
			}
			if !strings.Contains(got, "Fix the bug") {
				t.Error("should contain the original instruction")
			}
			if tc.wantDone && !strings.Contains(got, "/tq:done") {
				t.Error("should contain /tq:done postamble")
			}
			if !tc.wantDone && strings.Contains(got, "/tq:done") {
				t.Error("should NOT contain /tq:done postamble")
			}
			if tc.wantDone && !strings.Contains(got, "/tq:failed") {
				t.Error("should contain /tq:failed postamble")
			}
			if !tc.wantDone && strings.Contains(got, "/tq:failed") {
				t.Error("should NOT contain /tq:failed postamble")
			}
		})
	}
}

func TestWrapInstruction_InstructionComesFirst(t *testing.T) {
	got := wrapInstruction("/daily-report", 100, 50, ModeNonInteractive)
	if !strings.HasPrefix(got, "/daily-report") {
		t.Errorf("instruction should be the first line, got: %q", got[:min(len(got), 80)])
	}
}

func TestValidateActionMetadata(t *testing.T) {
	tests := []struct {
		name    string
		meta    map[string]any
		wantErr bool
	}{
		{
			name:    "valid instruction",
			meta:    map[string]any{MetaKeyInstruction: "do something"},
			wantErr: false,
		},
		{
			name:    "missing instruction key",
			meta:    map[string]any{"mode": "interactive"},
			wantErr: true,
		},
		{
			name:    "empty instruction",
			meta:    map[string]any{MetaKeyInstruction: ""},
			wantErr: true,
		},
		{
			name:    "whitespace-only instruction",
			meta:    map[string]any{MetaKeyInstruction: "   "},
			wantErr: true,
		},
		{
			name:    "non-string instruction",
			meta:    map[string]any{MetaKeyInstruction: 123},
			wantErr: true,
		},
		{
			name:    "empty metadata",
			meta:    map[string]any{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateActionMetadata(tc.meta)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveWorkDir_Recovery(t *testing.T) {
	tests := []struct {
		name          string
		taskWorkDir   string
		existingPaths map[string]bool // path -> exists
		wantWorkDir   string
		wantRecovery  bool
	}{
		{
			name:          "task work_dir exists on disk",
			taskWorkDir:   "/valid/path",
			existingPaths: map[string]bool{"/valid/path": true},
			wantWorkDir:   "/valid/path",
			wantRecovery:  false,
		},
		{
			name:          "task work_dir missing, project work_dir exists",
			taskWorkDir:   "/gone/worktree",
			existingPaths: map[string]bool{"/gone/worktree": false, "/project/dir": true},
			wantWorkDir:   "/project/dir",
			wantRecovery:  true,
		},
		{
			name:          "task work_dir missing, project work_dir also missing",
			taskWorkDir:   "/gone/worktree",
			existingPaths: map[string]bool{"/gone/worktree": false, "/project/dir": false},
			wantWorkDir:   ".",
			wantRecovery:  true,
		},
		{
			name:          "task work_dir not set, falls through to project",
			taskWorkDir:   "",
			existingPaths: map[string]bool{"/project/dir": true},
			wantWorkDir:   "/project/dir",
			wantRecovery:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restore := SetDirExists(func(path string) bool {
				if exists, ok := tc.existingPaths[path]; ok {
					return exists
				}
				return false
			})
			defer restore()

			d := testutil.NewTestDB(t)
			projID, err := d.InsertProject("test-proj", "/project/dir", "{}")
			if err != nil {
				t.Fatal(err)
			}
			taskID, err := d.InsertTask(projID, "test-task", "{}", tc.taskWorkDir)
			if err != nil {
				t.Fatal(err)
			}

			meta, _ := json.Marshal(map[string]any{MetaKeyInstruction: "do something"})
			actionID, err := d.InsertAction("test", taskID, string(meta), db.ActionStatusPending, nil)
			if err != nil {
				t.Fatal(err)
			}
			action, err := d.GetAction(actionID)
			if err != nil {
				t.Fatal(err)
			}

			got, recovery, err := resolveWorkDir(d, action)
			if err != nil {
				t.Fatalf("resolveWorkDir() unexpected error: %v", err)
			}
			if got != tc.wantWorkDir {
				t.Errorf("resolveWorkDir() = %q, want %q", got, tc.wantWorkDir)
			}
			if (recovery != nil) != tc.wantRecovery {
				t.Errorf("recovery presence = %v, want %v", recovery != nil, tc.wantRecovery)
			}

			// resolveWorkDir is read-only.
			taskBefore, _ := d.GetTask(taskID)
			if taskBefore.WorkDir != tc.taskWorkDir {
				t.Errorf("after resolveWorkDir: task.WorkDir = %q, want unchanged %q",
					taskBefore.WorkDir, tc.taskWorkDir)
			}

			applyWorkDirRecovery(d, recovery)
			taskAfter, _ := d.GetTask(taskID)
			if tc.wantRecovery {
				if taskAfter.WorkDir != "" {
					t.Errorf("after applyWorkDirRecovery: task.WorkDir = %q, want cleared",
						taskAfter.WorkDir)
				}
				if recovery.TaskID != taskID {
					t.Errorf("recovery.TaskID = %d, want %d", recovery.TaskID, taskID)
				}
				if recovery.Fallback != tc.wantWorkDir {
					t.Errorf("recovery.Fallback = %q, want %q", recovery.Fallback, tc.wantWorkDir)
				}
			} else {
				if taskAfter.WorkDir != tc.taskWorkDir {
					t.Errorf("no recovery expected, but task.WorkDir = %q, want %q",
						taskAfter.WorkDir, tc.taskWorkDir)
				}
			}
		})
	}
}

// TestResolveWorkDir_NoWriteFromReaperPath asserts that resolveWorkDir is
// pure: callers that ignore the recovery descriptor (e.g. the reaper) never
// trigger a DB write, even on the fallback path.
func TestResolveWorkDir_NoWriteFromReaperPath(t *testing.T) {
	restore := SetDirExists(func(path string) bool {
		return path == "/project/dir"
	})
	defer restore()

	d := testutil.NewTestDB(t)
	projID, err := d.InsertProject("test-proj", "/project/dir", "{}")
	if err != nil {
		t.Fatal(err)
	}
	taskID, err := d.InsertTask(projID, "test-task", "{}", "/gone/worktree")
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]any{MetaKeyInstruction: "do something"})
	actionID, err := d.InsertAction("test", taskID, string(meta), db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	action, err := d.GetAction(actionID)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := resolveWorkDir(d, action); err != nil {
		t.Fatalf("resolveWorkDir error: %v", err)
	}

	task, _ := d.GetTask(taskID)
	if task.WorkDir != "/gone/worktree" {
		t.Errorf("task.WorkDir = %q, want unchanged %q",
			task.WorkDir, "/gone/worktree")
	}
}

func TestValidateClaudeArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "nil", args: nil, wantErr: false},
		{name: "empty", args: []string{}, wantErr: false},
		{name: "valid", args: []string{"--max-turns", "5", "--model", "opus"}, wantErr: false},
		{name: "blocked -p", args: []string{"-p"}, wantErr: true},
		{name: "blocked --print", args: []string{"--print"}, wantErr: true},
		{name: "blocked --output-format", args: []string{"--output-format", "json"}, wantErr: true},
		{name: "blocked --remote", args: []string{"--remote"}, wantErr: true},
		{name: "blocked mixed", args: []string{"--max-turns", "3", "--remote"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClaudeArgs(tc.args)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name string
		raw  []any
		want []string
	}{
		{name: "strings", raw: []any{"--max-turns", "5"}, want: []string{"--max-turns", "5"}},
		{name: "mixed types", raw: []any{"--max-turns", 5}, want: []string{"--max-turns"}},
		{name: "empty", raw: []any{}, want: []string{}},
		{name: "nil", raw: nil, want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toStringSlice(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d: %v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestExecuteAction_NoInstruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	d.InsertAction("task", taskID, "{}", db.ActionStatusPending, nil)

	action, _ := d.NextPending(context.Background())

	worker := &countingWorker{}
	workerFunc := func() Worker { return worker }

	_, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
	}, action)

	if err == nil {
		t.Fatal("expected error for no instruction")
	}

	a, _ := d.GetAction(action.ID)
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
	}
}

// markFailedErrStore wraps a db.Store and forces MarkFailed to return an error,
// simulating a transient write error or FK violation.
type markFailedErrStore struct {
	db.Store
	err            error
	markFailedHits int
}

func (s *markFailedErrStore) MarkFailed(id int64, result string) error {
	s.markFailedHits++
	_ = s.Store.MarkFailed(id, result) // best-effort underlying call so investigation lookup works as it would in prod
	return s.err
}

func TestExecuteAction_MarkFailedErrorIsLogged(t *testing.T) {
	tests := []struct {
		name       string
		promptMode string
	}{
		{name: "noninteractive worker failure", promptMode: ModeNonInteractive},
		{name: "interactive worker failure", promptMode: ModeInteractive},
		{name: "remote worker failure", promptMode: ModeRemote},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			origLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
			t.Cleanup(func() { slog.SetDefault(origLogger) })

			base := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, base)

			markFailedErr := errors.New("simulated FK violation")
			store := &markFailedErrStore{Store: base, err: markFailedErr}

			meta, _ := json.Marshal(map[string]any{"instruction": "do the task", "mode": tc.promptMode})
			taskID, _ := base.InsertTask(1, "Test task", `{}`, "")
			actionID, _ := base.InsertAction("test", taskID, string(meta), db.ActionStatusPending, nil)

			action, _ := base.NextPending(context.Background())

			worker := &countingWorker{err: fmt.Errorf("boom")}
			workerFunc := func() Worker { return worker }

			_, err := ExecuteAction(context.Background(), ExecuteParams{
				DispatchConfig: DispatchConfig{
					DB:                 store,
					NonInteractiveFunc: workerFunc,
					InteractiveFunc:    workerFunc,
					RemoteFunc:         workerFunc,
				},
			}, action)

			var af *ActionFailedError
			if !errors.As(err, &af) {
				t.Fatalf("expected ActionFailedError, got %v", err)
			}
			if af.ActionID != actionID {
				t.Errorf("ActionID = %d, want %d", af.ActionID, actionID)
			}
			if store.markFailedHits == 0 {
				t.Fatalf("expected MarkFailed to be called on wrapper store")
			}

			logs := logBuf.String()
			if !strings.Contains(logs, "mark action failed") {
				t.Errorf("expected log line for mark action failed, got:\n%s", logs)
			}
			if !strings.Contains(logs, fmt.Sprintf("action_id=%d", actionID)) {
				t.Errorf("expected log to contain action_id=%d, got:\n%s", actionID, logs)
			}
			if !strings.Contains(logs, "simulated FK violation") {
				t.Errorf("expected log to contain underlying error, got:\n%s", logs)
			}
		})
	}
}

func TestExecuteAction_NonInteractiveNilCheckerNoError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	meta, _ := json.Marshal(map[string]any{"instruction": "do the task", "mode": "noninteractive"})
	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	d.InsertAction("check", taskID, string(meta), db.ActionStatusPending, nil)

	action, _ := d.NextPending(context.Background())

	worker := &countingWorker{result: `{"ok":true}`}

	_, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return worker },
			InteractiveFunc:    func() Worker { return worker },
			RemoteFunc:         func() Worker { return worker },
		},
	}, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
