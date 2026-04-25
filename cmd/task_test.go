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

func TestTaskCreate(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantOutContains []string
		wantErrContains string
		wantErr         bool
		check           func(t *testing.T, d db.Store)
	}{
		{
			name:            "success",
			args:            []string{"task", "create", "test task", "--project", "1", "--meta", `{"url":"https://example.com"}`},
			wantOutContains: []string{"task #1 created", "project: immedio"},
			check: func(t *testing.T, d db.Store) {
				t.Helper()
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
			},
		},
		{
			name:            "invalid meta JSON",
			args:            []string{"task", "create", "test task", "--project", "1", "--meta", "{invalid}"},
			wantErr:         true,
			wantErrContains: "invalid JSON for --meta (must be a JSON object)",
		},
		{
			name:    "missing --project flag",
			args:    []string{"task", "create", "test"},
			wantErr: true,
		},
		{
			name:    "unknown project ID",
			args:    []string{"task", "create", "test", "--project", "999"},
			wantErr: true,
		},
		{
			name:    "missing title argument",
			args:    []string{"task", "create", "--project", "1"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tc.wantErrContains != "" && !contains(err.Error(), tc.wantErrContains) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			out := buf.String()
			for _, want := range tc.wantOutContains {
				if !contains(out, want) {
					t.Errorf("output = %q, want to contain %q", out, want)
				}
			}
			if tc.check != nil {
				tc.check(t, d)
			}
		})
	}
}

func TestTaskList(t *testing.T) {
	tests := []struct {
		name    string
		seed    func(t *testing.T, d db.Store)
		args    []string
		wantOut string
		check   func(t *testing.T, rows []map[string]any)
	}{
		{
			name: "ordered DESC by id",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				d.InsertTask(1, "task A", `{"url":"https://example.com/a"}`, "")
				d.InsertTask(1, "task B", "{}", "")
			},
			args: []string{"task", "list"},
			check: func(t *testing.T, rows []map[string]any) {
				t.Helper()
				if len(rows) != 2 {
					t.Fatalf("expected 2 rows, got %d", len(rows))
				}
				if rows[0]["title"] != "task B" {
					t.Errorf("first row title = %v, want %q", rows[0]["title"], "task B")
				}
				if rows[1]["title"] != "task A" {
					t.Errorf("second row title = %v, want %q", rows[1]["title"], "task A")
				}
			},
		},
		{
			name: "JSON full fields",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				d.InsertTask(1, "test task", `{"key":"value","url":"https://example.com"}`, "")
			},
			args: []string{"task", "list"},
			check: func(t *testing.T, rows []map[string]any) {
				t.Helper()
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
			},
		},
		{
			name: "null updated_at for new task",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				d.InsertTask(1, "new task", "{}", "")
			},
			args: []string{"task", "list"},
			check: func(t *testing.T, rows []map[string]any) {
				t.Helper()
				if len(rows) != 1 {
					t.Fatalf("expected 1 row, got %d", len(rows))
				}
				if rows[0]["updated_at"] != nil {
					t.Errorf("updated_at should be null for new task, got %v", rows[0]["updated_at"])
				}
			},
		},
		{
			name: "status filter",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				d.InsertTask(1, "open task", "{}", "")
				id2, _ := d.InsertTask(1, "done task", "{}", "")
				d.UpdateTask(id2, db.TaskStatusDone, "")
			},
			args: []string{"task", "list", "--status", "open"},
			check: func(t *testing.T, rows []map[string]any) {
				t.Helper()
				if len(rows) != 1 {
					t.Fatalf("expected 1 row, got %d", len(rows))
				}
				if rows[0]["title"] != "open task" {
					t.Errorf("title = %v, want %q", rows[0]["title"], "open task")
				}
			},
		},
		{
			name: "project filter",
			seed: func(t *testing.T, d db.Store) {
				t.Helper()
				d.InsertTask(1, "immedio task", "{}", "")
				d.InsertTask(2, "hearable task", "{}", "")
			},
			args: []string{"task", "list", "--project", "1"},
			check: func(t *testing.T, rows []map[string]any) {
				t.Helper()
				if len(rows) != 1 {
					t.Fatalf("expected 1 row, got %d", len(rows))
				}
				if rows[0]["title"] != "immedio task" {
					t.Errorf("title = %v, want %q", rows[0]["title"], "immedio task")
				}
			},
		},
		{
			name:    "empty",
			args:    []string{"task", "list"},
			wantOut: "[]",
			check: func(t *testing.T, rows []map[string]any) {
				t.Helper()
				if len(rows) != 0 {
					t.Errorf("expected 0 rows, got %d", len(rows))
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

			if tc.wantOut != "" && !contains(buf.String(), tc.wantOut) {
				t.Errorf("output = %q, want to contain %q", buf.String(), tc.wantOut)
			}

			var rows []map[string]any
			if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
				t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
			}
			if tc.check != nil {
				tc.check(t, rows)
			}
		})
	}
}

