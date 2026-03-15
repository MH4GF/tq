package cmd_test

import (
	"bytes"
	"encoding/json"
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
	root.SetArgs([]string{"task", "create", "test task", "--project", "1", "--url", "https://example.com"})

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
	root.SetArgs([]string{"task", "create", "test", "--project", "999"})

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
	root.SetArgs([]string{"task", "create", "--project", "1"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing title argument")
	}
}

func TestTaskList(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task A", "https://example.com/a", "{}", "")
	d.InsertTask(1, "task B", "", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["title"] != "task A" {
		t.Errorf("first row title = %v, want %q", rows[0]["title"], "task A")
	}
	if rows[1]["title"] != "task B" {
		t.Errorf("second row title = %v, want %q", rows[1]["title"], "task B")
	}
}

func TestTaskList_JSON(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "test task", "https://example.com", `{"key":"value"}`, "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row["id"] != float64(1) {
		t.Errorf("id = %v, want 1", row["id"])
	}
	if row["project_id"] != float64(1) {
		t.Errorf("project_id = %v, want 1", row["project_id"])
	}
	if row["title"] != "test task" {
		t.Errorf("title = %v, want %q", row["title"], "test task")
	}
	if row["url"] != "https://example.com" {
		t.Errorf("url = %v, want %q", row["url"], "https://example.com")
	}
	if row["metadata"] != `{"key":"value"}` {
		t.Errorf("metadata = %v, want %q", row["metadata"], `{"key":"value"}`)
	}
	if row["status"] != "open" {
		t.Errorf("status = %v, want %q", row["status"], "open")
	}
	if row["created_at"] == nil {
		t.Error("created_at should not be null")
	}
}

func TestTaskList_JSON_NullFields(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "new task", "", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]
	if row["updated_at"] != nil {
		t.Errorf("updated_at should be null for new task, got %v", row["updated_at"])
	}
}

func TestTaskList_StatusFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "open task", "", "{}", "")
	id2, _ := d.InsertTask(1, "done task", "", "{}", "")
	d.UpdateTask(id2, "done", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list", "--status", "open"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["title"] != "open task" {
		t.Errorf("title = %v, want %q", rows[0]["title"], "open task")
	}
}

func TestTaskList_ProjectFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "immedio task", "", "{}", "")
	d.InsertTask(2, "hearable task", "", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list", "--project", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["title"] != "immedio task" {
		t.Errorf("title = %v, want %q", rows[0]["title"], "immedio task")
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
	if !contains(out, "[]") {
		t.Errorf("output = %q, want '[]'", out)
	}
}

func TestTaskUpdate_ProjectOnly(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task to move", "", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "update", "1", "--project", "2"})

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

	d.InsertTask(1, "task to update", "", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "update", "1", "--status", "done", "--project", "2"})

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

	d.InsertTask(1, "task", "", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", "1", "--project", "999"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestTaskUpdate_NeitherStatusNorProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task", "", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", "1"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error when neither --status nor --project is given")
	}
}

func TestTaskList_WithActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID1, _ := d.InsertTask(1, "task with actions", "", "{}", "")
	taskID2, _ := d.InsertTask(1, "task without actions", "", "{}", "")
	d.InsertAction("review-pr", "review-pr", taskID1, `{"pr":1}`, "pending")
	d.InsertAction("implement", "implement", taskID1, "{}", "done")
	_ = taskID2

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Task with actions
	actions1, ok := rows[0]["actions"].([]any)
	if !ok {
		t.Fatalf("actions field missing or wrong type for task 1: %v", rows[0]["actions"])
	}
	if len(actions1) != 2 {
		t.Errorf("task 1 actions = %d, want 2", len(actions1))
	}
	firstAction := actions1[0].(map[string]any)
	if firstAction["prompt_id"] != "review-pr" {
		t.Errorf("first action prompt_id = %v, want %q", firstAction["prompt_id"], "review-pr")
	}

	// Task without actions — should be empty array, not null
	actions2, ok := rows[1]["actions"].([]any)
	if !ok {
		t.Fatalf("actions field missing or wrong type for task 2: %v", rows[1]["actions"])
	}
	if len(actions2) != 0 {
		t.Errorf("task 2 actions = %d, want 0", len(actions2))
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
