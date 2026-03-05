package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmpl "github.com/MH4GF/tq/template"
	"github.com/MH4GF/tq/testutil"
)

type countingWorker struct {
	count  int
	result string
	err    error
}

func (w *countingWorker) Execute(ctx context.Context, prompt string, cfg tmpl.Config, workDir string, actionID int64) (string, error) {
	w.count++
	return w.result, w.err
}

func setupTemplatesDir(t *testing.T) string {
	t.Helper()
	tqDir := t.TempDir()
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeTestTemplate(t, templatesDir, "check-pr-status", false)
	writeTestTemplate(t, templatesDir, "fix-conflict", true)
	writeTestTemplate(t, templatesDir, "respond-review", true)
	writeTestTemplate(t, templatesDir, "fix-ci", true)
	writeTestTemplate(t, templatesDir, "merge-pr", true)
	return tqDir
}

func writeTestTemplate(t *testing.T, dir, name string, interactive bool) {
	t.Helper()
	writeTestTemplateWithOnDone(t, dir, name, interactive, "")
}

func writeTestTemplateWithOnDone(t *testing.T, dir, name string, interactive bool, onDone string) {
	t.Helper()
	interactiveStr := "false"
	if interactive {
		interactiveStr = "true"
	}

	onDoneLine := ""
	if onDone != "" {
		onDoneLine = fmt.Sprintf("on_done: %s\n", onDone)
	}

	content := fmt.Sprintf(`---
description: %s
interactive: %s
%s---
Do %s for {{.Task.Title}}.
`, name, interactiveStr, onDoneLine, name)

	os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644)
}

func TestRalphLoop_ProcessesAndStops(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	d.InsertAction("check-pr-status", &taskID, "{}", "pending", "test")

	tqDir := setupTemplatesDir(t)

	worker := &countingWorker{result: `{"ok":true}`}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := RalphConfig{
		TQDir:          tqDir,
		DB:             d,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func(tqDir string) Worker {
			return worker
		},
		InteractiveFunc: func(tqDir string) Worker {
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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	d.InsertAction("fix-conflict", &taskID, "{}", "pending", "test")

	tqDir := setupTemplatesDir(t)

	// Simulate an already-running interactive session
	d.InsertAction("respond-review", &taskID, "{}", "running", "test")
	d.Exec("UPDATE actions SET session_id = 'session-1' WHERE id = 2")

	interactiveWorker := &countingWorker{result: "interactive:session=test"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := RalphConfig{
		TQDir:          tqDir,
		DB:             d,
		MaxInteractive: 1,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func(tqDir string) Worker {
			return &countingWorker{result: `{"ok":true}`}
		},
		InteractiveFunc: func(tqDir string) Worker {
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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	d.InsertAction("check-pr-status", &taskID, "{}", "pending", "test")

	tqDir := setupTemplatesDir(t)

	worker := &countingWorker{err: context.DeadlineExceeded}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cfg := RalphConfig{
		TQDir:          tqDir,
		DB:             d,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func(tqDir string) Worker {
			return worker
		},
		InteractiveFunc: func(tqDir string) Worker {
			return worker
		},
	}

	_ = RalphLoop(ctx, cfg)

	action, _ := d.GetAction(1)
	if action.Status != "waiting_human" {
		t.Errorf("action status = %q, want waiting_human", action.Status)
	}
}

func TestRalphLoop_OnDoneTriggersFollowUp(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := t.TempDir()
	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	writeTestTemplateWithOnDone(t, templatesDir, "check-pr", false, "review")
	writeTestTemplate(t, templatesDir, "review", false)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	d.InsertAction("check-pr", &taskID, "{}", "pending", "test")

	worker := &countingWorker{result: `{"status":"merged"}`}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg := RalphConfig{
		TQDir:          tqDir,
		DB:             d,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func(tqDir string) Worker {
			return worker
		},
		InteractiveFunc: func(tqDir string) Worker {
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
	if review.TemplateID != "review" {
		t.Errorf("follow-up template = %q, want review", review.TemplateID)
	}
	if review.Source != "on_done" {
		t.Errorf("follow-up source = %q, want on_done", review.Source)
	}
}

type mockTmuxChecker struct {
	windows []string
	err     error
}

func (m *mockTmuxChecker) ListWindows(ctx context.Context, session string) ([]string, error) {
	return m.windows, m.err
}

func TestReapStaleActions_DetectsStale(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	d.InsertAction("fix-conflict", &taskID, "{}", "running", "test")
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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	d.InsertAction("fix-conflict", &taskID, "{}", "running", "test")
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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	d.InsertAction("fix-conflict", &taskID, "{}", "running", "test")
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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	d.InsertAction("fix-conflict", &taskID, "{}", "running", "test")
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

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	d.InsertAction("fix-conflict", &taskID, "{}", "running", "test")
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

func TestDispatchOne_NoPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tqDir := setupTemplatesDir(t)

	cfg := RalphConfig{
		TQDir:        tqDir,
		DB:           d,
		PollInterval: 50 * time.Millisecond,
		NonInteractiveFunc: func(tqDir string) Worker {
			return &countingWorker{}
		},
		InteractiveFunc: func(tqDir string) Worker {
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
