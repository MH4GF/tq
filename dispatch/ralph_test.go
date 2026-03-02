package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	writeTestTemplate(t, templatesDir, "check-pr-status", false, 0)
	writeTestTemplate(t, templatesDir, "fix-conflict", true, 0)
	writeTestTemplate(t, templatesDir, "respond-review", true, 0)
	writeTestTemplate(t, templatesDir, "fix-ci", true, 1)
	writeTestTemplate(t, templatesDir, "merge-pr", true, 0)
	return tqDir
}

func writeTestTemplate(t *testing.T, dir, name string, interactive bool, maxRetries int) {
	t.Helper()
	interactiveStr := "false"
	if interactive {
		interactiveStr = "true"
	}

	content := fmt.Sprintf(`---
description: %s
auto: true
interactive: %s
timeout: 10
max_retries: %d
---
Do %s for {{.Task.Title}}.
`, name, interactiveStr, maxRetries, name)

	os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644)
}

func TestRalphLoop_ProcessesAndStops(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}")
	d.InsertAction("check-pr-status", &taskID, "{}", "pending", 0, "test")

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
	d.InsertAction("fix-conflict", &taskID, "{}", "pending", 0, "test")

	tqDir := setupTemplatesDir(t)

	// Simulate an already-running interactive session
	d.InsertAction("respond-review", &taskID, "{}", "running", 0, "test")
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
	d.InsertAction("check-pr-status", &taskID, "{}", "pending", 0, "test")

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
	// check-pr-status has max_retries=0, should escalate to waiting_human
	if action.Status != "waiting_human" {
		t.Errorf("action status = %q, want waiting_human", action.Status)
	}
}

func TestRalphLoop_RetryOnFailure(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "https://example.com", "{}")
	// fix-ci has max_retries=1 and interactive=true
	d.InsertAction("fix-ci", &taskID, "{}", "pending", 0, "test")

	tqDir := setupTemplatesDir(t)

	callCount := 0

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cfg := RalphConfig{
		TQDir:          tqDir,
		DB:             d,
		MaxInteractive: 3,
		PollInterval:   50 * time.Millisecond,
		NonInteractiveFunc: func(tqDir string) Worker {
			return &countingWorker{result: `{"ok":true}`}
		},
		InteractiveFunc: func(tqDir string) Worker {
			callCount++
			if callCount == 1 {
				return &countingWorker{err: context.DeadlineExceeded}
			}
			return &countingWorker{result: "interactive:done"}
		},
	}

	_ = RalphLoop(ctx, cfg)

	// fix-ci has max_retries=1, first failure resets to pending, then retried
	if callCount < 2 {
		t.Errorf("interactive worker called %d times, want >= 2 (retry)", callCount)
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
