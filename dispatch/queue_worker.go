package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/MH4GF/tq/db"
)

const (
	DefaultMaxInteractive              = 3
	DefaultMaxNonInteractive           = 5
	DefaultStaleThreshold              = 30 * time.Second
	DefaultPollInterval                = 10 * time.Second
	DefaultBgMissingJobGrace           = 30 * time.Second
	DefaultBgNonInteractiveHardTimeout = 4 * time.Hour
)

var defaultDeferBackoff = 30 * time.Second

type WorkerConfig struct {
	DispatchConfig
	MaxInteractive              int
	MaxNonInteractive           int
	PollInterval                time.Duration
	BgStateReader               BgStateReader
	BgMissingJobGrace           time.Duration
	BgNonInteractiveHardTimeout time.Duration
}

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
	if cfg.BgMissingJobGrace <= 0 {
		cfg.BgMissingJobGrace = DefaultBgMissingJobGrace
	}
	if cfg.BgNonInteractiveHardTimeout <= 0 {
		cfg.BgNonInteractiveHardTimeout = DefaultBgNonInteractiveHardTimeout
	}

	slog.Info("queue worker started",
		"max_interactive", cfg.MaxInteractive,
		"max_noninteractive", cfg.MaxNonInteractive,
		"poll_interval", cfg.PollInterval,
	)

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

		reapStaleActions(cfg)

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

func reapStaleActions(cfg WorkerConfig) {
	now := time.Now()
	reapOrphans(cfg, now)
	if cfg.BgStateReader != nil {
		reapBg(cfg, now)
	}
}

func reapOrphans(cfg WorkerConfig, _ time.Time) {
	actions, err := cfg.DB.ListRunningOrphans(cfg.BgMissingJobGrace)
	if err != nil {
		slog.Error("list running orphans", "error", err)
		return
	}
	if len(actions) == 0 {
		return
	}
	failures := make([]db.ActionFailureUpdate, 0, len(actions))
	for _, a := range actions {
		failures = append(failures, db.ActionFailureUpdate{
			ID:     a.ID,
			Reason: fmt.Sprintf("orphaned: running for >%v with no daemon_short recorded", cfg.BgMissingJobGrace),
		})
		slog.Warn("reaped orphan running action", "action_id", a.ID)
	}
	if err := cfg.DB.BulkMarkFailed(failures); err != nil {
		slog.Error("bulk mark orphan actions failed", "error", err, "count", len(failures))
	}
}

func parseStartedAt(a *db.Action) (started time.Time, ok bool) {
	if !a.StartedAt.Valid {
		return time.Time{}, false
	}
	t, err := time.Parse(db.TimeLayout, a.StartedAt.String)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

type bgStatePayload struct {
	State     string `json:"state"`
	SessionID string `json:"sessionId"`
	Output    struct {
		Result string `json:"result"`
	} `json:"output"`
	Detail string `json:"detail"`
}

const (
	bgStateDone   = "done"
	bgStateFailed = "failed"
)

func reapBg(cfg WorkerConfig, now time.Time) {
	actions, err := cfg.DB.ListRunningWithDaemonShort()
	if err != nil {
		slog.Error("list running with daemon_short for state poll", "error", err)
		return
	}
	if len(actions) == 0 {
		return
	}

	var (
		dones    []db.ActionDoneUpdate
		failures []db.ActionFailureUpdate
		merges   []db.ActionMetadataMerge
	)

	for _, a := range actions {
		meta, err := ParseActionMetadata(a.Metadata)
		if err != nil {
			slog.Warn("bg reaper: parse metadata", "action_id", a.ID, "error", err)
			continue
		}
		short, _ := meta[MetaKeyDaemonShort].(string)
		if short == "" {
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
			slog.Warn("bg reaper: decode state.json", "action_id", a.ID, "short", short, "error", err)
			continue
		}

		if payload.SessionID != "" {
			if existing, _ := meta[MetaKeyClaudeSessionID].(string); existing == "" {
				merges = append(merges, db.ActionMetadataMerge{
					ID: a.ID,
					Updates: map[string]any{
						MetaKeyClaudeSessionID: payload.SessionID,
					},
				})
			}
		}

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
		default:
			mode, _ := meta[MetaKeyMode].(string)
			if mode != ModeNonInteractive {
				continue
			}
			started, ok := parseStartedAt(&a)
			if !ok || now.Sub(started) < cfg.BgNonInteractiveHardTimeout {
				continue
			}
			failures = append(failures, db.ActionFailureUpdate{
				ID:     a.ID,
				Reason: fmt.Sprintf("bg hard timeout (%v) elapsed in state=%q", cfg.BgNonInteractiveHardTimeout, payload.State),
			})
			slog.Warn("bg reaper: hard timeout reached",
				"action_id", a.ID, "mode", mode, "state", payload.State, "elapsed", now.Sub(started))
		}
	}

	if len(merges) > 0 {
		if err := cfg.DB.BulkMergeActionMetadata(merges); err != nil {
			slog.Warn("bg reaper: bulk merge metadata", "error", err, "count", len(merges))
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

func bgJobMissingForTooLong(a *db.Action, now time.Time, grace time.Duration) bool {
	started, ok := parseStartedAt(a)
	if !ok {
		return false
	}
	return now.Sub(started) >= grace
}

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
