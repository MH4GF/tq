package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/prompt"
	"github.com/MH4GF/tq/testutil"
)

type countingWorker struct {
	count  int
	result string
	err    error
}

func (w *countingWorker) Execute(ctx context.Context, prompt string, cfg prompt.Config, workDir string, actionID, taskID int64) (string, error) {
	w.count++
	return w.result, w.err
}

func setupPromptsDir(t *testing.T) string {
	t.Helper()
	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeTestPrompt(t, promptsDir, "check-pr-status", false)
	writeTestPrompt(t, promptsDir, "fix-conflict", true)
	writeTestPrompt(t, promptsDir, "respond-review", true)
	writeTestPrompt(t, promptsDir, "fix-ci", true)
	writeTestPrompt(t, promptsDir, "merge-pr", true)
	writeTestPromptWithMode(t, promptsDir, "remote-task", "remote", "")
	return tqDir
}

func writeTestPromptFull(t *testing.T, dir, name, mode, onDone, onCancel string) {
	t.Helper()
	hooks := ""
	if onDone != "" {
		hooks += fmt.Sprintf("on_done: %s\n", onDone)
	}
	if onCancel != "" {
		hooks += fmt.Sprintf("on_cancel: %s\n", onCancel)
	}
	content := fmt.Sprintf(`---
description: %s
mode: %s
%s---
Do %s for {{.Task.Title}}.
`, name, mode, hooks, name)
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write prompt %q: %v", name, err)
	}
}

func writeTestPromptWithMode(t *testing.T, dir, name, mode, onDone string) {
	t.Helper()
	writeTestPromptFull(t, dir, name, mode, onDone, "")
}

func writeTestPrompt(t *testing.T, dir, name string, interactive bool) {
	t.Helper()
	writeTestPromptWithOnDone(t, dir, name, interactive, "")
}

func writeTestPromptWithOnDone(t *testing.T, dir, name string, interactive bool, onDone string) {
	t.Helper()
	mode := "noninteractive"
	if interactive {
		mode = "interactive"
	}
	writeTestPromptFull(t, dir, name, mode, onDone, "")
}

func TestRunWorker_ProcessesAndStops(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr-status", "check-pr-status", taskID, "{}", db.ActionStatusPending)

	tqDir := setupPromptsDir(t)

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
		UserConfigDir:  tqDir,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
	}

	err := RunWorker(ctx, cfg)
	if err != context.DeadlineExceeded {
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
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusPending)

	tqDir := setupPromptsDir(t)

	// Simulate an already-running interactive session
	d.InsertAction("respond-review", "respond-review", taskID, "{}", db.ActionStatusRunning)
	d.Exec("UPDATE actions SET session_id = 'session-1' WHERE id = 2")

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
		UserConfigDir:  tqDir,
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
	d.InsertAction("check-pr-status", "check-pr-status", taskID, "{}", db.ActionStatusPending)

	tqDir := setupPromptsDir(t)

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
		UserConfigDir:  tqDir,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
	}

	_ = RunWorker(ctx, cfg)

	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusFailed {
		t.Errorf("action status = %q, want %q", action.Status, db.ActionStatusFailed)
	}
}

func TestRunWorker_OnDoneTriggersFollowUp(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeTestPromptWithOnDone(t, promptsDir, "check-pr", false, "review")
	writeTestPrompt(t, promptsDir, "review", false)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", "check-pr", taskID, "{}", db.ActionStatusPending)

	worker := &countingWorker{result: `{"status":"merged"}`}

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
		UserConfigDir:  tqDir,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
	}

	err := RunWorker(ctx, cfg)
	if err != context.DeadlineExceeded {
		t.Fatalf("RunWorker error = %v, want context.DeadlineExceeded", err)
	}

	// check-pr should be done
	action, _ := d.GetAction(1)
	if action.Status != db.ActionStatusDone {
		t.Errorf("check-pr status = %q, want %q", action.Status, db.ActionStatusDone)
	}

	// review should have been auto-created as pending
	actions, _ := d.ListActions("", nil, 0)
	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions, got %d", len(actions))
	}

	review := actions[0]
	if review.PromptID != "review" {
		t.Errorf("follow-up template = %q, want review", review.PromptID)
	}
}

func TestRunWorker_FailureCreatesInvestigateAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := setupPromptsDir(t)

	taskID, _ := d.InsertTask(1, "Test task", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr-status", "check-pr-status", taskID, "{}", "pending")

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
		UserConfigDir:  tqDir,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
	}

	_ = RunWorker(ctx, cfg)

	// Original action should be failed
	action, _ := d.GetAction(1)
	if action.Status != "failed" {
		t.Errorf("action status = %q, want failed", action.Status)
	}

	// Investigate-failure action should have been auto-created
	actions, _ := d.ListActions("", nil, 0)
	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions, got %d", len(actions))
	}

	investigate := actions[0]
	if investigate.PromptID != "internal:investigate-failure" {
		t.Errorf("follow-up prompt_id = %q, want internal:investigate-failure", investigate.PromptID)
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
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusRunning)
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

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
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusRunning)
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

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
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusRunning)
	// started_at is now (within grace period)
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now') WHERE id = 1")

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
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusRunning)
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

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
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusRunning)
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1' WHERE id = 1")

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{DB: d},
		TmuxChecker:    nil,
	}

	// Should not panic
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
	d.InsertAction("remote-task", "remote-task", taskID, "{}", db.ActionStatusPending)

	tqDir := setupPromptsDir(t)

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
		UserConfigDir:  tqDir,
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
	// First: a remote action (pending)
	d.InsertAction("remote-task", "remote-task", taskID, "{}", db.ActionStatusPending)
	// Second: an interactive action (pending)
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusPending)

	// Simulate an already-running interactive session to fill max
	d.InsertAction("respond-review", "respond-review", taskID, "{}", db.ActionStatusRunning)
	d.Exec("UPDATE actions SET session_id = 'session-1' WHERE id = 3")

	tqDir := setupPromptsDir(t)

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
		UserConfigDir:  tqDir,
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
	d.InsertAction("fix-conflict", "fix-conflict", taskID, "{}", db.ActionStatusRunning)
	d.Exec("UPDATE actions SET session_id = 'work', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

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

func TestDispatchOne_NoPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := setupPromptsDir(t)

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
		UserConfigDir: tqDir,
		PollInterval:  50 * time.Millisecond,
	}

	dispatched, err := dispatchOne(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dispatchOne error: %v", err)
	}
	if dispatched {
		t.Error("expected dispatched=false when no pending actions")
	}
}
