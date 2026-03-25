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
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	actionID, _ := d.InsertAction("original", taskID, `{"k":"v"}`, db.ActionStatusPending)

	root := cmd.GetRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	idStr := fmt.Sprintf("%d", actionID)
	root.SetArgs([]string{"action", "update", idStr, "--title", "updated"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := fmt.Sprintf("action #%d updated", actionID)
	if !contains(out.String(), expected) {
		t.Errorf("output = %q, want to contain %q", out.String(), expected)
	}

	a, _ := d.GetAction(actionID)
	if a.Title != "updated" {
		t.Errorf("title = %q, want %q", a.Title, "updated")
	}
}

func TestActionUpdate_NoFlags(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	actionID, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "update", fmt.Sprintf("%d", actionID)})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
	if !contains(err.Error(), "at least one flag") {
		t.Errorf("error = %q, want to contain 'at least one flag'", err.Error())
	}
}

func TestActionUpdate_DoneAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	actionID, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusPending)
	d.MarkDone(actionID, "done")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "update", fmt.Sprintf("%d", actionID), "--title", "nope"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for done action")
	}
	if !contains(err.Error(), "only pending or failed") {
		t.Errorf("error = %q, want to contain 'only pending or failed'", err.Error())
	}
}

func TestActionUpdate_InvalidMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	actionID, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "update", fmt.Sprintf("%d", actionID), "--meta", "{invalid}"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON meta")
	}
	if !contains(err.Error(), "invalid JSON for --meta (must be a JSON object)") {
		t.Errorf("error = %q, want to contain 'invalid JSON for --meta'", err.Error())
	}
}
