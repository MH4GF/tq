package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func ptr[T any](v T) *T { return &v }

type countingWorker struct {
	count   int
	result  string
	err     error
	denials []PermissionDenial
}

func (w *countingWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	w.count++
	return w.result, w.err
}

func (w *countingWorker) LastDenials() []PermissionDenial { return w.denials }

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

	if worker.count != 1 {
		t.Errorf("worker.count = %d, want 1", worker.count)
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
	d.SetActionSessionInfoForTest(2, ptr("session-1"), nil, nil)

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

	if interactiveWorker.count != 0 {
		t.Errorf("interactive worker called %d times, want 0 (limit reached)", interactiveWorker.count)
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

	if remoteWorker.count != 1 {
		t.Errorf("remote worker called %d times, want 1", remoteWorker.count)
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
	d.SetActionSessionInfoForTest(3, ptr("session-1"), nil, nil)

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

	if remoteWorker.count != 1 {
		t.Errorf("remote worker called %d times, want 1 (should not be limited)", remoteWorker.count)
	}
	if interactiveWorker.count != 0 {
		t.Errorf("interactive worker called %d times, want 0 (limit reached)", interactiveWorker.count)
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

type mockSessionLogChecker struct {
	active    bool
	sessionID string
	err       error
}

func (m *mockSessionLogChecker) IsSessionActive(workDir string, freshnessThreshold time.Duration) (bool, string, error) {
	return m.active, m.sessionID, m.err
}

func TestReapStaleActions_Interactive(t *testing.T) {
	tests := []struct {
		name               string
		startedOffset      time.Duration
		omitStartedAt      bool
		tmux               *mockTmuxChecker
		log                *mockSessionLogChecker
		tmuxSession        string
		wantStatus         string
		wantResultContains string
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
			name:          "skips on tmux error",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{err: fmt.Errorf("tmux not available")},
			wantStatus:    db.ActionStatusRunning,
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
			log:           &mockSessionLogChecker{active: true, sessionID: "sess-123"},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "reaps when session log stale and window gone",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{windows: []string{"zsh"}},
			log:           &mockSessionLogChecker{active: false},
			wantStatus:    db.ActionStatusFailed,
		},
		{
			name:          "reaps via tmux fallback when no log checker",
			startedOffset: -5 * time.Minute,
			tmux:          &mockTmuxChecker{windows: []string{"zsh"}},
			wantStatus:    db.ActionStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
			d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil)

			windowName := "tq-action-1"
			var startedAt *time.Time
			if !tt.omitStartedAt {
				s := time.Now().Add(tt.startedOffset)
				startedAt = &s
			}
			d.SetActionSessionInfoForTest(1, ptr("main"), &windowName, startedAt)

			cfg := WorkerConfig{
				DispatchConfig:     DispatchConfig{DB: d},
				StaleGracePeriod:   30 * time.Second,
				HeartbeatFreshness: 120 * time.Second,
			}
			if tt.tmux != nil {
				cfg.TmuxChecker = tt.tmux
			}
			if tt.log != nil {
				cfg.SessionLogChecker = tt.log
			}
			if tt.tmuxSession != "" {
				cfg.TmuxSession = tt.tmuxSession
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
		log                *mockSessionLogChecker
		wantStatus         string
		wantResultContains string
		wantMetaSessionID  string
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
			log:           &mockSessionLogChecker{active: true, sessionID: "sess-456"},
			wantStatus:    db.ActionStatusRunning,
		},
		{
			name:          "reaped by stale heartbeat",
			startedOffset: -25 * time.Minute,
			log:           &mockSessionLogChecker{active: false},
			wantStatus:    db.ActionStatusFailed,
		},
		{
			name:          "reaped when checker errors",
			startedOffset: -25 * time.Minute,
			log:           &mockSessionLogChecker{err: fmt.Errorf("permission denied")},
			wantStatus:    db.ActionStatusFailed,
		},
		{
			name:              "saves session id to metadata",
			startedOffset:     -25 * time.Minute,
			log:               &mockSessionLogChecker{active: true, sessionID: "sess-789"},
			wantStatus:        db.ActionStatusRunning,
			wantMetaSessionID: "sess-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
			d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)

			if !tt.omitStartedAt {
				started := time.Now().Add(tt.startedOffset)
				d.SetActionSessionInfoForTest(1, nil, nil, &started)
			}

			cfg := WorkerConfig{
				DispatchConfig:     DispatchConfig{DB: d},
				HeartbeatFreshness: 120 * time.Second,
			}
			if tt.log != nil {
				cfg.SessionLogChecker = tt.log
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
			if tt.wantMetaSessionID != "" {
				var meta map[string]any
				if err := json.Unmarshal([]byte(action.Metadata), &meta); err != nil {
					t.Fatalf("parse metadata: %v", err)
				}
				if meta["claude_session_id"] != tt.wantMetaSessionID {
					t.Errorf("claude_session_id = %v, want %q", meta["claude_session_id"], tt.wantMetaSessionID)
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
