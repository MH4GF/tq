package dispatch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func ptr[T any](v T) *T { return &v }

// withShortDeferBackoff shrinks defaultDeferBackoff to the polling-loop scale
// for slot-defer tests that expect a deferred action to resume promptly after
// the slot frees. Restored via t.Cleanup.
func withShortDeferBackoff(t *testing.T) {
	t.Helper()
	original := defaultDeferBackoff
	defaultDeferBackoff = 50 * time.Millisecond
	t.Cleanup(func() { defaultDeferBackoff = original })
}

type countingWorker struct {
	mu      sync.Mutex
	count   int
	result  string
	err     error
	denials []PermissionDenial
}

func (w *countingWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.count++
	return w.result, w.err
}

func (w *countingWorker) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

func (w *countingWorker) Set(result string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.result = result
	w.err = err
}

func (w *countingWorker) LastDenials() []PermissionDenial { return w.denials }

// blockingWorker.Execute blocks on the `block` channel until released, and
// records the call count. Used to simulate a long-running noninteractive
// claude -p in queue worker tests.
type blockingWorker struct {
	mu          sync.Mutex
	count       int
	block       chan struct{}
	releaseOnce sync.Once
	result      string
	err         error
}

func newBlockingWorker() *blockingWorker {
	return &blockingWorker{block: make(chan struct{})}
}

func (w *blockingWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	w.mu.Lock()
	w.count++
	w.mu.Unlock()
	select {
	case <-w.block:
		return w.result, w.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (w *blockingWorker) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

func (w *blockingWorker) Release() {
	w.releaseOnce.Do(func() { close(w.block) })
}

func (w *blockingWorker) LastDenials() []PermissionDenial { return nil }

// waitFor polls fn until it returns true or the timeout expires.
func waitFor(t *testing.T, timeout time.Duration, msg string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitFor timeout after %v: %s", timeout, msg)
}

func TestRunWorker_ProcessesAndStops(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr-status", taskID, `{"instruction":"check pr status","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	worker := &countingWorker{result: `{"ok":true}`}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB: d,
			NonInteractiveFunc: func() Worker {
				return worker
			},
			InteractiveFunc: func() Worker {
				return worker
			},
		},
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
	}

	err := RunWorker(ctx, cfg)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunWorker error = %v, want context.DeadlineExceeded", err)
	}

	if worker.Count() != 1 {
		t.Errorf("worker.count = %d, want 1", worker.Count())
	}

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusDone {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusDone)
	}
}

func TestRunWorker_InteractiveLimitEnforced(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, `{"instruction":"fix conflict","mode":"interactive"}`, db.ActionStatusPending, nil, "")

	d.InsertAction("respond-review", taskID, "{}", db.ActionStatusRunning, nil, "")
	d.SetActionTmuxInfoForTest(2, ptr("session-1"), nil, nil)

	interactiveWorker := &countingWorker{result: "interactive:session=test"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB: d,
			NonInteractiveFunc: func() Worker {
				return &countingWorker{result: `{"ok":true}`}
			},
			InteractiveFunc: func() Worker {
				return interactiveWorker
			},
		},
		MaxInteractive: 1,
		PollInterval:   50 * time.Millisecond,
	}

	_ = RunWorker(ctx, cfg)

	if interactiveWorker.Count() != 0 {
		t.Errorf("interactive worker called %d times, want 0 (limit reached)", interactiveWorker.Count())
	}
}

func TestRunWorker_FailureEscalation(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr-status", taskID, `{"instruction":"check pr status","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	worker := &countingWorker{err: context.DeadlineExceeded}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB: d,
			NonInteractiveFunc: func() Worker {
				return worker
			},
			InteractiveFunc: func() Worker {
				return worker
			},
		},
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
	}

	_ = RunWorker(ctx, cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusFailed)
	}
}

// TestRunWorker_FailureDoesNotCreateFollowUp verifies that worker failure
// marks the action as failed without spawning a follow-up investigation
// action. Per-failure auto-investigation was retired 2026-05-14 in favor of
// the batched /tq:investigate-incidents skill.
func TestRunWorker_FailureDoesNotCreateFollowUp(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr-status", taskID, `{"instruction":"check pr status","mode":"noninteractive"}`, "pending", nil, "")

	worker := &countingWorker{err: fmt.Errorf("something went wrong")}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB: d,
			NonInteractiveFunc: func() Worker {
				return worker
			},
			InteractiveFunc: func() Worker {
				return worker
			},
		},
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
	}

	_ = RunWorker(ctx, cfg)

	action, _ := d.GetAction(1)
	if action.Status != "failed" {
		t.Errorf("action status = %q, want failed", action.Status)
	}
	if action.TaskID != taskID {
		t.Errorf("action task_id = %d, want %d", action.TaskID, taskID)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 1 {
		t.Errorf("expected exactly 1 action (no auto-generated follow-up), got %d", len(actions))
	}
}

func TestRunWorker_RemoteDispatch(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Remote task", `{"url":"https://example.com"}`, "")
	d.InsertAction("remote-task", taskID, `{"instruction":"do remote task","mode":"remote"}`, db.ActionStatusPending, nil, "")

	remoteWorker := &countingWorker{result: "remote:session=https://console.anthropic.com/p/abc"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB: d,
			NonInteractiveFunc: func() Worker {
				return &countingWorker{result: `{"ok":true}`}
			},
			InteractiveFunc: func() Worker {
				return &countingWorker{result: "interactive:action=1"}
			},
			RemoteFunc: func() Worker {
				return remoteWorker
			},
		},
		MaxInteractive: 1,
		PollInterval:   50 * time.Millisecond,
	}

	_ = RunWorker(ctx, cfg)

	if remoteWorker.Count() != 1 {
		t.Errorf("remote worker called %d times, want 1", remoteWorker.Count())
	}

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusDispatched {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusDispatched)
	}
}

