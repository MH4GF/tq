package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/prompt"
)

const (
	ModeRemote         = "remote"
	ModeInteractive    = "interactive"
	ModeNonInteractive = "noninteractive"
)

// DispatchConfig holds shared dispatch settings used by both WorkerConfig and ExecuteParams.
type DispatchConfig struct {
	DB                 db.Store
	NonInteractiveFunc func() Worker
	InteractiveFunc    func() Worker
	RemoteFunc         func() Worker
	TmuxSession        string
}

type ExecuteParams struct {
	DispatchConfig
	PromptsDir        string
	BeforeInteractive func(action *db.Action) error
}

type ExecuteResult struct {
	Mode   string
	Output string
}

type ActionFailedError struct {
	ActionID int64
	Err      error
}

func (e *ActionFailedError) Error() string {
	return fmt.Sprintf("action #%d failed: %v", e.ActionID, e.Err)
}

func (e *ActionFailedError) Unwrap() error {
	return e.Err
}

var ErrInteractiveDeferred = errors.New("interactive deferred")

// WindowName returns the tmux window name for an action.
func WindowName(actionID int64) string {
	return fmt.Sprintf("tq-action-%d", actionID)
}

// ExecuteAction loads the prompt, renders it, and dispatches via the appropriate worker.
func ExecuteAction(ctx context.Context, params ExecuteParams, action *db.Action) (*ExecuteResult, error) {
	promptData, err := BuildPromptData(params.DB, action)
	if err != nil {
		failMsg := fmt.Sprintf("build prompt data: %v", err)
		_ = params.DB.MarkFailed(action.ID, failMsg)
		CreateInvestigateFailureAction(params.DB, action, failMsg)
		return nil, fmt.Errorf("build prompt data: %w", err)
	}

	var rendered string
	var cfg prompt.Config

	if action.PromptID == "" {
		instruction, ok := promptData.Action.Meta["instruction"].(string)
		if !ok || instruction == "" {
			_ = params.DB.MarkFailed(action.ID, "no prompt_id and no instruction in metadata")
			return nil, errors.New("no prompt_id and no instruction in metadata")
		}
		rendered = instruction
		cfg = prompt.Config{Mode: ModeInteractive}

		if modeStr, ok := promptData.Action.Meta["mode"].(string); ok {
			cfg.Mode = modeStr
		}
	} else {
		lr, err := prompt.Load(params.PromptsDir, action.PromptID)
		if err != nil {
			failMsg := fmt.Sprintf("prompt load error: %v", err)
			_ = params.DB.MarkFailed(action.ID, failMsg)
			CreateInvestigateFailureAction(params.DB, action, failMsg)
			return nil, fmt.Errorf("load prompt %q: %w", action.PromptID, err)
		}

		if len(lr.UnknownFields) > 0 {
			CreateSelfImprovementAction(params.DB, params.PromptsDir, action.PromptID, lr.UnknownFields)
		}

		if len(lr.DeprecatedPatterns) > 0 {
			created, ferr := CreateParseErrorFixAction(params.DB, params.PromptsDir, action.PromptID, lr.DeprecatedPatterns)
			var msg string
			switch {
			case ferr != nil:
				msg = fmt.Sprintf("prompt %q uses deprecated patterns: %v (failed to create fix action: %v)", action.PromptID, lr.DeprecatedPatterns, ferr)
			case created:
				msg = fmt.Sprintf("prompt %q uses deprecated patterns: %v — a fix action has been created", action.PromptID, lr.DeprecatedPatterns)
			default:
				msg = fmt.Sprintf("prompt %q uses deprecated patterns: %v", action.PromptID, lr.DeprecatedPatterns)
			}
			_ = params.DB.MarkFailed(action.ID, msg)
			return nil, fmt.Errorf("prompt %q uses deprecated patterns: %v", action.PromptID, lr.DeprecatedPatterns)
		}

		rendered, err = lr.Prompt.Render(promptData)
		if err != nil {
			failMsg := fmt.Sprintf("render error: %v", err)
			_ = params.DB.MarkFailed(action.ID, failMsg)
			CreateInvestigateFailureAction(params.DB, action, failMsg)
			return nil, fmt.Errorf("render prompt: %w", err)
		}

		cfg = lr.Prompt.Config
	}

	if wt, ok := promptData.Action.Meta["worktree"].(bool); ok {
		cfg.Worktree = wt
	}

	workDir := ResolveWorkDir(promptData)

	if cfg.IsRemote() {
		return executeRemote(ctx, params, action, rendered, cfg, workDir)
	}
	if cfg.IsInteractive() {
		return executeInteractive(ctx, params, action, rendered, cfg, workDir)
	}
	return executeNonInteractive(ctx, params, action, rendered, cfg, workDir)
}

