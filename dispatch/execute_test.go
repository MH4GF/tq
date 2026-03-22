package dispatch

import (
	"context"
	"encoding/json"
	"errors"
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
			d.InsertAction("test-"+tc.promptMode, taskID, string(meta), db.ActionStatusPending)

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
	d.InsertAction("review", taskID, string(meta), db.ActionStatusPending)

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
	d.InsertAction("task", taskID, string(meta), db.ActionStatusPending)

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

func TestExecuteAction_NoPromptNoInstruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{}`, "")
	d.InsertAction("task", taskID, "{}", db.ActionStatusPending)

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
		t.Fatal("expected error for no prompt and no instruction")
	}

	a, _ := d.GetAction(action.ID)
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
	}
}
