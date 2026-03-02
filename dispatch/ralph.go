package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/template"
)

// RalphConfig configures the Ralph Loop.
type RalphConfig struct {
	TQDir              string
	DB                 *db.DB
	MaxInteractive     int
	PollInterval       time.Duration
	NonInteractiveFunc func(tqDir string) Worker
	InteractiveFunc    func(tqDir string) Worker
}

// RalphLoop continuously dispatches pending actions.
// It processes one action per iteration, sleeping when idle.
func RalphLoop(ctx context.Context, cfg RalphConfig) error {
	if cfg.MaxInteractive <= 0 {
		cfg.MaxInteractive = 3
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 10 * time.Second
	}

	slog.Info("ralph loop started", "max_interactive", cfg.MaxInteractive, "poll_interval", cfg.PollInterval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("ralph loop stopped")
			return ctx.Err()
		default:
		}

		dispatched, err := dispatchOne(ctx, cfg)
		if err != nil {
			slog.Error("dispatch error", "error", err)
		}

		if !dispatched {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.PollInterval):
			}
		}
	}
}

func dispatchOne(ctx context.Context, cfg RalphConfig) (bool, error) {
	action, err := cfg.DB.NextPending(ctx)
	if err != nil {
		return false, fmt.Errorf("next pending: %w", err)
	}
	if action == nil {
		return false, nil
	}

	templatesDir := filepath.Join(cfg.TQDir, "templates")
	tmpl, err := template.Load(templatesDir, action.TemplateID)
	if err != nil {
		_ = cfg.DB.MarkFailed(action.ID, fmt.Sprintf("template load error: %v", err))
		return true, fmt.Errorf("load template %q: %w", action.TemplateID, err)
	}

	if !tmpl.Config.Auto {
		_ = cfg.DB.MarkWaitingHuman(action.ID, "auto=false, requires human approval")
		slog.Info("action requires human approval", "action_id", action.ID, "template", action.TemplateID)
		return true, nil
	}

	promptData, err := buildPromptDataFromDB(cfg.DB, action, cfg.TQDir)
	if err != nil {
		_ = cfg.DB.MarkFailed(action.ID, fmt.Sprintf("build prompt data: %v", err))
		return true, fmt.Errorf("build prompt data: %w", err)
	}

	prompt, err := tmpl.Render(promptData)
	if err != nil {
		_ = cfg.DB.MarkFailed(action.ID, fmt.Sprintf("render error: %v", err))
		return true, fmt.Errorf("render template: %w", err)
	}

	workDir := cfg.TQDir
	if promptData.Project.WorkDir != "" {
		workDir = expandHome(promptData.Project.WorkDir)
	}

	if tmpl.Config.Interactive {
		return dispatchInteractive(ctx, cfg, action, prompt, tmpl.Config, workDir)
	}
	return dispatchNonInteractive(ctx, cfg, action, prompt, tmpl.Config, workDir)
}

func dispatchInteractive(ctx context.Context, cfg RalphConfig, action *db.Action, prompt string, tmplCfg template.Config, workDir string) (bool, error) {
	running, err := cfg.DB.CountRunningInteractive()
	if err != nil {
		return true, fmt.Errorf("count running interactive: %w", err)
	}
	if running >= cfg.MaxInteractive {
		_ = cfg.DB.ResetToPending(action.ID)
		slog.Info("interactive limit reached, deferring", "action_id", action.ID, "running", running, "max", cfg.MaxInteractive)
		return false, nil
	}

	worker := cfg.InteractiveFunc(cfg.TQDir)
	result, err := worker.Execute(ctx, prompt, tmplCfg, workDir, action.ID)
	if err != nil {
		handleFailure(cfg, action, tmplCfg, err)
		return true, nil
	}

	slog.Info("interactive action dispatched", "action_id", action.ID, "result", result)
	return true, nil
}

func dispatchNonInteractive(ctx context.Context, cfg RalphConfig, action *db.Action, prompt string, tmplCfg template.Config, workDir string) (bool, error) {
	worker := cfg.NonInteractiveFunc(cfg.TQDir)
	result, err := worker.Execute(ctx, prompt, tmplCfg, workDir, action.ID)
	if err != nil {
		handleFailure(cfg, action, tmplCfg, err)
		return true, nil
	}

	if err := cfg.DB.MarkDone(action.ID, result); err != nil {
		return true, fmt.Errorf("mark done: %w", err)
	}
	slog.Info("action done", "action_id", action.ID)
	return true, nil
}

func handleFailure(cfg RalphConfig, action *db.Action, tmplCfg template.Config, execErr error) {
	if tmplCfg.MaxRetries > 0 {
		_ = cfg.DB.ResetToPending(action.ID)
		slog.Warn("action failed, retrying", "action_id", action.ID, "error", execErr)
		return
	}
	_ = cfg.DB.MarkWaitingHuman(action.ID, execErr.Error())
	slog.Error("action failed, escalating to human", "action_id", action.ID, "error", execErr)
}

func buildPromptDataFromDB(database *db.DB, action *db.Action, tqDir string) (template.PromptData, error) {
	var data template.PromptData
	data.TQDir = tqDir

	actionMeta := make(map[string]any)
	if action.Metadata != "" && action.Metadata != "{}" {
		if err := json.Unmarshal([]byte(action.Metadata), &actionMeta); err != nil {
			return data, fmt.Errorf("parse action metadata: %w", err)
		}
	}
	data.Action = template.ActionData{
		ID:         action.ID,
		TemplateID: action.TemplateID,
		Status:     action.Status,
		Priority:   action.Priority,
		Source:     action.Source,
		Meta:       actionMeta,
	}

	if action.TaskID.Valid {
		task, err := database.GetTask(action.TaskID.Int64)
		if err != nil {
			return data, fmt.Errorf("get task: %w", err)
		}
		taskMeta := make(map[string]any)
		if task.Metadata != "" && task.Metadata != "{}" {
			if err := json.Unmarshal([]byte(task.Metadata), &taskMeta); err != nil {
				return data, fmt.Errorf("parse task metadata: %w", err)
			}
		}
		data.Task = template.TaskData{
			ID:     task.ID,
			Title:  task.Title,
			URL:    task.URL,
			Status: task.Status,
			Meta:   taskMeta,
		}

		project, err := database.GetProjectByID(task.ProjectID)
		if err != nil {
			return data, fmt.Errorf("get project: %w", err)
		}
		projectMeta := make(map[string]any)
		if project.Metadata != "" && project.Metadata != "{}" {
			if err := json.Unmarshal([]byte(project.Metadata), &projectMeta); err != nil {
				return data, fmt.Errorf("parse project metadata: %w", err)
			}
		}
		data.Project = template.ProjectData{
			ID:      project.ID,
			Name:    project.Name,
			WorkDir: project.WorkDir,
			Meta:    projectMeta,
		}
	}

	return data, nil
}

func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
