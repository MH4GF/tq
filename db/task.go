package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strings"
)

const (
	TaskStatusOpen     = "open"
	TaskStatusDone     = "done"
	TaskStatusArchived = "archived"
)

const NoteKindTriageKeep = "triage_keep"

var ValidTaskStatuses = map[string]bool{
	TaskStatusOpen:     true,
	TaskStatusDone:     true,
	TaskStatusArchived: true,
}

func IsTerminalTaskStatus(status string) bool {
	return status == TaskStatusDone || status == TaskStatusArchived
}

func (db *DB) EnsureTaskOpenForAttach(taskID int64, op string) error {
	parent, err := db.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("get parent task #%d: %w", taskID, err)
	}
	if IsTerminalTaskStatus(parent.Status) {
		return fmt.Errorf(
			"cannot %s task #%d (status=%s): "+
				"verify the task ID is correct, or reopen with "+
				"'tq task update %d --status open' if this is intentional",
			op, taskID, parent.Status, taskID,
		)
	}
	return nil
}

type Task struct {
	ID        int64
	ProjectID int64
	Title     string
	Metadata  string
	Status    string
	WorkDir   string
	CreatedAt string
	UpdatedAt sql.NullString
}

const taskColumns = "id, project_id, title, metadata, status, work_dir, created_at, updated_at"

func (t *Task) scanFields() []any {
	return []any{&t.ID, &t.ProjectID, &t.Title, &t.Metadata, &t.Status, &t.WorkDir, &t.CreatedAt, &t.UpdatedAt}
}

func (t Task) MatchesDate(date string) bool {
	if matchesDateLocal(t.CreatedAt, date) {
		return true
	}
	if t.UpdatedAt.Valid && matchesDateLocal(t.UpdatedAt.String, date) {
		return true
	}
	return false
}

