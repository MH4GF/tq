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

	// Interactive stale check (claude session log heartbeat, fallback to tmux window check)
	if cfg.ClaudeSessionLogChecker != nil || cfg.TmuxChecker != nil {
		actions, err := cfg.DB.ListRunningInteractive()
		if err != nil {
			slog.Error("list running interactive for stale check", "error", err)
		} else if len(actions) > 0 {
			// Build tmux window set for fallback
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

				if reapEarlyStale(cfg, &a, startedAt, now) {
					continue
				}

				if reapCheckClaudeSessionLog(cfg, &a) {
					continue
				}

				// Fallback: tmux window check
				if windowSet == nil {
					// Tmux unavailable and session log did not confirm liveness.
					// Without any positive signal, fall back to a hard timeout so a
					// broken tmux server cannot wedge the interactive slot forever.
					if !startedAt.IsZero() && now.Sub(startedAt) >= cfg.InteractiveHardTimeout {
						result := fmt.Sprintf("stale: tmux unavailable and action exceeded hard timeout (%v)", cfg.InteractiveHardTimeout)
						if err := markActionFailed(cfg.DB, &a, result); err != nil {
							slog.Error("mark stale action failed", "action_id", a.ID, "error", err)
							continue
						}
						slog.Warn("reaped stale action via hard timeout", "action_id", a.ID, "hard_timeout", cfg.InteractiveHardTimeout)
					}
					continue
				}
				if _, exists := windowSet[WindowName(a.ID)]; exists {
					continue
				}

				result := fmt.Sprintf("stale: session log not fresh and tmux window %q no longer exists", WindowName(a.ID))
				if err := markActionFailed(cfg.DB, &a, result); err != nil {
					slog.Error("mark stale action failed", "action_id", a.ID, "error", err)
					continue
				}
				slog.Warn("reaped stale action", "action_id", a.ID)
			}
		}
	}

	// Noninteractive stale check (session log heartbeat, fallback to time-based)
	niActions, err := cfg.DB.ListRunningNonInteractive()
	if err != nil {
		slog.Error("list running noninteractive for stale check", "error", err)
		return
	}
	staleThreshold := time.Duration(defaultTimeout*nonInteractiveStaleMultiplier) * time.Second
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

		if reapCheckClaudeSessionLog(cfg, &a) {
			continue
		}

		result := fmt.Sprintf("stale: noninteractive action exceeded timeout (%v)", staleThreshold)
		if err := markActionFailed(cfg.DB, &a, result); err != nil {
			slog.Error("mark stale noninteractive action failed", "action_id", a.ID, "error", err)
			continue
		}
		slog.Warn("reaped stale noninteractive action", "action_id", a.ID)
	}
}

// reapEarlyStale catches dispatches that crashed before writing any session log,
// where the tmux window may still be alive (so reapCheckClaudeSessionLog cannot help).
func reapEarlyStale(cfg WorkerConfig, a *db.Action, startedAt, now time.Time) bool {
	if cfg.ClaudeSessionLogChecker == nil || startedAt.IsZero() {
		return false
	}
	sinceStart := now.Sub(startedAt)
	if sinceStart < cfg.EarlyDispatchTimeout {
		return false
	}

	workDir, _, err := resolveWorkDir(cfg.DB, a)
	if err != nil {
		slog.Warn("early-stale watchdog: resolve work_dir failed", "action_id", a.ID, "error", err)
	}
	active, _, err := cfg.ClaudeSessionLogChecker.IsClaudeSessionActive(workDir, sinceStart)
	if err != nil {
		slog.Warn("early-stale watchdog: claude session log check failed", "action_id", a.ID, "error", err)
		return false
	}
	if active {
		return false
	}

	result := fmt.Sprintf("early-stale: no claude session log within %v of dispatch", cfg.EarlyDispatchTimeout)
	if err := markActionFailed(cfg.DB, a, result); err != nil {
		slog.Error("mark early-stale action failed", "action_id", a.ID, "error", err)
		return false
	}
	slog.Warn("reaped early-stale action", "action_id", a.ID, "elapsed", sinceStart)
	return true
}

// reapCheckClaudeSessionLog checks if the action's Claude Code session is still active.
// Returns true if the session is active (action should NOT be reaped).
// Also saves the discovered claude_session_id to action metadata for future use.
func reapCheckClaudeSessionLog(cfg WorkerConfig, a *db.Action) bool {
	if cfg.ClaudeSessionLogChecker == nil {
		return false
	}

	workDir, _, err := resolveWorkDir(cfg.DB, a)
	if err != nil {
		slog.Warn("claude session log check: resolve work_dir failed", "action_id", a.ID, "error", err)
	}
	active, claudeSessionID, err := cfg.ClaudeSessionLogChecker.IsClaudeSessionActive(workDir, cfg.HeartbeatFreshness)
	if err != nil {
		slog.Warn("claude session log check failed", "action_id", a.ID, "error", err)
		return false
	}
	if !active {
		return false
	}

	slog.Info("action claude session log is fresh, skipping stale check",
		"action_id", a.ID, "claude_session_id", claudeSessionID)

	// Save claude_session_id to metadata for future use (claude --resume, log investigation).
	// Skip if already saved to avoid redundant DB writes on every poll cycle.
	if claudeSessionID != "" && !MetadataHasValue(a.Metadata, MetaKeyClaudeSessionID, claudeSessionID) {
		if err := cfg.DB.MergeActionMetadata(a.ID, map[string]any{
			MetaKeyClaudeSessionID: claudeSessionID,
		}); err != nil {
			slog.Warn("failed to save claude_session_id to metadata", "action_id", a.ID, "error", err)
		}
	}

	return true
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
