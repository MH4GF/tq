package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/prompt"
	"github.com/MH4GF/tq/testutil"
)

type countingWorker struct {
	count  int
	result string
	err    error
}

func (w *countingWorker) Execute(ctx context.Context, prompt string, cfg prompt.Config, workDir string, actionID int64) (string, error) {
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

func writeTestPromptWithMode(t *testing.T, dir, name, mode, onDone string) {
	t.Helper()
	onDoneLine := ""
	if onDone != "" {
		onDoneLine = fmt.Sprintf("on_done: %s\n", onDone)
	}
	content := fmt.Sprintf(`---
description: %s
mode: %s
%s---
Do %s for {{.Task.Title}}.
`, name, mode, onDoneLine, name)
	os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644)
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

	onDoneLine := ""
	if onDone != "" {
		onDoneLine = fmt.Sprintf("on_done: %s\n", onDone)
	}

	content := fmt.Sprintf(`---
description: %s
mode: %s
%s---
Do %s for {{.Task.Title}}.
`, name, mode, onDoneLine, name)

	os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644)
}

func TestRalphLoop_ProcessesAndStops(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	d.InsertAction("check-pr-status", "check-pr-status", &taskID, "{}", "pending")

	tqDir := setupPromptsDir(t)

	worker := &countingWorker{result: `{"ok":true}`}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := RalphConfig{
		UserConfigDir:          tqDir,
		DB:             d,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func() Worker {
			return worker
		},
		InteractiveFunc: func() Worker {
			return worker
		},
	}

	err := RalphLoop(ctx, cfg)
	if err != context.DeadlineExceeded {
		t.Fatalf("RalphLoop error = %v, want context.DeadlineExceeded", err)
	}

	if worker.count != 1 {
		t.Errorf("worker.count = %d, want 1", worker.count)
	}

	action, _ := d.GetAction(1)
	if action.Status != "done" {
		t.Errorf("action status = %q, want done", action.Status)
	}
}

func TestRalphLoop_InteractiveLimitEnforced(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "pending")

	tqDir := setupPromptsDir(t)

	// Simulate an already-running interactive session
	d.InsertAction("respond-review", "respond-review", &taskID, "{}", "running")
	d.Exec("UPDATE actions SET session_id = 'session-1' WHERE id = 2")

	interactiveWorker := &countingWorker{result: "interactive:session=test"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := RalphConfig{
		UserConfigDir:          tqDir,
		DB:             d,
		MaxInteractive: 1,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func() Worker {
			return &countingWorker{result: `{"ok":true}`}
		},
		InteractiveFunc: func() Worker {
			return interactiveWorker
		},
	}

	_ = RalphLoop(ctx, cfg)

	if interactiveWorker.count != 0 {
		t.Errorf("interactive worker called %d times, want 0 (limit reached)", interactiveWorker.count)
	}
}

func TestRalphLoop_FailureEscalation(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("check-pr-status", "check-pr-status", &taskID, "{}", "pending")

	tqDir := setupPromptsDir(t)

	worker := &countingWorker{err: context.DeadlineExceeded}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := RalphConfig{
		UserConfigDir:          tqDir,
		DB:             d,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func() Worker {
			return worker
		},
		InteractiveFunc: func() Worker {
			return worker
		},
	}

	_ = RalphLoop(ctx, cfg)

	action, _ := d.GetAction(1)
	if action.Status != "failed" {
		t.Errorf("action status = %q, want failed", action.Status)
	}
}

func TestRalphLoop_OnDoneTriggersFollowUp(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0o755)

	writeTestPromptWithOnDone(t, promptsDir, "check-pr", false, "review")
	writeTestPrompt(t, promptsDir, "review", false)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	d.InsertAction("check-pr", "check-pr", &taskID, "{}", "pending")

	worker := &countingWorker{result: `{"status":"merged"}`}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := RalphConfig{
		UserConfigDir:          tqDir,
		DB:             d,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func() Worker {
			return worker
		},
		InteractiveFunc: func() Worker {
			return worker
		},
	}

	err := RalphLoop(ctx, cfg)
	if err != context.DeadlineExceeded {
		t.Fatalf("RalphLoop error = %v, want context.DeadlineExceeded", err)
	}

	// check-pr should be done
	action, _ := d.GetAction(1)
	if action.Status != "done" {
		t.Errorf("check-pr status = %q, want done", action.Status)
	}

	// review should have been auto-created as pending
	actions, _ := d.ListActions("", nil)
	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions, got %d", len(actions))
	}

	review := actions[1]
	if review.PromptID != "review" {
		t.Errorf("follow-up template = %q, want review", review.PromptID)
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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "running")
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

	checker := &mockTmuxChecker{windows: []string{"zsh", "other-window"}}

	cfg := RalphConfig{
		DB:               d,
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != "failed" {
		t.Errorf("action status = %q, want failed", action.Status)
	}
	if !action.Result.Valid || !strings.Contains(action.Result.String, "stale") {
		t.Errorf("expected result containing 'stale', got %v", action.Result)
	}
}

