package db

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// IsLibsqlURL reports whether dsn is a libsql:// URL that should be
// opened through the libsql driver. We deliberately do NOT match
// `https://` / `ws://` etc. — keeping the trigger to a single explicit
// scheme avoids misrouting arbitrary URLs, and the libsql client itself
// maps `libsql://...?tls=0` (and similar transport flags) onto the
// underlying http/ws connection internally.
func IsLibsqlURL(dsn string) bool {
	return strings.HasPrefix(dsn, "libsql://")
}

type DB struct {
	*sql.DB
}

func Open(dsn string) (*DB, error) {
	driver := "sqlite"
	if IsLibsqlURL(dsn) {
		driver = "libsql"
	} else {
		dsn = sqliteDSN(dsn)
	}
	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" && isMemoryDSN(dsn) {
		if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
		if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}
	if driver == "libsql" {
		if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}
	return &DB{sqlDB}, nil
}

func sqliteDSN(dsn string) string {
	if isMemoryDSN(dsn) {
		return dsn
	}
	const pragmas = "_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)&_pragma=journal_mode(WAL)&_txlock=immediate"
	if strings.HasPrefix(dsn, "file:") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + pragmas
	}
	return "file:" + dsn + "?" + pragmas
}

func isMemoryDSN(dsn string) bool {
	return dsn == ":memory:" || strings.HasPrefix(dsn, "file::memory:")
}

