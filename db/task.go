package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

const (
	TaskStatusOpen     = "open"
	TaskStatusDone     = "done"
	TaskStatusArchived = "archived"
)

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
	var from string
	if err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", id).Scan(&from); err != nil {
		return fmt.Errorf("get current status: %w", err)
	}

	_, err := db.Exec(
		"UPDATE tasks SET status = ?, updated_at = datetime('now') WHERE id = ?",
		status, id,
	)
	if err == nil {
		db.emitEvent("task", id, "task.status_changed", map[string]any{
			"from": from, "to": status, "reason": reason,
		})
	}
	return err
}

func (db *DB) UpdateTaskProject(id int64, projectID int64) error {
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
	for k, v := range updates {
		merged[k] = v
	}

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

func (db *DB) ListTasks(projectID int64, status string) ([]Task, error) {
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
	query += " ORDER BY id"

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
	return db.ListTasks(projectID, "")
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
	if err != sql.ErrNoRows {
		return 0, err
	}
	return db.InsertTask(projectID, title, "{}", "")
}

func (db *DB) ListTasksByStatus(status string) ([]Task, error) {
	return db.ListTasks(0, status)
}
