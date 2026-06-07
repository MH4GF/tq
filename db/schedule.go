package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Schedule struct {
	ID          int64
	TaskID      int64
	Instruction string
	Title       string
	CronExpr    string
	Metadata    string
	Enabled     bool
	LastRunAt   sql.NullString
	LastError   sql.NullString
	CreatedAt   string
}

const scheduleColumns = "id, task_id, instruction, title, cron_expr, metadata, enabled, last_run_at, last_error, created_at"

func (s *Schedule) scanFields() []any {
	return []any{&s.ID, &s.TaskID, &s.Instruction, &s.Title, &s.CronExpr, &s.Metadata, &s.Enabled, &s.LastRunAt, &s.LastError, &s.CreatedAt}
}

func (db *DB) InsertSchedule(taskID int64, instruction, title, cronExpr, metadata string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO schedules (task_id, instruction, title, cron_expr, metadata) VALUES (?, ?, ?, ?, ?)",
		taskID, instruction, title, cronExpr, metadata,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.emitEvent("schedule", id, "schedule.created", map[string]any{
		"task_id": taskID, "instruction": instruction, "cron_expr": cronExpr,
	})
	return id, nil
}

func (db *DB) ListSchedules(limit int) ([]Schedule, error) {
	query := "SELECT " + scheduleColumns + " FROM schedules"
	var args []any
	query, args = appendOrderLimit(query, args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var schedules []Schedule
	for rows.Next() {
		var s Schedule
		if err := rows.Scan(s.scanFields()...); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}

func (db *DB) GetSchedule(id int64) (*Schedule, error) {
	s := &Schedule{}
	err := db.QueryRow(
		"SELECT "+scheduleColumns+" FROM schedules WHERE id = ?", id,
	).Scan(s.scanFields()...)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (db *DB) UpdateScheduleEnabled(id int64, enabled bool) error {
	_, err := db.Exec("UPDATE schedules SET enabled = ? WHERE id = ?", enabled, id)
	return err
}

func (db *DB) UpdateSchedule(id int64, title, cronExpr, metadata, instruction *string, taskID *int64) error {
	var setClauses []string
	var args []any
	payload := map[string]any{}

	if title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *title)
		payload["title"] = *title
	}
	if cronExpr != nil {
		setClauses = append(setClauses, "cron_expr = ?")
		args = append(args, *cronExpr)
		payload["cron_expr"] = *cronExpr
	}
	if metadata != nil {
		setClauses = append(setClauses, "metadata = ?")
		args = append(args, *metadata)
		payload["metadata"] = *metadata
	}
	if instruction != nil {
		setClauses = append(setClauses, "instruction = ?")
		args = append(args, *instruction)
		payload["instruction"] = *instruction
	}
	if taskID != nil {
		setClauses = append(setClauses, "task_id = ?")
		args = append(args, *taskID)
		payload["task_id"] = *taskID
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	query := "UPDATE schedules SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)
	_, err := db.Exec(query, args...)
	if err == nil {
		db.emitEvent("schedule", id, "schedule.updated", payload)
	}
	return err
}

func (db *DB) EnabledScheduleIDs(taskID int64) ([]int64, error) {
	rows, err := db.Query("SELECT id FROM schedules WHERE task_id = ? AND enabled = 1", taskID)
	if err != nil {
		return nil, fmt.Errorf("query enabled schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan schedule id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ScheduleRunUpdate describes one tick result for BulkUpdateScheduleRuns.
// LastError == "" clears the column (success path); non-empty preserves the
// reason so /schedule list can surface it.
type ScheduleRunUpdate struct {
	ID        int64
	LastRunAt string
	LastError string
}

func (db *DB) BulkInsertScheduledActions(specs []ActionInsertSpec, runs []ScheduleRunUpdate) ([]int64, error) {
	if len(specs) != len(runs) {
		return nil, fmt.Errorf("bulk insert scheduled actions: specs (%d) and runs (%d) length mismatch", len(specs), len(runs))
	}
	if len(specs) == 0 {
		return nil, nil
	}
	for i, s := range specs {
		if !ValidActionStatuses[s.Status] {
			return nil, fmt.Errorf("specs[%d]: invalid action status %q", i, s.Status)
		}
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("bulk insert scheduled actions: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var sb strings.Builder
	sb.WriteString("INSERT INTO actions (title, task_id, metadata, status, dispatch_after, work_dir) VALUES ")
	args := make([]any, 0, len(specs)*6)
	for i, s := range specs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?, ?, ?, ?, ?, ?)")
		args = append(args, s.Title, s.TaskID, s.Metadata, s.Status, s.DispatchAfter, s.WorkDir)
	}
	sb.WriteString(" RETURNING id")

	rows, err := tx.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("bulk insert scheduled actions: insert: %w", err)
	}
	ids := make([]int64, 0, len(specs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("bulk insert scheduled actions: scan id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("bulk insert scheduled actions: insert rows: %w", err)
	}
	_ = rows.Close()
	if len(ids) != len(specs) {
		return nil, fmt.Errorf("bulk insert scheduled actions: expected %d ids, got %d", len(specs), len(ids))
	}

	for _, u := range runs {
		if _, err := tx.ExecContext(ctx,
			"UPDATE schedules SET last_run_at = ?, last_error = NULLIF(?, '') WHERE id = ?",
			u.LastRunAt, u.LastError, u.ID,
		); err != nil {
			return nil, fmt.Errorf("bulk insert scheduled actions: update schedule id=%d: %w", u.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("bulk insert scheduled actions: commit: %w", err)
	}

	for i, id := range ids {
		evt := map[string]any{
			"status":  specs[i].Status,
			"task_id": specs[i].TaskID,
			"title":   specs[i].Title,
		}
		if specs[i].DispatchAfter != nil {
			evt["dispatch_after"] = *specs[i].DispatchAfter
		}
		if specs[i].WorkDir != "" {
			evt["work_dir"] = specs[i].WorkDir
		}
		db.emitEvent("action", id, "action.created", evt)
	}
	for _, u := range runs {
		db.emitEvent("schedule", u.ID, "schedule.ran", map[string]any{
			"last_run_at": u.LastRunAt,
			"last_error":  u.LastError,
		})
	}
	return ids, nil
}

// BulkUpdateScheduleRuns records one tick result across many schedules in a
// single transaction. Tx-atomic: any UPDATE failure rolls back the entire
// batch.
func (db *DB) BulkUpdateScheduleRuns(updates []ScheduleRunUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("bulk update schedule runs: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, u := range updates {
		if _, err := tx.ExecContext(ctx,
			"UPDATE schedules SET last_run_at = ?, last_error = NULLIF(?, '') WHERE id = ?",
			u.LastRunAt, u.LastError, u.ID,
		); err != nil {
			return fmt.Errorf("bulk update schedule runs: update id=%d: %w", u.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("bulk update schedule runs: commit: %w", err)
	}

	for _, u := range updates {
		db.emitEvent("schedule", u.ID, "schedule.ran", map[string]any{
			"last_run_at": u.LastRunAt,
			"last_error":  u.LastError,
		})
	}
	return nil
}

// HasActiveActionsForSchedules returns, for each scheduleID in the input, true
// if at least one pending/running/dispatched action references it via metadata
// schedule_id. Missing IDs are absent from the map.
func (db *DB) HasActiveActionsForSchedules(scheduleIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(scheduleIDs))
	if len(scheduleIDs) == 0 {
		return result, nil
	}

	args := make([]any, 0, len(scheduleIDs)+3)
	args = append(args, ActionStatusPending, ActionStatusRunning, ActionStatusDispatched)
	placeholders := make([]string, len(scheduleIDs))
	for i, id := range scheduleIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	// schedule_id may be stored in metadata as a JSON string ("42") or a JSON
	// number (42) depending on the writer; CAST normalizes both sides to
	// INTEGER so the match does not depend on SQLite's implicit TEXT/INTEGER
	// coercion (which would break under strict typing).
	query := "SELECT DISTINCT CAST(json_extract(metadata, '$.schedule_id') AS INTEGER) AS sid " +
		"FROM actions WHERE status IN (?, ?, ?) " +
		"AND CAST(json_extract(metadata, '$.schedule_id') AS INTEGER) IN (" + strings.Join(placeholders, ", ") + ")"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("has active actions for schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var sid sql.NullInt64
		if err := rows.Scan(&sid); err != nil {
			return nil, fmt.Errorf("has active actions for schedules: scan: %w", err)
		}
		if !sid.Valid {
			continue
		}
		result[sid.Int64] = true
	}
	return result, rows.Err()
}

func (db *DB) DeleteSchedule(id int64) error {
	var taskID int64
	if err := db.QueryRow("SELECT task_id FROM schedules WHERE id = ?", id).Scan(&taskID); err != nil {
		return fmt.Errorf("get schedule task_id: %w", err)
	}

	_, err := db.Exec("DELETE FROM schedules WHERE id = ?", id)
	if err == nil {
		db.emitEvent("schedule", id, "schedule.deleted", map[string]any{
			"task_id": taskID,
		})
	}
	return err
}
