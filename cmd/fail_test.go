package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestFail(t *testing.T) {
	tests := []struct {
		name               string
		startStatus        string
		args               []string
		wantErr            bool
		wantOutContains    []string
		wantOutNotContains []string
		wantResult         string
	}{
		{
			name:            "with reason",
			startStatus:     db.ActionStatusRunning,
			args:            []string{"action", "fail", "1", "outcome: API down"},
			wantOutContains: []string{"action #1 failed", "outcome: API down"},
			wantResult:      "outcome: API down",
		},
		{
			name:               "without reason",
			startStatus:        db.ActionStatusPending,
			args:               []string{"action", "fail", "1"},
			wantOutContains:    []string{"action #1 failed"},
			wantOutNotContains: []string{"reason:"},
		},
		{
			name:    "invalid ID",
			args:    []string{"action", "fail", "999"},
			wantErr: true,
		},
		{
			name:        "from dispatched",
			startStatus: db.ActionStatusDispatched,
			args:        []string{"action", "fail", "1", "stuck"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			var id int64
			if tc.startStatus != "" {
				taskID, _ := d.InsertTask(1, "test", "{}", "")
				id, _ = d.InsertAction("test", taskID, "{}", tc.startStatus, nil)
			}

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			out := buf.String()
			for _, want := range tc.wantOutContains {
				if !contains(out, want) {
					t.Errorf("output = %q, want to contain %q", out, want)
				}
			}
			for _, notWant := range tc.wantOutNotContains {
				if contains(out, notWant) {
					t.Errorf("output = %q, should NOT contain %q", out, notWant)
				}
			}

			a, err := d.GetAction(id)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			if a.Status != db.ActionStatusFailed {
				t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
			}
			if tc.wantResult != "" {
				if !a.Result.Valid || a.Result.String != tc.wantResult {
					t.Errorf("result = %v, want %q", a.Result, tc.wantResult)
				}
			}
			if !a.CompletedAt.Valid {
				t.Error("completed_at should be set")
			}
		})
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
			d.InsertAction("test", taskID, "{}", tc.startStatus, nil)

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
