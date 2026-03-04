package cmd_test

import (
	"bytes"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestTaskCreate(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "create", "test task", "--project", "immedio", "--url", "https://example.com"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "task #1 created") {
		t.Errorf("output = %q, want to contain 'task #1 created'", out)
	}
	if !contains(out, "project: immedio") {
		t.Errorf("output = %q, want to contain 'project: immedio'", out)
	}

	task, err := d.GetTask(1)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Title != "test task" {
		t.Errorf("title = %q, want %q", task.Title, "test task")
	}
	if task.URL != "https://example.com" {
		t.Errorf("url = %q, want %q", task.URL, "https://example.com")
	}
}

func TestTaskCreate_MissingProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "create", "test"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --project flag")
	}
}

func TestTaskCreate_UnknownProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "create", "test", "--project", "nonexistent"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestTaskCreate_MissingTitle(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "create", "--project", "immedio"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing title argument")
	}
}

func TestTaskList(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task A", "https://example.com/a", "{}")
	d.InsertTask(1, "task B", "", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "task A") {
		t.Errorf("output should contain 'task A', got %q", out)
	}
	if !contains(out, "task B") {
		t.Errorf("output should contain 'task B', got %q", out)
	}
}

func TestTaskList_StatusFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "open task", "", "{}")
	id2, _ := d.InsertTask(1, "done task", "", "{}")
	d.UpdateTask(id2, "done")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list", "--status", "open"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "open task") {
		t.Errorf("output should contain 'open task', got %q", out)
	}
	if contains(out, "done task") {
		t.Errorf("output should not contain 'done task', got %q", out)
	}
}

func TestTaskList_ProjectFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "immedio task", "", "{}")
	d.InsertTask(2, "hearable task", "", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list", "--project", "immedio"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "immedio task") {
		t.Errorf("output should contain 'immedio task', got %q", out)
	}
	if contains(out, "hearable task") {
		t.Errorf("output should not contain 'hearable task', got %q", out)
	}
}

func TestTaskList_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "no tasks found") {
		t.Errorf("output = %q, want 'no tasks found'", out)
	}
}

func TestTaskUpdate_ProjectOnly(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task to move", "", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "update", "1", "--project", "hearable"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "project: hearable") {
		t.Errorf("output = %q, want to contain 'project: hearable'", out)
	}

	task, err := d.GetTask(1)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.ProjectID != 2 {
		t.Errorf("project_id = %d, want 2", task.ProjectID)
	}
}

func TestTaskUpdate_StatusAndProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task to update", "", "{}")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "update", "1", "--status", "done", "--project", "hearable"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "project: hearable") {
		t.Errorf("output = %q, want to contain 'project: hearable'", out)
	}
	if !contains(out, "status: done") {
		t.Errorf("output = %q, want to contain 'status: done'", out)
	}

	task, err := d.GetTask(1)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.ProjectID != 2 {
		t.Errorf("project_id = %d, want 2", task.ProjectID)
	}
	if task.Status != "done" {
		t.Errorf("status = %q, want %q", task.Status, "done")
	}
}

func TestTaskUpdate_UnknownProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task", "", "{}")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", "1", "--project", "nonexistent"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestTaskUpdate_NeitherStatusNorProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task", "", "{}")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", "1"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error when neither --status nor --project is given")
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
