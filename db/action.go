package db

import (
	"context"
	"database/sql"
)

type Action struct {
	ID          int64
	TemplateID  string
	TaskID      sql.NullInt64
	Metadata    string
	Status      string
	Priority    int
	Result      sql.NullString
	SessionID   sql.NullString
	TmuxPane    sql.NullString
	Source      string
	CreatedAt   string
	StartedAt   sql.NullString
	CompletedAt sql.NullString
}

func (db *DB) InsertAction(templateID string, taskID *int64, metadata string, status string, priority int, source string) (int64, error) {
	var tid sql.NullInt64
	if taskID != nil {
		tid = sql.NullInt64{Int64: *taskID, Valid: true}
	}
	res, err := db.Exec(
		"INSERT INTO actions (template_id, task_id, metadata, status, priority, source) VALUES (?, ?, ?, ?, ?, ?)",
		templateID, tid, metadata, status, priority, source,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) HasPendingOrRunning(taskID int64, templateID string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE task_id = ? AND template_id = ? AND status IN ('pending', 'running')",
		taskID, templateID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *DB) NextPending(ctx context.Context) (*Action, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	a := &Action{}
	err = tx.QueryRowContext(ctx,
		"SELECT id, template_id, task_id, metadata, status, priority, result, session_id, tmux_pane, source, created_at, started_at, completed_at FROM actions WHERE status = 'pending' ORDER BY priority DESC, id ASC LIMIT 1",
	).Scan(&a.ID, &a.TemplateID, &a.TaskID, &a.Metadata, &a.Status, &a.Priority, &a.Result, &a.SessionID, &a.TmuxPane, &a.Source, &a.CreatedAt, &a.StartedAt, &a.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, "UPDATE actions SET status = 'running', started_at = datetime('now') WHERE id = ?", a.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	a.Status = "running"
	return a, nil
}

func (db *DB) MarkDone(id int64, result string) error {
	_, err := db.Exec(
		"UPDATE actions SET status = 'done', result = ?, completed_at = datetime('now') WHERE id = ?",
		result, id,
	)
	return err
}

func (db *DB) MarkFailed(id int64, result string) error {
	_, err := db.Exec(
		"UPDATE actions SET status = 'failed', result = ?, completed_at = datetime('now') WHERE id = ?",
		result, id,
	)
	return err
}

func (db *DB) MarkWaitingHuman(id int64, result string) error {
	_, err := db.Exec(
		"UPDATE actions SET status = 'waiting_human', result = ? WHERE id = ?",
		result, id,
	)
	return err
}

func (db *DB) ListActions(status string, taskID *int64) ([]Action, error) {
	query := "SELECT id, template_id, task_id, metadata, status, priority, result, session_id, tmux_pane, source, created_at, started_at, completed_at FROM actions WHERE 1=1"
	var args []any

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if taskID != nil {
		query += " AND task_id = ?"
		args = append(args, *taskID)
	}
	query += " ORDER BY id"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []Action
	for rows.Next() {
		var a Action
		if err := rows.Scan(&a.ID, &a.TemplateID, &a.TaskID, &a.Metadata, &a.Status, &a.Priority, &a.Result, &a.SessionID, &a.TmuxPane, &a.Source, &a.CreatedAt, &a.StartedAt, &a.CompletedAt); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

func (db *DB) CountByStatus() (map[string]int, error) {
	rows, err := db.Query("SELECT status, COUNT(*) FROM actions GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func (db *DB) CountRunningInteractive() (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE status = 'running' AND session_id IS NOT NULL",
	).Scan(&count)
	return count, err
}

func (db *DB) ResetToPending(id int64) error {
	_, err := db.Exec(
		"UPDATE actions SET status = 'pending', started_at = NULL WHERE id = ?",
		id,
	)
	return err
}

func (db *DB) GetAction(id int64) (*Action, error) {
	a := &Action{}
	err := db.QueryRow(
		"SELECT id, template_id, task_id, metadata, status, priority, result, session_id, tmux_pane, source, created_at, started_at, completed_at FROM actions WHERE id = ?",
		id,
	).Scan(&a.ID, &a.TemplateID, &a.TaskID, &a.Metadata, &a.Status, &a.Priority, &a.Result, &a.SessionID, &a.TmuxPane, &a.Source, &a.CreatedAt, &a.StartedAt, &a.CompletedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}