func TestTaskUpdate(t *testing.T) {
	tests := []struct {
		name            string
		insertMetadata  string
		args            []string
		wantErr         bool
		wantErrContains string
		wantOutContains []string
		wantProjectID   int64
		wantStatus      string
		wantMetadata    string
	}{
		{
			name:            "project only",
			args:            []string{"task", "update", "1", "--project", "2"},
			wantOutContains: []string{"project: hearable"},
			wantProjectID:   2,
		},
		{
			name:            "status and project",
			args:            []string{"task", "update", "1", "--status", "done", "--note", "test transition", "--project", "2"},
			wantOutContains: []string{"project: hearable", "status: done"},
			wantProjectID:   2,
			wantStatus:      db.TaskStatusDone,
		},
		{
			name:    "unknown project",
			args:    []string{"task", "update", "1", "--project", "999"},
			wantErr: true,
		},
		{
			name:    "neither status nor project",
			args:    []string{"task", "update", "1"},
			wantErr: true,
		},
		{
			name:            "meta only",
			insertMetadata:  `{"old":"data"}`,
			args:            []string{"task", "update", "1", "--meta", `{"url":"https://example.com"}`},
			wantOutContains: []string{"metadata: updated"},
			wantMetadata:    `{"old":"data","url":"https://example.com"}`,
		},
		{
			name:    "invalid meta",
			args:    []string{"task", "update", "1", "--meta", "not-json"},
			wantErr: true,
		},
		{
			name:    "invalid status",
			args:    []string{"task", "update", "1", "--status", "closed", "--note", "irrelevant"},
			wantErr: true,
		},
		{
			name:            "status requires note",
			args:            []string{"task", "update", "1", "--status", "done"},
			wantErr:         true,
			wantErrContains: "--note is required when --status is given",
		},
		{
			name:            "note requires status",
			args:            []string{"task", "update", "1", "--note", "stray note", "--project", "2"},
			wantErr:         true,
			wantErrContains: "--note requires --status",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			meta := tc.insertMetadata
			if meta == "" {
				meta = "{}"
			}
			d.InsertTask(1, "task", meta, "")

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tc.wantErrContains != "" && !contains(err.Error(), tc.wantErrContains) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			out := buf.String()
			for _, want := range tc.wantOutContains {
				if !contains(out, want) {
					t.Errorf("output = %q, want to contain %q", out, want)
				}
			}

			task, err := d.GetTask(1)
			if err != nil {
				t.Fatalf("get task: %v", err)
			}
			if tc.wantProjectID != 0 && task.ProjectID != tc.wantProjectID {
				t.Errorf("project_id = %d, want %d", task.ProjectID, tc.wantProjectID)
			}
			if tc.wantStatus != "" && task.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", task.Status, tc.wantStatus)
			}
			if tc.wantMetadata != "" && task.Metadata != tc.wantMetadata {
				t.Errorf("metadata = %q, want %q", task.Metadata, tc.wantMetadata)
			}
		})
	}
}

func TestTaskUpdate_WithNote(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertTask(1, "task with note", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "update", fmt.Sprintf("%d", id), "--status", "done", "--note", "merged in PR #99"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events, err := d.ListEvents("task", id)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var found bool
	for _, e := range events {
		if e.EventType != "task.status_changed" {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal([]byte(e.Payload), &p); err != nil {
			t.Fatalf("parse payload: %v", err)
		}
		if p["reason"] != "merged in PR #99" {
			t.Errorf("reason = %v, want %q", p["reason"], "merged in PR #99")
		}
		if p["from"] != "open" || p["to"] != "done" {
			t.Errorf("from/to = %v/%v, want open/done", p["from"], p["to"])
		}
		found = true
	}
	if !found {
		t.Fatal("expected task.status_changed event")
	}
}

func TestTaskGet_StatusHistory(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertTask(1, "task with history", "{}", "")
	if err := d.UpdateTask(id, db.TaskStatusDone, "first reason"); err != nil {
		t.Fatalf("update 1: %v", err)
	}
	if err := d.UpdateTask(id, db.TaskStatusOpen, ""); err != nil {
		t.Fatalf("update 2: %v", err)
	}

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "get", fmt.Sprintf("%d", id)})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}
	history, ok := row["status_history"].([]any)
	if !ok {
		t.Fatalf("status_history missing or wrong type: %v", row["status_history"])
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d: %v", len(history), history)
	}

	first := history[0].(map[string]any)
	if first["from"] != "open" || first["to"] != "done" {
		t.Errorf("first entry from/to = %v/%v, want open/done", first["from"], first["to"])
	}
	if first["reason"] != "first reason" {
		t.Errorf("first entry reason = %v, want %q", first["reason"], "first reason")
	}
	if first["at"] == nil || first["at"] == "" {
		t.Errorf("first entry at missing: %v", first["at"])
	}

	second := history[1].(map[string]any)
	if second["from"] != "done" || second["to"] != "open" {
		t.Errorf("second entry from/to = %v/%v, want done/open", second["from"], second["to"])
	}
	if _, hasReason := second["reason"]; hasReason {
		t.Errorf("second entry should omit reason when empty, got %v", second["reason"])
	}
}

