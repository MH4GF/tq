package cmd

import (
	"os"
	"path/filepath"

	"github.com/MH4GF/tq/db"
)

func resolveTemplatesDir(projectWorkDir string) string {
	if projectWorkDir != "" {
		projDir := filepath.Join(projectWorkDir, "tq", "templates")
		if info, err := os.Stat(projDir); err == nil && info.IsDir() {
			return projDir
		}
	}
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "templates")
}

func getProjectWorkDir(action *db.Action) string {
	if !action.TaskID.Valid {
		return ""
	}
	task, err := database.GetTask(action.TaskID.Int64)
	if err != nil {
		return ""
	}
	project, err := database.GetProjectByID(task.ProjectID)
	if err != nil {
		return ""
	}
	return project.WorkDir
}
