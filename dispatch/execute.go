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

// dirExists reports whether the given path exists on disk.
var dirExists = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const (
	ModeRemote         = "remote"
	ModeInteractive    = "interactive"
	ModeNonInteractive = "noninteractive"
	// ModeBg dispatches via `claude --bg`, registering the session with the
	// daemon supervisor so it appears in `claude agents`. The string value is
	// `experimental_bg` during the research-preview phase and will be flipped
	// to `bg` on promotion — keep db/action.go:bgModePredicate in sync.
	ModeBg = "experimental_bg"

	MetaKeyInstruction = "instruction"
	MetaKeyMode        = "mode"
	MetaKeyClaudeArgs  = "claude_args"
	MetaKeyScheduleID  = "schedule_id"
	// MetaKeyPermissionDenials carries the denial summaries reported by
	// `claude -p` for cross-event analysis by the /tq:investigate-incidents
	// skill.
	MetaKeyPermissionDenials = "permission_denials"
	MetaKeyClaudeSessionID   = "claude_session_id"
	MetaKeyParentActionID    = "parent_action_id"
	MetaKeyIsResume          = "is_resume"
	MetaKeyExecutor          = "executor"
	// MetaKeyDaemonShort holds the 8-char short id returned by `claude --bg`
	// for ModeBg actions. Used by the queue worker to poll
	// ~/.claude/jobs/<short>/state.json for lifecycle transitions.
	MetaKeyDaemonShort = "daemon_short"
)

// Executor values for metadata.executor. Distinguishes where the action's
// claude session is actually running, independent of `mode` (which selects
// tq's dispatch worker). Stamped by the SessionStart hook from
// $CLAUDE_CODE_REMOTE, or auto-stamped by `tq action create --status running`
// for cloud-side self-claimed actions (Cloud Routines).
const (
	ExecutorLocal = "local"
	ExecutorCloud = "cloud"
)

// IsCloudExecution reports whether the current process is running inside a
// Claude Code cloud session (Claude Code on the web, including Cloud Routines).
// Anthropic sets CLAUDE_CODE_REMOTE=true in those environments.
func IsCloudExecution() bool {
	return os.Getenv("CLAUDE_CODE_REMOTE") == "true"
}

// DispatchConfig holds shared dispatch settings used by both WorkerConfig and ExecuteParams.
type DispatchConfig struct {
	DB                      db.Store
	NonInteractiveFunc      func() Worker
	InteractiveFunc         func() Worker
	RemoteFunc              func() Worker
	BgFunc                  func() Worker
	TmuxSession             string
	ClaudeSessionLogChecker ClaudeSessionLogChecker
}

