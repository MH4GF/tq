package dispatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/MH4GF/tq/db"
)

const (
	DefaultMaxInteractive   = 3
	DefaultStaleThreshold   = 30 * time.Second
	DefaultPollInterval     = 10 * time.Second
	DefaultStaleGracePeriod = 30 * time.Second
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
	MaxInteractive   int
	PollInterval     time.Duration
	TmuxChecker      TmuxChecker
	StaleGracePeriod time.Duration
}

// RunWorker continuously dispatches pending actions.
// It processes one action per iteration, sleeping when idle.
func RunWorker(ctx context.Context, cfg WorkerConfig) error {
	if cfg.MaxInteractive <= 0 {
		cfg.MaxInteractive = DefaultMaxInteractive
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

	slog.Info("queue worker started", "max_interactive", cfg.MaxInteractive, "poll_interval", cfg.PollInterval)

	var lastHeartbeat time.Time
	for {
		select {
		case <-ctx.Done():
			slog.Info("queue worker stopped")
			return ctx.Err()
		default:
		}

		if time.Since(lastHeartbeat) >= cfg.PollInterval {
			if err := cfg.DB.UpdateWorkerHeartbeat(); err != nil {
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
			started, err := time.Parse(db.TimeLayout, a.StartedAt.String)
			if err == nil && now.Sub(started) < cfg.StaleGracePeriod {
				continue
			}
		}

		windowName := WindowName(a.ID)
		if _, exists := windowSet[windowName]; exists {
			continue
		}

		result := fmt.Sprintf("stale: tmux window %q no longer exists", windowName)
		if err := cfg.DB.MarkFailed(a.ID, result); err != nil {
			slog.Error("mark stale action failed", "action_id", a.ID, "error", err)
			continue
		}
		slog.Warn("reaped stale action", "action_id", a.ID, "window", windowName)

		CreateInvestigateFailureAction(cfg.DB, &a, result)
	}
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
			running, err := cfg.DB.CountRunningInteractive()
			if err != nil {
				return fmt.Errorf("count running interactive: %w", err)
			}
			if running >= cfg.MaxInteractive {
				slog.Info("interactive limit reached, deferring", "action_id", a.ID, "running", running, "max", cfg.MaxInteractive)
				return ErrInteractiveDeferred
			}
			return nil
		},
	}, action)

	if errors.Is(err, ErrInteractiveDeferred) {
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
