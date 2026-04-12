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

func TestDispatch_NoArgs(t *testing.T) {
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided")
	}
}

func TestDispatch_Success(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	cmd.SetConfigDir(t.TempDir())

	taskID, _ := d.InsertTask(1, "Fix bug", `{"url":"https://github.com/test/1"}`, "")
	d.InsertAction("review-pr", taskID, `{"instruction":"Review PR for Fix bug.","mode":"noninteractive"}`, db.ActionStatusPending, nil)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{result: `{"review":"approved"}`}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 done") {
		t.Errorf("output = %q, want to contain 'action #1 done'", out)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != db.ActionStatusDone {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusDone)
	}
	if !a.Result.Valid || a.Result.String != `{"review":"approved"}` {
		t.Errorf("result = %v, want %q", a.Result, `{"review":"approved"}`)
	}
}

func TestDispatch_WithActionID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	cmd.SetConfigDir(t.TempDir())

	taskID, _ := d.InsertTask(1, "Fix bug", `{"url":"https://github.com/test/1"}`, "")
	d.InsertAction("review-pr", taskID, `{"instruction":"Review PR.","mode":"noninteractive"}`, db.ActionStatusPending, nil)
	d.InsertAction("review-pr", taskID, `{"instruction":"Review PR.","mode":"noninteractive"}`, db.ActionStatusPending, nil)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{result: `{"review":"approved"}`}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch", "2"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #2 done") {
		t.Errorf("output = %q, want to contain 'action #2 done'", out)
	}

	// action 2 should be done
	a2, _ := d.GetAction(2)
	if a2.Status != db.ActionStatusDone {
		t.Errorf("action 2 status = %q, want done", a2.Status)
	}

	// action 1 should still be pending
	a1, _ := d.GetAction(1)
	if a1.Status != db.ActionStatusPending {
		t.Errorf("action 1 status = %q, want pending", a1.Status)
	}
}

func TestDispatch_WithInvalidActionID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	cmd.SetConfigDir(t.TempDir())

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch", "999"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent action ID")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err)
	}
}

func TestDispatch_WorkerError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	cmd.SetConfigDir(t.TempDir())

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("test", taskID, `{"instruction":"Do something.","mode":"noninteractive"}`, db.ActionStatusPending, nil)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: context.DeadlineExceeded}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "failed") {
		t.Errorf("output = %q, want to contain 'failed'", out)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
	}
}
