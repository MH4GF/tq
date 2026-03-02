package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	cmd.SetTQDir(t.TempDir())

	taskID, _ := d.InsertTask(1, "Task 1", "", "{}")
	d.InsertAction("check-pr", &taskID, "{}", "pending", 0, "test")
	d.InsertAction("fix-ci", &taskID, "{}", "running", 0, "test")
	d.InsertAction("merge", &taskID, "{}", "done", 0, "test")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "total:") {
		t.Errorf("output = %q, want to contain 'total:'", out)
	}
	if !contains(out, "pending:") {
		t.Errorf("output = %q, want to contain 'pending:'", out)
	}
	if !contains(out, "running:") {
		t.Errorf("output = %q, want to contain 'running:'", out)
	}
}