func (db *DB) hasColumn(table, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (db *DB) Migrate() error {
	staleCountTrigger, err := db.dropStaleCountUpdateTrigger()
	if err != nil {
		return fmt.Errorf("migrate task_action_counts trigger: %w", err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}

	// Drop priority column from existing DBs (idempotent)
	if has, err := db.hasColumn("actions", "priority"); err != nil {
		return err
	} else if has {
		if _, err := db.Exec("DROP INDEX IF EXISTS idx_actions_dispatch"); err != nil {
			return err
		}
		if _, err := db.Exec("ALTER TABLE actions DROP COLUMN priority"); err != nil {
			return err
		}
		if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_actions_dispatch ON actions(status, id ASC)"); err != nil {
			return err
		}
	}

	// Rename template_id → prompt_id (idempotent)
	if has, err := db.hasColumn("actions", "template_id"); err != nil {
		return err
	} else if has {
		if _, err := db.Exec("ALTER TABLE actions RENAME COLUMN template_id TO prompt_id"); err != nil {
			return err
		}
	}

	// Rename classify → classify-gh-notification in prompt_id (idempotent)
	if has, err := db.hasColumn("actions", "prompt_id"); err != nil {
		return err
	} else if has {
		if _, err := db.Exec(`UPDATE actions SET prompt_id = 'classify-gh-notification' WHERE prompt_id = 'classify'`); err != nil {
			return err
		}
	}

	// Drop source column from existing DBs (idempotent)
	if has, err := db.hasColumn("actions", "source"); err != nil {
		return err
	} else if has {
		if _, err := db.Exec("ALTER TABLE actions DROP COLUMN source"); err != nil {
			return err
		}
	}

	// Add dispatch_enabled column to projects (idempotent)
	if has, err := db.hasColumn("projects", "dispatch_enabled"); err != nil {
		return err
	} else if !has {
		if _, err := db.Exec("ALTER TABLE projects ADD COLUMN dispatch_enabled INTEGER NOT NULL DEFAULT 1"); err != nil {
			return err
		}
	}

	// Add work_dir column to tasks (idempotent)
	if has, err := db.hasColumn("tasks", "work_dir"); err != nil {
		return err
	} else if !has {
		if _, err := db.Exec("ALTER TABLE tasks ADD COLUMN work_dir TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE tasks SET work_dir = (SELECT work_dir FROM projects WHERE projects.id = tasks.project_id) WHERE work_dir = ''"); err != nil {
			return err
		}
	}

	// Add title column to actions (idempotent)
	if has, err := db.hasColumn("actions", "title"); err != nil {
		return err
	} else if !has {
		if _, err := db.Exec("ALTER TABLE actions ADD COLUMN title TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE actions SET title = prompt_id WHERE title = ''"); err != nil {
			return err
		}
	}

	if _, err := db.Exec("DELETE FROM actions WHERE task_id IS NULL"); err != nil {
		return err
	}

	// Drop prompt_id column from actions (idempotent)
	if has, err := db.hasColumn("actions", "prompt_id"); err != nil {
		return err
	} else if has {
		if _, err := db.Exec("ALTER TABLE actions DROP COLUMN prompt_id"); err != nil {
			return err
		}
	}

	// Migrate schedules: prompt_id → instruction (idempotent)
	if has, err := db.hasColumn("schedules", "prompt_id"); err != nil {
		return err
	} else if has {
		if hasInstruction, err := db.hasColumn("schedules", "instruction"); err != nil {
			return err
		} else if !hasInstruction {
			if _, err := db.Exec("ALTER TABLE schedules ADD COLUMN instruction TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
		}
		if _, err := db.Exec("UPDATE schedules SET instruction = prompt_id WHERE instruction = ''"); err != nil {
			return err
		}
		if _, err := db.Exec("ALTER TABLE schedules DROP COLUMN prompt_id"); err != nil {
			return err
		}
	}

	if has, err := db.hasColumn("worker_heartbeats", "max_interactive"); err != nil {
		return fmt.Errorf("migrate max_interactive: check column: %w", err)
	} else if !has {
		if _, err := db.Exec("ALTER TABLE worker_heartbeats ADD COLUMN max_interactive INTEGER NOT NULL DEFAULT 3"); err != nil {
			return fmt.Errorf("migrate max_interactive: alter table: %w", err)
		}
	}

	// Migrate url column values into metadata JSON, then drop the column (idempotent)
	if has, err := db.hasColumn("tasks", "url"); err != nil {
		return fmt.Errorf("migrate url: check column: %w", err)
	} else if has {
		rows, err := db.Query("SELECT id, url, metadata FROM tasks WHERE url IS NOT NULL AND url != ''")
		if err != nil {
			return fmt.Errorf("migrate url: query tasks: %w", err)
		}
		defer func() { _ = rows.Close() }()
		type row struct {
			id       int64
			url      string
			metadata string
		}
		var toUpdate []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.url, &r.metadata); err != nil {
				return fmt.Errorf("migrate url: scan task: %w", err)
			}
			toUpdate = append(toUpdate, r)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("migrate url: iterate tasks: %w", err)
		}
		_ = rows.Close()

		for _, r := range toUpdate {
			m := make(map[string]any)
			if r.metadata != "" && r.metadata != "{}" {
				if err := json.Unmarshal([]byte(r.metadata), &m); err != nil {
					return fmt.Errorf("migrate url: parse metadata for task %d: %w", r.id, err)
				}
			}
			if _, exists := m["url"]; exists {
				continue
			}
			m["url"] = r.url
			newMeta, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("migrate url: marshal metadata for task %d: %w", r.id, err)
			}
			if _, err := db.Exec("UPDATE tasks SET metadata = ? WHERE id = ?", string(newMeta), r.id); err != nil {
				return fmt.Errorf("migrate url: update task %d: %w", r.id, err)
			}
		}

		if _, err := db.Exec("ALTER TABLE tasks DROP COLUMN url"); err != nil {
			return fmt.Errorf("migrate url: drop column: %w", err)
		}
	}

	// Add dispatch_after column to actions (idempotent)
	if has, err := db.hasColumn("actions", "dispatch_after"); err != nil {
		return fmt.Errorf("migrate dispatch_after: check column: %w", err)
	} else if !has {
		if _, err := db.Exec("ALTER TABLE actions ADD COLUMN dispatch_after TEXT"); err != nil {
			return fmt.Errorf("migrate dispatch_after: alter table: %w", err)
		}
	}

	// Add work_dir column to actions (idempotent)
	if has, err := db.hasColumn("actions", "work_dir"); err != nil {
		return fmt.Errorf("migrate actions work_dir: check column: %w", err)
	} else if !has {
		if _, err := db.Exec("ALTER TABLE actions ADD COLUMN work_dir TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("migrate actions work_dir: alter table: %w", err)
		}
	}

	if has, err := db.hasColumn("actions", "session_id"); err != nil {
		return fmt.Errorf("migrate tmux_session: check column: %w", err)
	} else if has {
		if _, err := db.Exec("ALTER TABLE actions RENAME COLUMN session_id TO tmux_session"); err != nil {
			return fmt.Errorf("migrate tmux_session: rename column: %w", err)
		}
	}

	if has, err := db.hasColumn("actions", "tmux_pane"); err != nil {
		return fmt.Errorf("migrate tmux_window: check column: %w", err)
	} else if has {
		if _, err := db.Exec("ALTER TABLE actions RENAME COLUMN tmux_pane TO tmux_window"); err != nil {
			return fmt.Errorf("migrate tmux_window: rename column: %w", err)
		}
	}

	if has, err := db.hasColumn("schedules", "last_error"); err != nil {
		return fmt.Errorf("migrate last_error: check column: %w", err)
	} else if !has {
		if _, err := db.Exec("ALTER TABLE schedules ADD COLUMN last_error TEXT"); err != nil {
			return fmt.Errorf("migrate last_error: alter table: %w", err)
		}
	}

	if err := db.migrateExperimentalBgToInteractive(); err != nil {
		return fmt.Errorf("migrate experimental_bg mode: %w", err)
	}

	if staleCountTrigger {
		if err := db.rebuildTaskActionCounts(); err != nil {
			return fmt.Errorf("rebuild task_action_counts: %w", err)
		}
	}

	if err := db.backfillTaskActionCounts(); err != nil {
		return fmt.Errorf("backfill task_action_counts: %w", err)
	}

	if err := db.backfillSearchFTS(); err != nil {
		return fmt.Errorf("backfill search FTS: %w", err)
	}

	return nil
}

