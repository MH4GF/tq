package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
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
	root.SetArgs([]string{"task", "create", "test task", "--project", "1", "--meta", `{"url":"https://example.com"}`})

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
	if !contains(task.Metadata, "https://example.com") {
		t.Errorf("metadata = %q, want to contain URL", task.Metadata)
	}
}

func TestTaskCreate_InvalidMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "create", "test task", "--project", "1", "--meta", "{invalid}"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON meta")
	}
	if !contains(err.Error(), "invalid JSON for --meta (must be a JSON object)") {
		t.Errorf("error = %q, want to contain 'invalid JSON for --meta (must be a JSON object)'", err.Error())
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

	d.InsertTask(1, "task A", `{"url":"https://example.com/a"}`, "")
	d.InsertTask(1, "task B", "{}", "")

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
	if rows[0]["title"] != "task B" {
		t.Errorf("first row title = %v, want %q", rows[0]["title"], "task B")
	}
	if rows[1]["title"] != "task A" {
		t.Errorf("second row title = %v, want %q", rows[1]["title"], "task A")
	}
}

func TestTaskList_JSON(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "test task", `{"key":"value","url":"https://example.com"}`, "")

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
	if row["metadata"] != `{"key":"value","url":"https://example.com"}` {
		t.Errorf("metadata = %v, want %q", row["metadata"], `{"key":"value","url":"https://example.com"}`)
	}
	if row["status"] != db.TaskStatusOpen {
		t.Errorf("status = %v, want %q", row["status"], db.TaskStatusOpen)
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

	d.InsertTask(1, "new task", "{}", "")

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

	d.InsertTask(1, "open task", "{}", "")
	id2, _ := d.InsertTask(1, "done task", "{}", "")
	d.UpdateTask(id2, db.TaskStatusDone, "")

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

	d.InsertTask(1, "immedio task", "{}", "")
	d.InsertTask(2, "hearable task", "{}", "")

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

	d.InsertTask(1, "task to move", "{}", "")

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

	d.InsertTask(1, "task to update", "{}", "")

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
	if task.Status != db.TaskStatusDone {
		t.Errorf("status = %q, want %q", task.Status, db.TaskStatusDone)
	}
}

func TestTaskUpdate_UnknownProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task", "{}", "")

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

	d.InsertTask(1, "task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", "1"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error when neither --status nor --project is given")
	}
}

func TestTaskUpdate_MetaOnly(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertTask(1, "task", `{"old":"data"}`, "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", fmt.Sprintf("%d", id), "--meta", `{"url":"https://example.com"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if task.Metadata != `{"old":"data","url":"https://example.com"}` {
		t.Errorf("expected merged metadata, got %s", task.Metadata)
	}
	if !strings.Contains(buf.String(), "metadata: updated") {
		t.Errorf("expected output to mention metadata update, got %s", buf.String())
	}
}

func TestTaskUpdate_InvalidMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", "1", "--meta", "not-json"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTaskUpdate_InvalidStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"task", "update", "1", "--status", "closed"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestTaskGet(t *testing.T) {
	tests := []struct {
		name      string
		setupTask bool
		args      []string
		wantErr   bool
		check     func(t *testing.T, output []byte)
	}{
		{
			name:      "success with actions",
			setupTask: true,
			wantErr:   false,
			check: func(t *testing.T, output []byte) {
				t.Helper()
				var row map[string]any
				if err := json.Unmarshal(output, &row); err != nil {
					t.Fatalf("JSON parse error: %v\noutput: %s", err, string(output))
				}
				if row["title"] != "my task" {
					t.Errorf("title = %v, want %q", row["title"], "my task")
				}
				if row["metadata"] != `{"url":"https://example.com"}` {
					t.Errorf("metadata = %v", row["metadata"])
				}
				actions, ok := row["actions"].([]any)
				if !ok {
					t.Fatalf("actions field missing or wrong type: %v", row["actions"])
				}
				if len(actions) != 1 {
					t.Fatalf("expected 1 action, got %d", len(actions))
				}
				action := actions[0].(map[string]any)
				if action["title"] != "review action" {
					t.Errorf("action title = %v, want %q", action["title"], "review action")
				}
			},
		},
		{
			name:    "not found",
			args:    []string{"task", "get", "999"},
			wantErr: true,
		},
		{
			name:    "invalid ID",
			args:    []string{"task", "get", "abc"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			args := tc.args
			if tc.setupTask {
				taskID, _ := d.InsertTask(1, "my task", `{"url":"https://example.com"}`, "")
				d.InsertAction("review action", taskID, `{"pr":1}`, db.ActionStatusPending, nil)
				args = []string{"task", "get", fmt.Sprintf("%d", taskID)}
			}

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(args)

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, buf.Bytes())
			}
		})
	}
}

func TestTaskList_WithActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID1, _ := d.InsertTask(1, "task with actions", "{}", "")
	taskID2, _ := d.InsertTask(1, "task without actions", "{}", "")
	d.InsertAction("review-pr", taskID1, `{"pr":1}`, db.ActionStatusPending, nil)
	d.InsertAction("implement", taskID1, "{}", db.ActionStatusDone, nil)
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

	// Task without actions (task2, higher ID = first in DESC) — should be empty array, not null
	actions2, ok := rows[0]["actions"].([]any)
	if !ok {
		t.Fatalf("actions field missing or wrong type for task 2: %v", rows[0]["actions"])
	}
	if len(actions2) != 0 {
		t.Errorf("task 2 actions = %d, want 0", len(actions2))
	}

	// Task with actions (task1, lower ID = second in DESC)
	actions1, ok := rows[1]["actions"].([]any)
	if !ok {
		t.Fatalf("actions field missing or wrong type for task 1: %v", rows[1]["actions"])
	}
	if len(actions1) != 2 {
		t.Errorf("task 1 actions = %d, want 2", len(actions1))
	}
	firstAction := actions1[0].(map[string]any)
	if firstAction["title"] != "review-pr" {
		t.Errorf("first action title = %v, want %q", firstAction["title"], "review-pr")
	}
}

func TestTaskCreateHelp_WithProjects(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "create", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "Create a new task under a project") {
		t.Errorf("output missing standard help text:\n%s", out)
	}
	if !contains(out, "[agent hint] Available projects:") {
		t.Errorf("output missing agent hint:\n%s", out)
	}
	if !contains(out, "1: immedio") {
		t.Errorf("output missing project 1:\n%s", out)
	}
	if !contains(out, "2: hearable") {
		t.Errorf("output missing project 2:\n%s", out)
	}
	if !contains(out, "3: works") {
		t.Errorf("output missing project 3:\n%s", out)
	}
}

func TestTaskCreateHelp_NoProjects(t *testing.T) {
	d := testutil.NewTestDB(t)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "create", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "Create a new task under a project") {
		t.Errorf("output missing standard help text:\n%s", out)
	}
	if contains(out, "[agent hint]") {
		t.Errorf("output should not contain agent hint when no projects exist:\n%s", out)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