func TestTaskGet_StatusHistory_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertTask(1, "fresh task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "get", fmt.Sprintf("%d", id)})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}
	history, ok := row["status_history"].([]any)
	if !ok {
		t.Fatalf("status_history should be empty array, got: %v", row["status_history"])
	}
	if len(history) != 0 {
		t.Errorf("expected empty history for new task, got %d entries", len(history))
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

func TestTaskList_ActionsLimit(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		totalActions int
		wantLen      int
		wantFirst    string
		wantLast     string
	}{
		{name: "task list under limit", endpoint: "list", totalActions: 3, wantLen: 3, wantFirst: "action-1", wantLast: "action-3"},
		{name: "task list at limit", endpoint: "list", totalActions: 10, wantLen: 10, wantFirst: "action-1", wantLast: "action-10"},
		{name: "task list over limit trims oldest", endpoint: "list", totalActions: 15, wantLen: 10, wantFirst: "action-6", wantLast: "action-15"},
		{name: "task get under limit", endpoint: "get", totalActions: 3, wantLen: 3, wantFirst: "action-1", wantLast: "action-3"},
		{name: "task get at limit", endpoint: "get", totalActions: 10, wantLen: 10, wantFirst: "action-1", wantLast: "action-10"},
		{name: "task get over limit trims oldest", endpoint: "get", totalActions: 15, wantLen: 10, wantFirst: "action-6", wantLast: "action-15"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			taskID, _ := d.InsertTask(1, "task", "{}", "")
			for i := 1; i <= tc.totalActions; i++ {
				d.InsertAction(fmt.Sprintf("action-%d", i), taskID, "{}", db.ActionStatusPending, nil)
			}

			var args []string
			switch tc.endpoint {
			case "list":
				args = []string{"task", "list"}
			case "get":
				args = []string{"task", "get", fmt.Sprintf("%d", taskID)}
			}

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var actions []any
			switch tc.endpoint {
			case "list":
				var rows []map[string]any
				if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
					t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
				}
				if len(rows) != 1 {
					t.Fatalf("expected 1 row, got %d", len(rows))
				}
				actions = rows[0]["actions"].([]any)
			case "get":
				var row map[string]any
				if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
					t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
				}
				actions = row["actions"].([]any)
			}

			if len(actions) != tc.wantLen {
				t.Fatalf("actions len = %d, want %d", len(actions), tc.wantLen)
			}
			first := actions[0].(map[string]any)["title"].(string)
			last := actions[len(actions)-1].(map[string]any)["title"].(string)
			if first != tc.wantFirst {
				t.Errorf("first action title = %q, want %q", first, tc.wantFirst)
			}
			if last != tc.wantLast {
				t.Errorf("last action title = %q, want %q", last, tc.wantLast)
			}
		})
	}
}

func TestActionList_NoLimit_ReturnsAll(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "task", "{}", "")
	const total = 15
	for i := 1; i <= total; i++ {
		d.InsertAction(fmt.Sprintf("action-%d", i), taskID, "{}", db.ActionStatusPending, nil)
	}

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--task", fmt.Sprintf("%d", taskID)})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}
	if len(rows) != total {
		t.Errorf("action list len = %d, want %d (should not be capped)", len(rows), total)
	}
}

