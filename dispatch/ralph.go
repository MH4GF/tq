package dispatch

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/prompt"
)

// TmuxChecker checks for the existence of tmux windows.
type TmuxChecker interface {
	ListWindows(ctx context.Context, session string) ([]string, error)
}

// ExecTmuxChecker implements TmuxChecker using real tmux commands.
type ExecTmuxChecker struct {
	Runner CommandRunner
}

func (c *ExecTmuxChecker) ListWindows(ctx context.Context, session string) ([]string, error) {
	out, err := c.Runner.Run(ctx, "tmux", []string{
		"list-windows", "-t", session, "-F", "#{window_name}",
	}, "", nil)
	if err != nil {
		return nil, fmt.Errorf("tmux list-windows: %w (output: %s)", err, string(out))
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// RalphConfig configures the Ralph Loop.
type RalphConfig struct {
	UserConfigDir      string
	DB                 *db.DB
	MaxInteractive     int
	PollInterval       time.Duration
	NonInteractiveFunc func() Worker
	InteractiveFunc    func() Worker
	RemoteFunc         func() Worker
	TmuxChecker        TmuxChecker
	StaleGracePeriod   time.Duration
	TmuxSession        string
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
	if cfg.StaleGracePeriod <= 0 {
		cfg.StaleGracePeriod = 30 * time.Second
	}
	if cfg.TmuxSession == "" {
		cfg.TmuxSession = "main"
	}

	slog.Info("ralph loop started", "max_interactive", cfg.MaxInteractive, "poll_interval", cfg.PollInterval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("ralph loop stopped")
			return ctx.Err()
		default:
		}

		reapStaleActions(ctx, cfg)

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

func reapStaleActions(ctx context.Context, cfg RalphConfig) {
	if cfg.TmuxChecker == nil {
		return
	}

	actions, err := cfg.DB.ListRunningInteractive()
	if err != nil {
		slog.Error("list running interactive for stale check", "error", err)
		return
	}
	if len(actions) == 0 {
		return
	}

	windows, err := cfg.TmuxChecker.ListWindows(ctx, cfg.TmuxSession)
	if err != nil {
		slog.Warn("tmux list-windows failed, skipping stale check", "error", err)
		return
	}

	windowSet := make(map[string]struct{}, len(windows))
	for _, w := range windows {
		windowSet[w] = struct{}{}
	}

	now := time.Now()
	for _, a := range actions {
		if a.StartedAt.Valid {
			started, err := time.Parse("2006-01-02 15:04:05", a.StartedAt.String)
			if err == nil && now.Sub(started) < cfg.StaleGracePeriod {
				continue
			}
		}

		windowName := fmt.Sprintf("tq-action-%d", a.ID)
		if _, exists := windowSet[windowName]; exists {
			continue
		}

		result := fmt.Sprintf("stale: tmux window %q no longer exists", windowName)
		if err := cfg.DB.MarkFailed(a.ID, result); err != nil {
			slog.Error("mark stale action failed", "action_id", a.ID, "error", err)
			continue
		}
		slog.Warn("reaped stale action", "action_id", a.ID, "window", windowName)
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

	promptsDir := resolvePromptsDir(cfg.UserConfigDir)
	tmpl, err := prompt.Load(promptsDir, action.PromptID)
	if err != nil {
		_ = cfg.DB.MarkFailed(action.ID, fmt.Sprintf("prompt load error: %v", err))
		return true, fmt.Errorf("load prompt %q: %w", action.PromptID, err)
	}

	promptData, err := buildPromptDataFromDB(cfg.DB, action)
	if err != nil {
		_ = cfg.DB.MarkFailed(action.ID, fmt.Sprintf("build prompt data: %v", err))
		return true, fmt.Errorf("build prompt data: %w", err)
	}

	prompt, err := tmpl.Render(promptData)
	if err != nil {
		_ = cfg.DB.MarkFailed(action.ID, fmt.Sprintf("render error: %v", err))
		return true, fmt.Errorf("render prompt: %w", err)
	}

	workDir := "."
	if promptData.Project.WorkDir != "" {
		workDir = expandHome(promptData.Project.WorkDir)
	}

	if tmpl.Config.IsRemote() {
		return dispatchRemote(ctx, cfg, action, prompt, tmpl.Config, workDir)
	}
	if tmpl.Config.IsInteractive() {
		return dispatchInteractive(ctx, cfg, action, prompt, tmpl.Config, workDir)
	}
	return dispatchNonInteractive(ctx, cfg, action, prompt, tmpl.Config, workDir)
}

func dispatchInteractive(ctx context.Context, cfg RalphConfig, action *db.Action, prompt string, tmplCfg prompt.Config, workDir string) (bool, error) {
	running, err := cfg.DB.CountRunningInteractive()
	if err != nil {
		return true, fmt.Errorf("count running interactive: %w", err)
	}
	if running >= cfg.MaxInteractive {
		_ = cfg.DB.ResetToPending(action.ID)
		slog.Info("interactive limit reached, deferring", "action_id", action.ID, "running", running, "max", cfg.MaxInteractive)
		return false, nil
	}

	worker := cfg.InteractiveFunc()
	result, err := worker.Execute(ctx, prompt, tmplCfg, workDir, action.ID, nullInt64ToPtr(action.TaskID))
	if err != nil {
		handleFailure(cfg, action, err)
		return true, nil
	}

	windowName := fmt.Sprintf("tq-action-%d", action.ID)
	if err := cfg.DB.SetSessionInfo(action.ID, cfg.TmuxSession, windowName); err != nil {
		slog.Warn("failed to save session info", "action_id", action.ID, "error", err)
	}

	slog.Info("interactive action dispatched", "action_id", action.ID, "result", result)
	return true, nil
}

func dispatchRemote(ctx context.Context, cfg RalphConfig, action *db.Action, prompt string, tmplCfg prompt.Config, workDir string) (bool, error) {
	worker := cfg.RemoteFunc()
	result, err := worker.Execute(ctx, prompt, tmplCfg, workDir, action.ID, nullInt64ToPtr(action.TaskID))
	if err != nil {
		handleFailure(cfg, action, err)
		return true, nil
	}

	if err := cfg.DB.MergeActionMetadata(action.ID, map[string]any{
		"remote_session": result,
	}); err != nil {
		slog.Warn("failed to save remote session info", "action_id", action.ID, "error", err)
	}

	slog.Info("remote action dispatched", "action_id", action.ID, "result", result)
	return true, nil
}

func dispatchNonInteractive(ctx context.Context, cfg RalphConfig, action *db.Action, prompt string, tmplCfg prompt.Config, workDir string) (bool, error) {
	worker := cfg.NonInteractiveFunc()
	result, err := worker.Execute(ctx, prompt, tmplCfg, workDir, action.ID, nullInt64ToPtr(action.TaskID))
	if err != nil {
		handleFailure(cfg, action, err)
		return true, nil
	}

	if err := cfg.DB.MarkDone(action.ID, result); err != nil {
		return true, fmt.Errorf("mark done: %w", err)
	}

	promptsDir := resolvePromptsDir(cfg.UserConfigDir)
	if err := TriggerOnDone(cfg.DB, promptsDir, action, result); err != nil {
		slog.Warn("on_done trigger failed", "action_id", action.ID, "error", err)
	}

	slog.Info("action done", "action_id", action.ID)
	return true, nil
}

func nullInt64ToPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

func handleFailure(cfg RalphConfig, action *db.Action, execErr error) {
	_ = cfg.DB.MarkFailed(action.ID, execErr.Error())
	slog.Error("action failed", "action_id", action.ID, "error", execErr)
}

func buildPromptDataFromDB(database *db.DB, action *db.Action) (prompt.PromptData, error) {
	var data prompt.PromptData

	actionMeta := make(map[string]any)
	if action.Metadata != "" && action.Metadata != "{}" {
		if err := json.Unmarshal([]byte(action.Metadata), &actionMeta); err != nil {
			return data, fmt.Errorf("parse action metadata: %w", err)
		}
	}
	data.Action = prompt.ActionData{
		ID:       action.ID,
		PromptID: action.PromptID,
		Status:   action.Status,
		Meta:     actionMeta,
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
		data.Task = prompt.TaskData{
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
		data.Project = prompt.ProjectData{
			ID:      project.ID,
			Name:    project.Name,
			WorkDir: project.WorkDir,
			Meta:    projectMeta,
		}
	}

	return data, nil
}

func resolvePromptsDir(userConfigDir string) string {
	return filepath.Join(userConfigDir, "prompts")
}

func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