func TestRunWorker_RemoteDoesNotCountTowardInteractiveLimit(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("remote-task", taskID, `{"instruction":"do remote task","mode":"remote"}`, db.ActionStatusPending, nil, "")
	d.InsertAction("fix-conflict", taskID, `{"instruction":"fix conflict","mode":"interactive"}`, db.ActionStatusPending, nil, "")

	d.InsertAction("respond-review", taskID, `{"instruction":"respond to review","mode":"interactive"}`, db.ActionStatusRunning, nil, "")
	d.SetActionTmuxInfoForTest(3, ptr("session-1"), nil, nil)

	remoteWorker := &countingWorker{result: "remote:session=https://example.com"}
	interactiveWorker := &countingWorker{result: "interactive:action=2"}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB: d,
			NonInteractiveFunc: func() Worker {
				return &countingWorker{result: `{"ok":true}`}
			},
			InteractiveFunc: func() Worker {
				return interactiveWorker
			},
			RemoteFunc: func() Worker {
				return remoteWorker
			},
		},
		MaxInteractive: 1,
		PollInterval:   50 * time.Millisecond,
	}

	_ = RunWorker(ctx, cfg)

	if remoteWorker.Count() != 1 {
		t.Errorf("remote worker called %d times, want 1 (should not be limited)", remoteWorker.Count())
	}
	if interactiveWorker.Count() != 0 {
		t.Errorf("interactive worker called %d times, want 0 (limit reached)", interactiveWorker.Count())
	}
}

type mockTmuxChecker struct {
	windows       []string
	err           error
	calledSession string
}

func (m *mockTmuxChecker) ListWindows(ctx context.Context, session string) ([]string, error) {
	m.calledSession = session
	return m.windows, m.err
}

type mockClaudeSessionLogChecker struct {
	mu     sync.Mutex
	active bool
	err    error
	calls  int
}

func (m *mockClaudeSessionLogChecker) IsClaudeSessionActive(workDir string, freshnessThreshold time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.active, m.err
}

