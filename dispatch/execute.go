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

	MetaKeyInstruction       = "instruction"
	MetaKeyMode              = "mode"
	MetaKeyClaudeArgs        = "claude_args"
	MetaKeyScheduleID        = "schedule_id"
	MetaKeyPermissionDenials = "permission_denials"
	MetaKeyClaudeSessionID   = "claude_session_id"
	MetaKeyParentActionID    = "parent_action_id"
	MetaKeyIsResume          = "is_resume"
	MetaKeyExecutor          = "executor"
	MetaKeyDaemonShort       = "daemon_short"
	MetaKeyRemoteSession     = "remote_session"
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
	DB         db.Store
	BgFunc     func() Worker
	RemoteFunc func() Worker
}

type ExecuteParams struct {
	DispatchConfig
	BeforeInteractive    func(action *db.Action) error
	BeforeNonInteractive func(action *db.Action) error
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
)

// ActionConfig holds execution configuration extracted from action metadata.
type ActionConfig struct {
	Mode       string
	ClaudeArgs []string
}

func (c ActionConfig) IsInteractive() bool    { return c.Mode == ModeInteractive }
func (c ActionConfig) IsNonInteractive() bool { return c.Mode == ModeNonInteractive }
func (c ActionConfig) IsRemote() bool         { return c.Mode == ModeRemote }

// validModeList is the canonical ordered set of tq dispatch modes; validModes
// is derived from it. ValidModesList renders it for error messages so adding a
// mode updates every message in one place.
var validModeList = []string{ModeInteractive, ModeNonInteractive, ModeRemote}

var validModes = func() map[string]bool {
	m := make(map[string]bool, len(validModeList))
	for _, mode := range validModeList {
		m[mode] = true
	}
	return m
}()

// SettingKeyDefaultMode is the global setting key for the default dispatch
// mode. Keep in sync with db.SettingDefaultMode (db cannot import dispatch and
// dispatch cannot import db, so the constant is duplicated by layer).
const SettingKeyDefaultMode = "default_mode"

// IsValidMode reports whether s is one of tq's dispatch modes.
func IsValidMode(s string) bool { return validModes[s] }

// ValidModesList returns the valid modes as a comma-separated string for
// error messages, e.g. "interactive, noninteractive, remote".
func ValidModesList() string { return strings.Join(validModeList, ", ") }

// ResolveDefaultMode decides the mode to stamp into a new action's metadata
// when --meta did not specify one. It returns "" when nothing should be
// stamped (an explicit mode is already present, or no global default is
// configured) — the caller leaves metadata untouched and dispatch falls back
// to ModeInteractive. A non-empty globalDefault that is not a valid mode is
// an error so a misconfigured setting fails action creation loudly instead of
// silently falling back.
func ResolveDefaultMode(actionMeta map[string]any, globalDefault string) (string, error) {
	if s, ok := actionMeta[MetaKeyMode].(string); ok && s != "" {
		return "", nil
	}
	if globalDefault == "" {
		return "", nil
	}
	if !IsValidMode(globalDefault) {
		return "", fmt.Errorf(
			`configured default mode %q is invalid: must be one of %s `+
				`(fix with 'tq config set %s <mode>')`,
			globalDefault, ValidModesList(), SettingKeyDefaultMode,
		)
	}
	return globalDefault, nil
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
			`metadata "mode" must be one of: %s (got %q). `+
				`If you intended Claude permission-mode, use claude_args instead, e.g. `+
				`{"claude_args":["--permission-mode","%s"]}`,
			ValidModesList(), s, s,
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

	instruction = RenderPrompt(instruction, action.ID, action.TaskID, cfg.Mode)

	workDir, recovery, err := resolveWorkDir(params.DB, action)
	if err != nil {
		slog.Warn("resolve work_dir failed", "action_id", action.ID, "error", err)
	}
	applyWorkDirRecovery(params.DB, recovery)

	if cfg.IsRemote() {
		return executeRemote(ctx, params, action, instruction, cfg, workDir)
	}
	return executeViaBg(ctx, params, action, instruction, cfg, workDir)
}

func deferOrFail(store db.Store, actionID int64, sentinel error) error {
	if rpErr := store.DeferToPending(actionID, defaultDeferBackoff); rpErr != nil {
		failMsg := fmt.Sprintf("defer to pending failed: %v", rpErr)
		if mfErr := store.MarkFailed(actionID, failMsg); mfErr != nil {
			slog.Error("mark action failed", "action_id", actionID, "error", mfErr)
		}
		return &ActionFailedError{ActionID: actionID, Err: rpErr}
	}
	return sentinel
}

func executeViaBg(ctx context.Context, params ExecuteParams, action *db.Action, instruction string, cfg ActionConfig, workDir string) (*ExecuteResult, error) {
	before, deferErr := admissionFor(params, cfg)
	if before != nil {
		if err := before(action); err != nil {
			if !errors.Is(err, deferErr) {
				slog.Warn("admission check errored, deferring action", "action_id", action.ID, "error", err)
			}
			return nil, deferOrFail(params.DB, action.ID, deferErr)
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

	return &ExecuteResult{Mode: cfg.Mode, Output: short}, nil
}

func admissionFor(params ExecuteParams, cfg ActionConfig) (func(*db.Action) error, error) {
	if cfg.IsNonInteractive() {
		return params.BeforeNonInteractive, ErrNonInteractiveDeferred
	}
	return params.BeforeInteractive, ErrInteractiveDeferred
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
		MetaKeyRemoteSession: result,
	}); err != nil {
		slog.Warn("failed to save remote session info", "action_id", action.ID, "error", err)
	}

	if err := params.DB.MarkDispatched(action.ID); err != nil {
		slog.Warn("failed to mark action as dispatched", "action_id", action.ID, "error", err)
	}

	return &ExecuteResult{Mode: ModeRemote, Output: result}, nil
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
	emptyWorkDir := ""
	if err := database.UpdateTaskFields(recovery.TaskID, db.TaskFieldChanges{WorkDir: &emptyWorkDir}); err != nil {
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

func isTqModeName(s string) bool {
	return validModes[s] || s == "experimental_bg"
}

func ValidateClaudeArgs(args []string) error {
	for i, arg := range args {
		if blockedClaudeArgs[arg] {
			return fmt.Errorf("claude_args cannot include %q (managed by tq internally)", arg)
		}
		if arg == "--permission-mode" && i+1 < len(args) && isTqModeName(args[i+1]) {
			return fmt.Errorf(
				`claude_args has --permission-mode %q, but %q is a tq dispatch mode (metadata "mode"), not a Claude permission-mode value. `+
					`Valid Claude permission-mode values: acceptEdits, auto, bypassPermissions, default, dontAsk, plan`,
				args[i+1], args[i+1],
			)
		}
	}
	return nil
}
