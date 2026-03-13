package db

import (
	"database/sql"
	"strings"
)

type Task struct {
	ID        int64
	ProjectID int64
	Title     string
	URL       string
	Metadata  string
	Status    string
	WorkDir   string
	CreatedAt string
	UpdatedAt sql.NullString
}

func (t Task) MatchesDate(date string) bool {
	if strings.HasPrefix(t.CreatedAt, date) {
		return true
	}
	if t.UpdatedAt.Valid && strings.HasPrefix(t.UpdatedAt.String, date) {
		return true
	}
	return false
}

func (db *DB) InsertTask(projectID int64, title, url, metadata, workDir string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO tasks (project_id, title, url, metadata, work_dir) VALUES (?, ?, ?, ?, ?)",
		projectID, title, url, metadata, workDir,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) UpdateTask(id int64, status string) error {
	_, err := db.Exec(
		"UPDATE tasks SET status = ?, updated_at = datetime('now') WHERE id = ?",
		status, id,
	)
	return err
}

func (db *DB) UpdateTaskProject(id int64, projectID int64) error {
	_, err := db.Exec(
		"UPDATE tasks SET project_id = ?, updated_at = datetime('now') WHERE id = ?",
		projectID, id,
	)
	return err
}

func (db *DB) UpdateTaskWorkDir(id int64, workDir string) error {
	_, err := db.Exec(
		"UPDATE tasks SET work_dir = ?, updated_at = datetime('now') WHERE id = ?",
		workDir, id,
	)
	return err
}

func (db *DB) GetTask(id int64) (*Task, error) {
	row := db.QueryRow("SELECT id, project_id, title, url, metadata, status, work_dir, created_at, updated_at FROM tasks WHERE id = ?", id)
	t := &Task{}
	err := row.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.WorkDir, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (db *DB) ListTasks(projectID int64, status string) ([]Task, error) {
	query := "SELECT t.id, t.project_id, t.title, t.url, t.metadata, t.status, t.work_dir, t.created_at, t.updated_at FROM tasks t"
	var args []any
	var conditions []string

	if projectID != 0 {
		conditions = append(conditions, "t.project_id = ?")
		args = append(args, projectID)
	}
	if status != "" {
		conditions = append(conditions, "t.status = ?")
		args = append(args, status)
	}
	for i, c := range conditions {
		if i == 0 {
			query += " WHERE " + c
		} else {
			query += " AND " + c
		}
	}
	query += " ORDER BY t.id"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.WorkDir, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) ListTasksByProject(projectID int64) ([]Task, error) {
	rows, err := db.Query("SELECT id, project_id, title, url, metadata, status, work_dir, created_at, updated_at FROM tasks WHERE project_id = ? ORDER BY id", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.WorkDir, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) GetOrCreateTriageTask(projectID int64) (int64, error) {
	return db.EnsureTask(projectID, "triage")
}

func (db *DB) EnsureTask(projectID int64, title string) (int64, error) {
	var id int64
	err := db.QueryRow(
		"SELECT id FROM tasks WHERE project_id = ? AND title = ? AND status = 'open' ORDER BY id ASC LIMIT 1",
		projectID, title,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	return db.InsertTask(projectID, title, "", "{}", "")
}

func (db *DB) ListTasksByStatus(status string) ([]Task, error) {
	rows, err := db.Query("SELECT id, project_id, title, url, metadata, status, work_dir, created_at, updated_at FROM tasks WHERE status = ? ORDER BY id", status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.WorkDir, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