func (m *mockClaudeSessionLogChecker) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestReapStaleActions_Interactive(t *testing.T) {
	tests := []struct {
		name                   string
		startedOffset          time.Duration
		omitStartedAt          bool
		omitSessionInfo        bool
		metadata               string
		tmux                   *mockTmuxChecker
		log                    *mockClaudeSessionLogChecker
		tmuxSession            string
		interactiveHardTimeout time.Duration
		earlyDispatchTimeout   time.Duration
		wantStatus             string
		wantResultContains     string
	}{
		{
			name:               "reaps when tmux window missing",
			startedOffset:      -5 * time.Minute,
			tmux:               &mockTmuxChecker{windows: []string{"zsh", "other-window"}},
			wantStatus:         db.ActionStatusFailed,
			wantResultContains: "stale",
		},
		{
			name:          "skips when tmux window live",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "skips within grace period",
			startedOffset: 0,
			tmux:          &mockTmuxChecker{windows: []string{"zsh"}},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "skips on tmux error within hard timeout",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{err: fmt.Errorf("tmux not available")},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:                   "reaps on tmux error after hard timeout",
			startedOffset:          -2 * time.Hour,
			tmux:                   &mockTmuxChecker{err: fmt.Errorf("tmux not available")},
			interactiveHardTimeout: 1 * time.Hour,
			wantStatus:             db.ActionStatusFailed,
			wantResultContains:     "tmux unavailable",
		},
		{
			name:                   "reaps on tmux error after hard timeout even with stale session log",
			startedOffset:          -2 * time.Hour,
			tmux:                   &mockTmuxChecker{err: fmt.Errorf("tmux not available")},
			log:                    &mockClaudeSessionLogChecker{active: false},
			interactiveHardTimeout: 1 * time.Hour,
			earlyDispatchTimeout:   24 * time.Hour,
			wantStatus:             db.ActionStatusFailed,
			wantResultContains:     "hard timeout",
		},
		{
			name:                   "skips on tmux error when session log fresh even past hard timeout",
			startedOffset:          -2 * time.Hour,
			tmux:                   &mockTmuxChecker{err: fmt.Errorf("tmux not available")},
			log:                    &mockClaudeSessionLogChecker{active: true},
			interactiveHardTimeout: 1 * time.Hour,
			wantStatus:             db.ActionStatusRunning,
		},
		{
			name:          "no-op when no checker",
			omitStartedAt: true,
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "forwards custom tmux session",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}},
			tmuxSession:   "work",
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "skips when session log fresh",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{windows: []string{"zsh"}},
			log:           &mockClaudeSessionLogChecker{active: true},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:                 "reaps when session log stale and window gone (no watchdog)",
			startedOffset:        -5 * time.Minute,
			tmux:                 &mockTmuxChecker{windows: []string{"zsh"}},
			log:                  &mockClaudeSessionLogChecker{active: false},
			earlyDispatchTimeout: 24 * time.Hour,
			wantStatus:           db.ActionStatusFailed,
			wantResultContains:   "session log not fresh",
		},
		{
			name:          "reaps via tmux fallback when no log checker",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{windows: []string{"zsh"}},
			wantStatus:    db.ActionStatusFailed,
		},
		{
			name:               "watchdog reaps when no session log within early timeout (window still up)",
			startedOffset:      -6 * time.Minute,
			tmux:               &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}},
			log:                &mockClaudeSessionLogChecker{active: false},
			wantStatus:         db.ActionStatusFailed,
			wantResultContains: "early-stale",
		},
		{
			name:          "watchdog skips within early timeout",
			startedOffset: -2 * time.Minute,
			tmux:          &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}},
			log:           &mockClaudeSessionLogChecker{active: false},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:               "reaps session_id NULL action when window missing",
			startedOffset:      -5 * time.Minute,
			omitSessionInfo:    true,
			tmux:               &mockTmuxChecker{windows: []string{"zsh"}},
			wantStatus:         db.ActionStatusFailed,
			wantResultContains: "stale",
		},
		{
			name:                 "skips cloud executor action even when window gone and log stale",
			startedOffset:        -5 * time.Minute,
			metadata:             `{"executor":"cloud"}`,
			tmux:                 &mockTmuxChecker{windows: []string{"zsh"}},
			log:                  &mockClaudeSessionLogChecker{active: false},
			earlyDispatchTimeout: 24 * time.Hour,
			wantStatus:           db.ActionStatusRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
			meta := tt.metadata
			if meta == "" {
				meta = "{}"
			}
			d.InsertAction("fix-conflict", taskID, meta, db.ActionStatusRunning, nil, "")

			windowName := "tq-action-1"
			var startedAt *time.Time
			if !tt.omitStartedAt {
				s := time.Now().Add(tt.startedOffset)
				startedAt = &s
			}
			tmuxSession := ptr("main")
			tmuxWindowPtr := &windowName
			if tt.omitSessionInfo {
				tmuxSession = nil
				tmuxWindowPtr = nil
			}
			d.SetActionTmuxInfoForTest(1, tmuxSession, tmuxWindowPtr, startedAt)

			cfg := WorkerConfig{
				DispatchConfig:         DispatchConfig{DB: d},
				StaleGracePeriod:       30 * time.Second,
				HeartbeatFreshness:     120 * time.Second,
				InteractiveHardTimeout: DefaultInteractiveHardTimeout,
				EarlyDispatchTimeout:   DefaultEarlyDispatchTimeout,
			}
			if tt.tmux != nil {
				cfg.TmuxChecker = tt.tmux
			}
			if tt.log != nil {
				cfg.ClaudeSessionLogChecker = tt.log
			}
			if tt.tmuxSession != "" {
				cfg.TmuxSession = tt.tmuxSession
			}
			if tt.interactiveHardTimeout > 0 {
				cfg.InteractiveHardTimeout = tt.interactiveHardTimeout
			}
			if tt.earlyDispatchTimeout > 0 {
				cfg.EarlyDispatchTimeout = tt.earlyDispatchTimeout
			}

			reapStaleActions(context.Background(), cfg)

			action, _ := d.GetAction(1)
			if action.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", action.Status, tt.wantStatus)
			}
			if tt.wantResultContains != "" {
				if !action.Result.Valid || !strings.Contains(action.Result.String, tt.wantResultContains) {
					t.Errorf("result = %v, want containing %q", action.Result, tt.wantResultContains)
				}
			}
			if tt.tmuxSession != "" && tt.tmux != nil && tt.tmux.calledSession != tt.tmuxSession {
				t.Errorf("calledSession = %q, want %q", tt.tmux.calledSession, tt.tmuxSession)
			}
		})
	}
}

func TestReapStaleActions_NonInteractive(t *testing.T) {
	tests := []struct {
		name               string
		startedOffset      time.Duration
		omitStartedAt      bool
		metadata           string
		log                *mockClaudeSessionLogChecker
		wantStatus         string
		wantResultContains string
	}{
		{
			name:               "reaps when timeout exceeded",
			startedOffset:      -25 * time.Minute,
			wantStatus:         db.ActionStatusFailed,
			wantResultContains: "noninteractive",
		},
		{
			name:          "skips within threshold",
			startedOffset: -5 * time.Minute,
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "skips when started_at unset",
			omitStartedAt: true,
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "skipped by fresh heartbeat",
			startedOffset: -25 * time.Minute,
			log:           &mockClaudeSessionLogChecker{active: true},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "reaped by stale heartbeat",
			startedOffset: -25 * time.Minute,
			log:           &mockClaudeSessionLogChecker{active: false},
			wantStatus:    db.ActionStatusFailed,
		},
		{
			name:          "reaped when checker errors",
			startedOffset: -25 * time.Minute,
			log:           &mockClaudeSessionLogChecker{err: fmt.Errorf("permission denied")},
			wantStatus:    db.ActionStatusFailed,
		},
		{
			name:          "skips cloud executor noninteractive even past timeout",
			startedOffset: -25 * time.Minute,
			metadata:      `{"instruction":"x","mode":"noninteractive","executor":"cloud"}`,
			log:           &mockClaudeSessionLogChecker{active: false},
			wantStatus:    db.ActionStatusRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
			meta := tt.metadata
			if meta == "" {
				meta = `{"instruction":"check","mode":"noninteractive"}`
			}
			d.InsertAction("check-pr", taskID, meta, db.ActionStatusRunning, nil, "")

			if !tt.omitStartedAt {
				started := time.Now().Add(tt.startedOffset)
				d.SetActionTmuxInfoForTest(1, nil, nil, &started)
			}

			cfg := WorkerConfig{
				DispatchConfig:     DispatchConfig{DB: d},
				HeartbeatFreshness: 120 * time.Second,
			}
			if tt.log != nil {
				cfg.ClaudeSessionLogChecker = tt.log
			}

			reapStaleActions(context.Background(), cfg)

			action, _ := d.GetAction(1)
			if action.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", action.Status, tt.wantStatus)
			}
			if tt.wantResultContains != "" {
				if !action.Result.Valid || !strings.Contains(action.Result.String, tt.wantResultContains) {
					t.Errorf("result = %v, want containing %q", action.Result, tt.wantResultContains)
				}
			}
		})
	}
}