type ExecuteParams struct {
	DispatchConfig
	BeforeInteractive    func(action *db.Action) error
	BeforeNonInteractive func(action *db.Action) error
	BeforeBg             func(action *db.Action) error
	// Async, when non-nil, runs noninteractive worker.Execute and its
	// post-processing in a goroutine so the dispatch loop is not blocked.
	// When nil (e.g. in unit tests), execution stays synchronous.
	Async func(func())
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

var (
	ErrInteractiveDeferred    = errors.New("interactive deferred")
	ErrNonInteractiveDeferred = errors.New("noninteractive deferred")
	ErrBgDeferred             = errors.New("bg deferred")
)

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
func (c ActionConfig) IsBg() bool             { return c.Mode == ModeBg }

var validModes = map[string]bool{
	ModeInteractive:    true,
	ModeNonInteractive: true,
	ModeRemote:         true,
	ModeBg:             true,
}

// ValidateActionMode rejects Claude permission-mode values (auto, plan,
// acceptEdits, ...) passed by mistake as tq's mode, which previously caused
// silent fallback to noninteractive. Missing key and empty string are
// treated as unspecified — callers default to interactive.
func ValidateActionMode(meta map[string]any) error {
	raw, ok := meta[MetaKeyMode]
	if !ok {
		return nil
	}
	s, ok := raw.(string)
	if !ok {
		return fmt.Errorf(`metadata "mode" must be a string, got %T`, raw)
	}
	if s == "" {
		return nil
	}
	if !validModes[s] {
		return fmt.Errorf(
			`metadata "mode" must be one of: %s, %s, %s, %s (got %q). `+
				`If you intended Claude permission-mode, use claude_args instead, e.g. `+
				`{"claude_args":["--permission-mode","%s"]}`,
			ModeInteractive, ModeNonInteractive, ModeRemote, ModeBg, s, s,
		)
	}
	return nil
}

// ExecuteAction reads instruction from metadata and dispatches via the appropriate worker.
func ExecuteAction(ctx context.Context, params ExecuteParams, action *db.Action) (*ExecuteResult, error) {
	actionMeta, err := ParseActionMetadata(action.Metadata)
	if err != nil {
		failMsg := fmt.Sprintf("parse action metadata: %v", err)
		if mfErr := params.DB.MarkFailed(action.ID, failMsg); mfErr != nil {
			slog.Error("mark action failed", "action_id", action.ID, "error", mfErr)
		}
		return nil, fmt.Errorf("parse action metadata: %w", err)
	}

	if err := ValidateActionMetadata(actionMeta); err != nil {
		if mfErr := params.DB.MarkFailed(action.ID, err.Error()); mfErr != nil {
			slog.Error("mark action failed", "action_id", action.ID, "error", mfErr)
		}
		return nil, fmt.Errorf("validate action metadata: %w", err)
	}
	instruction := actionMeta[MetaKeyInstruction].(string)

	cfg := ActionConfig{Mode: ModeInteractive}
	if modeStr, ok := actionMeta[MetaKeyMode].(string); ok && modeStr != "" {
		cfg.Mode = modeStr
	}
	if rawArgs, ok := actionMeta[MetaKeyClaudeArgs].([]any); ok {
		cfg.ClaudeArgs = toStringSlice(rawArgs)
	}
	if err := ValidateClaudeArgs(cfg.ClaudeArgs); err != nil {
		failMsg := fmt.Sprintf("validate claude_args: %v", err)
		if mfErr := params.DB.MarkFailed(action.ID, failMsg); mfErr != nil {
			slog.Error("mark action failed", "action_id", action.ID, "error", mfErr)
		}
		return nil, fmt.Errorf("validate claude_args: %w", err)
	}

	if !cfg.IsInteractive() {
		instruction = RenderPrompt(instruction, action.ID, action.TaskID, cfg.Mode)
	}

	workDir, recovery, err := resolveWorkDir(params.DB, action)
	if err != nil {
		slog.Warn("resolve work_dir failed", "action_id", action.ID, "error", err)
	}
	applyWorkDirRecovery(params.DB, recovery)

	if cfg.IsRemote() {
		return executeRemote(ctx, params, action, instruction, cfg, workDir)
	}
	if cfg.IsBg() {
		return executeBg(ctx, params, action, instruction, cfg, workDir)
	}
	if cfg.IsInteractive() {
		return executeInteractive(ctx, params, action, instruction, cfg, workDir)
	}
	return executeNonInteractive(ctx, params, action, instruction, cfg, workDir)
}

func executeBg(ctx context.Context, params ExecuteParams, action *db.Action, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
	if params.BeforeBg != nil {
		if err := params.BeforeBg(action); err != nil {
			if errors.Is(err, ErrBgDeferred) {
				if rpErr := params.DB.DeferToPending(action.ID, defaultDeferBackoff); rpErr != nil {
					return nil, fmt.Errorf("defer to pending for action #%d: %w", action.ID, rpErr)
				}
				return nil, ErrBgDeferred
			}
			return nil, err
		}
	}

	worker := params.BgFunc()
	short, err := worker.Execute(ctx, instruction, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		if mfErr := params.DB.MarkFailed(action.ID, err.Error()); mfErr != nil {
			slog.Error("mark action failed", "action_id", action.ID, "error", mfErr)
		}
		return nil, &ActionFailedError{ActionID: action.ID, Err: err}
	}

	if err := params.DB.MergeActionMetadata(action.ID, map[string]any{
		MetaKeyDaemonShort: short,
	}); err != nil {
		slog.Warn("failed to save bg daemon_short", "action_id", action.ID, "error", err)
	}

	return &ExecuteResult{Mode: ModeBg, Output: short}, nil
}

func executeRemote(ctx context.Context, params ExecuteParams, action *db.Action, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
	worker := params.RemoteFunc()
	result, err := worker.Execute(ctx, instruction, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		if mfErr := params.DB.MarkFailed(action.ID, err.Error()); mfErr != nil {
			slog.Error("mark action failed", "action_id", action.ID, "error", mfErr)
		}
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
				if rpErr := params.DB.DeferToPending(action.ID, defaultDeferBackoff); rpErr != nil {
					return nil, fmt.Errorf("defer to pending for action #%d: %w", action.ID, rpErr)
				}
				return nil, ErrInteractiveDeferred
			}
			return nil, err
		}
	}

	worker := params.InteractiveFunc()
	result, err := worker.Execute(ctx, instruction, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		if mfErr := params.DB.MarkFailed(action.ID, err.Error()); mfErr != nil {
			slog.Error("mark action failed", "action_id", action.ID, "error", mfErr)
		}
		return nil, &ActionFailedError{ActionID: action.ID, Err: err}
	}

	if params.TmuxSession != "" {
		windowName := WindowName(action.ID)
		if err := params.DB.SetTmuxInfo(action.ID, params.TmuxSession, windowName); err != nil {
			slog.Warn("failed to save tmux info", "action_id", action.ID, "error", err)
		}
	}

	return &ExecuteResult{Mode: ModeInteractive, Output: result}, nil
}

