package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestProjectCreate(t *testing.T) {
	tests := []struct {
		name            string
		preSeed         func(db.Store)
		args            []string
		wantOutContains []string
		wantErr         bool
		wantErrContains string
		verify          func(*testing.T, db.Store)
	}{
		{
			name:            "success with meta",
			args:            []string{"project", "create", "myapp", "/tmp/myapp", "--meta", `{"key":"val"}`},
			wantOutContains: []string{"project #1 created", "myapp"},
			verify: func(t *testing.T, d db.Store) {
				t.Helper()
				p, err := d.GetProjectByName("myapp")
				if err != nil {
					t.Fatalf("get project: %v", err)
				}
				if p.WorkDir != "/tmp/myapp" {
					t.Errorf("work_dir = %q, want %q", p.WorkDir, "/tmp/myapp")
				}
			},
		},
		{
			name:            "invalid JSON meta",
			args:            []string{"project", "create", "myapp", "/tmp/myapp", "--meta", "{invalid}"},
			wantErr:         true,
			wantErrContains: "invalid JSON for --meta (must be a JSON object)",
		},
		{
			name:    "missing positional args",
			args:    []string{"project", "create", "myapp"},
			wantErr: true,
		},
		{
			name:    "duplicate name",
			preSeed: func(d db.Store) { d.InsertProject("dup", "/tmp/dup", "{}") },
			args:    []string{"project", "create", "dup", "/tmp/dup2"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			cmd.SetDB(d)
			cmd.ResetForTest()

			if tc.preSeed != nil {
				tc.preSeed(d)
			}

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
			if tc.verify != nil {
				tc.verify(t, d)
			}
		})
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
