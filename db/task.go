package db

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	res, err := db.Exec(
		"INSERT INTO tasks (project_id, title, metadata, work_dir) VALUES (?, ?, ?, ?)",
		projectID, title, metadata, workDir,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.emitEvent("task", id, "task.created", map[string]any{
		"project_id": projectID, "title": title,
	})
	return id, nil
}

func (db *DB) UpdateTask(id int64, status, reason string) error {
	if !ValidTaskStatuses[status] {
		return fmt.Errorf("invalid task status %q: must be one of open, done, archived", status)
	}

	var from string
	if err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current status: %w", err)
	}

	if status == TaskStatusDone || status == TaskStatusArchived {
		schedIDs, err := db.EnabledScheduleIDs(id)
		if err != nil {
			return fmt.Errorf("check enabled schedules: %w", err)
		}
		if len(schedIDs) > 0 {
			return fmt.Errorf("task #%d has enabled schedules %v — this task has recurring scheduled actions; it should usually stay open. [agent hint] this task is likely not meant to be closed; confirm with the user whether they really want to close it, and if so, run `tq schedule disable <id>` for each schedule first, then retry", id, schedIDs)
		}
	}

	if status == TaskStatusDone || status == TaskStatusArchived {
		var activeCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM actions WHERE task_id = ? AND status IN (?, ?)",
			id, ActionStatusPending, ActionStatusRunning,
		).Scan(&activeCount)
		if err != nil {
			return fmt.Errorf("check active actions: %w", err)
		}
		if activeCount > 0 {
			return fmt.Errorf("task #%d has %d pending/running action(s). Cancel or complete them before closing", id, activeCount)
		}
	}

	_, err := db.Exec(
		"UPDATE tasks SET status = ?, updated_at = datetime('now') WHERE id = ?",
		status, id,
	)
	if err == nil {
		db.emitEvent("task", id, EventTaskStatusChanged, map[string]any{
			"from": from, "to": status, "reason": reason,
		})
	}
	return err
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

func (db *DB) UpdateTaskProject(id, projectID int64) error {
	var from int64
	if err := db.QueryRow("SELECT project_id FROM tasks WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current project_id: %w", err)
	}

	_, err := db.Exec(
		"UPDATE tasks SET project_id = ?, updated_at = datetime('now') WHERE id = ?",
		projectID, id,
	)
	if err == nil {
		db.emitEvent("task", id, "task.project_changed", map[string]any{
			"from": from, "to": projectID,
		})
	}
	return err
}

func (db *DB) UpdateTaskWorkDir(id int64, workDir string) error {
	var from string
	if err := db.QueryRow("SELECT work_dir FROM tasks WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current work_dir: %w", err)
	}

	_, err := db.Exec(
		"UPDATE tasks SET work_dir = ?, updated_at = datetime('now') WHERE id = ?",
		workDir, id,
	)
	if err == nil {
		db.emitEvent("task", id, "task.workdir_changed", map[string]any{
			"from": from, "to": workDir,
		})
	}
	return err
}

func (db *DB) MergeTaskMetadata(id int64, updates map[string]any) error {
	var existing string
	err := db.QueryRow("SELECT metadata FROM tasks WHERE id = ?", id).Scan(&existing)
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

	_, err = db.Exec(
		"UPDATE tasks SET metadata = ?, updated_at = datetime('now') WHERE id = ?",
		string(data), id,
	)
	if err == nil {
		keys := make([]string, 0, len(updates))
		for k := range updates {
			keys = append(keys, k)
		}
		db.emitEvent("task", id, "task.metadata_merged", map[string]any{
			"keys_updated": keys,
		})
	}
	return err
}

func (db *DB) GetTask(id int64) (*Task, error) {
	t := &Task{}
	err := db.QueryRow("SELECT "+taskColumns+" FROM tasks WHERE id = ?", id).Scan(t.scanFields()...)
	if err != nil {
		return nil, err
	}
	return t, nil
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

func (db *DB) ListTasksByProject(projectID int64) ([]Task, error) {
	return db.ListTasks(projectID, "", 0)
}

func (db *DB) GetOrCreateTriageTask(projectID int64) (int64, error) {
	return db.EnsureTask(projectID, "triage")
}

func (db *DB) EnsureTask(projectID int64, title string) (int64, error) {
	var id int64
	err := db.QueryRow(
		"SELECT id FROM tasks WHERE project_id = ? AND title = ? AND status = ? ORDER BY id ASC LIMIT 1",
		projectID, title, TaskStatusOpen,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return db.InsertTask(projectID, title, "{}", "")
}

func (db *DB) ListTasksByStatus(status string) ([]Task, error) {
	return db.ListTasks(0, status, 0)
}