func (db *DB) InsertTask(projectID int64, title, metadata, workDir string) (int64, error) {
	var id int64
	err := withRetry(context.Background(), "InsertTask", func() error {
		res, err := db.Exec(
			"INSERT INTO tasks (project_id, title, metadata, work_dir) VALUES (?, ?, ?, ?)",
			projectID, title, metadata, workDir,
		)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, err
	}
	db.emitEvent("task", id, "task.created", map[string]any{
		"project_id": projectID, "title": title,
	})
	return id, nil
}

// TaskFieldChanges is the set of task fields to update. A nil pointer (or nil
// map) means "leave unchanged". All requested changes are applied atomically.
type TaskFieldChanges struct {
	ProjectID *int64
	WorkDir   *string
	Metadata  map[string]any // shallow-merged into existing metadata
	Status    *string
	Reason    string // recorded with the status change
}

// UpdateTaskFields applies every requested field change in a single
// transaction: either all changes commit, or none do. Status validation runs
// fail-fast before any write; events are emitted only after a successful
// commit (matching the established post-commit emitEvent pattern).
func (db *DB) UpdateTaskFields(id int64, c TaskFieldChanges) error {
	if c.Status != nil {
		status := *c.Status
		if !ValidTaskStatuses[status] {
			return fmt.Errorf("invalid task status %q: must be one of open, done, archived", status)
		}
		if IsTerminalTaskStatus(status) {
			schedIDs, err := db.EnabledScheduleIDs(id)
			if err != nil {
				return fmt.Errorf("check enabled schedules: %w", err)
			}
			if len(schedIDs) > 0 {
				return fmt.Errorf("task #%d has enabled schedules %v — this task has recurring scheduled actions; it should usually stay open. [agent hint] this task is likely not meant to be closed; confirm with the user whether they really want to close it, and if so, run `tq schedule disable <id>` for each schedule first, then retry", id, schedIDs)
			}
			activeCount, err := db.GetTaskActionCount(id, []string{ActionStatusPending, ActionStatusRunning, ActionStatusDispatched})
			if err != nil {
				return fmt.Errorf("check active actions: %w", err)
			}
			if activeCount > 0 {
				return fmt.Errorf("task #%d has %d pending/running/dispatched action(s). Cancel or complete them before closing", id, activeCount)
			}
		}
	}

	ctx := context.Background()
	var cur Task
	var metaKeys []string
	var noChange bool
	err := db.withTxRetry(ctx, "UpdateTask", func(tx *sql.Tx) error {
		cur = Task{}
		metaKeys = nil
		noChange = false
		if err := tx.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id = ?", id).Scan(cur.scanFields()...); err != nil {
			return fmt.Errorf("task #%d not found: %w", id, err)
		}
		setClauses := []string{"updated_at = datetime('now')"}
		var args []any
		if c.ProjectID != nil {
			setClauses = append(setClauses, "project_id = ?")
			args = append(args, *c.ProjectID)
		}
		if c.WorkDir != nil {
			setClauses = append(setClauses, "work_dir = ?")
			args = append(args, *c.WorkDir)
		}
		if c.Metadata != nil {
			data, keys, err := mergeMetadataJSON(cur.Metadata, c.Metadata)
			if err != nil {
				return err
			}
			setClauses = append(setClauses, "metadata = ?")
			args = append(args, data)
			metaKeys = keys
		}
		if c.Status != nil {
			setClauses = append(setClauses, "status = ?")
			args = append(args, *c.Status)
		}
		if len(args) == 0 {
			noChange = true
			return nil
		}
		args = append(args, id)
		// #nosec G202 -- setClauses holds only hardcoded column literals; all
		// values are bound via ? placeholders in args.
		query := "UPDATE tasks SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if noChange {
		return nil
	}
	if c.ProjectID != nil {
		db.emitEvent("task", id, EventTaskProjectChanged, map[string]any{"from": cur.ProjectID, "to": *c.ProjectID})
	}
	if c.WorkDir != nil {
		db.emitEvent("task", id, EventTaskWorkDirChanged, map[string]any{"from": cur.WorkDir, "to": *c.WorkDir})
	}
	if c.Metadata != nil {
		db.emitEvent("task", id, EventTaskMetadataMerged, map[string]any{"keys_updated": metaKeys})
	}
	if c.Status != nil {
		db.emitEvent("task", id, EventTaskStatusChanged, map[string]any{"from": cur.Status, "to": *c.Status, "reason": c.Reason})
	}
	return nil
}

// mergeMetadataJSON shallow-merges updates into the existing JSON metadata
// blob and returns the marshalled result plus the updated key names.
func mergeMetadataJSON(existing string, updates map[string]any) (string, []string, error) {
	merged := make(map[string]any)
	if existing != "" && existing != "{}" {
		if err := json.Unmarshal([]byte(existing), &merged); err != nil {
			return "", nil, fmt.Errorf("parse existing metadata: %w", err)
		}
	}
	maps.Copy(merged, updates)
	data, err := json.Marshal(merged)
	if err != nil {
		return "", nil, fmt.Errorf("marshal metadata: %w", err)
	}
	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	return string(data), keys, nil
}

func (db *DB) UpdateTask(id int64, status, reason string) error {
	return db.UpdateTaskFields(id, TaskFieldChanges{Status: &status, Reason: reason})
}

type TaskStatusHistoryEntry struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
	At     string `json:"at"`
}

