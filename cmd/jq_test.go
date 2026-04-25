package cmd_test

import (
	"bytes"
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

func TestJQFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		jqExpr  string
		wantOut string
		wantErr string
	}{
		{
			name:    "action list",
			args:    []string{"action", "list"},
			jqExpr:  ".[].title",
			wantOut: "deploy\nreview-pr",
		},
		{
			name:    "action get",
			args:    []string{"action", "get", "1"},
			jqExpr:  ".title",
			wantOut: "review-pr",
		},
		{
			name:    "task get",
			args:    []string{"task", "get", "1"},
			jqExpr:  ".title",
			wantOut: "login-bug",
		},
		{
			name:    "search",
			args:    []string{"search", "login"},
			jqExpr:  ".[].entity_type",
			wantOut: "task",
		},
		{
			name:    "event list",
			args:    []string{"event", "list"},
			jqExpr:  "length > 0",
			wantOut: "true",
		},
		{
			name:    "invalid expression",
			args:    []string{"action", "list"},
			jqExpr:  ".[invalid",
			wantErr: "jq parse error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			taskID, _ := d.InsertTask(1, "login-bug", "{}", "")
			d.InsertAction("review-pr", taskID, "{}", db.ActionStatusPending, nil)
			d.InsertAction("deploy", taskID, "{}", db.ActionStatusRunning, nil)

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(append(tt.args, "--jq", tt.jqExpr))

			err := root.Execute()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.TrimSpace(buf.String()); got != tt.wantOut {
				t.Errorf("output = %q, want %q", got, tt.wantOut)
			}
		})
	}
}
