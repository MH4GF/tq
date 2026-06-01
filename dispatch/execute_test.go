package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

type fakeWorker struct {
	output string
	err    error

	calls       int
	gotMode     string
	gotInstr    string
	gotWorkDir  string
	gotActionID int64
}

func (f *fakeWorker) Execute(_ context.Context, instruction string, cfg ActionConfig, workDir string, actionID, _ int64) (string, error) {
	f.calls++
	f.gotMode = cfg.Mode
	f.gotInstr = instruction
	f.gotWorkDir = workDir
	f.gotActionID = actionID
	if f.err != nil {
		return "", f.err
	}
	return f.output, nil
}

func newTaskForAction(t *testing.T, d db.Store) int64 {
	t.Helper()
	projectID, err := d.InsertProject("p", "", "{}")
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	taskID, err := d.InsertTask(projectID, "t", "{}", "")
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}
	return taskID
}

func insertPendingAction(t *testing.T, d db.Store, taskID int64, meta string) *db.Action {
	t.Helper()
	id, err := d.InsertAction("a", taskID, meta, db.ActionStatusPending, nil, "")
	if err != nil {
		t.Fatalf("insert action: %v", err)
	}
	if _, err := d.ClaimPending(context.Background(), id); err != nil {
		t.Fatalf("claim pending: %v", err)
	}
	a, err := d.GetAction(id)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	return a
}

func TestExecuteAction_RoutesBgPathForLocalModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "default (unset) goes through bg", mode: ""},
		{name: "interactive goes through bg", mode: ModeInteractive},
		{name: "noninteractive goes through bg", mode: ModeNonInteractive},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			taskID := newTaskForAction(t, d)
			meta := `{"instruction":"do the task"}`
			if tc.mode != "" {
				meta = `{"instruction":"do the task","mode":"` + tc.mode + `"}`
			}
			action := insertPendingAction(t, d, taskID, meta)

			bg := &fakeWorker{output: "abcd1234"}
			rem := &fakeWorker{}

			params := ExecuteParams{
				DispatchConfig: DispatchConfig{
					DB:         d,
					BgFunc:     func() Worker { return bg },
					RemoteFunc: func() Worker { return rem },
				},
			}
			result, err := ExecuteAction(context.Background(), params, action)
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			if bg.calls != 1 {
				t.Errorf("BgFunc called %d times, want 1", bg.calls)
			}
			if rem.calls != 0 {
				t.Errorf("RemoteFunc unexpectedly called %d times", rem.calls)
			}
			wantMode := tc.mode
			if wantMode == "" {
				wantMode = ModeInteractive
			}
			if bg.gotMode != wantMode {
				t.Errorf("worker received mode %q, want %q", bg.gotMode, wantMode)
			}
			if !strings.HasPrefix(bg.gotInstr, "do the task") {
				t.Errorf("worker received instruction %q, want prefix %q", bg.gotInstr, "do the task")
			}
			if !strings.Contains(bg.gotInstr, "tq action context") {
				t.Errorf("instruction missing rendered postamble: %q", bg.gotInstr)
			}
			if result.Output != "abcd1234" {
				t.Errorf("result.Output = %q, want %q", result.Output, "abcd1234")
			}
			updated, _ := d.GetAction(action.ID)
			if !strings.Contains(updated.Metadata, `"daemon_short":"abcd1234"`) {
				t.Errorf("daemon_short not merged: %q", updated.Metadata)
			}
		})
	}
}

func TestExecuteAction_RemoteRoutesToRemoteFunc(t *testing.T) {
	d := testutil.NewTestDB(t)
	taskID := newTaskForAction(t, d)
	action := insertPendingAction(t, d, taskID, `{"instruction":"do","mode":"remote"}`)

	bg := &fakeWorker{}
	rem := &fakeWorker{output: RemoteSessionPrefix + "https://example.com/s/1"}

	params := ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:         d,
			BgFunc:     func() Worker { return bg },
			RemoteFunc: func() Worker { return rem },
		},
	}
	result, err := ExecuteAction(context.Background(), params, action)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if bg.calls != 0 {
		t.Errorf("BgFunc unexpectedly called %d times", bg.calls)
	}
	if rem.calls != 1 {
		t.Errorf("RemoteFunc called %d times, want 1", rem.calls)
	}
	if result.Mode != ModeRemote {
		t.Errorf("result.Mode = %q, want %q", result.Mode, ModeRemote)
	}
	updated, _ := d.GetAction(action.ID)
	if updated.Status != db.ActionStatusDispatched {
		t.Errorf("status = %q, want %q", updated.Status, db.ActionStatusDispatched)
	}
}

