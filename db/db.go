package db

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	*sql.DB
}

func Open(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &DB{sqlDB}, nil
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
	if _, err := db.Exec(`UPDATE actions SET prompt_id = 'classify-gh-notification' WHERE prompt_id = 'classify'`); err != nil {
		return err
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