func TestReapStaleActions_MultipleStaleAllReaped(t *testing.T) {
	tests := []struct {
		name        string
		actionTitle string
		metadata    string
		staleAt     time.Duration
		tmuxSession *string
		cfgExtras   func(*WorkerConfig)
	}{
		{
			name:        "interactive",
			actionTitle: "fix-conflict",
			metadata:    "{}",
			staleAt:     -5 * time.Minute,
			tmuxSession: ptr("main"),
			cfgExtras: func(c *WorkerConfig) {
				c.StaleGracePeriod = 30 * time.Second
				c.InteractiveHardTimeout = DefaultInteractiveHardTimeout
				c.EarlyDispatchTimeout = DefaultEarlyDispatchTimeout
				c.TmuxChecker = &mockTmuxChecker{windows: []string{"zsh"}}
			},
		},
		{
			name:        "noninteractive",
			actionTitle: "check-pr",
			metadata:    `{"instruction":"check","mode":"noninteractive"}`,
			staleAt:     -25 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")

			staleAt := time.Now().Add(tt.staleAt)
			const n = 3
			for i := range n {
				if _, err := d.InsertAction(tt.actionTitle, taskID, tt.metadata, db.ActionStatusRunning, nil, ""); err != nil {
					t.Fatalf("seed action %d: %v", i, err)
				}
			}
			for i := int64(1); i <= n; i++ {
				if tt.tmuxSession != nil {
					d.SetActionTmuxInfoForTest(i, tt.tmuxSession, ptr(WindowName(i)), &staleAt)
				} else {
					d.SetActionTmuxInfoForTest(i, nil, nil, &staleAt)
				}
			}

			cfg := WorkerConfig{
				DispatchConfig:     DispatchConfig{DB: d},
				HeartbeatFreshness: 120 * time.Second,
			}
			if tt.cfgExtras != nil {
				tt.cfgExtras(&cfg)
			}

			reapStaleActions(context.Background(), cfg)

			for i := int64(1); i <= n; i++ {
				a, err := d.GetAction(i)
				if err != nil {
					t.Fatalf("GetAction(%d): %v", i, err)
				}
				if a.Status != db.ActionStatusFailed {
					t.Errorf("action %d status = %q, want %q", i, a.Status, db.ActionStatusFailed)
				}
			}
		})
	}
}

func TestDispatchOne_NoPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB: d,
			NonInteractiveFunc: func() Worker {
				return &countingWorker{}
			},
			InteractiveFunc: func() Worker {
				return &countingWorker{}
			},
		},
		PollInterval: 50 * time.Millisecond,
	}

	dispatched, err := dispatchOne(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dispatchOne error: %v", err)
	}
	if dispatched {
		t.Error("expected dispatched=false when no pending actions")
	}
}

// TestRunWorker_NonInteractiveDoesNotBlockInteractive verifies that a
// long-running noninteractive action does not block the dispatch loop from
// dispatching a queued interactive action — the regression that motivated
// switching noninteractive execution to a goroutine.
func TestRunWorker_NonInteractiveDoesNotBlockInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("long-ni", taskID, `{"instruction":"long","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	d.InsertAction("interactive", taskID, `{"instruction":"fix","mode":"interactive"}`, db.ActionStatusPending, nil, "")

	niWorker := newBlockingWorker()
	defer niWorker.Release()
	intWorker := &countingWorker{result: "interactive:session=test"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return niWorker },
			InteractiveFunc:    func() Worker { return intWorker },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 3,
		PollInterval:      30 * time.Millisecond,
	}

	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, cfg) }()

	waitFor(t, 1*time.Second, "interactive worker dispatched while noninteractive is running", func() bool {
		return intWorker.Count() >= 1 && niWorker.Count() >= 1
	})

	cancel()
	<-done

	if niWorker.Count() != 1 {
		t.Errorf("niWorker.count = %d, want 1", niWorker.Count())
	}
	if intWorker.Count() != 1 {
		t.Errorf("intWorker.count = %d, want 1", intWorker.Count())
	}
}

// TestRunWorker_NonInteractiveSlotLimit verifies MaxNonInteractive caps the
// number of in-flight noninteractive actions: when the cap is reached, new
// pending noninteractive actions are deferred (ResetToPending) until a slot
// frees up.
func TestRunWorker_NonInteractiveSlotLimit(t *testing.T) {
	withShortDeferBackoff(t)
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("ni-1", taskID, `{"instruction":"ni-1","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	d.InsertAction("ni-2", taskID, `{"instruction":"ni-2","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	niWorker := newBlockingWorker()
	niWorker.result = `{"ok":true}`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return niWorker },
			InteractiveFunc:    func() Worker { return &countingWorker{} },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 1,
		PollInterval:      30 * time.Millisecond,
	}

	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, cfg) }()

	// First slot fills, second action stays pending due to cap.
	waitFor(t, 1*time.Second, "first noninteractive starts running", func() bool {
		return niWorker.Count() >= 1
	})
	// Confirm the cap holds: count stays at 1 over the next few polls.
	time.Sleep(150 * time.Millisecond)
	if got := niWorker.Count(); got != 1 {
		t.Fatalf("niWorker.count = %d, want 1 (cap should hold)", got)
	}
	a2, _ := d.GetAction(2)
	if a2.Status != db.ActionStatusPending {
		t.Errorf("action #2 status = %q, want %q (deferred)", a2.Status, db.ActionStatusPending)
	}

	// Release the first action; the second should now claim the slot.
	niWorker.Release()
	waitFor(t, 1*time.Second, "second noninteractive dispatched after slot frees", func() bool {
		return niWorker.Count() >= 2
	})

	cancel()
	<-done
}