func executeNonInteractive(ctx context.Context, params ExecuteParams, action *db.Action, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
	if params.BeforeNonInteractive != nil {
		if err := params.BeforeNonInteractive(action); err != nil {
			if errors.Is(err, ErrNonInteractiveDeferred) {
				if rpErr := params.DB.DeferToPending(action.ID, defaultDeferBackoff); rpErr != nil {
					return nil, fmt.Errorf("defer to pending for action #%d: %w", action.ID, rpErr)
				}
				return nil, ErrNonInteractiveDeferred
			}
			return nil, err
		}
	}

	worker := params.NonInteractiveFunc()

	if params.Async == nil {
		// Synchronous path: keeps existing ExecuteAction unit tests deterministic.
		return runNonInteractive(ctx, params, action, worker, instruction, cfg, workDir)
	}

	// Async path: dispatch loop continues immediately so other actions can be
	// dispatched while claude -p runs. Errors are logged and DB is updated
	// inside the goroutine.
	actionCopy := *action
	params.Async(func() {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("dispatch goroutine panic: %v", r)
				if mfErr := params.DB.MarkFailed(actionCopy.ID, msg); mfErr != nil {
					slog.Error("mark action failed", "action_id", actionCopy.ID, "error", mfErr)
				}
				slog.Error("dispatch goroutine panic recovered", "action_id", actionCopy.ID, "error", r)
			}
		}()
		_, _ = runNonInteractive(ctx, params, &actionCopy, worker, instruction, cfg, workDir)
	})
	return &ExecuteResult{Mode: ModeNonInteractive}, nil
}

