package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestReset(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantOut string
		wantErr bool
	}{
		{"failed to pending", db.ActionStatusFailed, "action #1 reset to pending", false},
		{"done is rejected", db.ActionStatusDone, "", true},
		{"running to pending", db.ActionStatusRunning, "action #1 reset to pending", false},
		{"pending is rejected", db.ActionStatusPending, "", true},
		{"cancelled to pending", db.ActionStatusCancelled, "action #1 reset to pending", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			taskID, _ := d.InsertTask(1, "test", "{}", "")
			d.InsertAction("test", "test", taskID, "{}", tc.status)

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"action", "reset", "1"})

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			out := buf.String()
			if !contains(out, tc.wantOut) {
				t.Errorf("output = %q, want to contain %q", out, tc.wantOut)
			}

			a, err := d.GetAction(1)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			if a.Status != db.ActionStatusPending {
				t.Errorf("status = %q, want %q", a.Status, db.ActionStatusPending)
			}
		})
	}
}

func TestReset_UnknownStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	// Simulate an action with invalid status (e.g. "open") stuck in the DB
	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("test", "test", taskID, "{}", db.ActionStatusPending)
	d.Exec("UPDATE actions SET status = 'open' WHERE id = 1")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "reset", "1"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != db.ActionStatusPending {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusPending)
	}
}

func TestReset_InvalidID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "reset", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent action ID")
	}
}
