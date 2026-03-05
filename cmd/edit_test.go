package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestActionEdit(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantOut string
		wantErr bool
	}{
		{"failed to pending", "failed", "pending", "action #1 updated (status: pending)", false},
		{"waiting_human to failed", "waiting_human", "failed", "action #1 updated (status: failed)", false},
		{"running to failed", "running", "failed", "action #1 updated (status: failed)", false},
		{"running to pending", "running", "pending", "action #1 updated (status: pending)", false},
		{"done to pending", "done", "pending", "action #1 updated (status: pending)", false},
		{"pending to waiting_human", "pending", "waiting_human", "action #1 updated (status: waiting_human)", false},
		{"to done is rejected", "running", "done", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			d.InsertAction("test", nil, "{}", tc.from, "human")

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"action", "edit", "1", "--status", tc.to})

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
			if a.Status != tc.to {
				t.Errorf("status = %q, want %q", a.Status, tc.to)
			}
		})
	}
}

func TestActionEdit_InvalidID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "edit", "999", "--status", "pending"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent action ID")
	}
}
