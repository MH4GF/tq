package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestAttach(t *testing.T) {
	tests := []struct {
		name       string
		metadata   string
		wantErrMsg string
	}{
		{
			name:       "no daemon_short",
			metadata:   `{"instruction":"x","mode":"interactive"}`,
			wantErrMsg: "has no daemon_short",
		},
		{
			name:       "remote mode is not attachable",
			metadata:   `{"instruction":"x","mode":"remote"}`,
			wantErrMsg: "remote mode; attach is not supported",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			taskID, _ := d.InsertTask(1, "test", "{}", "")
			d.InsertAction("test", taskID, tc.metadata, db.ActionStatusRunning, nil, "")

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"action", "attach", "1"})

			err := root.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrMsg) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrMsg)
			}
		})
	}
}

func TestAttach_NonExistentID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "attach", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent action ID")
	}
}
