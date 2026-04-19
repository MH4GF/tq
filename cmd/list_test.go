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

func parseJSONRows(t *testing.T, output []byte, wantLen int) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal(output, &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, string(output))
	}
	if len(rows) != wantLen {
		t.Fatalf("expected %d rows, got %d", wantLen, len(rows))
	}
	return rows
}

func TestList(t *testing.T) {
	longResult := "Line 1\nLine 2\nThis is a very long result string that exceeds sixty characters and should NOT be truncated in JSON output"

	tests := []struct {
		name  string
		seed  func(t *testing.T, d db.Store)
		args  []string
		check func(t *testing.T, output []byte)
	}{
		{
			name: "default lists all actions",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				taskID, _ := d.InsertTask(1, "task1", "{}", "")
				d.InsertAction("review-pr", taskID, "{}", db.ActionStatusPending, nil)
				taskID2, _ := d.InsertTask(1, "task2", "{}", "")
				d.InsertAction("deploy", taskID2, "{}", db.ActionStatusRunning, nil)
			},
			args: []string{"action", "list"},
			check: func(t *testing.T, output []byte) {
				t.Helper()
				rows := parseJSONRows(t, output, 2)
				if rows[0]["title"] != "deploy" {
					t.Errorf("first row title = %v, want %q", rows[0]["title"], "deploy")
				}
				if rows[1]["title"] != "review-pr" {
					t.Errorf("second row title = %v, want %q", rows[1]["title"], "review-pr")
				}
			},
		},
		{
			name: "status filter",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				taskID, _ := d.InsertTask(1, "test", "{}", "")
				d.InsertAction("a", taskID, "{}", db.ActionStatusPending, nil)
				d.InsertAction("b", taskID, "{}", db.ActionStatusRunning, nil)
			},
			args: []string{"action", "list", "--status", "pending"},
			check: func(t *testing.T, output []byte) {
				t.Helper()
				rows := parseJSONRows(t, output, 1)
				if rows[0]["title"] != "a" {
					t.Errorf("title = %v, want %q", rows[0]["title"], "a")
				}
			},
		},
		{
			name: "task filter",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				taskID1, _ := d.InsertTask(1, "task1", "{}", "")
				taskID2, _ := d.InsertTask(1, "task2", "{}", "")
				d.InsertAction("a", taskID1, "{}", db.ActionStatusPending, nil)
				d.InsertAction("b", taskID2, "{}", db.ActionStatusPending, nil)
			},
			args: []string{"action", "list", "--task", "1"},
			check: func(t *testing.T, output []byte) {
				t.Helper()
				rows := parseJSONRows(t, output, 1)
				if rows[0]["title"] != "a" {
					t.Errorf("title = %v, want %q", rows[0]["title"], "a")
				}
			},
		},
		{
			name: "empty",
			args: []string{"action", "list"},
			check: func(t *testing.T, output []byte) {
				t.Helper()
				out := string(output)
				if !contains(out, "[]") {
					t.Errorf("output = %q, want '[]'", out)
				}
			},
		},
		{
			name: "JSON preserves full result for done action",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				taskID, _ := d.InsertTask(1, "test task", "{}", "")
				actionID, _ := d.InsertAction("review-pr", taskID, "{}", db.ActionStatusPending, nil)
				d.MarkDone(actionID, longResult)
			},
			args: []string{"action", "list", "--task", "1", "--status", "done"},
			check: func(t *testing.T, output []byte) {
				t.Helper()
				row := parseJSONRows(t, output, 1)[0]
				if row["title"] != "review-pr" {
					t.Errorf("title = %v, want %q", row["title"], "review-pr")
				}
				if row["result"] != longResult {
					t.Errorf("result should contain full text including newlines, got %v", row["result"])
				}
				if row["status"] != db.ActionStatusDone {
					t.Errorf("status = %v, want %q", row["status"], db.ActionStatusDone)
				}
				if row["task_id"] != float64(1) {
					t.Errorf("task_id = %v, want %v", row["task_id"], 1)
				}
				if row["completed_at"] == nil {
					t.Error("completed_at should not be null for done action")
				}
			},
		},
		{
			name: "JSON emits null for missing optional fields",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				taskID, _ := d.InsertTask(1, "test", "{}", "")
				d.InsertAction("implement", taskID, "{}", db.ActionStatusPending, nil)
			},
			args: []string{"action", "list", "--status", "pending", "--task", "0"},
			check: func(t *testing.T, output []byte) {
				t.Helper()
				row := parseJSONRows(t, output, 1)[0]
				if row["task_id"] != float64(1) {
					t.Errorf("task_id = %v, want %v", row["task_id"], 1)
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
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			if tc.seed != nil {
				tc.seed(t, d)
			}

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			if err := root.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tc.check(t, buf.Bytes())
		})
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
				actionID, _ := d.InsertAction("review action", taskID, `{"pr":1}`, db.ActionStatusPending, nil)
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
