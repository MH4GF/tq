package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"
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

func IsTerminalActionStatus(status string) bool {
	return status == ActionStatusDone || status == ActionStatusFailed || status == ActionStatusCancelled
}

// isResultAmendable reports whether an action's result may be edited after the
// fact. Running/dispatched actions are in-flight, so their result is set by
// done/fail instead.
func isResultAmendable(status string) bool {
	return status == ActionStatusPending || status == ActionStatusFailed ||
		status == ActionStatusDone || status == ActionStatusCancelled
}

type Action struct {
	ID            int64
	Title         string
	TaskID        int64
	Metadata      string
	Status        string
	Result        sql.NullString
	TmuxSession   sql.NullString
	TmuxWindow    sql.NullString
	DispatchAfter sql.NullString
	WorkDir       string
	CreatedAt     string
	StartedAt     sql.NullString
	CompletedAt   sql.NullString
}

const actionColumns = "id, title, task_id, metadata, status, result, tmux_session, tmux_window, dispatch_after, work_dir, created_at, started_at, completed_at"

// actionColumnsA is actionColumns with each column prefixed by "a." for JOIN
// queries that need to disambiguate (e.g. NextPending joins tasks/projects).
var actionColumnsA = "a." + strings.ReplaceAll(actionColumns, ", ", ", a.")

