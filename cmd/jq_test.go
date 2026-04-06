package cmd_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		jqExpr  string
		fields  []string
		want    string
		wantErr string
	}{
		{
			name:   "no filter",
			data:   []map[string]any{{"id": 1, "name": "test"}},
			jqExpr: "",
			want:   "[\n  {\n",
		},
		{
			name:   "field select",
			data:   []map[string]any{{"id": 1, "name": "alice"}, {"id": 2, "name": "bob"}},
			jqExpr: ".[].name",
			want:   "alice\nbob\n",
		},
		{
			name:   "map",
			data:   []map[string]any{{"id": 1}, {"id": 2}},
			jqExpr: "[.[].id]",
			want:   "[1,2]\n",
		},
		{
			name:   "length",
			data:   []any{1, 2, 3},
			jqExpr: "length",
			want:   "3\n",
		},
		{
			name:   "empty array",
			data:   []any{},
			jqExpr: "length",
			want:   "0\n",
		},
		{
			name:   "string raw output",
			data:   map[string]any{"name": "hello"},
			jqExpr: ".name",
			want:   "hello\n",
		},
		{
			name:   "null value",
			data:   map[string]any{"x": nil},
			jqExpr: ".x",
			want:   "null\n",
		},
		{
			name:    "invalid expression",
			data:    []any{},
			jqExpr:  ".[invalid",
			wantErr: "jq parse error",
		},
		{
			name:    "runtime error with fields hint",
			data:    "not-an-array",
			jqExpr:  ".[]",
			fields:  []string{"id", "name"},
			wantErr: "available fields: id, name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := cmd.WriteJSON(&buf, tt.data, tt.jqExpr, tt.fields)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != "" && !strings.Contains(buf.String(), tt.want) {
				t.Errorf("output = %q, want containing %q", buf.String(), tt.want)
			}
		})
	}
}

func TestList_JQ(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "task1", "{}", "")
	d.InsertAction("review-pr", taskID, "{}", db.ActionStatusPending)
	d.InsertAction("deploy", taskID, "{}", db.ActionStatusRunning)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--jq", ".[].title"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out)
	}
	if lines[0] != "deploy" {
		t.Errorf("line 0 = %q, want %q", lines[0], "deploy")
	}
	if lines[1] != "review-pr" {
		t.Errorf("line 1 = %q, want %q", lines[1], "review-pr")
	}
}

func TestActionGet_JQ(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "task1", "{}", "")
	actionID, _ := d.InsertAction("review-pr", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "get", fmt.Sprintf("%d", actionID), "--jq", ".title"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "review-pr" {
		t.Errorf("got %q, want %q", got, "review-pr")
	}
}

func TestTaskGet_JQ(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "my-task", "{}", "")
	d.InsertAction("action1", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"task", "get", fmt.Sprintf("%d", taskID), "--jq", ".title"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "my-task" {
		t.Errorf("got %q, want %q", got, "my-task")
	}
}

func TestSearch_JQ(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "login-bug", "{}", "")
	d.InsertAction("fix-login", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"search", "login", "--jq", ".[].entity_type"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("expected search results, got empty output")
	}
	for line := range strings.SplitSeq(out, "\n") {
		if line != "task" && line != "action" {
			t.Errorf("unexpected entity_type %q", line)
		}
	}
}

func TestEventList_JQ(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "task1", "{}", "")
	d.InsertAction("action1", taskID, "{}", db.ActionStatusPending)

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"event", "list", "--jq", ".[].event_type"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("expected events, got empty output")
	}
}

func TestList_JQ_InvalidExpr(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "list", "--jq", ".[invalid"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid jq expression")
	}
	if !strings.Contains(err.Error(), "jq parse error") {
		t.Errorf("error = %q, want containing 'jq parse error'", err.Error())
	}
}
