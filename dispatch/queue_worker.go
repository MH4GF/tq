package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	// DefaultBgMissingJobGrace is how long we wait after a bg action's dispatch
	// before deciding that a missing ~/.claude/jobs/<short>/state.json means
	// the daemon job has disappeared (and we should fail the action).
	DefaultBgMissingJobGrace = 30 * time.Second
)

// defaultDeferBackoff is the dispatch_after window applied when an action is
// deferred because its slot pool is full. Set to ~3× DefaultPollInterval so
// the worker can attempt 2-3 other pending actions before re-trying the
// deferred one.
var defaultDeferBackoff = 30 * time.Second

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
	BgStateReader          BgStateReader
	StaleGracePeriod       time.Duration
	HeartbeatFreshness     time.Duration
	InteractiveHardTimeout time.Duration
	EarlyDispatchTimeout   time.Duration
	BgMissingJobGrace      time.Duration
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
	if cfg.BgMissingJobGrace <= 0 {
		cfg.BgMissingJobGrace = DefaultBgMissingJobGrace
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
	if cfg.BgStateReader != nil {
		reapBg(cfg, now)
	}
}

// reapInteractive scans running interactive actions, classifies them in
// memory against prefetched task/project maps and a single tmux window
// snapshot, then bulk-marks the stale ones failed.
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

		if claudeSessionStillActive(cfg, workDir, cfg.HeartbeatFreshness) {
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
}

// reapNonInteractive mirrors reapInteractive without the tmux fallback:
// noninteractive actions are reaped on the time-based stale threshold once
// their session log goes silent.
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
		if claudeSessionStillActive(cfg, workDir, cfg.HeartbeatFreshness) {
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
}

// bgStatePayload mirrors the subset of ~/.claude/jobs/<short>/state.json the
// reaper consumes. Schema confirmed on 2026-05-13; if the daemon changes the
// shape, update this struct.
type bgStatePayload struct {
	State  string `json:"state"`
	Output struct {
		Result string `json:"result"`
	} `json:"output"`
	Detail string `json:"detail"`
}

const (
	bgStateDone   = "done"
	bgStateFailed = "failed"
)

// reapBg drives the lifecycle of experimental_bg actions by polling each
// action's daemon-maintained state.json. Lifecycle decisions are collected
// during the range loop and applied as bulk operations afterward (Rule 15:
// no db.Store calls inside the range body).
func reapBg(cfg WorkerConfig, now time.Time) {
	actions, err := cfg.DB.ListRunningBg()
	if err != nil {
		slog.Error("list running bg for state poll", "error", err)
		return
	}
	if len(actions) == 0 {
		return
	}

	var (
		dones    []db.ActionDoneUpdate
		failures []db.ActionFailureUpdate
	)

	for _, a := range actions {
		meta, err := ParseActionMetadata(a.Metadata)
		if err != nil {
			slog.Warn("bg reaper: parse metadata", "action_id", a.ID, "error", err)
			continue
		}
		short, _ := meta[MetaKeyDaemonShort].(string)
		if short == "" {
			// Dispatch hasn't recorded daemon_short yet (or the merge failed);
			// we have no handle to poll, so leave the action alone.
			continue
		}

		raw, err := cfg.BgStateReader.ReadState(short)
		if err != nil {
			if os.IsNotExist(err) {
				if bgJobMissingForTooLong(&a, now, cfg.BgMissingJobGrace) {
					failures = append(failures, db.ActionFailureUpdate{
						ID:     a.ID,
						Reason: fmt.Sprintf("daemon job dir missing: ~/.claude/jobs/%s", short),
					})
					slog.Warn("bg reaper: marking missing-job action as failed", "action_id", a.ID, "short", short)
				}
				continue
			}
			slog.Warn("bg reaper: read state.json", "action_id", a.ID, "short", short, "error", err)
			continue
		}

		var payload bgStatePayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			// Daemon may be mid-write; do not flip the action to failed on a
			// transient parse error.
			slog.Warn("bg reaper: decode state.json", "action_id", a.ID, "short", short, "error", err)
			continue
		}

		// Only the two terminal states transition the action. Anything else
		// (working, blocked, queued, future daemon-internal states) keeps the
		// action running — `blocked` in particular signals "waiting on user
		// input via claude agents" and must not be flipped to failed.
		switch payload.State {
		case bgStateDone:
			dones = append(dones, db.ActionDoneUpdate{ID: a.ID, Result: payload.Output.Result})
		case bgStateFailed:
			msg := payload.Detail
			if msg == "" {
				msg = payload.Output.Result
			}
			if msg == "" {
				msg = "bg session reported state=failed"
			}
			failures = append(failures, db.ActionFailureUpdate{ID: a.ID, Reason: msg})
		}
	}

	if len(dones) > 0 {
		if err := cfg.DB.BulkMarkDone(dones); err != nil {
			slog.Error("bg reaper: bulk mark done", "error", err, "count", len(dones))
		}
	}
	if len(failures) > 0 {
		if err := cfg.DB.BulkMarkFailed(failures); err != nil {
			slog.Error("bg reaper: bulk mark failed", "error", err, "count", len(failures))
		}
	}
}

// bgJobMissingForTooLong reports whether enough time has passed since the
// action was claimed (StartedAt) to treat a missing state.json as a real
// disappearance rather than a race with daemon initialization.
func bgJobMissingForTooLong(a *db.Action, now time.Time, grace time.Duration) bool {
	if !a.StartedAt.Valid {
		return false
	}
	started, err := time.Parse(db.TimeLayout, a.StartedAt.String)
	if err != nil {
		return false
	}
	return now.Sub(started) >= grace
}

// prefetchWorkDirContext bulk-fetches the tasks (and their projects) needed
// to resolve work_dir for every action. Returns empty maps on empty input.
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

// workDirFromMaps mirrors resolveWorkDir's read-only decision tree but reads
// from prefetched maps. Returns "." for any unresolvable case.
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
	active, err := cfg.ClaudeSessionLogChecker.IsClaudeSessionActive(workDir, sinceStart)
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

// claudeSessionStillActive reports whether the action's session log shows
// recent activity. Errors and missing checker count as "not active" so
// callers fall through to other liveness signals.
func claudeSessionStillActive(cfg WorkerConfig, workDir string, freshness time.Duration) bool {
	if cfg.ClaudeSessionLogChecker == nil {
		return false
	}
	active, err := cfg.ClaudeSessionLogChecker.IsClaudeSessionActive(workDir, freshness)
	if err != nil {
		slog.Warn("claude session log check failed", "error", err)
		return false
	}
	return active
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
			// `MaxInteractive=N` means "up to N concurrent" (inclusive). The
			// count combines interactive and bg actions because they share the
			// MaxInteractive slot pool (bg sessions are interactive via the
			// `claude agents` view).
			running, err := cfg.DB.CountRunningInteractiveOrBg()
			if err != nil {
				return fmt.Errorf("count running interactive+bg: %w", err)
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
		BeforeBg: func(a *db.Action) error {
			running, err := cfg.DB.CountRunningInteractiveOrBg()
			if err != nil {
				return fmt.Errorf("count running interactive+bg: %w", err)
			}
			if running > cfg.MaxInteractive {
				slog.Debug("interactive+bg limit reached, deferring bg", "action_id", a.ID, "running", running, "max", cfg.MaxInteractive)
				return ErrBgDeferred
			}
			return nil
		},
		Async: cfg.async,
	}, action)

	if errors.Is(err, ErrInteractiveDeferred) || errors.Is(err, ErrNonInteractiveDeferred) || errors.Is(err, ErrBgDeferred) {
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