// runNonInteractive returns (nil, *ActionFailedError) on worker failure so
// the synchronous path preserves caller-visible behavior; the async path
// swallows the error after logging.
func runNonInteractive(ctx context.Context, params ExecuteParams, action *db.Action, worker Worker, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
	result, err := worker.Execute(ctx, instruction, cfg, workDir, action.ID, action.TaskID)
	if err != nil {
		// Shutdown cancel: leave in-flight running so the next dispatch
		// cycle's stale reaper can either find them alive (heartbeat fresh)
		// or reap them legitimately.
		if errors.Is(err, context.Canceled) {
			slog.Warn("noninteractive interrupted by shutdown; leaving for reaper", "action_id", action.ID)
			return nil, err
		}
		if mfErr := params.DB.MarkFailed(action.ID, err.Error()); mfErr != nil {
			slog.Error("mark action failed", "action_id", action.ID, "error", mfErr)
		}
		return nil, &ActionFailedError{ActionID: action.ID, Err: err}
	}

	if p, ok := worker.(interface{ LastDenials() []PermissionDenial }); ok {
		if denials := p.LastDenials(); len(denials) > 0 {
			if err := params.DB.MergeActionMetadata(action.ID, map[string]any{
				MetaKeyPermissionDenials: denialSummaries(denials),
			}); err != nil {
				slog.Error("merge permission_denials metadata", "action_id", action.ID, "error", err)
			}
		}
	}

	if err := params.DB.MarkDone(action.ID, result); err != nil {
		slog.Error("mark done", "action_id", action.ID, "error", err)
		return nil, fmt.Errorf("mark done: %w", err)
	}

	return &ExecuteResult{Mode: ModeNonInteractive, Output: result}, nil
}

// RenderPrompt builds the wrapped claude prompt for an action: the user-provided
// instruction followed by a postamble that injects tq action context (action ID,
// required first step, /tq:done /tq:failed selector). Shared by Go-side dispatch
// (noninteractive/remote) and the `tq action prompt` CLI subcommand so both
// callers produce byte-identical output.
func RenderPrompt(instruction string, actionID, taskID int64, mode string) string {
	var b strings.Builder
	b.WriteString(instruction)
	b.WriteString("\n\n---\n\n## tq action context\n\n")
	fmt.Fprintf(&b, "You are executing **action #%d** (task #%d).\n", actionID, taskID)

	fmt.Fprintf(&b,
		"\n**Required first step (do not skip):** run `tq action list --task %d` to load the task history — completed actions, their outcomes, and the decisions that led here. Skipping this step leads to duplicated or contradictory work.\n",
		taskID,
	)

	if mode != ModeRemote {
		b.WriteString("\n**When the action terminates, choose exactly one:**\n\n")
		b.WriteString("- **Completed** — run `/tq:done` to record the result.\n")
		b.WriteString("- **Blocked** (missing permissions, broken environment, external blocker, etc.) — run `/tq:failed` instead.\n")
	}

	return b.String()
}

func ValidateActionMetadata(meta map[string]any) error {
	inst, ok := meta[MetaKeyInstruction].(string)
	if !ok || strings.TrimSpace(inst) == "" {
		return errors.New("metadata must contain a non-empty \"instruction\" field")
	}
	if err := ValidateActionMode(meta); err != nil {
		return err
	}
	return nil
}

// ParseActionMetadata decodes the JSON metadata blob stored on an action,
// short-circuiting empty / "{}" payloads to avoid an Unmarshal of an empty map.
func ParseActionMetadata(raw string) (map[string]any, error) {
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
//
// Priority: action.WorkDir → task.WorkDir → project.WorkDir → ".".
// A missing action.WorkDir is logged but never auto-cleared from the DB —
// the per-action override is treated as explicit user intent.
func resolveWorkDir(database db.Store, action *db.Action) (string, *workDirRecovery, error) {
	if action.WorkDir != "" {
		expanded := expandHome(action.WorkDir)
		if dirExists(expanded) {
			return expanded, nil, nil
		}
		slog.Warn("action work_dir does not exist, falling back to task chain",
			"action_id", action.ID,
			"missing_path", expanded,
		)
	}

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
