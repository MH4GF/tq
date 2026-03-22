package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
)

const (
	ActionStatusPending    = "pending"
	ActionStatusRunning    = "running"
	ActionStatusDispatched = "dispatched"
	ActionStatusDone       = "done"
	ActionStatusFailed     = "failed"
	ActionStatusCancelled  = "cancelled"
)

var ValidActionStatuses = map[string]bool{
	ActionStatusPending:    true,
	ActionStatusRunning:    true,
	ActionStatusDispatched: true,
	ActionStatusDone:       true,
	ActionStatusFailed:     true,
	ActionStatusCancelled:  true,
}

type Action struct {
	ID          int64
	Title       string
	PromptID    string
	TaskID      int64
	Metadata    string
	Status      string
	Result      sql.NullString
	SessionID   sql.NullString
	TmuxPane    sql.NullString
	CreatedAt   string
	StartedAt   sql.NullString
	CompletedAt sql.NullString
}

const actionColumns = "id, title, prompt_id, task_id, metadata, status, result, session_id, tmux_pane, created_at, started_at, completed_at"

func (a *Action) scanFields() []any {
	return []any{&a.ID, &a.Title, &a.PromptID, &a.TaskID, &a.Metadata, &a.Status, &a.Result, &a.SessionID, &a.TmuxPane, &a.CreatedAt, &a.StartedAt, &a.CompletedAt}
}

func (a Action) MatchesDate(date string) bool {
	if matchesDateLocal(a.CreatedAt, date) {
		return true
	}
	if a.StartedAt.Valid && matchesDateLocal(a.StartedAt.String, date) {
		return true
	}
	if a.CompletedAt.Valid && matchesDateLocal(a.CompletedAt.String, date) {
		return true
	}
	return false
}

