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
	"time"

	"github.com/MH4GF/tq/db"
)

const postExecutionFreshness = 5 * time.Minute

// dirExists reports whether the given path exists on disk.
var dirExists = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const (
	ModeRemote         = "remote"
	ModeInteractive    = "interactive"
	ModeNonInteractive = "noninteractive"

	MetaKeyInstruction       = "instruction"
	MetaKeyMode              = "mode"
	MetaKeyClaudeArgs        = "claude_args"
	MetaKeyScheduleID        = "schedule_id"
	MetaKeyIsInvestigation   = "is_investigate_failure"
	MetaKeyFailedActionID    = "failed_action_id"
	MetaKeyIsPermissionBlock = "is_permission_block"
	MetaKeyBlockedActionID   = "blocked_action_id"
	MetaKeyClaudeSessionID   = "claude_session_id"
)

// DispatchConfig holds shared dispatch settings used by both WorkerConfig and ExecuteParams.
type DispatchConfig struct {
	DB                 db.Store
	NonInteractiveFunc func() Worker
	InteractiveFunc    func() Worker
	RemoteFunc         func() Worker
	TmuxSession        string
	SessionLogChecker  SessionLogChecker
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
	Mode       string
	ClaudeArgs []string
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
	if rawArgs, ok := actionMeta[MetaKeyClaudeArgs].([]any); ok {
		cfg.ClaudeArgs = toStringSlice(rawArgs)
	}
	if err := ValidateClaudeArgs(cfg.ClaudeArgs); err != nil {
		failMsg := fmt.Sprintf("validate claude_args: %v", err)
		_ = params.DB.MarkFailed(action.ID, failMsg)
		return nil, fmt.Errorf("validate claude_args: %w", err)
	}

	instruction = wrapInstruction(instruction, action.ID, action.TaskID, cfg.Mode)

	workDir, recovery, err := resolveWorkDir(params.DB, action)
	if err != nil {
		slog.Warn("resolve work_dir failed", "action_id", action.ID, "error", err)
	}
	applyWorkDirRecovery(params.DB, recovery)

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

	// The worker polling loop cannot discover session ID during synchronous execution.
	saveSessionID(params.DB, params.SessionLogChecker, action.ID, workDir)

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

func saveSessionID(store db.Store, checker SessionLogChecker, actionID int64, workDir string) {
	if checker == nil {
		return
	}
	active, sessionID, err := checker.IsSessionActive(workDir, postExecutionFreshness)
	if err != nil {
		slog.Warn("post-execution session log check failed", "action_id", actionID, "error", err)
		return
	}
	if !active || sessionID == "" {
		return
	}
	if err := store.MergeActionMetadata(actionID, map[string]any{
		MetaKeyClaudeSessionID: sessionID,
	}); err != nil {
		slog.Warn("failed to save session id to metadata", "action_id", actionID, "error", err)
	}
}

func wrapInstruction(instruction string, actionID, taskID int64, mode string) string {
	postamble := fmt.Sprintf("\n\nYou are executing action #%d (task #%d).\n\n", actionID, taskID)
	postamble += fmt.Sprintf("First, run `tq action list --task %d` to understand the task history (completed actions, their results, etc.).", taskID)
	if mode != ModeRemote {
		postamble += "\n\nWhen you finish, run `/tq:done` to mark this action as complete." +
			"\nIf you cannot complete the action (missing permissions, broken environment, external blocker, etc.), run `/tq:failed` instead."
	}
	return instruction + postamble
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

// workDirRecovery describes a fallback that resolveWorkDir applied when the
// task's recorded work_dir was missing on disk. Pass to applyWorkDirRecovery
// to commit the corresponding DB correction.
type workDirRecovery struct {
	TaskID      int64
	MissingPath string
	Fallback    string
}

// resolveWorkDir returns the effective working directory for action execution.
// It is read-only: when the task's work_dir does not exist on disk, it returns
// a non-nil recovery descriptor rather than writing to the DB. Callers that
// want the auto-correction must invoke applyWorkDirRecovery explicitly.
func resolveWorkDir(database db.Store, action *db.Action) (string, *workDirRecovery, error) {
	task, err := database.GetTask(action.TaskID)
	if err != nil {
		return ".", nil, fmt.Errorf("get task: %w", err)
	}

	if task.WorkDir != "" {
		expanded := expandHome(task.WorkDir)
		if dirExists(expanded) {
			return expanded, nil, nil
		}

		project, err := database.GetProjectByID(task.ProjectID)
		if err != nil {
			return ".", nil, fmt.Errorf("get project for fallback: %w", err)
		}

		fallback := "."
		if project.WorkDir != "" {
			projExpanded := expandHome(project.WorkDir)
			if dirExists(projExpanded) {
				fallback = projExpanded
			}
		}

		return fallback, &workDirRecovery{
			TaskID:      task.ID,
			MissingPath: expanded,
			Fallback:    fallback,
		}, nil
	}

	project, err := database.GetProjectByID(task.ProjectID)
	if err != nil {
		return ".", nil, fmt.Errorf("get project: %w", err)
	}
	if project.WorkDir != "" {
		return expandHome(project.WorkDir), nil, nil
	}
	return ".", nil, nil
}

// applyWorkDirRecovery clears the task's stale work_dir so future dispatches
// resolve via the project's work_dir naturally. Safe to call with nil.
func applyWorkDirRecovery(database db.Store, recovery *workDirRecovery) {
	if recovery == nil {
		return
	}
	slog.Warn("work_dir auto-recovery: task work_dir does not exist, falling back",
		"task_id", recovery.TaskID,
		"missing_path", recovery.MissingPath,
		"fallback_path", recovery.Fallback,
	)
	if err := database.UpdateTaskWorkDir(recovery.TaskID, ""); err != nil {
		slog.Warn("work_dir auto-recovery: failed to clear task work_dir",
			"task_id", recovery.TaskID,
			"error", err,
		)
	}
}

func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func toStringSlice(raw []any) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

var blockedClaudeArgs = map[string]bool{
	"-p":              true,
	"--print":         true,
	"--output-format": true,
	"--remote":        true,
}

func ValidateClaudeArgs(args []string) error {
	for _, arg := range args {
		if blockedClaudeArgs[arg] {
			return fmt.Errorf("claude_args cannot include %q (managed by tq internally)", arg)
		}
	}
	return nil
}