// TestRunWorker_NonInteractiveFailureDoesNotStopLoop verifies that a failed
// noninteractive action does not wedge the dispatch loop: a follow-up
// noninteractive action queued afterwards still gets dispatched on the next
// poll cycle.
func TestRunWorker_NonInteractiveFailureDoesNotStopLoop(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("ni-fail", taskID, `{"instruction":"fail","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	failedThenOK := &countingWorker{err: fmt.Errorf("boom")}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return failedThenOK },
			InteractiveFunc:    func() Worker { return &countingWorker{} },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 3,
		PollInterval:      30 * time.Millisecond,
	}

	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, cfg) }()

	waitFor(t, 1*time.Second, "first noninteractive marked failed", func() bool {
		a, err := d.GetAction(1)
		return err == nil && a.Status == db.ActionStatusFailed
	})

	// Queue another noninteractive after the failure.
	d.InsertAction("ni-after", taskID, `{"instruction":"after","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	failedThenOK.Set(`{"ok":true}`, nil)

	waitFor(t, 1*time.Second, "follow-up noninteractive dispatched after failure", func() bool {
		// id 1 was the failed ni-fail; id 2 is our manually queued ni-after
		// (no auto-generated follow-up exists since per-failure investigation
		// was retired 2026-05-14).
		a, err := d.GetAction(2)
		return err == nil && (a.Status == db.ActionStatusRunning || a.Status == db.ActionStatusDone)
	})

	cancel()
	<-done
}

// TestRunWorker_ShutdownDoesNotMarkInflightFailed verifies that ctx cancel
// during shutdown does NOT mark in-flight noninteractive actions as failed.
// They should remain running so the next worker session's stale reaper can
// re-evaluate.
func TestRunWorker_ShutdownDoesNotMarkInflightFailed(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("long-ni", taskID, `{"instruction":"long","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	niWorker := newBlockingWorker()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return niWorker },
			InteractiveFunc:    func() Worker { return &countingWorker{} },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 3,
		PollInterval:      30 * time.Millisecond,
	}

	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, cfg) }()

	waitFor(t, 1*time.Second, "noninteractive action starts running", func() bool {
		a, err := d.GetAction(1)
		return err == nil && a.Status == db.ActionStatusRunning
	})

	// Cancel the worker while the noninteractive is in flight.
	cancel()
	<-done

	a, _ := d.GetAction(1)
	if a.Status == db.ActionStatusFailed {
		t.Errorf("action status = %q, want still running (shutdown should not mark in-flight as failed)", a.Status)
	}
}

// fakeBgStateReader is an in-memory BgStateReader. Map values are returned
// verbatim; absence of a key triggers os.ErrNotExist (so the reaper applies
// its "job dir missing" grace logic).
type fakeBgStateReader struct {
	mu    sync.Mutex
	files map[string][]byte
	err   error
}

func (f *fakeBgStateReader) ReadState(short string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.files[short]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: short, Err: fs.ErrNotExist}
	}
	return data, nil
}

func TestReapBg(t *testing.T) {
	now := time.Now()
	stateWorking := []byte(`{"state":"working"}`)
	stateDone := []byte(`{"state":"done","output":{"result":"shipped"}}`)
	stateFailed := []byte(`{"state":"failed","detail":"daemon abort"}`)
	stateFailedNoDetail := []byte(`{"state":"failed","output":{"result":"agg"}}`)
	stateMalformed := []byte(`{not json`)
	stateUnknown := []byte(`{"state":"banana"}`)

	tests := []struct {
		name              string
		metadata          string
		startedOffset     time.Duration // applied to StartedAt; 0 means "do not set"
		omitStartedAt     bool
		files             map[string][]byte
		short             string
		wantStatus        string
		wantResultPiece   string
		wantNoInvestigate bool
	}{
		{
			name:          "working leaves action untouched",
			metadata:      `{"instruction":"x","mode":"experimental_bg","daemon_short":"aaaaaaaa"}`,
			startedOffset: -1 * time.Minute,
			files:         map[string][]byte{"aaaaaaaa": stateWorking},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:            "done marks action done with output.result",
			metadata:        `{"instruction":"x","mode":"experimental_bg","daemon_short":"bbbbbbbb"}`,
			startedOffset:   -1 * time.Minute,
			files:           map[string][]byte{"bbbbbbbb": stateDone},
			wantStatus:      db.ActionStatusDone,
			wantResultPiece: "shipped",
		},
		{
			name:            "failed (with detail) marks failed and prefers detail over result",
			metadata:        `{"instruction":"x","mode":"experimental_bg","daemon_short":"cccccccc"}`,
			startedOffset:   -1 * time.Minute,
			files:           map[string][]byte{"cccccccc": stateFailed},
			wantStatus:      db.ActionStatusFailed,
			wantResultPiece: "daemon abort",
		},
		{
			name:            "failed (no detail) falls back to output.result",
			metadata:        `{"instruction":"x","mode":"experimental_bg","daemon_short":"dddddddd"}`,
			startedOffset:   -1 * time.Minute,
			files:           map[string][]byte{"dddddddd": stateFailedNoDetail},
			wantStatus:      db.ActionStatusFailed,
			wantResultPiece: "agg",
		},
		{
			name:            "missing state file past grace marks failed",
			metadata:        `{"instruction":"x","mode":"experimental_bg","daemon_short":"eeeeeeee"}`,
			startedOffset:   -2 * time.Minute,
			files:           map[string][]byte{},
			wantStatus:      db.ActionStatusFailed,
			wantResultPiece: "daemon job dir missing",
		},
		{
			name:          "missing state file within grace skips",
			metadata:      `{"instruction":"x","mode":"experimental_bg","daemon_short":"ffffffff"}`,
			startedOffset: -1 * time.Second,
			files:         map[string][]byte{},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "missing daemon_short skips",
			metadata:      `{"instruction":"x","mode":"experimental_bg"}`,
			startedOffset: -1 * time.Minute,
			files:         map[string][]byte{},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "malformed JSON does not flip status",
			metadata:      `{"instruction":"x","mode":"experimental_bg","daemon_short":"gggggggg"}`,
			startedOffset: -1 * time.Minute,
			files:         map[string][]byte{"gggggggg": stateMalformed},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "unknown state value does not flip status",
			metadata:      `{"instruction":"x","mode":"experimental_bg","daemon_short":"hhhhhhhh"}`,
			startedOffset: -1 * time.Minute,
			files:         map[string][]byte{"hhhhhhhh": stateUnknown},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "blocked state (awaiting user input via claude agents) keeps running",
			metadata:      `{"instruction":"x","mode":"experimental_bg","daemon_short":"iiiiiiii"}`,
			startedOffset: -1 * time.Minute,
			files:         map[string][]byte{"iiiiiiii": []byte(`{"state":"blocked","detail":"awaiting reply"}`)},
			wantStatus:    db.ActionStatusRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Task", "{}", "")
			if _, err := d.InsertAction("bg-action", taskID, tt.metadata, db.ActionStatusRunning, nil, ""); err != nil {
				t.Fatalf("seed action: %v", err)
			}
			if !tt.omitStartedAt {
				started := now.Add(tt.startedOffset)
				d.SetActionTmuxInfoForTest(1, nil, nil, &started)
			}

			cfg := WorkerConfig{
				DispatchConfig:    DispatchConfig{DB: d},
				BgStateReader:     &fakeBgStateReader{files: tt.files},
				BgMissingJobGrace: 30 * time.Second,
			}
			reapBg(cfg, now)

			action, _ := d.GetAction(1)
			if action.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", action.Status, tt.wantStatus)
			}
			if tt.wantResultPiece != "" {
				if !action.Result.Valid || !strings.Contains(action.Result.String, tt.wantResultPiece) {
					t.Errorf("result = %v, want containing %q", action.Result, tt.wantResultPiece)
				}
			}
		})
	}
}

func TestReapBg_NilStateReaderSkips(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "Task", "{}", "")
	if _, err := d.InsertAction("bg-action", taskID,
		`{"instruction":"x","mode":"experimental_bg","daemon_short":"aaaaaaaa"}`,
		db.ActionStatusRunning, nil, "",
	); err != nil {
		t.Fatalf("seed action: %v", err)
	}

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgMissingJobGrace: 30 * time.Second,
	}
	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("status = %q, want %q (reaper must no-op when BgStateReader is nil)", action.Status, db.ActionStatusRunning)
	}
}

func TestDispatchOne_InteractiveBgShareSlotPool(t *testing.T) {
	tests := []struct {
		name              string
		seededInteractive int
		seededBg          int
		newMode           string
		maxInteractive    int
		wantDispatched    bool
		wantClaimedStatus string
		wantInvocationLog string
	}{
		{
			name:              "interactive 1 + bg 1 with cap 3 admits new bg",
			seededInteractive: 1,
			seededBg:          1,
			newMode:           ModeBg,
			maxInteractive:    3,
			wantDispatched:    true,
			wantClaimedStatus: db.ActionStatusRunning,
		},
		{
			name:              "interactive 2 + bg 1 with cap 3 defers new bg",
			seededInteractive: 2,
			seededBg:          1,
			newMode:           ModeBg,
			maxInteractive:    3,
			wantDispatched:    false,
			wantClaimedStatus: db.ActionStatusPending,
		},
		{
			name:              "interactive 1 + bg 2 with cap 3 defers new interactive",
			seededInteractive: 1,
			seededBg:          2,
			newMode:           ModeInteractive,
			maxInteractive:    3,
			wantDispatched:    false,
			wantClaimedStatus: db.ActionStatusPending,
		},
		{
			name:              "bg alone exceeding cap defers new bg",
			seededInteractive: 0,
			seededBg:          3,
			newMode:           ModeBg,
			maxInteractive:    3,
			wantDispatched:    false,
			wantClaimedStatus: db.ActionStatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, _ := d.InsertTask(1, "Task", "{}", "")

			for i := 0; i < tt.seededInteractive; i++ {
				if _, err := d.InsertAction("seed-int", taskID,
					`{"instruction":"x","mode":"interactive"}`,
					db.ActionStatusRunning, nil, "",
				); err != nil {
					t.Fatalf("seed interactive %d: %v", i, err)
				}
			}
			for i := 0; i < tt.seededBg; i++ {
				if _, err := d.InsertAction("seed-bg", taskID,
					`{"instruction":"x","mode":"experimental_bg","daemon_short":"00000000"}`,
					db.ActionStatusRunning, nil, "",
				); err != nil {
					t.Fatalf("seed bg %d: %v", i, err)
				}
			}

			meta := fmt.Sprintf(`{"instruction":"new","mode":%q}`, tt.newMode)
			newID, err := d.InsertAction("new-action", taskID, meta, db.ActionStatusPending, nil, "")
			if err != nil {
				t.Fatalf("seed pending: %v", err)
			}

			interactiveCalls := &countingWorker{}
			bgCalls := &countingWorker{result: "1ff90554"}

			cfg := WorkerConfig{
				DispatchConfig: DispatchConfig{
					DB:                 d,
					InteractiveFunc:    func() Worker { return interactiveCalls },
					NonInteractiveFunc: func() Worker { return interactiveCalls },
					RemoteFunc:         func() Worker { return interactiveCalls },
					BgFunc:             func() Worker { return bgCalls },
					TmuxSession:        "main",
				},
				MaxInteractive: tt.maxInteractive,
			}

			dispatched, err := dispatchOne(context.Background(), cfg)
			if err != nil {
				t.Fatalf("dispatchOne: %v", err)
			}
			if dispatched != tt.wantDispatched {
				t.Errorf("dispatched = %v, want %v", dispatched, tt.wantDispatched)
			}

			action, _ := d.GetAction(newID)
			if action.Status != tt.wantClaimedStatus {
				t.Errorf("new action status = %q, want %q", action.Status, tt.wantClaimedStatus)
			}

			if tt.wantClaimedStatus == db.ActionStatusPending {
				if !action.DispatchAfter.Valid {
					t.Errorf("deferred action should have dispatch_after set, got NULL")
				} else {
					got, err := time.Parse(db.TimeLayout, action.DispatchAfter.String)
					if err != nil {
						t.Errorf("parse dispatch_after: %v", err)
					} else if !got.After(time.Now().UTC().Add(-1 * time.Second)) {
						t.Errorf("dispatch_after = %v, want future", got)
					}
				}
			}
		})
	}
}

// TestRunWorker_DeferredActionYieldsToOtherSlotPool verifies that when the
// shared interactive+bg slot pool is full, a deferred bg action does not
// monopolize NextPending — a higher-id pending noninteractive action (separate
// slot pool) gets a turn within the same poll cycle window thanks to the
// dispatch_after backoff stamped by DeferToPending.
func TestRunWorker_DeferredActionYieldsToOtherSlotPool(t *testing.T) {
	withShortDeferBackoff(t)

	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "Task", "{}", "")

	for i := range 3 {
		if _, err := d.InsertAction("seed-bg", taskID,
			`{"instruction":"x","mode":"experimental_bg","daemon_short":"00000000"}`,
			db.ActionStatusRunning, nil, ""); err != nil {
			t.Fatalf("seed bg %d: %v", i, err)
		}
	}

	bgPendingID, _ := d.InsertAction("bg-A", taskID,
		`{"instruction":"bg-A","mode":"experimental_bg"}`, db.ActionStatusPending, nil, "")
	niPendingID, _ := d.InsertAction("ni-B", taskID,
		`{"instruction":"ni-B","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	niWorker := &countingWorker{result: `{"ok":true}`}
	bgWorker := &countingWorker{result: "1ff90554"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return niWorker },
			InteractiveFunc:    func() Worker { return &countingWorker{} },
			BgFunc:             func() Worker { return bgWorker },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 5,
		PollInterval:      20 * time.Millisecond,
	}

	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, cfg) }()

	waitFor(t, 1*time.Second, "noninteractive dispatched despite shared slot full", func() bool {
		return niWorker.Count() >= 1
	})

	bgA, _ := d.GetAction(bgPendingID)
	if bgA.Status != db.ActionStatusPending {
		t.Errorf("bg-A status = %q, want pending (slot still full)", bgA.Status)
	}
	if !bgA.DispatchAfter.Valid {
		t.Error("bg-A should have dispatch_after set after defer")
	}

	niB, _ := d.GetAction(niPendingID)
	if niB.Status != db.ActionStatusRunning && niB.Status != db.ActionStatusDone {
		t.Errorf("ni-B status = %q, want running or done", niB.Status)
	}

	cancel()
	<-done
}

// TestRunWorker_DeferDoesNotProduceEventStorm verifies that a single pending
// action repeatedly hitting a full slot does not generate one status_changed
// event per poll cycle. With dispatch_after backoff, events are bounded by
// elapsed_time / deferBackoff rather than elapsed_time / pollInterval.
func TestRunWorker_DeferDoesNotProduceEventStorm(t *testing.T) {
	withShortDeferBackoff(t)

	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "Task", "{}", "")

	for i := range 3 {
		if _, err := d.InsertAction("seed-bg", taskID,
			`{"instruction":"x","mode":"experimental_bg","daemon_short":"00000000"}`,
			db.ActionStatusRunning, nil, ""); err != nil {
			t.Fatalf("seed bg %d: %v", i, err)
		}
	}
	bgPendingID, _ := d.InsertAction("bg-A", taskID,
		`{"instruction":"bg-A","mode":"experimental_bg"}`, db.ActionStatusPending, nil, "")

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:              d,
			InteractiveFunc: func() Worker { return &countingWorker{} },
			BgFunc:          func() Worker { return &countingWorker{result: "1ff90554"} },
		},
		MaxInteractive: 3,
		PollInterval:   10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, cfg) }()
	<-done

	events, err := d.ListEvents("action", bgPendingID)
	if err != nil {
		t.Fatal(err)
	}
	var statusChanges int
	for _, e := range events {
		if e.EventType == "action.status_changed" {
			statusChanges++
		}
	}

	// 500ms run ÷ 50ms deferBackoff ≈ 10 cycles. Allow generous headroom.
	// Pre-fix behavior: 500ms ÷ 10ms pollInterval = ~50 cycles. The bound
	// catches both the storm regression and accidental over-defer.
	if statusChanges > 20 {
		t.Errorf("status_changed events = %d, want ≤ 20 (storm regression)", statusChanges)
	}
	if statusChanges == 0 {
		t.Errorf("status_changed events = 0, want ≥ 1 (action should be deferred at least once)")
	}
}

