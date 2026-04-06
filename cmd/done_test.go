package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestDone(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "done", "1", `{"status":"ok"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 done") {
		t.Errorf("output = %q, want to contain 'action #1 done'", out)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != db.ActionStatusDone {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusDone)
	}
	if !a.Result.Valid || a.Result.String != `{"status":"ok"}` {
		t.Errorf("result = %v, want %q", a.Result, `{"status":"ok"}`)
	}
}

func TestDone_NoResult(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "done", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 done") {
		t.Errorf("output = %q, want to contain 'action #1 done'", out)
	}
}

func TestDone_InvalidID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "done", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent action ID")
	}
}
