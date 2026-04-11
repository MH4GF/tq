package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestFail(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "fail", "1", "outcome: API down"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 failed") {
		t.Errorf("output = %q, want to contain 'action #1 failed'", out)
	}
	if !contains(out, "outcome: API down") {
		t.Errorf("output = %q, want to contain reason", out)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
	}
	if !a.Result.Valid || a.Result.String != "outcome: API down" {
		t.Errorf("result = %v, want %q", a.Result, "outcome: API down")
	}
	if !a.CompletedAt.Valid {
		t.Error("completed_at should be set")
	}
}

func TestFail_NoReason(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "fail", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 failed") {
		t.Errorf("output = %q, want to contain 'action #1 failed'", out)
	}
	if contains(out, "reason:") {
		t.Errorf("output = %q, should NOT contain 'reason:' when no reason given", out)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
	}
	if !a.CompletedAt.Valid {
		t.Error("completed_at should be set")
	}
}

func TestFail_InvalidID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "fail", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent action ID")
	}
}

func TestFail_AlreadyTerminal(t *testing.T) {
	tests := []struct {
		name        string
		startStatus string
	}{
		{"done", db.ActionStatusDone},
		{"failed", db.ActionStatusFailed},
		{"cancelled", db.ActionStatusCancelled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			taskID, _ := d.InsertTask(1, "test", "{}", "")
			d.InsertAction("test", taskID, "{}", tc.startStatus)

			root := cmd.GetRootCmd()
			root.SetOut(new(bytes.Buffer))
			root.SetErr(new(bytes.Buffer))
			root.SetArgs([]string{"action", "fail", "1", "reason"})

			err := root.Execute()
			if err == nil {
				t.Fatalf("expected error when failing %s action", tc.startStatus)
			}
			if !contains(err.Error(), "already") {
				t.Errorf("error = %q, want to contain 'already'", err.Error())
			}
		})
	}
}

func TestFail_FromDispatched(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusDispatched)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "fail", "1", "stuck"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
	}
}