func TestReapStaleActions_SkipsLiveWindows(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "running")
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

	checker := &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}}

	cfg := RalphConfig{
		DB:               d,
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != "running" {
		t.Errorf("action status = %q, want running", action.Status)
	}
}

func TestReapStaleActions_GracePeriod(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "running")
	// started_at is now (within grace period)
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now') WHERE id = 1")

	checker := &mockTmuxChecker{windows: []string{"zsh"}}

	cfg := RalphConfig{
		DB:               d,
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != "running" {
		t.Errorf("action status = %q, want running (within grace period)", action.Status)
	}
}

func TestReapStaleActions_TmuxError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "running")
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

	checker := &mockTmuxChecker{err: fmt.Errorf("tmux not available")}

	cfg := RalphConfig{
		DB:               d,
		TmuxChecker:      checker,
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != "running" {
		t.Errorf("action status = %q, want running (tmux error should skip)", action.Status)
	}
}

func TestReapStaleActions_NilChecker(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "running")
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1' WHERE id = 1")

	cfg := RalphConfig{
		DB:          d,
		TmuxChecker: nil,
	}

	// Should not panic
	reapStaleActions(context.Background(), cfg)

	action, _ := d.GetAction(1)
	if action.Status != "running" {
		t.Errorf("action status = %q, want running (nil checker should no-op)", action.Status)
	}
}

func TestRalphLoop_RemoteDispatch(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Remote task", "https://example.com", "{}", "")
	d.InsertAction("remote-task", "remote-task", &taskID, "{}", "pending")

	tqDir := setupPromptsDir(t)

	remoteWorker := &countingWorker{result: "remote:session=https://console.anthropic.com/p/abc"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := RalphConfig{
		UserConfigDir:  tqDir,
		DB:             d,
		MaxInteractive: 1,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func() Worker {
			return &countingWorker{result: `{"ok":true}`}
		},
		InteractiveFunc: func() Worker {
			return &countingWorker{result: "interactive:action=1"}
		},
		RemoteFunc: func() Worker {
			return remoteWorker
		},
	}

	_ = RalphLoop(ctx, cfg)

	if remoteWorker.count != 1 {
		t.Errorf("remote worker called %d times, want 1", remoteWorker.count)
	}

	action, _ := d.GetAction(1)
	if action.Status != "running" {
		t.Errorf("action status = %q, want running (fire-and-forget)", action.Status)
	}
}

func TestRalphLoop_RemoteDoesNotCountTowardInteractiveLimit(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	// First: a remote action (pending)
	d.InsertAction("remote-task", "remote-task", &taskID, "{}", "pending")
	// Second: an interactive action (pending)
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "pending")

	// Simulate an already-running interactive session to fill max
	d.InsertAction("respond-review", "respond-review", &taskID, "{}", "running")
	d.Exec("UPDATE actions SET session_id = 'session-1' WHERE id = 3")

	tqDir := setupPromptsDir(t)

	remoteWorker := &countingWorker{result: "remote:session=https://example.com"}
	interactiveWorker := &countingWorker{result: "interactive:action=2"}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cfg := RalphConfig{
		UserConfigDir:  tqDir,
		DB:             d,
		MaxInteractive: 1,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func() Worker {
			return &countingWorker{result: `{"ok":true}`}
		},
		InteractiveFunc: func() Worker {
			return interactiveWorker
		},
		RemoteFunc: func() Worker {
			return remoteWorker
		},
	}

	_ = RalphLoop(ctx, cfg)

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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}", "")
	d.InsertAction("fix-conflict", "fix-conflict", &taskID, "{}", "running")
	d.Exec("UPDATE actions SET session_id = 'work', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")

	checker := &mockTmuxChecker{windows: []string{"zsh", "tq-action-1"}}

	cfg := RalphConfig{
		DB:               d,
		TmuxChecker:      checker,
		TmuxSession:      "work",
		StaleGracePeriod: 30 * time.Second,
	}

	reapStaleActions(context.Background(), cfg)

	if checker.calledSession != "work" {
		t.Errorf("ListWindows called with session %q, want %q", checker.calledSession, "work")
	}

	action, _ := d.GetAction(1)
	if action.Status != "running" {
		t.Errorf("action status = %q, want running", action.Status)
	}
}

func TestDispatchOne_NoPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := setupPromptsDir(t)

	cfg := RalphConfig{
		UserConfigDir:        tqDir,
		DB:           d,
		PollInterval: 50 * time.Millisecond,
		NonInteractiveFunc: func() Worker {
			return &countingWorker{}
		},
		InteractiveFunc: func() Worker {
			return &countingWorker{}
		},
	}

	dispatched, err := dispatchOne(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dispatchOne error: %v", err)
	}
	if dispatched {
		t.Error("expected dispatched=false when no pending actions")
	}
}
