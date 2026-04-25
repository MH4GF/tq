package cmd_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestActionUpdate(t *testing.T) {
	tests := []struct {
		name           string
		markDoneFirst  bool
		extraArgs      []string
		wantErrSubstr  string
		wantOutSubstr  string
		wantTitleAfter string
	}{
		{
			name:           "updates title",
			extraArgs:      []string{"--title", "updated"},
			wantOutSubstr:  "updated",
			wantTitleAfter: "updated",
		},
		{
			name:          "no flags returns error",
			wantErrSubstr: "at least one flag",
		},
		{
			name:          "done action returns error",
			markDoneFirst: true,
			extraArgs:     []string{"--title", "nope"},
			wantErrSubstr: "only pending or failed",
		},
		{
			name:          "invalid meta returns error",
			extraArgs:     []string{"--meta", "{invalid}"},
			wantErrSubstr: "invalid JSON for --meta (must be a JSON object)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			taskID, _ := d.InsertTask(1, "test task", "{}", "")
			actionID, _ := d.InsertAction("original", taskID, `{"k":"v"}`, db.ActionStatusPending, nil)

			if tt.markDoneFirst {
				if err := d.MarkDone(actionID, "done"); err != nil {
					t.Fatalf("MarkDone: %v", err)
				}
			}

			root := cmd.GetRootCmd()
			out := new(bytes.Buffer)
			root.SetOut(out)
			root.SetErr(new(bytes.Buffer))
			args := append([]string{"action", "update", fmt.Sprintf("%d", actionID)}, tt.extraArgs...)
			root.SetArgs(args)

			err := root.Execute()

			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrSubstr)
				}
				if !contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := fmt.Sprintf("action #%d %s", actionID, tt.wantOutSubstr)
			if !contains(out.String(), expected) {
				t.Errorf("output = %q, want to contain %q", out.String(), expected)
			}
			if tt.wantTitleAfter != "" {
				a, _ := d.GetAction(actionID)
				if a.Title != tt.wantTitleAfter {
					t.Errorf("title = %q, want %q", a.Title, tt.wantTitleAfter)
				}
			}
		})
	}
}