func (db *DB) TaskStatusHistory(taskID int64) ([]TaskStatusHistoryEntry, error) {
	events, err := db.ListEvents("task", taskID)
	if err != nil {
		return nil, err
	}
	history := make([]TaskStatusHistoryEntry, 0)
	for _, e := range events {
		if e.EventType != EventTaskStatusChanged {
			continue
		}
		var p struct {
			From   string `json:"from"`
			To     string `json:"to"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal([]byte(e.Payload), &p); err != nil {
			slog.Warn("status_history: parse payload", "event_id", e.ID, "error", err)
			continue
		}
		history = append(history, TaskStatusHistoryEntry{
			From:   p.From,
			To:     p.To,
			Reason: p.Reason,
			At:     FormatLocal(e.CreatedAt),
		})
	}
	return history, nil
}

type TaskNoteEntry struct {
	Kind     string         `json:"kind"`
	Reason   string         `json:"reason"`
	Metadata map[string]any `json:"metadata,omitempty"`
	At       string         `json:"at"`
}

func (db *DB) RecordTaskNote(taskID int64, kind, reason string, metadata map[string]any) error {
	if kind == "" {
		return fmt.Errorf("kind is required")
	}
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	var exists int
	if err := db.QueryRow("SELECT 1 FROM tasks WHERE id = ?", taskID).Scan(&exists); err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	payload := map[string]any{
		"kind":   kind,
		"reason": reason,
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}
	db.emitEvent("task", taskID, EventTaskNote, payload)
	return nil
}

func (db *DB) TaskNotes(taskID int64, kindFilter string) ([]TaskNoteEntry, error) {
	events, err := db.ListEvents("task", taskID)
	if err != nil {
		return nil, err
	}
	notes := make([]TaskNoteEntry, 0)
	for _, e := range events {
		if e.EventType != EventTaskNote {
			continue
		}
		entry, ok := decodeTaskNote(e)
		if !ok {
			continue
		}
		if kindFilter != "" && entry.Kind != kindFilter {
			continue
		}
		notes = append(notes, entry)
	}
	return notes, nil
}

func (db *DB) LatestTaskNotes(taskIDs []int64, kindFilter string) (map[int64]TaskNoteEntry, error) {
	out := make(map[int64]TaskNoteEntry)
	if len(taskIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(taskIDs))
	args := make([]any, 0, len(taskIDs)+1)
	for i, id := range taskIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, EventTaskNote)
	query := "SELECT " + eventColumns + " FROM events WHERE entity_type = 'task' AND entity_id IN (" + strings.Join(placeholders, ",") + ") AND event_type = ? ORDER BY id"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	events, err := scanEvents(rows)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		entry, ok := decodeTaskNote(e)
		if !ok {
			continue
		}
		if kindFilter != "" && entry.Kind != kindFilter {
			continue
		}
		out[e.EntityID] = entry
	}
	return out, nil
}

func decodeTaskNote(e Event) (TaskNoteEntry, bool) {
	var p struct {
		Kind     string         `json:"kind"`
		Reason   string         `json:"reason"`
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(e.Payload), &p); err != nil {
		slog.Warn("task_note: parse payload", "event_id", e.ID, "error", err)
		return TaskNoteEntry{}, false
	}
	return TaskNoteEntry{
		Kind:     p.Kind,
		Reason:   p.Reason,
		Metadata: p.Metadata,
		At:       FormatLocal(e.CreatedAt),
	}, true
}

func (db *DB) GetTask(id int64) (*Task, error) {
	t := &Task{}
	err := db.QueryRow("SELECT "+taskColumns+" FROM tasks WHERE id = ?", id).Scan(t.scanFields()...)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// GetTasksByIDs returns the requested tasks keyed by ID. Missing IDs are
// absent from the map (no error).
func (db *DB) GetTasksByIDs(ids []int64) (map[int64]*Task, error) {
	result := make(map[int64]*Task, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT " + taskColumns + " FROM tasks WHERE id IN (" + strings.Join(placeholders, ", ") + ")"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get tasks by ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var t Task
		if err := rows.Scan(t.scanFields()...); err != nil {
			return nil, fmt.Errorf("get tasks by ids: scan: %w", err)
		}
		tt := t
		result[tt.ID] = &tt
	}
	return result, rows.Err()
}

func (db *DB) ListTasks(projectID int64, status string, limit int) ([]Task, error) {
	query := "SELECT " + taskColumns + " FROM tasks WHERE 1=1"
	var args []any

	if projectID != 0 {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query, args = appendOrderLimit(query, args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(t.scanFields()...); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) ListTasksByProjectIDs(projectIDs []int64) (map[int64][]Task, error) {
	result := make(map[int64][]Task)
	if len(projectIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(projectIDs))
	args := make([]any, len(projectIDs))
	for i, id := range projectIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	// ORDER BY project_id, id DESC matches idx_tasks_project so the planner
	// can stream rows in index order without a temp B-tree sort.
	query := "SELECT " + taskColumns + " FROM tasks WHERE project_id IN (" + strings.Join(placeholders, ", ") + ") ORDER BY project_id, id DESC"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var t Task
		if err := rows.Scan(t.scanFields()...); err != nil {
			return nil, err
		}
		result[t.ProjectID] = append(result[t.ProjectID], t)
	}
	return result, rows.Err()
}
