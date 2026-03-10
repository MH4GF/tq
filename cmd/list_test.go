package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestList(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "task1", "", "{}", "")
	d.InsertAction("review-pr", "review-pr", &taskID, "{}", "pending")
	d.InsertAction("deploy", "deploy", nil, "{}", "running")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list"})

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
	if rows[0]["prompt_id"] != "review-pr" {
		t.Errorf("first row prompt_id = %v, want %q", rows[0]["prompt_id"], "review-pr")
	}
	if rows[1]["prompt_id"] != "deploy" {
		t.Errorf("second row prompt_id = %v, want %q", rows[1]["prompt_id"], "deploy")
	}
}

func TestList_StatusFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertAction("a", "a", nil, "{}", "pending")
	d.InsertAction("b", "b", nil, "{}", "running")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--status", "pending"})

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
	if rows[0]["prompt_id"] != "a" {
		t.Errorf("prompt_id = %v, want %q", rows[0]["prompt_id"], "a")
	}
}

func TestList_TaskFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID1, _ := d.InsertTask(1, "task1", "", "{}", "")
	taskID2, _ := d.InsertTask(1, "task2", "", "{}", "")
	d.InsertAction("a", "a", &taskID1, "{}", "pending")
	d.InsertAction("b", "b", &taskID2, "{}", "pending")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--task", "1"})

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
	if rows[0]["prompt_id"] != "a" {
		t.Errorf("prompt_id = %v, want %q", rows[0]["prompt_id"], "a")
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

func TestList_JSON(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "", "{}", "")
	actionID, _ := d.InsertAction("review-pr", "review-pr", &taskID, "{}", "pending")

	longResult := "Line 1\nLine 2\nThis is a very long result string that exceeds sixty characters and should NOT be truncated in JSON output"
	d.MarkDone(actionID, longResult)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--task", "1", "--status", "done"})

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

	if row["prompt_id"] != "review-pr" {
		t.Errorf("prompt_id = %v, want %q", row["prompt_id"], "review-pr")
	}
	if row["result"] != longResult {
		t.Errorf("result should contain full text including newlines, got %v", row["result"])
	}
	if row["status"] != "done" {
		t.Errorf("status = %v, want %q", row["status"], "done")
	}
	if row["task_id"] != float64(taskID) {
		t.Errorf("task_id = %v, want %v", row["task_id"], taskID)
	}
	if row["completed_at"] == nil {
		t.Error("completed_at should not be null for done action")
	}
}

func TestList_JSON_NullFields(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertAction("implement", "implement", nil, "{}", "pending")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--status", "pending", "--task", "0"})

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

	if row["task_id"] != nil {
		t.Errorf("task_id should be null, got %v", row["task_id"])
	}
	if row["result"] != nil {
		t.Errorf("result should be null, got %v", row["result"])
	}
	if row["session_id"] != nil {
		t.Errorf("session_id should be null, got %v", row["session_id"])
	}
	if row["started_at"] != nil {
		t.Errorf("started_at should be null, got %v", row["started_at"])
	}
	if row["completed_at"] != nil {
		t.Errorf("completed_at should be null, got %v", row["completed_at"])
	}
	}
