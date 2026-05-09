package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/MH4GF/tq/db"
)

const (
	DefaultMaxInteractive         = 3
	DefaultMaxNonInteractive      = 5
	DefaultStaleThreshold         = 30 * time.Second
	DefaultPollInterval           = 10 * time.Second
	DefaultStaleGracePeriod       = 30 * time.Second
	DefaultHeartbeatFreshness     = 120 * time.Second
	DefaultInteractiveHardTimeout = 1 * time.Hour
	// Must exceed init-hook duration (worktree setup, migrations), not just
	// claude startup — hooks run before any session log is written.
	DefaultEarlyDispatchTimeout = 5 * time.Minute
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
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// WorkerConfig configures the queue worker.
type WorkerConfig struct {
	DispatchConfig
	MaxInteractive         int
	MaxNonInteractive      int
	PollInterval           time.Duration
	TmuxChecker            TmuxChecker
	StaleGracePeriod       time.Duration
	HeartbeatFreshness     time.Duration
	InteractiveHardTimeout time.Duration
	EarlyDispatchTimeout   time.Duration
	// async, when non-nil, is used to launch noninteractive worker.Execute
	// in the background so the dispatch loop is not blocked. RunWorker sets
	// this internally before calling dispatchOne; tests usually leave it nil
	// (synchronous path).
	async func(func())
}

// RunWorker continuously dispatches pending actions.
// It processes one action per iteration, sleeping when idle.
func RunWorker(ctx context.Context, cfg WorkerConfig) error {
	if cfg.MaxInteractive <= 0 {
		cfg.MaxInteractive = DefaultMaxInteractive
	}
	if cfg.MaxNonInteractive <= 0 {
		cfg.MaxNonInteractive = DefaultMaxNonInteractive
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.StaleGracePeriod <= 0 {
		cfg.StaleGracePeriod = DefaultStaleGracePeriod
	}
	if cfg.TmuxSession == "" {
		cfg.TmuxSession = "main"
	}
	if cfg.HeartbeatFreshness <= 0 {
		cfg.HeartbeatFreshness = DefaultHeartbeatFreshness
	}
	if cfg.InteractiveHardTimeout <= 0 {
		cfg.InteractiveHardTimeout = DefaultInteractiveHardTimeout
	}
	if cfg.EarlyDispatchTimeout <= 0 {
		cfg.EarlyDispatchTimeout = DefaultEarlyDispatchTimeout
	}

	slog.Info("queue worker started",
		"max_interactive", cfg.MaxInteractive,
		"max_noninteractive", cfg.MaxNonInteractive,
		"poll_interval", cfg.PollInterval,
	)

	var wg sync.WaitGroup
	cfg.async = func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("dispatch goroutine panic recovered", "error", r)
				}
			}()
			fn()
		}()
	}
	defer wg.Wait()

	var lastHeartbeat time.Time
	for {
		select {
		case <-ctx.Done():
			slog.Info("queue worker stopped")
			return ctx.Err()
		default:
		}

		if time.Since(lastHeartbeat) >= cfg.PollInterval {
			if err := cfg.DB.UpdateWorkerHeartbeat(cfg.MaxInteractive); err != nil {
				slog.Error("update worker heartbeat", "error", err)
			}
			lastHeartbeat = time.Now()
		}

		reapStaleActions(ctx, cfg)

		if err := CheckSchedules(cfg.DB, time.Now()); err != nil {
			slog.Error("schedule check error", "error", err)
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

func reapStaleActions(ctx context.Context, cfg WorkerConfig) {
	now := time.Now()

	if cfg.ClaudeSessionLogChecker != nil || cfg.TmuxChecker != nil {
		reapInteractive(ctx, cfg, now)
	}
	reapNonInteractive(cfg, now)
}

// reapInteractive scans running interactive actions and bulk-marks the stale
// ones failed. The classify pass is in-memory only (no Store calls); workDir
// is resolved from prefetched task/project maps so the loop body never touches
// the database. Bulk mark failed runs once after classification.
func reapInteractive(ctx context.Context, cfg WorkerConfig, now time.Time) {
	actions, err := cfg.DB.ListRunningInteractive()
	if err != nil {
		slog.Error("list running interactive for stale check", "error", err)
		return
	}
	if len(actions) == 0 {
		return
	}

	taskMap, projectMap, err := prefetchWorkDirContext(cfg.DB, actions)
	if err != nil {
		slog.Error("prefetch work_dir context for interactive reaper", "error", err)
		return
	}

	var windowSet map[string]struct{}
	if cfg.TmuxChecker != nil {
		windows, err := cfg.TmuxChecker.ListWindows(ctx, cfg.TmuxSession)
		if err != nil {
			slog.Warn("tmux list-windows failed", "error", err)
		} else {
			windowSet = make(map[string]struct{}, len(windows))
			for _, w := range windows {
				windowSet[w] = struct{}{}
			}
		}
	}

	var failures []db.ActionFailureUpdate
	sessionPatches := make(map[int64]map[string]any)
	for _, a := range actions {
		if MetadataHasValue(a.Metadata, MetaKeyExecutor, ExecutorCloud) {
			continue
		}
		var startedAt time.Time
		if a.StartedAt.Valid {
			if s, err := time.Parse(db.TimeLayout, a.StartedAt.String); err == nil {
				startedAt = s
				if now.Sub(startedAt) < cfg.StaleGracePeriod {
					continue
				}
			}
		}

		workDir := workDirFromMaps(&a, taskMap, projectMap)

		if reason, decided := classifyEarlyStale(cfg, &a, workDir, startedAt, now); decided {
			if reason != "" {
				failures = append(failures, db.ActionFailureUpdate{ID: a.ID, Reason: reason})
				slog.Warn("reaped early-stale action", "action_id", a.ID, "elapsed", now.Sub(startedAt))
			}
			continue
		}

		active, sessionID := claudeSessionStillActive(cfg, workDir, cfg.HeartbeatFreshness)
		if active {
			collectSessionPatch(sessionPatches, &a, sessionID)
			slog.Info("action claude session log is fresh, skipping stale check", "action_id", a.ID)
			continue
		}

		if windowSet == nil {
			if !startedAt.IsZero() && now.Sub(startedAt) >= cfg.InteractiveHardTimeout {
				reason := fmt.Sprintf("stale: tmux unavailable and action exceeded hard timeout (%v)", cfg.InteractiveHardTimeout)
				failures = append(failures, db.ActionFailureUpdate{ID: a.ID, Reason: reason})
				slog.Warn("reaped stale action via hard timeout", "action_id", a.ID, "hard_timeout", cfg.InteractiveHardTimeout)
			}
			continue
		}
		if _, exists := windowSet[WindowName(a.ID)]; exists {
			continue
		}
		reason := fmt.Sprintf("stale: session log not fresh and tmux window %q no longer exists", WindowName(a.ID))
		failures = append(failures, db.ActionFailureUpdate{ID: a.ID, Reason: reason})
		slog.Warn("reaped stale action", "action_id", a.ID)
	}

	if len(failures) > 0 {
		if err := cfg.DB.BulkMarkFailed(failures); err != nil {
			slog.Error("bulk mark stale interactive actions failed", "error", err, "count", len(failures))
		}
	}
	if len(sessionPatches) > 0 {
		if err := cfg.DB.BulkMergeActionMetadata(sessionPatches); err != nil {
			slog.Warn("bulk merge claude_session_id patches failed", "error", err, "count", len(sessionPatches))
		}
	}
}

// reapNonInteractive mirrors reapInteractive but uses a simple time-based
// timeout fallback instead of the tmux window check.
func reapNonInteractive(cfg WorkerConfig, now time.Time) {
	niActions, err := cfg.DB.ListRunningNonInteractive()
	if err != nil {
		slog.Error("list running noninteractive for stale check", "error", err)
		return
	}
	if len(niActions) == 0 {
		return
	}

	taskMap, projectMap, err := prefetchWorkDirContext(cfg.DB, niActions)
	if err != nil {
		slog.Error("prefetch work_dir context for noninteractive reaper", "error", err)
		return
	}

	staleThreshold := time.Duration(defaultTimeout*nonInteractiveStaleMultiplier) * time.Second

	var failures []db.ActionFailureUpdate
	sessionPatches := make(map[int64]map[string]any)
	for _, a := range niActions {
		if MetadataHasValue(a.Metadata, MetaKeyExecutor, ExecutorCloud) {
			continue
		}
		if !a.StartedAt.Valid {
			continue
		}
		started, err := time.Parse(db.TimeLayout, a.StartedAt.String)
		if err != nil {
			continue
		}
		if now.Sub(started) < staleThreshold {
			continue
		}

		workDir := workDirFromMaps(&a, taskMap, projectMap)
		active, sessionID := claudeSessionStillActive(cfg, workDir, cfg.HeartbeatFreshness)
		if active {
			collectSessionPatch(sessionPatches, &a, sessionID)
			slog.Info("noninteractive action claude session log is fresh, skipping stale check", "action_id", a.ID)
			continue
		}

		reason := fmt.Sprintf("stale: noninteractive action exceeded timeout (%v)", staleThreshold)
		failures = append(failures, db.ActionFailureUpdate{ID: a.ID, Reason: reason})
		slog.Warn("reaped stale noninteractive action", "action_id", a.ID)
	}

	if len(failures) > 0 {
		if err := cfg.DB.BulkMarkFailed(failures); err != nil {
			slog.Error("bulk mark stale noninteractive actions failed", "error", err, "count", len(failures))
		}
	}
	if len(sessionPatches) > 0 {
		if err := cfg.DB.BulkMergeActionMetadata(sessionPatches); err != nil {
			slog.Warn("bulk merge claude_session_id patches (NI) failed", "error", err, "count", len(sessionPatches))
		}
	}
}

// collectSessionPatch records a discovered claude_session_id for later bulk
// metadata merge, mirroring the dedupe check from the original per-action
// MergeActionMetadata path so we don't re-write metadata that already has the
// expected session id.
func collectSessionPatch(out map[int64]map[string]any, a *db.Action, sessionID string) {
	if sessionID == "" {
		return
	}
	if MetadataHasValue(a.Metadata, MetaKeyClaudeSessionID, sessionID) {
		return
	}
	out[a.ID] = map[string]any{MetaKeyClaudeSessionID: sessionID}
}

// prefetchWorkDirContext bulk-fetches every task and project referenced by the
// given actions so the reaper classify loop can resolve work_dir without per-
// action DB calls. Returns empty maps on empty input.
func prefetchWorkDirContext(database db.Store, actions []db.Action) (map[int64]*db.Task, map[int64]*db.Project, error) {
	taskIDSet := make(map[int64]struct{}, len(actions))
	for _, a := range actions {
		taskIDSet[a.TaskID] = struct{}{}
	}
	taskIDs := make([]int64, 0, len(taskIDSet))
	for id := range taskIDSet {
		taskIDs = append(taskIDs, id)
	}
	taskMap, err := database.GetTasksByIDs(taskIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get tasks: %w", err)
	}

	projectIDSet := make(map[int64]struct{}, len(taskMap))
	for _, t := range taskMap {
		projectIDSet[t.ProjectID] = struct{}{}
	}
	projectIDs := make([]int64, 0, len(projectIDSet))
	for id := range projectIDSet {
		projectIDs = append(projectIDs, id)
	}
	projectMap, err := database.GetProjectsByIDs(projectIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("get projects: %w", err)
	}
	return taskMap, projectMap, nil
}

// workDirFromMaps replays the read-only resolveWorkDir logic against prefetched
// maps. It returns "." when the relevant task/project is missing or has no
// work_dir set, matching the original fallback chain.
func workDirFromMaps(a *db.Action, taskMap map[int64]*db.Task, projectMap map[int64]*db.Project) string {
	task, ok := taskMap[a.TaskID]
	if !ok {
		return "."
	}
	if task.WorkDir != "" {
		expanded := expandHome(task.WorkDir)
		if dirExists(expanded) {
			return expanded
		}
		// Fall through to project fallback (mirrors resolveWorkDir's recovery path).
		project, ok := projectMap[task.ProjectID]
		if !ok {
			return "."
		}
		if project.WorkDir != "" {
			projExpanded := expandHome(project.WorkDir)
			if dirExists(projExpanded) {
				return projExpanded
			}
		}
		return "."
	}
	project, ok := projectMap[task.ProjectID]
	if !ok {
		return "."
	}
	if project.WorkDir != "" {
		return expandHome(project.WorkDir)
	}
	return "."
}

// classifyEarlyStale evaluates the early-stale watchdog: an action that has
// exceeded EarlyDispatchTimeout but never produced a session log. Returns
// (reason, true) to schedule a failure, ("", true) to suppress further checks
// for this action (session is active), and ("", false) to defer to the next
// classifier in the chain.
func classifyEarlyStale(cfg WorkerConfig, a *db.Action, workDir string, startedAt, now time.Time) (string, bool) {
	if cfg.ClaudeSessionLogChecker == nil || startedAt.IsZero() {
		return "", false
	}
	sinceStart := now.Sub(startedAt)
	if sinceStart < cfg.EarlyDispatchTimeout {
		return "", false
	}
	active, _, err := cfg.ClaudeSessionLogChecker.IsClaudeSessionActive(workDir, sinceStart)
	if err != nil {
		slog.Warn("early-stale watchdog: claude session log check failed", "action_id", a.ID, "error", err)
		return "", false
	}
	if active {
		// Session is alive even past the early window — defer to nothing else;
		// the regular liveness check would also see this as fresh.
		return "", true
	}
	return fmt.Sprintf("early-stale: no claude session log within %v of dispatch", cfg.EarlyDispatchTimeout), true
}

// claudeSessionStillActive returns (active, discoveredSessionID). The session
// id is non-empty when the checker found a fresh log and surfaced its session
// id; the reaper batches these into a single BulkMergeActionMetadata call so
// the existing per-action MergeActionMetadata pattern is gone from the loop.
func claudeSessionStillActive(cfg WorkerConfig, workDir string, freshness time.Duration) (bool, string) {
	if cfg.ClaudeSessionLogChecker == nil {
		return false, ""
	}
	active, claudeSessionID, err := cfg.ClaudeSessionLogChecker.IsClaudeSessionActive(workDir, freshness)
	if err != nil {
		slog.Warn("claude session log check failed", "error", err)
		return false, ""
	}
	return active, claudeSessionID
}

// MetadataHasValue reports whether the action's metadata JSON has the given
// string-typed key set to the given value. False on empty/invalid JSON or
// non-string values.
func MetadataHasValue(raw, key, value string) bool {
	if raw == "" || raw == "{}" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return false
	}
	v, ok := m[key].(string)
	return ok && v == value
}

func dispatchOne(ctx context.Context, cfg WorkerConfig) (bool, error) {
	action, err := cfg.DB.NextPending(ctx)
	if err != nil {
		return false, fmt.Errorf("next pending: %w", err)
	}
	if action == nil {
		return false, nil
	}

	result, err := ExecuteAction(ctx, ExecuteParams{
		DispatchConfig: cfg.DispatchConfig,
		BeforeInteractive: func(a *db.Action) error {
			// NextPending has already marked this action running, so `running`
			// includes the just-claimed action itself. Compare with `>` so
			// `MaxInteractive=N` means "up to N concurrent" (inclusive).
			running, err := cfg.DB.CountRunningInteractive()
			if err != nil {
				return fmt.Errorf("count running interactive: %w", err)
			}
			if running > cfg.MaxInteractive {
				slog.Debug("interactive limit reached, deferring", "action_id", a.ID, "running", running, "max", cfg.MaxInteractive)
				return ErrInteractiveDeferred
			}
			return nil
		},
		BeforeNonInteractive: func(a *db.Action) error {
			// `running` includes the just-claimed action (see BeforeInteractive).
			running, err := cfg.DB.CountRunningNonInteractive()
			if err != nil {
				return fmt.Errorf("count running noninteractive: %w", err)
			}
			if running > cfg.MaxNonInteractive {
				slog.Debug("noninteractive limit reached, deferring", "action_id", a.ID, "running", running, "max", cfg.MaxNonInteractive)
				return ErrNonInteractiveDeferred
			}
			return nil
		},
		Async: cfg.async,
	}, action)

	if errors.Is(err, ErrInteractiveDeferred) || errors.Is(err, ErrNonInteractiveDeferred) {
		return false, nil
	}
	var af *ActionFailedError
	if errors.As(err, &af) {
		slog.Error("action failed", "action_id", af.ActionID, "error", af.Err)
		return true, nil
	}
	if err != nil {
		return true, err
	}

	slog.Info("action dispatched", "action_id", action.ID, "mode", result.Mode)
	return true, nil
}
