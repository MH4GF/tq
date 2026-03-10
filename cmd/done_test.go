package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestDone(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertAction("test", nil, "{}", "running")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "done", "1", `{"status":"ok"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 done") {
		t.Errorf("output = %q, want to contain 'action #1 done'", out)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != "done" {
		t.Errorf("status = %q, want %q", a.Status, "done")
	}
	if !a.Result.Valid || a.Result.String != `{"status":"ok"}` {
		t.Errorf("result = %v, want %q", a.Result, `{"status":"ok"}`)
	}
}

func TestDone_NoResult(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertAction("test", nil, "{}", "running")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "done", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "action #1 done") {
		t.Errorf("output = %q, want to contain 'action #1 done'", out)
	}
}

func TestDone_InvalidID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"action", "done", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent action ID")
	}
}

func TestDone_TriggersOnDone(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	for _, tc := range []struct{ name string; auto bool; onDone string }{
		{"check-pr", true, "review"},
		{"review", true, ""},
	} {
		content := fmt.Sprintf("---\ndescription: %s\nauto: %v\non_done: %s\n---\nDo %s.\n", tc.name, tc.auto, tc.onDone, tc.name)
		os.WriteFile(filepath.Join(promptsDir, tc.name+".md"), []byte(content), 0o644)
	}

	cmd.SetConfigDir(tqDir)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	d.InsertAction("check-pr", &taskID, "{}", "running")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "done", "1", `{"result":"PR merged"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions, _ := d.ListActions("", nil)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	followUp := actions[1]
	if followUp.PromptID != "review" {
		t.Errorf("template_id = %q, want review", followUp.PromptID)
	}
	if followUp.Status != "pending" {
		t.Errorf("status = %q, want pending", followUp.Status)
	}
}
