package db

import (
	"fmt"
	"strings"
	"time"
)

const testTimeFormat = "2006-01-02 15:04:05"

type setClause struct {
	column string
	value  any
}

func (db *DB) execTestUpdate(table string, id int64, clauses []setClause, caller string) error {
	if len(clauses) == 0 {
		return fmt.Errorf("%s: no fields to set", caller)
	}
	sets := make([]string, len(clauses))
	args := make([]any, len(clauses)+1)
	for i, c := range clauses {
		sets[i] = c.column + " = ?"
		args[i] = c.value
	}
	args[len(clauses)] = id
	_, err := db.Exec("UPDATE "+table+" SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return fmt.Errorf("%s: %w", caller, err)
	}
	return nil
}

func (db *DB) SetActionSessionInfoForTest(id int64, sessionID, tmuxPane *string, startedAt *time.Time) error {
	var clauses []setClause
	if sessionID != nil {
		clauses = append(clauses, setClause{"session_id", *sessionID})
	}
	if tmuxPane != nil {
		clauses = append(clauses, setClause{"tmux_pane", *tmuxPane})
	}
	if startedAt != nil {
		clauses = append(clauses, setClause{"started_at", startedAt.UTC().Format(testTimeFormat)})
	}
	return db.execTestUpdate("actions", id, clauses, "SetActionSessionInfoForTest")
}

func (db *DB) SetScheduleTimestampsForTest(id int64, createdAt, lastRunAt *time.Time) error {
	var clauses []setClause
	if createdAt != nil {
		clauses = append(clauses, setClause{"created_at", createdAt.UTC().Format(testTimeFormat)})
	}
	if lastRunAt != nil {
		clauses = append(clauses, setClause{"last_run_at", lastRunAt.UTC().Format(testTimeFormat)})
	}
	return db.execTestUpdate("schedules", id, clauses, "SetScheduleTimestampsForTest")
}

func (db *DB) SetActionTimestampsForTest(id int64, createdAt, completedAt *time.Time) error {
	var clauses []setClause
	if createdAt != nil {
		clauses = append(clauses, setClause{"created_at", createdAt.UTC().Format(testTimeFormat)})
	}
	if completedAt != nil {
		clauses = append(clauses, setClause{"completed_at", completedAt.UTC().Format(testTimeFormat)})
	}
	return db.execTestUpdate("actions", id, clauses, "SetActionTimestampsForTest")
}

func (db *DB) SetTaskTimestampsForTest(id int64, createdAt, updatedAt *time.Time) error {
	var clauses []setClause
	if createdAt != nil {
		clauses = append(clauses, setClause{"created_at", createdAt.UTC().Format(testTimeFormat)})
	}
	if updatedAt != nil {
		clauses = append(clauses, setClause{"updated_at", updatedAt.UTC().Format(testTimeFormat)})
	}
	return db.execTestUpdate("tasks", id, clauses, "SetTaskTimestampsForTest")
}

func (db *DB) SetActionStatusForTest(id int64, status string) error {
	_, err := db.Exec("UPDATE actions SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("SetActionStatusForTest: %w", err)
	}
	return nil
}
