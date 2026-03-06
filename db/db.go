package db

import (
	"database/sql"
	_ "embed"

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
		sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &DB{sqlDB}, nil
}

func (db *DB) Migrate() error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}

	// Drop priority column from existing DBs (idempotent)
	rows, err := db.Query("PRAGMA table_info(actions)")
	if err != nil {
		return err
	}
	defer rows.Close()

	hasPriority := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "priority" {
			hasPriority = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if hasPriority {
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
	rows2, err := db.Query("PRAGMA table_info(actions)")
	if err != nil {
		return err
	}
	defer rows2.Close()

	hasTemplateID := false
	for rows2.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dfltValue *string
		var pk int
		if err := rows2.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "template_id" {
			hasTemplateID = true
		}
	}
	if err := rows2.Err(); err != nil {
		return err
	}

	if hasTemplateID {
		if _, err := db.Exec("ALTER TABLE actions RENAME COLUMN template_id TO prompt_id"); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}
