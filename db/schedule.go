package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type Schedule struct {
	ID        int64
	TaskID    int64
	PromptID  string
	Title     string
	CronExpr  string
	Metadata  string
	Enabled   bool
	LastRunAt sql.NullString
	CreatedAt string
}

const scheduleColumns = "id, task_id, prompt_id, title, cron_expr, metadata, enabled, last_run_at, created_at"

func (s *Schedule) scanFields() []any {
	return []any{&s.ID, &s.TaskID, &s.PromptID, &s.Title, &s.CronExpr, &s.Metadata, &s.Enabled, &s.LastRunAt, &s.CreatedAt}
}

func (db *DB) InsertSchedule(taskID int64, promptID, title, cronExpr, metadata string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO schedules (task_id, prompt_id, title, cron_expr, metadata) VALUES (?, ?, ?, ?, ?)",
		taskID, promptID, title, cronExpr, metadata,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.emitEvent("schedule", id, "schedule.created", map[string]any{
		"task_id": taskID, "prompt_id": promptID, "cron_expr": cronExpr,
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

func (db *DB) UpdateScheduleLastRunAt(id int64, t string) error {
	_, err := db.Exec("UPDATE schedules SET last_run_at = ? WHERE id = ?", t, id)
	if err == nil {
		db.emitEvent("schedule", id, "schedule.ran", map[string]any{
			"last_run_at": t,
		})
	}
	return err
}

func (db *DB) UpdateSchedule(id int64, title, cronExpr, metadata, promptID *string, taskID *int64) error {
	var setClauses []string
	var args []any

	if title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *title)
	}
	if cronExpr != nil {
		setClauses = append(setClauses, "cron_expr = ?")
		args = append(args, *cronExpr)
	}
	if metadata != nil {
		setClauses = append(setClauses, "metadata = ?")
		args = append(args, *metadata)
	}
	if promptID != nil {
		setClauses = append(setClauses, "prompt_id = ?")
		args = append(args, *promptID)
	}
	if taskID != nil {
		setClauses = append(setClauses, "task_id = ?")
		args = append(args, *taskID)
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	query := "UPDATE schedules SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)
	_, err := db.Exec(query, args...)
	return err
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
