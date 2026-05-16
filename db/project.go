package db

import (
	"fmt"
	"strings"
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

func (db *DB) GetProjectByID(id int64) (*Project, error) {
	p := &Project{}
	err := db.QueryRow("SELECT "+projectColumns+" FROM projects WHERE id = ?", id).Scan(p.scanFields()...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetProjectsByIDs returns the requested projects keyed by ID. Missing IDs are
// absent from the map (no error).
func (db *DB) GetProjectsByIDs(ids []int64) (map[int64]*Project, error) {
	result := make(map[int64]*Project, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT " + projectColumns + " FROM projects WHERE id IN (" + strings.Join(placeholders, ", ") + ")"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get projects by ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var p Project
		if err := rows.Scan(p.scanFields()...); err != nil {
			return nil, fmt.Errorf("get projects by ids: scan: %w", err)
		}
		pp := p
		result[pp.ID] = &pp
	}
	return result, rows.Err()
}

func (db *DB) ListProjects(limit int) ([]Project, error) {
	query := "SELECT " + projectColumns + " FROM projects"
	var args []any
	query, args = appendOrderLimit(query, args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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

func (db *DB) DeleteProject(id int64, cascade bool) error {
	var name string
	if err := db.QueryRow("SELECT name FROM projects WHERE id = ?", id).Scan(&name); err != nil {
		return fmt.Errorf("get project name: %w", err)
	}

	if cascade {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		if _, err := tx.Exec("DELETE FROM actions WHERE task_id IN (SELECT id FROM tasks WHERE project_id = ?)", id); err != nil {
			return fmt.Errorf("delete actions: %w", err)
		}
		if _, err := tx.Exec("DELETE FROM schedules WHERE task_id IN (SELECT id FROM tasks WHERE project_id = ?)", id); err != nil {
			return fmt.Errorf("delete schedules: %w", err)
		}
		if _, err := tx.Exec("DELETE FROM tasks WHERE project_id = ?", id); err != nil {
			return fmt.Errorf("delete tasks: %w", err)
		}
		if _, err := tx.Exec("DELETE FROM projects WHERE id = ?", id); err != nil {
			return fmt.Errorf("delete project: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
	} else {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM tasks WHERE project_id = ?", id).Scan(&count); err != nil {
			return fmt.Errorf("count tasks: %w", err)
		}
		if count > 0 {
			return fmt.Errorf("project #%d has %d task(s); cannot delete without cascade", id, count)
		}
		if _, err := db.Exec("DELETE FROM projects WHERE id = ?", id); err != nil {
			return err
		}
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
	db.emitEvent("project", projectID, "project.work_dir_changed", map[string]any{
		"work_dir": workDir,
	})
	return nil
}
