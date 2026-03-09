package cmd_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/prompt"
	"github.com/MH4GF/tq/testutil"
)

type mockWorker struct {
	result string
	err    error
}

func (m *mockWorker) Execute(ctx context.Context, prompt string, cfg prompt.Config, workDir string, actionID int64) (string, error) {
	return m.result, m.err
}

func TestDispatch_NoPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	cmd.SetConfigDir(t.TempDir())

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"dispatch"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "no pending actions") {
		t.Errorf("output = %q, want to contain 'no pending actions'", out)
	}
}

func TestDispatch_Success(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	os.WriteFile(filepath.Join(promptsDir, "review-pr.md"), []byte(`---
description: Review PR
mode: noninteractive
---
Review PR for {{.Task.Title}}.
`), 0644)

	taskID, _ := d.InsertTask(1, "Fix bug", "https://github.com/test/1", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending")

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{result: `{"review":"approved"}`}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"dispatch"})

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
	if a.Status != "done" {
		t.Errorf("status = %q, want %q", a.Status, "done")
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

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	os.WriteFile(filepath.Join(promptsDir, "review-pr.md"), []byte(`---
description: Review PR
mode: noninteractive
---
Review PR for {{.Task.Title}}.
`), 0644)

	taskID, _ := d.InsertTask(1, "Fix bug", "https://github.com/test/1", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending")
	d.InsertAction("review-pr", &taskID, "{}", "pending")

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{result: `{"review":"approved"}`}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"dispatch", "2"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #2 done") {
		t.Errorf("output = %q, want to contain 'action #2 done'", out)
	}

	// action 2 should be done
	a2, _ := d.GetAction(2)
	if a2.Status != "done" {
		t.Errorf("action 2 status = %q, want done", a2.Status)
	}

	// action 1 should still be pending
	a1, _ := d.GetAction(1)
	if a1.Status != "pending" {
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
	root.SetArgs([]string{"dispatch", "999"})

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

	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte(`---
description: Test
mode: noninteractive
---
Do something.
`), 0644)

	d.InsertAction("test", nil, "{}", "pending")

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: context.DeadlineExceeded}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"dispatch"})

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
	if a.Status != "failed" {
		t.Errorf("status = %q, want %q", a.Status, "failed")
	}
}
