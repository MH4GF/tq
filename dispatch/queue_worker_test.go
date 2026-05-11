package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func ptr[T any](v T) *T { return &v }

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
	d.InsertAction("check-pr-status", taskID, `{"instruction":"check pr status","mode":"noninteractive"}`, db.ActionStatusPending, nil)

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
	d.InsertAction("fix-conflict", taskID, `{"instruction":"fix conflict","mode":"interactive"}`, db.ActionStatusPending, nil)

	d.InsertAction("respond-review", taskID, "{}", db.ActionStatusRunning, nil)
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
	d.InsertAction("check-pr-status", taskID, `{"instruction":"check pr status","mode":"noninteractive"}`, db.ActionStatusPending, nil)

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

func TestRunWorker_FailureCreatesInvestigateAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr-status", taskID, `{"instruction":"check pr status","mode":"noninteractive"}`, "pending", nil)

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

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions, got %d", len(actions))
	}

	investigate := actions[0]
	var meta map[string]any
	if err := json.Unmarshal([]byte(investigate.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if _, ok := meta["is_investigate_failure"]; !ok {
		t.Errorf("metadata missing is_investigate_failure key, got %v", meta)
	}
	if investigate.TaskID != taskID {
		t.Errorf("follow-up task_id = %d, want %d", investigate.TaskID, taskID)
	}
}

func TestRunWorker_RemoteDispatch(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Remote task", `{"url":"https://example.com"}`, "")
	d.InsertAction("remote-task", taskID, `{"instruction":"do remote task","mode":"remote"}`, db.ActionStatusPending, nil)

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
	d.InsertAction("remote-task", taskID, `{"instruction":"do remote task","mode":"remote"}`, db.ActionStatusPending, nil)
	d.InsertAction("fix-conflict", taskID, `{"instruction":"fix conflict","mode":"interactive"}`, db.ActionStatusPending, nil)

	d.InsertAction("respond-review", taskID, `{"instruction":"respond to review","mode":"interactive"}`, db.ActionStatusRunning, nil)
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
			d.InsertAction("fix-conflict", taskID, meta, db.ActionStatusRunning, nil)

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
			d.InsertAction("check-pr", taskID, meta, db.ActionStatusRunning, nil)

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

func TestReapStaleActions_MultipleStaleInteractiveAllReaped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")

	staleAt := time.Now().Add(-5 * time.Minute)
	const n = 3
	for i := range n {
		if _, err := d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil); err != nil {
			t.Fatalf("seed action %d: %v", i, err)
		}
	}
	for i := int64(1); i <= n; i++ {
		d.SetActionTmuxInfoForTest(i, ptr("main"), ptr(WindowName(i)), &staleAt)
	}

	cfg := WorkerConfig{
		DispatchConfig:         DispatchConfig{DB: d},
		StaleGracePeriod:       30 * time.Second,
		HeartbeatFreshness:     120 * time.Second,
		InteractiveHardTimeout: DefaultInteractiveHardTimeout,
		EarlyDispatchTimeout:   DefaultEarlyDispatchTimeout,
		TmuxChecker:            &mockTmuxChecker{windows: []string{"zsh"}},
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
}

func TestReapStaleActions_MultipleStaleNonInteractiveAllReaped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")

	staleAt := time.Now().Add(-25 * time.Minute)
	const n = 3
	for i := range n {
		if _, err := d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil); err != nil {
			t.Fatalf("seed action %d: %v", i, err)
		}
	}
	for i := int64(1); i <= n; i++ {
		d.SetActionTmuxInfoForTest(i, nil, nil, &staleAt)
	}

	cfg := WorkerConfig{
		DispatchConfig:     DispatchConfig{DB: d},
		HeartbeatFreshness: 120 * time.Second,
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
	d.InsertAction("long-ni", taskID, `{"instruction":"long","mode":"noninteractive"}`, db.ActionStatusPending, nil)
	d.InsertAction("interactive", taskID, `{"instruction":"fix","mode":"interactive"}`, db.ActionStatusPending, nil)

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
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("ni-1", taskID, `{"instruction":"ni-1","mode":"noninteractive"}`, db.ActionStatusPending, nil)
	d.InsertAction("ni-2", taskID, `{"instruction":"ni-2","mode":"noninteractive"}`, db.ActionStatusPending, nil)

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
	d.InsertAction("ni-fail", taskID, `{"instruction":"fail","mode":"noninteractive"}`, db.ActionStatusPending, nil)

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
	d.InsertAction("ni-after", taskID, `{"instruction":"after","mode":"noninteractive"}`, db.ActionStatusPending, nil)
	failedThenOK.Set(`{"ok":true}`, nil)

	waitFor(t, 1*time.Second, "follow-up noninteractive dispatched after failure", func() bool {
		// id 2 is the investigate-failure action created from #1's failure.
		// id 3 is our manually queued ni-after.
		a, err := d.GetAction(3)
		return err == nil && (a.Status == db.ActionStatusRunning || a.Status == db.ActionStatusDone)
	})

	cancel()
	<-done
}

// TestRunWorker_ShutdownDoesNotMarkInflightFailed verifies that ctx cancel
// during shutdown does NOT mark in-flight noninteractive actions as failed
// (and does not create investigate-failure follow-ups). They should remain
// running so the next worker session's stale reaper can re-evaluate.
func TestRunWorker_ShutdownDoesNotMarkInflightFailed(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("long-ni", taskID, `{"instruction":"long","mode":"noninteractive"}`, db.ActionStatusPending, nil)

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

	actions, _ := d.ListActions("", nil, 0)
	for _, x := range actions {
		if hasMetaKey(x.Metadata, MetaKeyIsInvestigation) {
			t.Errorf("shutdown created spurious investigate-failure action #%d", x.ID)
		}
	}
}