func TestExecuteAction_AdmissionRoutesByMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		wantInter int
		wantNon   int
		wantSent  error
	}{
		{name: "interactive admission gate", mode: ModeInteractive, wantInter: 1, wantSent: ErrInteractiveDeferred},
		{name: "default admission gate", mode: "", wantInter: 1, wantSent: ErrInteractiveDeferred},
		{name: "noninteractive admission gate", mode: ModeNonInteractive, wantNon: 1, wantSent: ErrNonInteractiveDeferred},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			taskID := newTaskForAction(t, d)
			meta := `{"instruction":"do"}`
			if tc.mode != "" {
				meta = `{"instruction":"do","mode":"` + tc.mode + `"}`
			}
			action := insertPendingAction(t, d, taskID, meta)

			var interCalls, nonCalls int
			params := ExecuteParams{
				DispatchConfig: DispatchConfig{
					DB:         d,
					BgFunc:     func() Worker { return &fakeWorker{output: "x"} },
					RemoteFunc: func() Worker { return &fakeWorker{} },
				},
				BeforeInteractive: func(_ *db.Action) error {
					interCalls++
					return ErrInteractiveDeferred
				},
				BeforeNonInteractive: func(_ *db.Action) error {
					nonCalls++
					return ErrNonInteractiveDeferred
				},
			}
			_, err := ExecuteAction(context.Background(), params, action)
			if !errors.Is(err, tc.wantSent) {
				t.Errorf("err = %v, want %v", err, tc.wantSent)
			}
			if interCalls != tc.wantInter {
				t.Errorf("BeforeInteractive called %d times, want %d", interCalls, tc.wantInter)
			}
			if nonCalls != tc.wantNon {
				t.Errorf("BeforeNonInteractive called %d times, want %d", nonCalls, tc.wantNon)
			}
			updated, _ := d.GetAction(action.ID)
			if updated.Status != db.ActionStatusPending {
				t.Errorf("status = %q, want %q (deferred)", updated.Status, db.ActionStatusPending)
			}
		})
	}
}

func TestExecuteAction_WorkerErrorMarksFailed(t *testing.T) {
	d := testutil.NewTestDB(t)
	taskID := newTaskForAction(t, d)
	action := insertPendingAction(t, d, taskID, `{"instruction":"do"}`)

	bg := &fakeWorker{err: errors.New("claude --bg crashed")}
	params := ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:         d,
			BgFunc:     func() Worker { return bg },
			RemoteFunc: func() Worker { return &fakeWorker{} },
		},
	}
	_, err := ExecuteAction(context.Background(), params, action)
	var af *ActionFailedError
	if !errors.As(err, &af) {
		t.Fatalf("err = %v, want *ActionFailedError", err)
	}
	updated, _ := d.GetAction(action.ID)
	if updated.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", updated.Status, db.ActionStatusFailed)
	}
}

func TestValidateActionMode(t *testing.T) {
	tests := []struct {
		name       string
		meta       map[string]any
		wantErrSub string
	}{
		{name: "missing mode is OK", meta: map[string]any{}},
		{name: "empty mode is OK", meta: map[string]any{MetaKeyMode: ""}},
		{name: "interactive", meta: map[string]any{MetaKeyMode: ModeInteractive}},
		{name: "noninteractive", meta: map[string]any{MetaKeyMode: ModeNonInteractive}},
		{name: "remote", meta: map[string]any{MetaKeyMode: ModeRemote}},
		{
			name:       "experimental_bg is rejected",
			meta:       map[string]any{MetaKeyMode: "experimental_bg"},
			wantErrSub: "must be one of: interactive, noninteractive, remote",
		},
		{
			name:       "claude permission-mode value is rejected",
			meta:       map[string]any{MetaKeyMode: "plan"},
			wantErrSub: "must be one of: interactive, noninteractive, remote",
		},
		{
			name:       "non-string mode is rejected",
			meta:       map[string]any{MetaKeyMode: 42},
			wantErrSub: `metadata "mode" must be a string`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateActionMode(tc.meta)
			if tc.wantErrSub == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
			}
			if !strings.Contains(err.Error(), tc.wantErrSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErrSub)
			}
		})
	}
}

func TestRenderPrompt_AppendsContext(t *testing.T) {
	got := RenderPrompt("do the task", 17, 5, ModeInteractive)
	if !strings.HasPrefix(got, "do the task") {
		t.Errorf("missing original instruction: %q", got)
	}
	if !strings.Contains(got, "action #17") {
		t.Errorf("missing action id: %q", got)
	}
	if !strings.Contains(got, "task 5") {
		t.Errorf("missing task id: %q", got)
	}
	if !strings.Contains(got, "/tq:done") {
		t.Errorf("missing /tq:done hint: %q", got)
	}
}

func TestRenderPrompt_RemoteOmitsDoneFailedHint(t *testing.T) {
	got := RenderPrompt("do the task", 17, 5, ModeRemote)
	if strings.Contains(got, "/tq:done") {
		t.Errorf("remote prompt should not mention /tq:done: %q", got)
	}
}

func TestValidateClaudeArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "empty is OK", args: nil},
		{name: "model arg is OK", args: []string{"--model", "sonnet"}},
		{name: "-p is blocked", args: []string{"-p", "x"}, wantErr: `claude_args cannot include "-p"`},
		{name: "--print is blocked", args: []string{"--print"}, wantErr: `claude_args cannot include "--print"`},
		{name: "--remote is blocked", args: []string{"--remote"}, wantErr: `claude_args cannot include "--remote"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClaudeArgs(tc.args)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