func executeRemote(ctx context.Context, params ExecuteParams, action *db.Action, rendered string, cfg prompt.Config, workDir string) (*ExecuteResult, error) {
	worker := params.RemoteFunc()
	result, err := worker.Execute(ctx, rendered, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		_ = params.DB.MarkFailed(action.ID, err.Error())
		CreateInvestigateFailureAction(params.DB, action, err.Error())
		return nil, &ActionFailedError{ActionID: action.ID, Err: err}
	}

	if err := params.DB.MergeActionMetadata(action.ID, map[string]any{
		"remote_session": result,
	}); err != nil {
		slog.Warn("failed to save remote session info", "action_id", action.ID, "error", err)
	}

	if err := params.DB.MarkDispatched(action.ID); err != nil {
		slog.Warn("failed to mark action as dispatched", "action_id", action.ID, "error", err)
	}

	return &ExecuteResult{Mode: ModeRemote, Output: result}, nil
}

func executeInteractive(ctx context.Context, params ExecuteParams, action *db.Action, rendered string, cfg prompt.Config, workDir string) (*ExecuteResult, error) {
	if params.BeforeInteractive != nil {
		if err := params.BeforeInteractive(action); err != nil {
			if errors.Is(err, ErrInteractiveDeferred) {
				_ = params.DB.ResetToPending(action.ID)
				return nil, ErrInteractiveDeferred
			}
			return nil, err
		}
	}

	worker := params.InteractiveFunc()
	result, err := worker.Execute(ctx, rendered, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		_ = params.DB.MarkFailed(action.ID, err.Error())
		CreateInvestigateFailureAction(params.DB, action, err.Error())
		return nil, &ActionFailedError{ActionID: action.ID, Err: err}
	}

	if params.TmuxSession != "" {
		windowName := WindowName(action.ID)
		if err := params.DB.SetSessionInfo(action.ID, params.TmuxSession, windowName); err != nil {
			slog.Warn("failed to save session info", "action_id", action.ID, "error", err)
		}
	}

	return &ExecuteResult{Mode: ModeInteractive, Output: result}, nil
}

func executeNonInteractive(ctx context.Context, params ExecuteParams, action *db.Action, rendered string, cfg prompt.Config, workDir string) (*ExecuteResult, error) {
	worker := params.NonInteractiveFunc()
	result, err := worker.Execute(ctx, rendered, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		_ = params.DB.MarkFailed(action.ID, err.Error())
		CreateInvestigateFailureAction(params.DB, action, err.Error())
		return nil, &ActionFailedError{ActionID: action.ID, Err: err}
	}

	if err := params.DB.MarkDone(action.ID, result); err != nil {
		return nil, fmt.Errorf("mark done: %w", err)
	}

	if err := TriggerOnDone(params.DB, params.PromptsDir, action, result); err != nil {
		slog.Warn("on_done trigger failed", "action_id", action.ID, "error", err)
	}

	return &ExecuteResult{Mode: ModeNonInteractive, Output: result}, nil
}

func parseMetadata(raw string) (map[string]any, error) {
	m := make(map[string]any)
	if raw == "" || raw == "{}" {
		return m, nil
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// BuildPromptData builds prompt data by looking up the action's task and project from the database.
func BuildPromptData(database db.Store, action *db.Action) (prompt.PromptData, error) {
	var data prompt.PromptData

	actionMeta, err := parseMetadata(action.Metadata)
	if err != nil {
		return data, fmt.Errorf("parse action metadata: %w", err)
	}
	data.Action = prompt.ActionData{
		ID:       action.ID,
		PromptID: action.PromptID,
		Status:   action.Status,
		Meta:     actionMeta,
	}

	task, err := database.GetTask(action.TaskID)
	if err != nil {
		return data, fmt.Errorf("get task: %w", err)
	}
	taskMeta, err := parseMetadata(task.Metadata)
	if err != nil {
		return data, fmt.Errorf("parse task metadata: %w", err)
	}
	data.Task = prompt.TaskData{
		ID:      task.ID,
		Title:   task.Title,
		Status:  task.Status,
		WorkDir: task.WorkDir,
		Meta:    taskMeta,
	}

	project, err := database.GetProjectByID(task.ProjectID)
	if err != nil {
		return data, fmt.Errorf("get project: %w", err)
	}
	projectMeta, err := parseMetadata(project.Metadata)
	if err != nil {
		return data, fmt.Errorf("parse project metadata: %w", err)
	}
	data.Project = prompt.ProjectData{
		ID:      project.ID,
		Name:    project.Name,
		WorkDir: project.WorkDir,
		Meta:    projectMeta,
	}

	return data, nil
}

// ResolveWorkDir returns the effective working directory for prompt execution.
func ResolveWorkDir(data prompt.PromptData) string {
	if data.Task.WorkDir != "" {
		return expandHome(data.Task.WorkDir)
	}
	if data.Project.WorkDir != "" {
		return expandHome(data.Project.WorkDir)
	}
	return "."
}

func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