func (a *Action) scanFields() []any {
	return []any{&a.ID, &a.Title, &a.TaskID, &a.Metadata, &a.Status, &a.Result, &a.TmuxSession, &a.TmuxWindow, &a.DispatchAfter, &a.WorkDir, &a.CreatedAt, &a.StartedAt, &a.CompletedAt}
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

// Instruction returns the "instruction" metadata field, or "" if Metadata is
// empty, malformed, or lacks the field. Parse errors are intentionally
// swallowed: callers treat absent instruction the same as a parse failure.
func (a Action) Instruction() string {
	if a.Metadata == "" {
		return ""
	}
	var m struct {
		Instruction string `json:"instruction"`
	}
	if err := json.Unmarshal([]byte(a.Metadata), &m); err != nil {
		return ""
	}
	return m.Instruction
}

func (db *DB) InsertAction(title string, taskID int64, metadata, status string, dispatchAfter *string, workDir string) (int64, error) {
	if !ValidActionStatuses[status] {
		return 0, fmt.Errorf("invalid action status %q: must be one of pending, running, dispatched, done, failed, cancelled", status)
	}
	res, err := db.Exec(
		"INSERT INTO actions (title, task_id, metadata, status, dispatch_after, work_dir) VALUES (?, ?, ?, ?, ?, ?)",
		title, taskID, metadata, status, dispatchAfter, workDir,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	evt := map[string]any{
		"status": status, "task_id": taskID, "title": title,
	}
	if dispatchAfter != nil {
		evt["dispatch_after"] = *dispatchAfter
	}
	if workDir != "" {
		evt["work_dir"] = workDir
	}
	db.emitEvent("action", id, "action.created", evt)
	return id, nil
}

func (db *DB) HasActiveActionWithMeta(taskID int64, metaKey, metaValue string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE task_id = ? AND status IN (?, ?, ?) AND json_extract(metadata, '$.' || ?) = ?",
		taskID, ActionStatusPending, ActionStatusRunning, ActionStatusDispatched, metaKey, metaValue,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetTaskActionCount reads from the trigger-maintained task_action_counts
// table — an O(len(statuses)) PK lookup, not a scan of actions.
func (db *DB) GetTaskActionCount(taskID int64, statuses []string) (int64, error) {
	if len(statuses) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(statuses))
	args := make([]any, 0, len(statuses)+1)
	args = append(args, taskID)
	for i, s := range statuses {
		placeholders[i] = "?"
		args = append(args, s)
	}
	query := "SELECT COALESCE(SUM(count), 0) FROM task_action_counts WHERE task_id = ? AND status IN (" + strings.Join(placeholders, ", ") + ")"
	var count int64
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func (db *DB) NextPending(ctx context.Context) (*Action, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	a := &Action{}
	err = tx.QueryRowContext(ctx,
		`SELECT `+actionColumnsA+`
		 FROM actions a
		 INNER JOIN tasks t ON a.task_id = t.id
		 INNER JOIN projects p ON t.project_id = p.id
		 WHERE a.status = ?
		   AND p.dispatch_enabled = 1
		   AND (a.dispatch_after IS NULL OR a.dispatch_after <= datetime('now'))
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
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var from string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM actions WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current status: %w", err)
	}

	if from == ActionStatusDone || from == ActionStatusFailed || from == ActionStatusCancelled {
		return nil
	}

	res, err := tx.ExecContext(ctx,
		"UPDATE actions SET status = ?, result = ?, completed_at = datetime('now') WHERE id = ? AND status = ?",
		status, result, id, from,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if n == 0 {
		return nil
	}
	db.emitEvent("action", id, "action.status_changed", map[string]any{
		"from": from, "to": status, "result": result,
	})
	return nil
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

type PendingCounts struct {
	Dispatchable int
	Total        int
}

func (pc PendingCounts) Unfocused() int {
	return pc.Total - pc.Dispatchable
}

func (pc PendingCounts) Label() string {
	if u := pc.Unfocused(); u > 0 {
		return fmt.Sprintf("%d pending (%d unfocused)", pc.Dispatchable, u)
	}
	return fmt.Sprintf("%d pending", pc.Dispatchable)
}

func (db *DB) CountPendingByDispatch() (PendingCounts, error) {
	var pc PendingCounts
	err := db.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN p.dispatch_enabled = 1 THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM actions a
		INNER JOIN tasks t ON a.task_id = t.id
		INNER JOIN projects p ON t.project_id = p.id
		WHERE a.status = ?
		  AND (a.dispatch_after IS NULL OR a.dispatch_after <= datetime('now'))`,
		ActionStatusPending,
	).Scan(&pc.Dispatchable, &pc.Total)
	return pc, err
}

func (db *DB) IsActionDispatchEnabled(actionID int64) (bool, error) {
	var enabled bool
	err := db.QueryRow(`
		SELECT p.dispatch_enabled
		FROM actions a
		INNER JOIN tasks t ON a.task_id = t.id
		INNER JOIN projects p ON t.project_id = p.id
		WHERE a.id = ?`, actionID).Scan(&enabled)
	if err != nil {
		return false, fmt.Errorf("check dispatch enabled for action %d: %w", actionID, err)
	}
	return enabled, nil
}

// interactiveModePredicate matches actions whose mode is interactive (the default
// when unset). Keep ListRunningInteractive and CountRunningInteractive in sync.
const interactiveModePredicate = "(json_extract(metadata, '$.mode') IS NULL OR json_extract(metadata, '$.mode') = 'interactive')"

// noninteractiveModePredicate matches actions whose mode is explicitly noninteractive.
// Keep ListRunningNonInteractive and CountRunningNonInteractive in sync.
const noninteractiveModePredicate = "json_extract(metadata, '$.mode') = 'noninteractive'"

// bgModePredicate matches the experimental_bg dispatch mode. The literal string
// MUST stay in sync with dispatch.ModeBg — the db package cannot import dispatch
// (layering constraint), so the value is duplicated intentionally.
const bgModePredicate = "json_extract(metadata, '$.mode') = 'experimental_bg'"

// interactiveOrBgModePredicate matches actions that compete for the same
// MaxInteractive slot pool: classic interactive sessions and bg sessions
// (which the user interacts with via `claude agents`).
const interactiveOrBgModePredicate = "(" + interactiveModePredicate + " OR " + bgModePredicate + ")"

func (db *DB) ListRunningInteractive() ([]Action, error) {
	rows, err := db.Query(
		"SELECT "+actionColumns+" FROM actions WHERE status = ? AND "+interactiveModePredicate+" ORDER BY id",
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

func (db *DB) ListRunningNonInteractive() ([]Action, error) {
	rows, err := db.Query(
		"SELECT "+actionColumns+" FROM actions WHERE status = ? AND "+noninteractiveModePredicate+" ORDER BY id",
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

func (db *DB) ListRunningBg() ([]Action, error) {
	rows, err := db.Query(
		"SELECT "+actionColumns+" FROM actions WHERE status = ? AND "+bgModePredicate+" ORDER BY id",
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
		"SELECT COUNT(*) FROM actions WHERE status = ? AND "+interactiveModePredicate,
		ActionStatusRunning,
	).Scan(&count)
	return count, err
}

func (db *DB) CountRunningNonInteractive() (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE status = ? AND "+noninteractiveModePredicate,
		ActionStatusRunning,
	).Scan(&count)
	return count, err
}

// CountRunningInteractiveOrBg returns the total number of running actions that
// compete for the MaxInteractive concurrency slot pool: classic interactive
// sessions and experimental_bg sessions combined.
func (db *DB) CountRunningInteractiveOrBg() (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE status = ? AND "+interactiveOrBgModePredicate,
		ActionStatusRunning,
	).Scan(&count)
	return count, err
}

func (db *DB) ResetToPending(id int64) error {
	return db.movePending(id, false, "", "")
}

// DeferToPending rolls a running action back to pending with a future
// dispatch_after timestamp so NextPending skips it for retryAfter, preventing
// head-of-line blocking when a slot pool is full. Errors if the action is not
// currently running — the defer path always claims first via NextPending, so
// any other source status indicates a code-level misuse.
func (db *DB) DeferToPending(id int64, retryAfter time.Duration) error {
	// SQLite stores datetime to second precision; round up so a sub-second
	// retryAfter still produces a non-zero window (otherwise NextPending could
	// re-qualify the action on the very next poll inside the same second).
	dispatchAfter := time.Now().UTC().Add(retryAfter).Truncate(time.Second)
	if retryAfter > 0 && retryAfter%time.Second != 0 {
		dispatchAfter = dispatchAfter.Add(time.Second)
	}
	formatted := dispatchAfter.Format(TimeLayout)
	return db.movePending(id, true, formatted, ActionStatusRunning)
}

// movePending unifies the two pending-rollback paths. If requireFromStatus is
// non-empty, the current status must match (DeferToPending uses this to catch
// code-level misuse). When dispatchAfter is empty, the column is cleared.
func (db *DB) movePending(id int64, dispatchAfterValid bool, dispatchAfter, requireFromStatus string) error {
	var from string
	if err := db.QueryRow("SELECT status FROM actions WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current status: %w", err)
	}
	if requireFromStatus != "" && from != requireFromStatus {
		return fmt.Errorf("action #%d is not %s (current: %s), cannot transition", id, requireFromStatus, from)
	}

	var dispatchArg any
	if dispatchAfterValid {
		dispatchArg = dispatchAfter
	}
	res, err := db.Exec(
		"UPDATE actions SET status = ?, started_at = NULL, tmux_session = NULL, tmux_window = NULL, dispatch_after = ? WHERE id = ? AND status = ?",
		ActionStatusPending, dispatchArg, id, from,
	)
	if err != nil {
		return fmt.Errorf("move action #%d to pending: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("move action #%d to pending: rows affected: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("move action #%d to pending: status changed concurrently (expected from=%s)", id, from)
	}

	evt := map[string]any{"from": from, "to": ActionStatusPending}
	if dispatchAfterValid {
		evt["dispatch_after"] = dispatchAfter
	}
	db.emitEvent("action", id, "action.status_changed", evt)
	return nil
}

func (db *DB) SetTmuxInfo(id int64, tmuxSession, tmuxWindow string) error {
	_, err := db.Exec(
		"UPDATE actions SET tmux_session = ?, tmux_window = ? WHERE id = ?",
		tmuxSession, tmuxWindow, id,
	)
	if err == nil {
		db.emitEvent("action", id, "action.tmux_info_set", map[string]any{
			"tmux_session": tmuxSession, "tmux_window": tmuxWindow,
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

func (db *DB) UpdateAction(id int64, title *string, taskID *int64, metadata, workDir, result *string) error {
	var current Action
	err := db.QueryRow("SELECT "+actionColumns+" FROM actions WHERE id = ?", id).Scan(current.scanFields()...)
	if err != nil {
		return fmt.Errorf("action #%d not found: %w", id, err)
	}

	structuralUpdate := title != nil || taskID != nil || metadata != nil || workDir != nil
	if structuralUpdate && current.Status != ActionStatusPending && current.Status != ActionStatusFailed {
		return fmt.Errorf("action #%d has status %q: only pending or failed actions can be updated", id, current.Status)
	}
	if result != nil && !isResultAmendable(current.Status) {
		return fmt.Errorf("action #%d has status %q: result can only be amended on pending, failed, done, or cancelled actions", id, current.Status)
	}

	var setClauses []string
	var args []any

	if title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *title)
	}
	if taskID != nil {
		setClauses = append(setClauses, "task_id = ?")
		args = append(args, *taskID)
	}
	if metadata != nil {
		existing := make(map[string]any)
		if current.Metadata != "" && current.Metadata != "{}" {
			if err := json.Unmarshal([]byte(current.Metadata), &existing); err != nil {
				return fmt.Errorf("parse existing metadata: %w", err)
			}
		}
		updates := make(map[string]any)
		if err := json.Unmarshal([]byte(*metadata), &updates); err != nil {
			return fmt.Errorf("parse new metadata: %w", err)
		}
		maps.Copy(existing, updates)
		merged, err := json.Marshal(existing)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		setClauses = append(setClauses, "metadata = ?")
		args = append(args, string(merged))
	}
	if workDir != nil {
		setClauses = append(setClauses, "work_dir = ?")
		args = append(args, *workDir)
	}
	if result != nil {
		setClauses = append(setClauses, "result = ?")
		args = append(args, *result)
	}

	if len(setClauses) == 0 {
		return fmt.Errorf("no fields to update")
	}

	query := "UPDATE actions SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	args = append(args, id)
	_, err = db.Exec(query, args...)
	if err == nil {
		db.emitEvent("action", id, "action.updated", map[string]any{
			"fields_count": len(setClauses),
		})
	}
	return err
}

func (db *DB) ListActionsByTaskIDsForView(taskIDs []int64, dateFilter string) (map[int64][]Action, error) {
	result := make(map[int64][]Action)
	if len(taskIDs) == 0 {
		return result, nil
	}

	idPlaceholders := make([]string, len(taskIDs))
	args := make([]any, 0, len(taskIDs)+9)
	for i, id := range taskIDs {
		idPlaceholders[i] = "?"
		args = append(args, id)
	}

	// ORDER BY task_id, id DESC matches idx_actions_task partitions and
	// returns each task's actions newest-first, so callers can skip a
	// re-sort.
	var query string
	if dateFilter != "" {
		query = `SELECT ` + actionColumns + `
			FROM actions
			WHERE task_id IN (` + strings.Join(idPlaceholders, ", ") + `)
			  AND (
			    status IN (?, ?, ?)
			    OR (
			      status IN (?, ?, ?)
			      AND (
			           DATE(created_at,   'localtime') = ?
			        OR DATE(started_at,   'localtime') = ?
			        OR DATE(completed_at, 'localtime') = ?
			      )
			    )
			  )
			ORDER BY task_id, id DESC`
		args = append(args,
			ActionStatusPending, ActionStatusRunning, ActionStatusDispatched,
			ActionStatusDone, ActionStatusFailed, ActionStatusCancelled,
			dateFilter, dateFilter, dateFilter,
		)
	} else {
		query = `SELECT ` + actionColumns + `
			FROM actions
			WHERE task_id IN (` + strings.Join(idPlaceholders, ", ") + `)
			ORDER BY task_id, id DESC`
	}

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

// ActionInsertSpec describes one row to be inserted by BulkInsertActions.
type ActionInsertSpec struct {
	Title         string
	TaskID        int64
	Metadata      string
	Status        string
	DispatchAfter *string
	WorkDir       string
}

// ActionFailureUpdate describes one stale-action transition for BulkMarkFailed.
type ActionFailureUpdate struct {
	ID     int64
	Reason string
}

// ActionDoneUpdate describes one action-done transition for BulkMarkDone.
type ActionDoneUpdate struct {
	ID     int64
	Result string
}

// BulkInsertActions inserts all specs in a single multi-row INSERT and returns
// the assigned IDs in input order. Tx-atomic: any constraint violation rolls
// back the entire batch.
func (db *DB) BulkInsertActions(specs []ActionInsertSpec) ([]int64, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	for i, s := range specs {
		if !ValidActionStatuses[s.Status] {
			return nil, fmt.Errorf("specs[%d]: invalid action status %q", i, s.Status)
		}
	}

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

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("bulk insert actions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0, len(specs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("bulk insert actions: scan returned id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bulk insert actions: %w", err)
	}
	if len(ids) != len(specs) {
		return nil, fmt.Errorf("bulk insert actions: expected %d ids, got %d", len(specs), len(ids))
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
	return ids, nil
}

// BulkMarkFailed marks every update.ID as failed in one transaction.
// Already-terminal actions (done/failed/cancelled) are skipped silently so
// repeat calls are no-ops, mirroring MarkFailed.
func (db *DB) BulkMarkFailed(updates []ActionFailureUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("bulk mark failed: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	type committed struct {
		id     int64
		from   string
		result string
	}
	var done []committed

	for _, u := range updates {
		var from string
		if err := tx.QueryRowContext(ctx, "SELECT status FROM actions WHERE id = ?", u.ID).Scan(&from); err != nil {
			return fmt.Errorf("bulk mark failed: get status for id=%d: %w", u.ID, err)
		}
		if from == ActionStatusDone || from == ActionStatusFailed || from == ActionStatusCancelled {
			continue
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE actions SET status = ?, result = ?, completed_at = datetime('now') WHERE id = ? AND status = ?",
			ActionStatusFailed, u.Reason, u.ID, from,
		); err != nil {
			return fmt.Errorf("bulk mark failed: update id=%d: %w", u.ID, err)
		}

		done = append(done, committed{id: u.ID, from: from, result: u.Reason})
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("bulk mark failed: commit: %w", err)
	}

	for _, c := range done {
		db.emitEvent("action", c.id, "action.status_changed", map[string]any{
			"from": c.from, "to": ActionStatusFailed, "result": c.result,
		})
	}
	return nil
}

// BulkMarkDone marks every update.ID as done with its result in one
// transaction. Already-terminal actions (done/failed/cancelled) are skipped
// silently so repeat calls are no-ops, mirroring MarkDone.
func (db *DB) BulkMarkDone(updates []ActionDoneUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("bulk mark done: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	type committed struct {
		id     int64
		from   string
		result string
	}
	var done []committed

	for _, u := range updates {
		var from string
		if err := tx.QueryRowContext(ctx, "SELECT status FROM actions WHERE id = ?", u.ID).Scan(&from); err != nil {
			return fmt.Errorf("bulk mark done: get status for id=%d: %w", u.ID, err)
		}
		if from == ActionStatusDone || from == ActionStatusFailed || from == ActionStatusCancelled {
			continue
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE actions SET status = ?, result = ?, completed_at = datetime('now') WHERE id = ? AND status = ?",
			ActionStatusDone, u.Result, u.ID, from,
		); err != nil {
			return fmt.Errorf("bulk mark done: update id=%d: %w", u.ID, err)
		}

		done = append(done, committed{id: u.ID, from: from, result: u.Result})
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("bulk mark done: commit: %w", err)
	}

	for _, c := range done {
		db.emitEvent("action", c.id, "action.status_changed", map[string]any{
			"from": c.from, "to": ActionStatusDone, "result": c.result,
		})
	}
	return nil
}
