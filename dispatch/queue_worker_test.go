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
	count  int
	result string
	err    error
}

func (w *countingWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	w.count++
	return w.result, w.err
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

type mockTmuxChecker struct {
	windows       []string
	err           error
	calledSession string
}

func (m *mockTmuxChecker) ListWindows(ctx context.Context, session string) ([]string, error) {
	m.calledSession = session
	return m.windows, m.err
}

func TestReapStaleActions_DetectsStale(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), ptr(time.Now().Add(-5*time.Minute)))

	checker := &mockTmuxChecker{windows: []string{"zsh", "other-window"}}

	cfg := WorkerConfig{
		DispatchConfig:   DispatchConfig{DB: d},
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusFailed)
	}
	if !action.Result.Valid || !strings.Contains(action.Result.String, "stale") {
		t.Errorf("expected result containing 'stale', got %v", action.Result)
	}
}

func TestReapStaleActions_SkipsLiveWindows(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), ptr(time.Now().Add(-5*time.Minute)))

	checker := &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}}

	cfg := WorkerConfig{
		DispatchConfig:   DispatchConfig{DB: d},
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusRunning)
	}
}

func TestReapStaleActions_GracePeriod(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), ptr(time.Now()))

	checker := &mockTmuxChecker{windows: []string{"zsh"}}

	cfg := WorkerConfig{
		DispatchConfig:   DispatchConfig{DB: d},
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q (within grace period)", action.Status, db.ActionStatusRunning)
	}
}

func TestReapStaleActions_TmuxError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), ptr(time.Now().Add(-5*time.Minute)))

	checker := &mockTmuxChecker{err: fmt.Errorf("tmux not available")}

	cfg := WorkerConfig{
		DispatchConfig:   DispatchConfig{DB: d},
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q (tmux error should skip)", action.Status, db.ActionStatusRunning)
	}
}

func TestReapStaleActions_NilChecker(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), nil)

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{DB: d},
		TmuxChecker:    nil,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q (nil checker should no-op)", action.Status, db.ActionStatusRunning)
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

func TestReapStaleActions_CustomSession(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, "{}", db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("work"), ptr("tq-action-1"), ptr(time.Now().Add(-5*time.Minute)))

	checker := &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}}

	cfg := WorkerConfig{
		DispatchConfig:   DispatchConfig{DB: d, TmuxSession: "work"},
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	if checker.calledSession != "work" {
		t.Errorf("ListWindows called with session %q, want %q", checker.calledSession, "work")
	}

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusRunning)
	}
}

func TestReapStaleActions_NonInteractiveStale(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, nil, nil, ptr(time.Now().Add(-25*time.Minute)))

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{DB: d},
		TmuxChecker:    nil,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusFailed)
	}
	if !action.Result.Valid || !strings.Contains(action.Result.String, "noninteractive") {
		t.Errorf("expected result containing 'noninteractive', got %v", action.Result)
	}
}

func TestReapStaleActions_NonInteractiveNotYetStale(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, nil, nil, ptr(time.Now().Add(-5*time.Minute)))

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{DB: d},
		TmuxChecker:    nil,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q (within threshold)", action.Status, db.ActionStatusRunning)
	}
}

func TestReapStaleActions_NonInteractiveNoStartedAt(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{DB: d},
		TmuxChecker:    nil,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q (no started_at)", action.Status, db.ActionStatusRunning)
	}
}

type mockSessionLogChecker struct {
	active    bool
	sessionID string
	err       error
}

func (m *mockSessionLogChecker) IsSessionActive(workDir string, freshnessThreshold time.Duration) (bool, string, error) {
	return m.active, m.sessionID, m.err
}

func TestReapStaleActions_InteractiveLogFresh(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, `{"instruction":"fix"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), ptr(time.Now().Add(-5*time.Minute)))

	// tmux window is gone, but session log is fresh
	checker := &mockTmuxChecker{windows: []string{"zsh"}}
	sessionChecker := &mockSessionLogChecker{active: true, sessionID: "sess-123"}

	cfg := WorkerConfig{
		DispatchConfig:     DispatchConfig{DB: d, SessionLogChecker: sessionChecker},
		TmuxChecker:        checker,
		StaleGracePeriod:   30 * time.Second,
		HeartbeatFreshness: 120 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q (session log fresh)", action.Status, db.ActionStatusRunning)
	}
}

func TestReapStaleActions_InteractiveLogStale(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, `{"instruction":"fix"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), ptr(time.Now().Add(-5*time.Minute)))

	// tmux window gone AND session log stale
	checker := &mockTmuxChecker{windows: []string{"zsh"}}
	sessionChecker := &mockSessionLogChecker{active: false}

	cfg := WorkerConfig{
		DispatchConfig:     DispatchConfig{DB: d, SessionLogChecker: sessionChecker},
		TmuxChecker:        checker,
		StaleGracePeriod:   30 * time.Second,
		HeartbeatFreshness: 120 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusFailed)
	}
}

func TestReapStaleActions_InteractiveNilChecker(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("fix-conflict", taskID, `{"instruction":"fix"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, ptr("main"), ptr("tq-action-1"), ptr(time.Now().Add(-5*time.Minute)))

	// No session log checker, tmux window gone → fallback to tmux check → reaped
	checker := &mockTmuxChecker{windows: []string{"zsh"}}

	cfg := WorkerConfig{
		DispatchConfig:   DispatchConfig{DB: d},
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q (nil checker, window gone)", action.Status, db.ActionStatusFailed)
	}
}

func TestReapStaleActions_NonInteractiveSkippedByHeartbeat(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, nil, nil, ptr(time.Now().Add(-25*time.Minute)))

	sessionChecker := &mockSessionLogChecker{active: true, sessionID: "sess-456"}

	cfg := WorkerConfig{
		DispatchConfig:     DispatchConfig{DB: d, SessionLogChecker: sessionChecker},
		HeartbeatFreshness: 120 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Errorf("action status = %q, want %q (heartbeat active)", action.Status, db.ActionStatusRunning)
	}
}

func TestReapStaleActions_NonInteractiveReapedByStaleHeartbeat(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, nil, nil, ptr(time.Now().Add(-25*time.Minute)))

	sessionChecker := &mockSessionLogChecker{active: false}

	cfg := WorkerConfig{
		DispatchConfig:     DispatchConfig{DB: d, SessionLogChecker: sessionChecker},
		HeartbeatFreshness: 120 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusFailed)
	}
}

func TestReapStaleActions_NonInteractiveCheckerError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, nil, nil, ptr(time.Now().Add(-25*time.Minute)))

	sessionChecker := &mockSessionLogChecker{err: fmt.Errorf("permission denied")}

	cfg := WorkerConfig{
		DispatchConfig:     DispatchConfig{DB: d, SessionLogChecker: sessionChecker},
		HeartbeatFreshness: 120 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q (checker error → fallthrough)", action.Status, db.ActionStatusFailed)
	}
}

func TestReapStaleActions_SavesSessionIdToMetadata(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)
	d.SetActionSessionInfoForTest(1, nil, nil, ptr(time.Now().Add(-25*time.Minute)))

	sessionChecker := &mockSessionLogChecker{active: true, sessionID: "sess-789"}

	cfg := WorkerConfig{
		DispatchConfig:     DispatchConfig{DB: d, SessionLogChecker: sessionChecker},
		HeartbeatFreshness: 120 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusRunning {
		t.Fatalf("action status = %q, want %q", action.Status, db.ActionStatusRunning)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(action.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if meta["claude_session_id"] != "sess-789" {
		t.Errorf("claude_session_id = %v, want %q", meta["claude_session_id"], "sess-789")
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