func FilterForOpenTask(actions []Action, date string) []Action {
	if date == "" {
		return actions
	}
	var filtered []Action
	for _, a := range actions {
		if a.Status == ActionStatusPending || a.Status == ActionStatusRunning || a.Status == ActionStatusDispatched {
			filtered = append(filtered, a)
		} else if a.MatchesDate(date) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func FilterByDate(actions []Action, date string) []Action {
	if date == "" {
		return actions
	}
	var filtered []Action
	for _, a := range actions {
		if a.MatchesDate(date) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func (db *DB) InsertAction(title, promptID string, taskID int64, metadata, status string) (int64, error) {
	if !ValidActionStatuses[status] {
		return 0, fmt.Errorf("invalid action status %q: must be one of pending, running, dispatched, done, failed, cancelled", status)
	}
	res, err := db.Exec(
		"INSERT INTO actions (title, prompt_id, task_id, metadata, status) VALUES (?, ?, ?, ?, ?)",
		title, promptID, taskID, metadata, status,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.emitEvent("action", id, "action.created", map[string]any{
		"status": status, "prompt_id": promptID, "task_id": taskID, "title": title,
	})
	return id, nil
}

func (db *DB) HasActiveAction(taskID int64, promptID string) (bool, error) {
	if promptID == "" {
		return false, nil
	}
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE task_id = ? AND prompt_id = ? AND status IN (?, ?, ?)",
		taskID, promptID, ActionStatusPending, ActionStatusRunning, ActionStatusDispatched,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *DB) GetActiveAction(taskID int64, promptID string) (*Action, error) {
	a := &Action{}
	err := db.QueryRow(
		"SELECT "+actionColumns+" FROM actions WHERE task_id = ? AND prompt_id = ? AND status IN (?, ?, ?) ORDER BY id DESC LIMIT 1",
		taskID, promptID, ActionStatusPending, ActionStatusRunning, ActionStatusDispatched,
	).Scan(a.scanFields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (db *DB) HasActiveActionWithMeta(taskID int64, promptID, metaKey, metaValue string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE task_id = ? AND prompt_id = ? AND status IN (?, ?, ?) AND json_extract(metadata, '$.' || ?) = ?",
		taskID, promptID, ActionStatusPending, ActionStatusRunning, ActionStatusDispatched, metaKey, metaValue,
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
	defer func() { _ = tx.Rollback() }()

	a := &Action{}
	err = tx.QueryRowContext(ctx,
		`SELECT a.id, a.title, a.prompt_id, a.task_id, a.metadata, a.status, a.result,
		        a.session_id, a.tmux_pane, a.created_at, a.started_at, a.completed_at
		 FROM actions a
		 INNER JOIN tasks t ON a.task_id = t.id
		 INNER JOIN projects p ON t.project_id = p.id
		 WHERE a.status = ?
		   AND p.dispatch_enabled = 1
		 ORDER BY a.id ASC LIMIT 1`,
		ActionStatusPending,
	).Scan(a.scanFields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, "UPDATE actions SET status = ?, started_at = datetime('now') WHERE id = ?", ActionStatusRunning, a.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	db.emitEvent("action", a.ID, "action.claimed", map[string]any{
		"from": ActionStatusPending, "to": ActionStatusRunning,
	})
	a.Status = ActionStatusRunning
	return a, nil
}

func (db *DB) ClaimPending(ctx context.Context, id int64) (*Action, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	a := &Action{}
	err = tx.QueryRowContext(ctx,
		"SELECT "+actionColumns+" FROM actions WHERE id = ?",
		id,
	).Scan(a.scanFields()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("action #%d not found", id)
	}
	if err != nil {
		return nil, err
	}

	if a.Status != ActionStatusPending {
		return nil, fmt.Errorf("action #%d is not pending (current: %s)", id, a.Status)
	}

	_, err = tx.ExecContext(ctx, "UPDATE actions SET status = ?, started_at = datetime('now') WHERE id = ?", ActionStatusRunning, id)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	db.emitEvent("action", id, "action.claimed", map[string]any{
		"from": ActionStatusPending, "to": ActionStatusRunning,
	})
	a.Status = ActionStatusRunning
	return a, nil
}

func (db *DB) MarkDone(id int64, result string) error {
	return db.markTerminal(id, ActionStatusDone, result)
}

func (db *DB) MarkFailed(id int64, result string) error {
	return db.markTerminal(id, ActionStatusFailed, result)
}

func (db *DB) MarkCancelled(id int64, result string) error {
	return db.markTerminal(id, ActionStatusCancelled, result)
}

func (db *DB) markTerminal(id int64, status, result string) error {
	var from string
	if err := db.QueryRow("SELECT status FROM actions WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current status: %w", err)
	}

	_, err := db.Exec(
		"UPDATE actions SET status = ?, result = ?, completed_at = datetime('now') WHERE id = ?",
		status, result, id,
	)
	if err == nil {
		db.emitEvent("action", id, "action.status_changed", map[string]any{
			"from": from, "to": status, "result": result,
		})
	}
	return err
}

func (db *DB) MarkDispatched(id int64) error {
	res, err := db.Exec(
		"UPDATE actions SET status = ? WHERE id = ? AND status = ?",
		ActionStatusDispatched, id, ActionStatusRunning,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("action #%d is not running, cannot mark as dispatched", id)
	}
	db.emitEvent("action", id, "action.status_changed", map[string]any{
		"from": ActionStatusRunning, "to": ActionStatusDispatched,
	})
	return nil
}

func (db *DB) ListActions(status string, taskID *int64, limit int) ([]Action, error) {
	query := "SELECT " + actionColumns + " FROM actions WHERE 1=1"
	var args []any

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if taskID != nil {
		query += " AND task_id = ?"
		args = append(args, *taskID)
	}
	query, args = appendOrderLimit(query, args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var actions []Action
	for rows.Next() {
		var a Action
		if err := rows.Scan(a.scanFields()...); err != nil {
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
	defer func() { _ = rows.Close() }()

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

func (db *DB) ListRunningInteractive() ([]Action, error) {
	rows, err := db.Query(
		"SELECT "+actionColumns+" FROM actions WHERE status = ? AND session_id IS NOT NULL ORDER BY id",
		ActionStatusRunning,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var actions []Action
	for rows.Next() {
		var a Action
		if err := rows.Scan(a.scanFields()...); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

func (db *DB) CountRunningInteractive() (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE status = ? AND session_id IS NOT NULL",
		ActionStatusRunning,
	).Scan(&count)
	return count, err
}

func (db *DB) ResetToPending(id int64) error {
	var from string
	if err := db.QueryRow("SELECT status FROM actions WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current status: %w", err)
	}

	_, err := db.Exec(
		"UPDATE actions SET status = ?, started_at = NULL, session_id = NULL, tmux_pane = NULL WHERE id = ?",
		ActionStatusPending, id,
	)
	if err == nil {
		db.emitEvent("action", id, "action.status_changed", map[string]any{
			"from": from, "to": ActionStatusPending,
		})
	}
	return err
}

func (db *DB) SetSessionInfo(id int64, sessionID, tmuxPane string) error {
	_, err := db.Exec(
		"UPDATE actions SET session_id = ?, tmux_pane = ? WHERE id = ?",
		sessionID, tmuxPane, id,
	)
	if err == nil {
		db.emitEvent("action", id, "action.session_set", map[string]any{
			"session_id": sessionID, "tmux_pane": tmuxPane,
		})
	}
	return err
}

func (db *DB) MergeActionMetadata(id int64, updates map[string]any) error {
	var existing string
	err := db.QueryRow("SELECT metadata FROM actions WHERE id = ?", id).Scan(&existing)
	if err != nil {
		return err
	}

	merged := make(map[string]any)
	if existing != "" && existing != "{}" {
		if err := json.Unmarshal([]byte(existing), &merged); err != nil {
			return fmt.Errorf("parse existing metadata: %w", err)
		}
	}
	maps.Copy(merged, updates)

	data, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = db.Exec("UPDATE actions SET metadata = ? WHERE id = ?", string(data), id)
	if err == nil {
		keys := make([]string, 0, len(updates))
		for k := range updates {
			keys = append(keys, k)
		}
		db.emitEvent("action", id, "action.metadata_merged", map[string]any{
			"keys_updated": keys,
		})
	}
	return err
}

func (db *DB) ListActionsByTaskIDs(taskIDs []int64) (map[int64][]Action, error) {
	result := make(map[int64][]Action)
	if len(taskIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(taskIDs))
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT "+actionColumns+" FROM actions WHERE task_id IN (%s) ORDER BY id",
		strings.Join(placeholders, ", "),
	)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var a Action
		if err := rows.Scan(a.scanFields()...); err != nil {
			return nil, err
		}
		result[a.TaskID] = append(result[a.TaskID], a)
	}
	return result, rows.Err()
}

func (db *DB) GetAction(id int64) (*Action, error) {
	a := &Action{}
	err := db.QueryRow(
		"SELECT "+actionColumns+" FROM actions WHERE id = ?",
		id,
	).Scan(a.scanFields()...)
	if err != nil {
		return nil, err
	}
	return a, nil
}