// prefetchSpyStore counts the two work_dir-context SELECTs so tests can assert
// the reaper skips them when the in-memory pre-filter leaves no candidates.
type prefetchSpyStore struct {
	db.Store
	getTasksCalls    int
	getProjectsCalls int
}

func (s *prefetchSpyStore) GetTasksByIDs(ids []int64) (map[int64]*db.Task, error) {
	s.getTasksCalls++
	return s.Store.GetTasksByIDs(ids)
}

func (s *prefetchSpyStore) GetProjectsByIDs(ids []int64) (map[int64]*db.Project, error) {
	s.getProjectsCalls++
	return s.Store.GetProjectsByIDs(ids)
}

func TestReapStaleActions_SkipsPrefetchWhenNoCandidates(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		metadata      string
		startedOffset time.Duration
		withTmux      bool
		wantPrefetch  bool
	}{
		{
			name:          "interactive within grace skips prefetch",
			title:         "fix-conflict",
			metadata:      "{}",
			startedOffset: 0,
			withTmux:      true,
			wantPrefetch:  false,
		},
		{
			name:          "noninteractive within threshold skips prefetch",
			title:         "check-pr",
			metadata:      `{"instruction":"check","mode":"noninteractive"}`,
			startedOffset: -5 * time.Minute,
			wantPrefetch:  false,
		},
		{
			name:          "stale interactive still prefetches (control)",
			title:         "fix-conflict",
			metadata:      "{}",
			startedOffset: -5 * time.Minute,
			withTmux:      true,
			wantPrefetch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
			d.InsertAction(tt.title, taskID, tt.metadata, db.ActionStatusRunning, nil, "")

			started := time.Now().Add(tt.startedOffset)
			d.SetActionTmuxInfoForTest(1, ptr("main"), ptr(WindowName(1)), &started)

			spy := &prefetchSpyStore{Store: d}
			cfg := WorkerConfig{
				DispatchConfig:         DispatchConfig{DB: spy},
				StaleGracePeriod:       30 * time.Second,
				HeartbeatFreshness:     120 * time.Second,
				InteractiveHardTimeout: DefaultInteractiveHardTimeout,
				EarlyDispatchTimeout:   DefaultEarlyDispatchTimeout,
			}
			if tt.withTmux {
				// Window list omits tq-action-1 so a stale action is reapable.
				cfg.TmuxChecker = &mockTmuxChecker{windows: []string{"zsh"}}
			}

			reapStaleActions(context.Background(), cfg)

			gotPrefetch := spy.getTasksCalls > 0 || spy.getProjectsCalls > 0
			if gotPrefetch != tt.wantPrefetch {
				t.Errorf("prefetch called = %v (tasks=%d projects=%d), want %v",
					gotPrefetch, spy.getTasksCalls, spy.getProjectsCalls, tt.wantPrefetch)
			}
		})
	}
}

