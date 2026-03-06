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

func setupClassifyEnv(t *testing.T) (string, *bytes.Buffer) {
	t.Helper()
	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	os.WriteFile(filepath.Join(promptsDir, "classify.md"), []byte(`---
description: classify
mode: noninteractive
---
Classify: {{index .Action.Meta "notification"}}
Tasks: {{index .Action.Meta "existing_tasks"}}
`), 0644)

	buf := new(bytes.Buffer)
	return tqDir, buf
}

func setupInteractiveClassifyEnv(t *testing.T) (string, *bytes.Buffer) {
	t.Helper()
	tqDir := t.TempDir()
	cmd.SetConfigDir(tqDir)

	promptsDir := filepath.Join(tqDir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	os.WriteFile(filepath.Join(promptsDir, "classify.md"), []byte(`---
description: classify
mode: interactive
---
Classify: {{index .Action.Meta "notification"}}
Tasks: {{index .Action.Meta "existing_tasks"}}
`), 0644)

	buf := new(bytes.Buffer)
	return tqDir, buf
}

func TestClassify_Success(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyEnv(t)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{result: "task created, action created"}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"pull_request","action":"opened"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "task created, action created") {
		t.Errorf("output = %q, want to contain worker result", out)
	}
}

func TestClassify_Interactive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupInteractiveClassifyEnv(t)

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
	root.SetArgs([]string{"classify", `{"type":"review"}`})

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

func TestClassify_InteractiveError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupInteractiveClassifyEnv(t)

	cmd.SetInteractiveWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: fmt.Errorf("tmux not found")}
	})
	t.Cleanup(func() { cmd.SetInteractiveWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"push"}`})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	actions, _ := d.ListActions("failed", nil)
	if len(actions) != 1 {
		t.Fatalf("failed action count = %d, want 1", len(actions))
	}
	action := actions[0]
	if action.PromptID != "classify" {
		t.Errorf("template_id = %q, want %q", action.PromptID, "classify")
	}
	if !action.Result.Valid || !contains(action.Result.String, "tmux not found") {
		t.Errorf("result = %v, want to contain 'tmux not found'", action.Result)
	}
}

func TestClassify_ExecutionFailure(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyEnv(t)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: fmt.Errorf("LLM timeout")}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"push"}`})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	actions, _ := d.ListActions("failed", nil)
	if len(actions) != 1 {
		t.Fatalf("failed action count = %d, want 1", len(actions))
	}
	action := actions[0]
	if action.PromptID != "classify" {
		t.Errorf("template_id = %q, want %q", action.PromptID, "classify")
	}
	if action.Source != "classify" {
		t.Errorf("source = %q, want %q", action.Source, "classify")
	}
	if !action.Result.Valid || !contains(action.Result.String, "LLM timeout") {
		t.Errorf("result = %v, want to contain 'LLM timeout'", action.Result)
	}
	if !contains(action.Metadata, `"notification"`) {
		t.Errorf("metadata = %q, want to contain notification", action.Metadata)
	}
}

func TestClassify_FailureWithContextDeadline(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyEnv(t)

	cmd.SetWorkerFactory(func() dispatch.Worker {
		return &mockWorker{err: context.DeadlineExceeded}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"review","repo":"test/repo"}`})

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
