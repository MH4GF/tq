package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestExecuteAction(t *testing.T) {
	tests := []struct {
		name              string
		promptMode        string
		workerResult      string
		workerErr         error
		beforeInteractive func(*db.Action) error
		wantMode          string
		wantStatus        string
		wantErrType       string // "failed", "deferred", or ""
		wantWorkerCount   int
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
			beforeInteractive: func(a *db.Action) error {
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			meta, _ := json.Marshal(map[string]any{"instruction": "do the task", "mode": tc.promptMode})
			taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
			d.InsertAction("test-"+tc.promptMode, taskID, string(meta), db.ActionStatusPending, nil)

			action, _ := d.NextPending(context.Background())

			worker := &countingWorker{result: tc.workerResult, err: tc.workerErr}
			workerFunc := func() Worker { return worker }

			result, err := ExecuteAction(context.Background(), ExecuteParams{
				DispatchConfig: DispatchConfig{
					DB:                 d,
					NonInteractiveFunc: workerFunc,
					InteractiveFunc:    workerFunc,
					RemoteFunc:         workerFunc,
				},
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
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result.Mode != tc.wantMode {
					t.Errorf("mode = %q, want %q", result.Mode, tc.wantMode)
				}
			}

			if worker.count != tc.wantWorkerCount {
				t.Errorf("worker.count = %d, want %d", worker.count, tc.wantWorkerCount)
			}

			a, _ := d.GetAction(action.ID)
			if a.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", a.Status, tc.wantStatus)
			}
		})
	}
}

func TestExecuteAction_InstructionOnly(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	meta, _ := json.Marshal(map[string]any{"instruction": "/github-pr review this"})
	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("review", taskID, string(meta), db.ActionStatusPending, nil)

	action, _ := d.NextPending(context.Background())

	worker := &countingWorker{result: "interactive:done"}
	workerFunc := func() Worker { return worker }

	result, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
	}, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Mode != ModeInteractive {
		t.Errorf("mode = %q, want %q", result.Mode, ModeInteractive)
	}
	if worker.count != 1 {
		t.Errorf("worker.count = %d, want 1", worker.count)
	}
}

func TestExecuteAction_InstructionWithModeOverride(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	meta, _ := json.Marshal(map[string]any{"instruction": "do something", "mode": "noninteractive"})
	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("task", taskID, string(meta), db.ActionStatusPending, nil)

	action, _ := d.NextPending(context.Background())

	worker := &countingWorker{result: `{"ok":true}`}
	workerFunc := func() Worker { return worker }

	result, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
	}, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Mode != ModeNonInteractive {
		t.Errorf("mode = %q, want %q", result.Mode, ModeNonInteractive)
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
				t.Error("should contain preamble with tq action list and task ID")
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

type denialsWorker struct {
	result  string
	denials []PermissionDenial
}

func (w *denialsWorker) Execute(_ context.Context, _ string, _ ActionConfig, _ string, _, _ int64) (string, error) {
	return w.result, nil
}

func (w *denialsWorker) LastDenials() []PermissionDenial {
	return w.denials
}

func TestExecuteAction_NonInteractiveDenialsCreatesFollowUp(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	meta, _ := json.Marshal(map[string]any{"instruction": "do the task", "mode": "noninteractive"})
	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	d.InsertAction("watch", taskID, string(meta), db.ActionStatusPending, nil)

	action, _ := d.NextPending(context.Background())

	worker := &denialsWorker{
		result: "ok",
		denials: []PermissionDenial{
			{ToolName: "Bash", Input: map[string]any{"command": "gh api notifications"}},
		},
	}
	workerFunc := func() Worker { return worker }

	result, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
	}, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Mode != ModeNonInteractive {
		t.Errorf("mode = %q, want %q", result.Mode, ModeNonInteractive)
	}

	// Original action is done
	a, _ := d.GetAction(action.ID)
	if a.Status != db.ActionStatusDone {
		t.Errorf("original action status = %q, want done", a.Status)
	}

	// Follow-up permission_block action exists in pending
	actions, _ := d.ListActions("", nil, 0)
	var followupCount int
	for _, x := range actions {
		if hasMetaKey(x.Metadata, MetaKeyIsPermissionBlock) && x.Status == db.ActionStatusPending {
			followupCount++
		}
	}
	if followupCount != 1 {
		t.Errorf("expected 1 pending permission-block follow-up, got %d", followupCount)
	}
}

func TestExecuteAction_NonInteractiveNoDenials(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	meta, _ := json.Marshal(map[string]any{"instruction": "do the task", "mode": "noninteractive"})
	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	d.InsertAction("watch", taskID, string(meta), db.ActionStatusPending, nil)

	action, _ := d.NextPending(context.Background())

	worker := &denialsWorker{result: "ok", denials: nil}
	workerFunc := func() Worker { return worker }

	if _, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
	}, action); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil, 0)
	for _, x := range actions {
		if hasMetaKey(x.Metadata, MetaKeyIsPermissionBlock) {
			t.Errorf("did not expect permission-block follow-up, found one")
		}
	}
}

func TestExecuteAction_ClaudeArgs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	meta, _ := json.Marshal(map[string]any{
		"instruction": "do the task",
		"mode":        "noninteractive",
		"claude_args": []string{"--max-turns", "5", "--model", "opus"},
	})
	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	d.InsertAction("with-args", taskID, string(meta), db.ActionStatusPending, nil)

	action, _ := d.NextPending(context.Background())

	worker := &countingWorker{result: `{"ok":true}`}
	workerFunc := func() Worker { return worker }

	result, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
	}, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Mode != ModeNonInteractive {
		t.Errorf("mode = %q, want %q", result.Mode, ModeNonInteractive)
	}
}

func TestExecuteAction_ClaudeArgsBlocked(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	meta, _ := json.Marshal(map[string]any{
		"instruction": "do the task",
		"mode":        "noninteractive",
		"claude_args": []string{"--output-format", "text"},
	})
	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	d.InsertAction("blocked-args", taskID, string(meta), db.ActionStatusPending, nil)

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
		t.Fatal("expected error for blocked claude_args")
	}
	if !strings.Contains(err.Error(), "claude_args cannot include") {
		t.Errorf("error = %q, want to contain 'claude_args cannot include'", err.Error())
	}

	a, _ := d.GetAction(action.ID)
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
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
