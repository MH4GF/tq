package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestDone(t *testing.T) {
	tests := []struct {
		name            string
		startStatus     string
		args            []string
		wantOutContains string
		wantResult      string
	}{
		{
			name:            "pending to done",
			startStatus:     db.ActionStatusPending,
			args:            []string{"action", "done", "1"},
			wantOutContains: "action #1 done",
		},
		{
			name:            "running to done",
			startStatus:     db.ActionStatusRunning,
			args:            []string{"action", "done", "1"},
			wantOutContains: "action #1 done",
		},
		{
			name:            "with result",
			startStatus:     db.ActionStatusRunning,
			args:            []string{"action", "done", "1", `{"status":"ok"}`},
			wantOutContains: "action #1 done",
			wantResult:      `{"status":"ok"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			taskID, _ := d.InsertTask(1, "test", "{}", "")
			id, _ := d.InsertAction("test", taskID, "{}", tc.startStatus, nil)

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			err := root.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			out := buf.String()
			if !contains(out, tc.wantOutContains) {
				t.Errorf("output = %q, want to contain %q", out, tc.wantOutContains)
			}

			a, err := d.GetAction(id)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			if a.Status != db.ActionStatusDone {
				t.Errorf("status = %q, want %q", a.Status, db.ActionStatusDone)
			}
			if tc.wantResult != "" && (!a.Result.Valid || a.Result.String != tc.wantResult) {
				t.Errorf("result = %v, want %q", a.Result, tc.wantResult)
			}
			if !a.CompletedAt.Valid {
				t.Error("completed_at should be set")
			}
		})
	}
}

func TestDone_UnknownID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "done", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unknown action ID")
	}
}

func TestDone_AlreadyTerminal(t *testing.T) {
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
			id, _ := d.InsertAction("test", taskID, "{}", tc.startStatus, nil)

			root := cmd.GetRootCmd()
			root.SetOut(new(bytes.Buffer))
			root.SetErr(new(bytes.Buffer))
			root.SetArgs([]string{"action", "done", "1"})

			err := root.Execute()
			if err == nil {
				t.Fatalf("expected error when marking %s action as done", tc.startStatus)
			}
			if !contains(err.Error(), "already") {
				t.Errorf("error = %q, want to contain 'already'", err.Error())
			}

			a, err := d.GetAction(id)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			if a.Status != tc.startStatus {
				t.Errorf("status changed to %q, want unchanged %q", a.Status, tc.startStatus)
			}
		})
	}
}
