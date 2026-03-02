package db

import "database/sql"

type Task struct {
	ID        int64
	ProjectID int64
	Title     string
	URL       string
	Metadata  string
	Status    string
	CreatedAt string
	UpdatedAt sql.NullString
}

func (db *DB) InsertTask(projectID int64, title, url, metadata string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO tasks (project_id, title, url, metadata) VALUES (?, ?, ?, ?)",
		projectID, title, url, metadata,
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

func (db *DB) GetTask(id int64) (*Task, error) {
	row := db.QueryRow("SELECT id, project_id, title, url, metadata, status, created_at, updated_at FROM tasks WHERE id = ?", id)
	t := &Task{}
	err := row.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (db *DB) ListTasks(projectName, status string) ([]Task, error) {
	query := "SELECT t.id, t.project_id, t.title, t.url, t.metadata, t.status, t.created_at, t.updated_at FROM tasks t"
	var args []any
	var conditions []string

	if projectName != "" {
		query += " JOIN projects p ON t.project_id = p.id"
		conditions = append(conditions, "p.name = ?")
		args = append(args, projectName)
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
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) ListTasksByProject(projectID int64) ([]Task, error) {
	rows, err := db.Query("SELECT id, project_id, title, url, metadata, status, created_at, updated_at FROM tasks WHERE project_id = ? ORDER BY id", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *DB) ListTasksByStatus(status string) ([]Task, error) {
	rows, err := db.Query("SELECT id, project_id, title, url, metadata, status, created_at, updated_at FROM tasks WHERE status = ? ORDER BY id", status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.URL, &t.Metadata, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
