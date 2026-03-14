package db

import (
	"database/sql"
	"fmt"
)

type Project struct {
	ID              int64
	Name            string
	WorkDir         string
	Metadata        string
	DispatchEnabled bool
	CreatedAt       string
}

const projectColumns = "id, name, work_dir, metadata, dispatch_enabled, created_at"

func (p *Project) scanFields() []any {
	return []any{&p.ID, &p.Name, &p.WorkDir, &p.Metadata, &p.DispatchEnabled, &p.CreatedAt}
}

func (db *DB) GetProjectByName(name string) (*Project, error) {
	p := &Project{}
	err := db.QueryRow("SELECT "+projectColumns+" FROM projects WHERE name = ?", name).Scan(p.scanFields()...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) GetProjectByID(id int64) (*Project, error) {
	p := &Project{}
	err := db.QueryRow("SELECT "+projectColumns+" FROM projects WHERE id = ?", id).Scan(p.scanFields()...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) ListProjects() ([]Project, error) {
	rows, err := db.Query("SELECT " + projectColumns + " FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(p.scanFields()...); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (db *DB) InsertProject(name, workDir, metadata string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO projects (name, work_dir, metadata) VALUES (?, ?, ?)",
		name, workDir, metadata,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.emitEvent("project", id, "project.created", map[string]any{
		"name": name, "work_dir": workDir,
	})
	return id, nil
}

func (db *DB) DeleteProject(id int64) error {
	var name string
	db.QueryRow("SELECT name FROM projects WHERE id = ?", id).Scan(&name)

	res, err := db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("project %d not found", id)
	}
	db.emitEvent("project", id, "project.deleted", map[string]any{
		"name": name,
	})
	return nil
}

func (db *DB) SetDispatchEnabled(projectID int64, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	res, err := db.Exec("UPDATE projects SET dispatch_enabled = ? WHERE id = ?", val, projectID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("project %d not found", projectID)
	}
	db.emitEvent("project", projectID, "project.dispatch_changed", map[string]any{
		"enabled": enabled,
	})
	return nil
}

func (db *DB) SetWorkDir(projectID int64, workDir string) error {
	res, err := db.Exec("UPDATE projects SET work_dir = ? WHERE id = ?", workDir, projectID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("project %d not found", projectID)
	}
	return nil
}

func (db *DB) EnsureProject(name string) (int64, error) {
	p, err := db.GetProjectByName(name)
	if err == nil {
		return p.ID, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("get %s project: %w", name, err)
	}
	return db.InsertProject(name, "", "{}")
}

func (db *DB) EnsureNotificationsProject() (int64, error) {
	return db.EnsureProject("notifications")
}

func (db *DB) SetAllDispatchEnabled(enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := db.Exec("UPDATE projects SET dispatch_enabled = ?", val)
	return err
}
