package db_test

import (
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/db"
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

func TestIsLibsqlURL(t *testing.T) {
	tests := []struct {
		dsn  string
		want bool
	}{
		{"libsql://example.turso.io", true},
		{"libsql://localhost:8080?tls=0", true},
		// Other URL schemes are NOT libsql; route them through sqlite (which
		// will then fail loudly). We do not silently route http/ws etc. to
		// the libsql driver — the libsql client handles transport via
		// libsql:// URL flags.
		{"https://example.turso.io", false},
		{"http://localhost:8080", false},
		{"wss://example.turso.io", false},
		{"ws://localhost:8080", false},
		{"/Users/me/.config/tq/tq.db", false},
		{"./relative/path.db", false},
		{":memory:", false},
		{"file:test.db", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			if got := db.IsLibsqlURL(tt.dsn); got != tt.want {
				t.Errorf("IsLibsqlURL(%q) = %v, want %v", tt.dsn, got, tt.want)
			}
		})
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

func TestMigrate_RenamesTmuxColumns(t *testing.T) {
	d := testutil.NewTestDB(t)

	if _, err := d.Exec(`
		DROP TABLE actions;
		CREATE TABLE actions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL DEFAULT '',
			task_id INTEGER NOT NULL REFERENCES tasks(id),
			metadata TEXT NOT NULL DEFAULT '{}',
			status TEXT DEFAULT 'pending',
			result TEXT,
			session_id TEXT,
			tmux_pane TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO projects(name, work_dir) VALUES('p', '/tmp/p');
		INSERT INTO tasks(project_id, title) VALUES(last_insert_rowid(), 't');
		INSERT INTO actions(task_id, session_id, tmux_pane) VALUES(last_insert_rowid(), 'main', 'tq-action-1');
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if err := d.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var tmuxSession, tmuxWindow string
	if err := d.QueryRow("SELECT tmux_session, tmux_window FROM actions WHERE id = 1").Scan(&tmuxSession, &tmuxWindow); err != nil {
		t.Fatalf("read renamed columns: %v", err)
	}
	if tmuxSession != "main" {
		t.Errorf("tmux_session = %q, want %q", tmuxSession, "main")
	}
	if tmuxWindow != "tq-action-1" {
		t.Errorf("tmux_window = %q, want %q", tmuxWindow, "tq-action-1")
	}

	rows, err := d.Query("PRAGMA table_info(actions)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "session_id" || name == "tmux_pane" {
			t.Errorf("legacy column %q still present after migrate", name)
		}
	}

	if err := d.Migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
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
