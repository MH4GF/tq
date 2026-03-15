package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestProjectCreate(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"project", "create", "myapp", "/tmp/myapp", "--meta", `{"key":"val"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "project #1 created") {
		t.Errorf("output = %q, want to contain 'project #1 created'", out)
	}
	if !contains(out, "myapp") {
		t.Errorf("output = %q, want to contain 'myapp'", out)
	}

	p, err := d.GetProjectByName("myapp")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if p.WorkDir != "/tmp/myapp" {
		t.Errorf("work_dir = %q, want %q", p.WorkDir, "/tmp/myapp")
	}
}

func TestProjectCreate_MissingArgs(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"project", "create", "myapp"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing positional arguments")
	}
}

func TestProjectCreate_DuplicateName(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertProject("dup", "/tmp/dup", "{}")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"project", "create", "dup", "/tmp/dup2"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for duplicate project name")
	}
}

func TestProjectList(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"project", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, buf.String())
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(rows))
	}
	names := make(map[string]bool)
	for _, r := range rows {
		names[r["name"].(string)] = true
	}
	for _, want := range []string{"immedio", "hearable", "works"} {
		if !names[want] {
			t.Errorf("expected project %q in output", want)
		}
	}
}

func TestProjectList_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"project", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "[]") {
		t.Errorf("output = %q, want '[]'", out)
	}
}

func TestProjectDelete(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertProject("todelete", "/tmp/del", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"project", "delete", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "project #1 deleted") {
		t.Errorf("output = %q, want to contain 'project #1 deleted'", out)
	}

	_, err := d.GetProjectByID(id)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestProjectDelete_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"project", "delete", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent project")
	}
}
