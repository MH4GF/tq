package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestAttach(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "no session info",
			sessionID:  "",
			wantErr:    true,
			wantErrMsg: "has no tmux session info",
		},
		{
			name:      "with session info (tmux command fails outside tmux)",
			sessionID: "main",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			id, _ := d.InsertAction("test", "test", nil, "{}", "running")
			if tc.sessionID != "" {
				d.SetSessionInfo(id, tc.sessionID, "tq-action-1")
			}

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"action", "attach", "1"})

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrMsg != "" && !contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