func TestRunWorker_DependencyBlocksThenReleases(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	dep, _ := d.InsertAction("dep", taskID, `{"instruction":"x","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	_ = d.SetActionStatusForTest(dep, db.ActionStatusRunning)
	follower, _ := d.InsertAction("follower", taskID, `{"instruction":"y","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: dep}}); err != nil {
		t.Fatal(err)
	}

	worker := &countingWorker{result: `{"ok":true}`}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return worker },
			InteractiveFunc:    func() Worker { return worker },
		},
		MaxInteractive: 3,
		PollInterval:   20 * time.Millisecond,
	}
	go func() { _ = RunWorker(ctx, cfg) }()

	// While the dependency is unsatisfied the follower must stay pending.
	time.Sleep(150 * time.Millisecond)
	if a, _ := d.GetAction(follower); a.Status != db.ActionStatusPending {
		t.Fatalf("follower status = %q, want pending while blocked", a.Status)
	}

	// Satisfy the dependency; the worker should pick the follower up.
	if err := d.MarkDone(dep, "ok"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 2*time.Second, "follower dispatched after dep done", func() bool {
		a, _ := d.GetAction(follower)
		return a.Status == db.ActionStatusDone
	})
}

func TestRunWorker_FailedDependencyBlocksForever(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	dep, _ := d.InsertAction("dep", taskID, `{"instruction":"x","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	_ = d.SetActionStatusForTest(dep, db.ActionStatusFailed)
	follower, _ := d.InsertAction("follower", taskID, `{"instruction":"y","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: dep}}); err != nil {
		t.Fatal(err)
	}

	worker := &countingWorker{result: `{"ok":true}`}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: func() Worker { return worker },
			InteractiveFunc:    func() Worker { return worker },
		},
		MaxInteractive: 3,
		PollInterval:   20 * time.Millisecond,
	}
	_ = RunWorker(ctx, cfg)

	if a, _ := d.GetAction(follower); a.Status != db.ActionStatusPending {
		t.Fatalf("follower status = %q, want pending (blocked forever by failed dep)", a.Status)
	}
	if worker.Count() != 0 {
		t.Fatalf("worker.count = %d, want 0 (follower must never dispatch)", worker.Count())
	}
}