func TestTaskCreateHelp(t *testing.T) {
	tests := []struct {
		name            string
		seedProjects    bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "with projects",
			seedProjects: true,
			wantContains: []string{
				"Create a new task under a project",
				"[agent hint] Available projects:",
				"1: immedio",
				"2: hearable",
				"3: works",
			},
		},
		{
			name:         "no projects",
			seedProjects: false,
			wantContains: []string{
				"Create a new task under a project",
			},
			wantNotContains: []string{
				"[agent hint]",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			if tc.seedProjects {
				testutil.SeedTestProjects(t, d)
			}
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
			for _, s := range tc.wantContains {
				if !contains(out, s) {
					t.Errorf("output missing %q:\n%s", s, out)
				}
			}
			for _, s := range tc.wantNotContains {
				if contains(out, s) {
					t.Errorf("output should not contain %q:\n%s", s, out)
				}
			}
		})
	}
}

func TestTaskNote(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "success",
			args: []string{"task", "note", "1", "--kind", "triage_keep", "--reason", "PR review pending"},
		},
		{
			name: "with metadata",
			args: []string{"task", "note", "1", "--kind", "triage_keep", "--reason", "snoozed", "--metadata", `{"snooze_until":"2026-05-02"}`},
		},
		{
			name:            "missing kind",
			args:            []string{"task", "note", "1", "--reason", "x"},
			wantErr:         true,
			wantErrContains: "--kind is required",
		},
		{
			name:            "missing reason",
			args:            []string{"task", "note", "1", "--kind", "triage_keep"},
			wantErr:         true,
			wantErrContains: "--reason is required",
		},
		{
			name:            "invalid metadata JSON",
			args:            []string{"task", "note", "1", "--kind", "triage_keep", "--reason", "x", "--metadata", "{notjson"},
			wantErr:         true,
			wantErrContains: "invalid JSON for --metadata",
		},
		{
			name:    "task not found",
			args:    []string{"task", "note", "999", "--kind", "triage_keep", "--reason", "x"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()
			d.InsertTask(1, "task", "{}", "")

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tc.wantErrContains != "" && !contains(err.Error(), tc.wantErrContains) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTaskGet_Notes(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertTask(1, "task with notes", "{}", "")
	if err := d.RecordTaskNote(id, "triage_keep", "PR review pending", map[string]any{"snooze_until": "2026-05-02"}); err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTask(id, db.TaskStatusDone, "merged"); err != nil {
		t.Fatal(err)
	}

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "get", fmt.Sprintf("%d", id)})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, buf.String())
	}

	notes, ok := row["notes"].([]any)
	if !ok {
		t.Fatalf("notes missing or wrong type: %v", row["notes"])
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	note := notes[0].(map[string]any)
	if note["kind"] != "triage_keep" {
		t.Errorf("kind = %v, want triage_keep", note["kind"])
	}
	if note["reason"] != "PR review pending" {
		t.Errorf("reason = %v", note["reason"])
	}

	// status_history is unaffected by note
	history, ok := row["status_history"].([]any)
	if !ok {
		t.Fatalf("status_history missing: %v", row["status_history"])
	}
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestTaskGet_NotesEmpty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertTask(1, "fresh task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "get", fmt.Sprintf("%d", id)})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var row map[string]any
	if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}
	notes, ok := row["notes"].([]any)
	if !ok {
		t.Fatalf("notes should be empty array, got %v", row["notes"])
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

func TestTaskList_LatestTriageNote(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id1, _ := d.InsertTask(1, "with note", "{}", "")
	d.InsertTask(1, "without note", "{}", "")
	if err := d.RecordTaskNote(id1, "triage_keep", "design review pending", map[string]any{"snooze_until": "2026-05-02"}); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("JSON parse error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// ordered DESC: most recent (without note, id2) first
	if rows[0]["latest_triage_note"] != nil {
		t.Errorf("row 0 latest_triage_note should be null, got %v", rows[0]["latest_triage_note"])
	}
	note, ok := rows[1]["latest_triage_note"].(map[string]any)
	if !ok {
		t.Fatalf("row 1 latest_triage_note missing/wrong type: %v", rows[1]["latest_triage_note"])
	}
	if note["reason"] != "design review pending" {
		t.Errorf("reason = %v", note["reason"])
	}
	if note["snooze_until"] != "2026-05-02" {
		t.Errorf("snooze_until = %v", note["snooze_until"])
	}
}

func TestTaskUpdate_NoteIndependence(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	id, _ := d.InsertTask(1, "task", "{}", "")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "update", fmt.Sprintf("%d", id), "--status", "done", "--note", "merged"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notes, err := d.TaskNotes(id, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("task update --note should not create task.note events, got %d", len(notes))
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