func (db *DB) migrateExperimentalBgToInteractive() error {
	ctx := context.Background()
	return db.withTxRetry(ctx, "migrateExperimentalBgToInteractive", func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE actions
			 SET metadata = json_set(metadata, '$.mode', 'interactive')
			 WHERE metadata IS NOT NULL
			   AND json_valid(metadata)
			   AND json_extract(metadata, '$.mode') = 'experimental_bg'`,
		); err != nil {
			return fmt.Errorf("update actions: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE settings SET value = 'interactive'
			 WHERE key = ? AND value = 'experimental_bg'`,
			SettingDefaultMode,
		); err != nil {
			return fmt.Errorf("update settings: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE schedules
			 SET metadata = json_set(metadata, '$.mode', 'interactive')
			 WHERE metadata IS NOT NULL
			   AND json_valid(metadata)
			   AND json_extract(metadata, '$.mode') = 'experimental_bg'`,
		); err != nil {
			return fmt.Errorf("update schedules: %w", err)
		}
		return nil
	})
}

// backfillSearchFTS populates tasks_fts/actions_fts/events_fts from rows that
// predate the FTS schema. Idempotent and self-healing: only rows whose id is
// not already an FTS rowid are inserted, so repeated Migrate() calls and
// interrupted backfills converge without duplicates. New writes after this
// point are kept in sync by the schema triggers.
func (db *DB) backfillSearchFTS() error {
	stmts := []string{
		`INSERT INTO tasks_fts(rowid, title, metadata)
		 SELECT id, title, metadata FROM tasks
		 WHERE id NOT IN (SELECT rowid FROM tasks_fts)`,
		`INSERT INTO actions_fts(rowid, title, result, metadata)
		 SELECT id, title, COALESCE(result, ''), metadata FROM actions
		 WHERE id NOT IN (SELECT rowid FROM actions_fts)`,
		`INSERT INTO events_fts(rowid, reason)
		 SELECT id, json_extract(payload, '$.reason') FROM events
		 WHERE entity_type = 'task'
		   AND event_type = 'task.status_changed'
		   AND json_extract(payload, '$.reason') IS NOT NULL
		   AND json_extract(payload, '$.reason') != ''
		   AND id NOT IN (SELECT rowid FROM events_fts)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	return nil
}

func (db *DB) dropStaleCountUpdateTrigger() (bool, error) {
	var sqlText string
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = 'trg_actions_count_update'").Scan(&sqlText)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read trigger definition: %w", err)
	}
	if strings.Contains(sqlText, "OLD.task_id != NEW.task_id") {
		return false, nil
	}
	if _, err := db.Exec("DROP TRIGGER IF EXISTS trg_actions_count_update"); err != nil {
		return false, fmt.Errorf("drop stale trigger: %w", err)
	}
	return true, nil
}

const recountTaskActionCountsSQL = "INSERT INTO task_action_counts (task_id, status, count) SELECT task_id, status, COUNT(*) FROM actions GROUP BY task_id, status"

func (db *DB) rebuildTaskActionCounts() error {
	ctx := context.Background()
	return db.withTxRetry(ctx, "rebuildTaskActionCounts", func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM task_action_counts"); err != nil {
			return fmt.Errorf("clear: %w", err)
		}
		if _, err := tx.ExecContext(ctx, recountTaskActionCountsSQL); err != nil {
			return fmt.Errorf("recount: %w", err)
		}
		return nil
	})
}

// backfillTaskActionCounts populates task_action_counts from existing actions
// rows on the first migration. Idempotent: if the table already has rows,
// triggers have been keeping it in sync, so skip.
func (db *DB) backfillTaskActionCounts() error {
	var hasRows int
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM task_action_counts LIMIT 1)").Scan(&hasRows); err != nil {
		return fmt.Errorf("check rows: %w", err)
	}
	if hasRows != 0 {
		return nil
	}
	if _, err := db.Exec(recountTaskActionCountsSQL); err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	return nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

func appendOrderLimit(query string, args []any, limit int) (string, []any) {
	query += " ORDER BY id DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	return query, args
}
