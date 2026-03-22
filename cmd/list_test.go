package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestList(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "task1", "{}", "")
	d.InsertAction("review-pr", taskID, "{}", db.ActionStatusPending)
	taskID2, _ := d.InsertTask(1, "task2", "{}", "")
	d.InsertAction("deploy", taskID2, "{}", db.ActionStatusRunning)

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
	if rows[0]["title"] != "deploy" {
		t.Errorf("first row title = %v, want %q", rows[0]["title"], "deploy")
	}
	if rows[1]["title"] != "review-pr" {
		t.Errorf("second row title = %v, want %q", rows[1]["title"], "review-pr")
	}
}

func TestList_StatusFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusPending)
	d.InsertAction("b", taskID, "{}", db.ActionStatusRunning)

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
	if rows[0]["title"] != "a" {
		t.Errorf("title = %v, want %q", rows[0]["title"], "a")
	}
}

func TestList_TaskFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID1, _ := d.InsertTask(1, "task1", "{}", "")
	taskID2, _ := d.InsertTask(1, "task2", "{}", "")
	d.InsertAction("a", taskID1, "{}", db.ActionStatusPending)
	d.InsertAction("b", taskID2, "{}", db.ActionStatusPending)

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
	if rows[0]["title"] != "a" {
		t.Errorf("title = %v, want %q", rows[0]["title"], "a")
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
	if !contains(out, "[]") {
		t.Errorf("output = %q, want '[]'", out)
	}
}

func TestList_JSON(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	actionID, _ := d.InsertAction("review-pr", taskID, "{}", db.ActionStatusPending)

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

	if row["title"] != "review-pr" {
		t.Errorf("title = %v, want %q", row["title"], "review-pr")
	}
	if row["result"] != longResult {
		t.Errorf("result should contain full text including newlines, got %v", row["result"])
	}
	if row["status"] != db.ActionStatusDone {
		t.Errorf("status = %v, want %q", row["status"], db.ActionStatusDone)
	}
	if row["task_id"] != float64(taskID) {
		t.Errorf("task_id = %v, want %v", row["task_id"], taskID)
	}
	if row["completed_at"] == nil {
		t.Error("completed_at should not be null for done action")
	}
}

func TestActionGet(t *testing.T) {
	tests := []struct {
		name        string
		setupAction bool
		args        []string
		wantErr     bool
		check       func(t *testing.T, output []byte)
	}{
		{
			name:        "success",
			setupAction: true,
			wantErr:     false,
			check: func(t *testing.T, output []byte) {
				t.Helper()
				var row map[string]any
				if err := json.Unmarshal(output, &row); err != nil {
					t.Fatalf("JSON parse error: %v\noutput: %s", err, string(output))
				}
				if row["title"] != "review action" {
					t.Errorf("title = %v, want %q", row["title"], "review action")
				}
				if row["result"] != nil {
					t.Errorf("result should be null, got %v", row["result"])
				}
			},
		},
		{
			name:    "not found",
			args:    []string{"action", "get", "999"},
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
			if tc.setupAction {
				taskID, _ := d.InsertTask(1, "test task", "{}", "")
				actionID, _ := d.InsertAction("review action", taskID, `{"pr":1}`, db.ActionStatusPending)
				args = []string{"action", "get", fmt.Sprintf("%d", actionID)}
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

func TestList_JSON_MissingResult(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("implement", taskID, "{}", db.ActionStatusPending)

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

	if row["task_id"] != float64(taskID) {
		t.Errorf("task_id = %v, want %v", row["task_id"], taskID)
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
