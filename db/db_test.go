package db_test

import (
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestOpen(t *testing.T) {
	d := testutil.NewTestDB(t)
	var mode string
	if err := d.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" && mode != "memory" {
		t.Errorf("expected wal or memory journal mode, got %s", mode)
	}
}

func TestMigrate(t *testing.T) {
	d := testutil.NewTestDB(t)

	tables := []string{"projects", "tasks", "actions"}
	for _, table := range tables {
		var name string
		err := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestSeedTestProjects(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 projects, got %d", count)
	}
}

func TestClose(t *testing.T) {
	d := testutil.NewTestDB(t)
	if err := d.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestMigrateLegacyClaudeFlags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantArgs []string // nil means claude_args should be absent
	}{
		{
			name:     "permission_mode only",
			input:    `{"instruction":"x","permission_mode":"plan"}`,
			wantArgs: []string{"--permission-mode", "plan"},
		},
		{
			name:     "worktree only",
			input:    `{"instruction":"x","worktree":true}`,
			wantArgs: []string{"--worktree"},
		},
		{
			name:     "permission_mode and worktree",
			input:    `{"instruction":"x","permission_mode":"plan","worktree":true}`,
			wantArgs: []string{"--permission-mode", "plan", "--worktree"},
		},
		{
			name:     "merges with existing claude_args",
			input:    `{"instruction":"x","claude_args":["--max-turns","5"],"permission_mode":"plan","worktree":true}`,
			wantArgs: []string{"--max-turns", "5", "--permission-mode", "plan", "--worktree"},
		},
		{
			name:     "worktree false drops key without adding flag",
			input:    `{"instruction":"x","worktree":false}`,
			wantArgs: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "test", "{}", "")
			if _, err := d.Exec(
				"INSERT INTO actions(title, task_id, metadata, status) VALUES(?, ?, ?, 'pending')",
				"action", taskID, tc.input,
			); err != nil {
				t.Fatalf("insert action: %v", err)
			}
			if _, err := d.Exec(
				"INSERT INTO schedules(task_id, instruction, title, cron_expr, metadata) VALUES(?, ?, ?, ?, ?)",
				taskID, "/x", "sched", "* * * * *", tc.input,
			); err != nil {
				t.Fatalf("insert schedule: %v", err)
			}

			if err := d.Migrate(); err != nil {
				t.Fatalf("re-migrate: %v", err)
			}

			for _, table := range []string{"actions", "schedules"} {
				var got string
				if err := d.QueryRow("SELECT metadata FROM " + table + " ORDER BY id DESC LIMIT 1").Scan(&got); err != nil {
					t.Fatalf("read %s metadata: %v", table, err)
				}
				m := make(map[string]any)
				if err := json.Unmarshal([]byte(got), &m); err != nil {
					t.Fatalf("parse %s metadata %q: %v", table, got, err)
				}
				if _, exists := m["permission_mode"]; exists {
					t.Errorf("%s: legacy permission_mode key still present in %q", table, got)
				}
				if _, exists := m["worktree"]; exists {
					t.Errorf("%s: legacy worktree key still present in %q", table, got)
				}
				rawArgs, hasArgs := m["claude_args"]
				if tc.wantArgs == nil {
					if hasArgs {
						t.Errorf("%s: claude_args should be absent, got %v", table, rawArgs)
					}
					continue
				}
				arr, ok := rawArgs.([]any)
				if !ok {
					t.Fatalf("%s: claude_args missing or wrong type in %q", table, got)
				}
				if len(arr) != len(tc.wantArgs) {
					t.Fatalf("%s: claude_args len = %d, want %d (%v)", table, len(arr), len(tc.wantArgs), arr)
				}
				for i, want := range tc.wantArgs {
					if arr[i] != want {
						t.Errorf("%s: claude_args[%d] = %v, want %q", table, i, arr[i], want)
					}
				}
			}
		})
	}
}

func TestMigrateLegacyClaudeFlags_NoOpOnCleanRow(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "test", "{}", "")
	const clean = `{"instruction":"x","claude_args":["--max-turns","5"]}`
	if _, err := d.Exec(
		"INSERT INTO actions(title, task_id, metadata, status) VALUES(?, ?, ?, 'pending')",
		"action", taskID, clean,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := d.Migrate(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	var got string
	if err := d.QueryRow("SELECT metadata FROM actions ORDER BY id DESC LIMIT 1").Scan(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != clean {
		t.Errorf("clean row should be untouched. got %q, want %q", got, clean)
	}
}
