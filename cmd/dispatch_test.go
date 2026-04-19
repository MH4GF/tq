package cmd_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

type mockWorker struct {
	result string
	err    error
}

func (m *mockWorker) Execute(ctx context.Context, prompt string, cfg dispatch.ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	return m.result, m.err
}

func TestDispatch(t *testing.T) {
	type wantAction struct {
		id     int64
		status string
		result string
	}

	tests := []struct {
		name            string
		setup           func(d db.Store)
		workerResult    string
		workerErr       error
		useWorker       bool
		args            []string
		wantErr         bool
		wantErrContains string
		wantOutContains string
		wantActions     []wantAction
	}{
		{
			name:    "no args",
			args:    []string{"action", "dispatch"},
			wantErr: true,
		},
		{
			name: "success",
			setup: func(d db.Store) {
				taskID, _ := d.InsertTask(1, "Fix bug", `{"url":"https://github.com/test/1"}`, "")
				d.InsertAction("review-pr", taskID, `{"instruction":"Review PR for Fix bug.","mode":"noninteractive"}`, db.ActionStatusPending, nil)
			},
			useWorker:       true,
			workerResult:    `{"review":"approved"}`,
			args:            []string{"action", "dispatch", "1"},
			wantOutContains: "action #1 done",
			wantActions: []wantAction{
				{id: 1, status: db.ActionStatusDone, result: `{"review":"approved"}`},
			},
		},
		{
			name: "with action id",
			setup: func(d db.Store) {
				taskID, _ := d.InsertTask(1, "Fix bug", `{"url":"https://github.com/test/1"}`, "")
				d.InsertAction("review-pr", taskID, `{"instruction":"Review PR.","mode":"noninteractive"}`, db.ActionStatusPending, nil)
				d.InsertAction("review-pr", taskID, `{"instruction":"Review PR.","mode":"noninteractive"}`, db.ActionStatusPending, nil)
			},
			useWorker:       true,
			workerResult:    `{"review":"approved"}`,
			args:            []string{"action", "dispatch", "2"},
			wantOutContains: "action #2 done",
			wantActions: []wantAction{
				{id: 1, status: db.ActionStatusPending},
				{id: 2, status: db.ActionStatusDone},
			},
		},
		{
			name:            "invalid action id",
			args:            []string{"action", "dispatch", "999"},
			wantErr:         true,
			wantErrContains: "not found",
		},
		{
			name: "worker error",
			setup: func(d db.Store) {
				taskID, _ := d.InsertTask(1, "test", "{}", "")
				d.InsertAction("test", taskID, `{"instruction":"Do something.","mode":"noninteractive"}`, db.ActionStatusPending, nil)
			},
			useWorker:       true,
			workerErr:       context.DeadlineExceeded,
			args:            []string{"action", "dispatch", "1"},
			wantOutContains: "failed",
			wantActions: []wantAction{
				{id: 1, status: db.ActionStatusFailed},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()
			cmd.SetConfigDir(t.TempDir())

			if tt.setup != nil {
				tt.setup(d)
			}

			if tt.useWorker {
				worker := &mockWorker{result: tt.workerResult, err: tt.workerErr}
				cmd.SetWorkerFactory(func() dispatch.Worker { return worker })
				t.Cleanup(func() { cmd.SetWorkerFactory(nil) })
			}

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tt.args)

			err := root.Execute()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error = %q, want to contain %q", err, tt.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantOutContains != "" {
				out := buf.String()
				if !contains(out, tt.wantOutContains) {
					t.Errorf("output = %q, want to contain %q", out, tt.wantOutContains)
				}
			}

			for _, wa := range tt.wantActions {
				a, err := d.GetAction(wa.id)
				if err != nil {
					t.Fatalf("get action %d: %v", wa.id, err)
				}
				if a.Status != wa.status {
					t.Errorf("action %d status = %q, want %q", wa.id, a.Status, wa.status)
				}
				if wa.result != "" {
					if !a.Result.Valid || a.Result.String != wa.result {
						t.Errorf("action %d result = %v, want %q", wa.id, a.Result, wa.result)
					}
				}
			}
		})
	}
}
