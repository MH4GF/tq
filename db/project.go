package db

import "fmt"

type Project struct {
	ID        int64
	Name      string
	WorkDir   string
	Metadata  string
	CreatedAt string
}

func (db *DB) GetProjectByName(name string) (*Project, error) {
	row := db.QueryRow("SELECT id, name, work_dir, metadata, created_at FROM projects WHERE name = ?", name)
	p := &Project{}
	err := row.Scan(&p.ID, &p.Name, &p.WorkDir, &p.Metadata, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) GetProjectByID(id int64) (*Project, error) {
	row := db.QueryRow("SELECT id, name, work_dir, metadata, created_at FROM projects WHERE id = ?", id)
	p := &Project{}
	err := row.Scan(&p.ID, &p.Name, &p.WorkDir, &p.Metadata, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (db *DB) ListProjects() ([]Project, error) {
	rows, err := db.Query("SELECT id, name, work_dir, metadata, created_at FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.WorkDir, &p.Metadata, &p.CreatedAt); err != nil {
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
