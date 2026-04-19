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
		args            []string
		wantOutContains string
		wantErr         bool
	}{
		{
			name:            "with result",
			args:            []string{"action", "done", "1", `{"status":"ok"}`},
			wantOutContains: "action #1 done",
		},
		{
			name:            "no result",
			args:            []string{"action", "done", "1"},
			wantOutContains: "action #1 done",
		},
		{
			name:    "invalid ID",
			args:    []string{"action", "done", "999"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			if !contains(out, tc.wantOutContains) {
				t.Errorf("output = %q, want to contain %q", out, tc.wantOutContains)
			}
		})
	}
}
