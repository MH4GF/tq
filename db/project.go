package db

import "fmt"

type Project struct {
	ID              int64
	Name            string
	WorkDir         string
	Metadata        string
	DispatchEnabled bool
	CreatedAt       string
}

func (db *DB) GetProjectByName(name string) (*Project, error) {
	row := db.QueryRow("SELECT id, name, work_dir, metadata, dispatch_enabled, created_at FROM projects WHERE name = ?", name)
	p := &Project{}
	err := row.Scan(&p.ID, &p.Name, &p.WorkDir, &p.Metadata, &p.DispatchEnabled, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) GetProjectByID(id int64) (*Project, error) {
	row := db.QueryRow("SELECT id, name, work_dir, metadata, dispatch_enabled, created_at FROM projects WHERE id = ?", id)
	p := &Project{}
	err := row.Scan(&p.ID, &p.Name, &p.WorkDir, &p.Metadata, &p.DispatchEnabled, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) ListProjects() ([]Project, error) {
	rows, err := db.Query("SELECT id, name, work_dir, metadata, dispatch_enabled, created_at FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.WorkDir, &p.Metadata, &p.DispatchEnabled, &p.CreatedAt); err != nil {
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
	return res.LastInsertId()
}

func (db *DB) DeleteProject(id int64) error {
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
	return nil
}

func (db *DB) SetAllDispatchEnabled(enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := db.Exec("UPDATE projects SET dispatch_enabled = ?", val)
	return err
}
