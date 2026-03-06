package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

func setupClassifyGhNotificationEnv(t *testing.T) (string, *bytes.Buffer) {
	t.Helper()
	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	os.WriteFile(filepath.Join(promptsDir, "classify-gh-notification.md"), []byte(`---
description: classify-gh-notification
mode: noninteractive
---
Classify: {{index .Action.Meta "notification"}}
Tasks: {{index .Action.Meta "existing_tasks"}}
`), 0644)

	buf := new(bytes.Buffer)
	return tqDir, buf
}

func setupInteractiveClassifyGhNotificationEnv(t *testing.T) (string, *bytes.Buffer) {
	t.Helper()
	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	os.WriteFile(filepath.Join(promptsDir, "classify-gh-notification.md"), []byte(`---
description: classify-gh-notification
mode: interactive
---
Classify: {{index .Action.Meta "notification"}}
Tasks: {{index .Action.Meta "existing_tasks"}}
`), 0644)

	buf := new(bytes.Buffer)
	return tqDir, buf
}

func TestClassifyGhNotification_Success(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyGhNotificationEnv(t)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{result: "task created, action created"}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify-gh-notification", `{"type":"pull_request","action":"opened"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "task created, action created") {
		t.Errorf("output = %q, want to contain worker result", out)
	}
}

func TestClassifyGhNotification_Interactive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupInteractiveClassifyGhNotificationEnv(t)

	var usedInteractive bool
	cmd.SetInteractiveWorkerFactory(func() dispatch.Worker {
		usedInteractive = true
		return &mockWorker{result: "interactive:classify"}
	})
	t.Cleanup(func() { cmd.SetInteractiveWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify-gh-notification", `{"type":"review"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !usedInteractive {
		t.Error("expected interactive worker factory to be used")
	}

	out := buf.String()
	if !contains(out, "interactive:classify") {
		t.Errorf("output = %q, want to contain 'interactive:classify'", out)
	}
}

func TestClassifyGhNotification_InteractiveError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupInteractiveClassifyGhNotificationEnv(t)

	cmd.SetInteractiveWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: fmt.Errorf("tmux not found")}
	})
	t.Cleanup(func() { cmd.SetInteractiveWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify-gh-notification", `{"type":"push"}`})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	actions, _ := d.ListActions("failed", nil)
	if len(actions) != 1 {
		t.Fatalf("failed action count = %d, want 1", len(actions))
	}
	action := actions[0]
	if action.PromptID != "classify-gh-notification" {
		t.Errorf("template_id = %q, want %q", action.PromptID, "classify-gh-notification")
	}
	if !action.Result.Valid || !contains(action.Result.String, "tmux not found") {
		t.Errorf("result = %v, want to contain 'tmux not found'", action.Result)
	}
}

func TestClassifyGhNotification_ExecutionFailure(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyGhNotificationEnv(t)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: fmt.Errorf("LLM timeout")}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify-gh-notification", `{"type":"push"}`})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	actions, _ := d.ListActions("failed", nil)
	if len(actions) != 1 {
		t.Fatalf("failed action count = %d, want 1", len(actions))
	}
	action := actions[0]
	if action.PromptID != "classify-gh-notification" {
		t.Errorf("template_id = %q, want %q", action.PromptID, "classify-gh-notification")
	}
	if action.Source != "classify-gh-notification" {
		t.Errorf("source = %q, want %q", action.Source, "classify-gh-notification")
	}
	if !action.Result.Valid || !contains(action.Result.String, "LLM timeout") {
		t.Errorf("result = %v, want to contain 'LLM timeout'", action.Result)
	}
	if !contains(action.Metadata, `"notification"`) {
		t.Errorf("metadata = %q, want to contain notification", action.Metadata)
	}
}

func TestClassifyGhNotification_FailureWithContextDeadline(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyGhNotificationEnv(t)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: context.DeadlineExceeded}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify-gh-notification", `{"type":"review","repo":"test/repo"}`})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	actions, _ := d.ListActions("failed", nil)
	if len(actions) != 1 {
		t.Fatalf("failed action count = %d, want 1", len(actions))
	}
	action := actions[0]

	var meta map[string]json.RawMessage
	if err := json.Unmarshal([]byte(action.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	notif, ok := meta["notification"]
	if !ok {
		t.Fatal("metadata missing 'notification' key")
	}
	if !contains(string(notif), "test/repo") {
		t.Errorf("notification = %s, want to contain 'test/repo'", notif)
	}
}
