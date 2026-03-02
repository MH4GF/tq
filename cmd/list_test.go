package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestList(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "task1", "", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", 5, "auto")
	d.InsertAction("deploy", nil, "{}", "running", 0, "human")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "review-pr") {
		t.Errorf("output should contain 'review-pr', got %q", out)
	}
	if !contains(out, "deploy") {
		t.Errorf("output should contain 'deploy', got %q", out)
	}
}

func TestList_StatusFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertAction("a", nil, "{}", "pending", 0, "auto")
	d.InsertAction("b", nil, "{}", "running", 0, "auto")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--status", "pending"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "pending") {
		t.Errorf("output should contain 'pending', got %q", out)
	}
	// "running" should only appear in header, not as a data row for template "b"
	if contains(out, "\tb\t") {
		t.Errorf("output should not contain template 'b' row, got %q", out)
	}
}

func TestList_TaskFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID1, _ := d.InsertTask(1, "task1", "", "{}")
	taskID2, _ := d.InsertTask(1, "task2", "", "{}")
	d.InsertAction("a", &taskID1, "{}", "pending", 0, "auto")
	d.InsertAction("b", &taskID2, "{}", "pending", 0, "auto")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--task", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "a") {
		t.Errorf("output should contain template 'a', got %q", out)
	}
}

func TestList_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "no actions found") {
		t.Errorf("output = %q, want 'no actions found'", out)
	}
}
