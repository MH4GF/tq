package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/MH4GF/tq/db"
)

const (
	ModeRemote         = "remote"
	ModeInteractive    = "interactive"
	ModeNonInteractive = "noninteractive"

	MetaKeyInstruction       = "instruction"
	MetaKeyMode              = "mode"
	MetaKeyPermissionMode    = "permission_mode"
	MetaKeyScheduleID        = "schedule_id"
	MetaKeyIsInvestigation   = "is_investigate_failure"
	MetaKeyFailedActionID    = "failed_action_id"
	MetaKeyIsPermissionBlock = "is_permission_block"
	MetaKeyBlockedActionID   = "blocked_action_id"
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

// ActionConfig holds execution configuration extracted from action metadata.
type ActionConfig struct {
	Mode           string
	PermissionMode string
	Worktree       bool
}

func (c ActionConfig) IsInteractive() bool    { return c.Mode == ModeInteractive }
func (c ActionConfig) IsNonInteractive() bool { return c.Mode == ModeNonInteractive }
func (c ActionConfig) IsRemote() bool         { return c.Mode == ModeRemote }

// ExecuteAction reads instruction from metadata and dispatches via the appropriate worker.
func ExecuteAction(ctx context.Context, params ExecuteParams, action *db.Action) (*ExecuteResult, error) {
	actionMeta, err := parseMetadata(action.Metadata)
	if err != nil {
		failMsg := fmt.Sprintf("parse action metadata: %v", err)
		_ = params.DB.MarkFailed(action.ID, failMsg)
		CreateInvestigateFailureAction(params.DB, action, failMsg)
		return nil, fmt.Errorf("parse action metadata: %w", err)
	}

	if err := ValidateActionMetadata(actionMeta); err != nil {
		_ = params.DB.MarkFailed(action.ID, err.Error())
		return nil, fmt.Errorf("validate action metadata: %w", err)
	}
	instruction := actionMeta[MetaKeyInstruction].(string)

	cfg := ActionConfig{Mode: ModeInteractive}
	if modeStr, ok := actionMeta[MetaKeyMode].(string); ok {
		cfg.Mode = modeStr
	}
	if permMode, ok := actionMeta[MetaKeyPermissionMode].(string); ok {
		cfg.PermissionMode = permMode
	}
	if wt, ok := actionMeta["worktree"].(bool); ok {
		cfg.Worktree = wt
	}

	instruction = wrapInstruction(instruction, action.ID, action.TaskID, cfg.Mode)

	workDir := resolveWorkDir(params.DB, action)

	if cfg.IsRemote() {
		return executeRemote(ctx, params, action, instruction, cfg, workDir)
	}
	if cfg.IsInteractive() {
		return executeInteractive(ctx, params, action, instruction, cfg, workDir)
	}
	return executeNonInteractive(ctx, params, action, instruction, cfg, workDir)
}

func executeRemote(ctx context.Context, params ExecuteParams, action *db.Action, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
	worker := params.RemoteFunc()
	result, err := worker.Execute(ctx, instruction, cfg, workDir, action.ID, action.TaskID)
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

func executeInteractive(ctx context.Context, params ExecuteParams, action *db.Action, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
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
	result, err := worker.Execute(ctx, instruction, cfg, workDir, action.ID, action.TaskID)
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

func executeNonInteractive(ctx context.Context, params ExecuteParams, action *db.Action, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
	worker := params.NonInteractiveFunc()
	result, err := worker.Execute(ctx, instruction, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		_ = params.DB.MarkFailed(action.ID, err.Error())
		CreateInvestigateFailureAction(params.DB, action, err.Error())
		return nil, &ActionFailedError{ActionID: action.ID, Err: err}
	}

	if p, ok := worker.(interface{ LastDenials() []PermissionDenial }); ok {
		if denials := p.LastDenials(); len(denials) > 0 {
			CreatePermissionBlockAction(params.DB, action, denials)
		}
	}

	if err := params.DB.MarkDone(action.ID, result); err != nil {
		return nil, fmt.Errorf("mark done: %w", err)
	}

	return &ExecuteResult{Mode: ModeNonInteractive, Output: result}, nil
}

func wrapInstruction(instruction string, actionID, taskID int64, mode string) string {
	preamble := fmt.Sprintf("You are executing action #%d (task #%d).\n\n", actionID, taskID)
	preamble += fmt.Sprintf("First, run `tq action list --task %d` to understand the task history (completed actions, their results, etc.).\n\n", taskID)
	result := preamble + instruction
	if mode != ModeRemote {
		result += "\n\nWhen you finish, run `/tq:done` to mark this action as complete." +
			"\nIf you cannot complete the action (missing permissions, broken environment, external blocker, etc.), run `/tq:failed` instead."
	}
	return result
}

func ValidateActionMetadata(meta map[string]any) error {
	inst, ok := meta[MetaKeyInstruction].(string)
	if !ok || strings.TrimSpace(inst) == "" {
		return errors.New("metadata must contain a non-empty \"instruction\" field")
	}
	return nil
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

// resolveWorkDir returns the effective working directory for action execution.
func resolveWorkDir(database db.Store, action *db.Action) string {
	task, err := database.GetTask(action.TaskID)
	if err != nil {
		return "."
	}
	if task.WorkDir != "" {
		return expandHome(task.WorkDir)
	}
	project, err := database.GetProjectByID(task.ProjectID)
	if err != nil {
		return "."
	}
	if project.WorkDir != "" {
		return expandHome(project.WorkDir)
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
