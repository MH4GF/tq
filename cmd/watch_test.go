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
	"github.com/MH4GF/tq/source"
	"github.com/MH4GF/tq/testutil"
)

type mockSource struct {
	name           string
	notifications  []source.Notification
	fetchErr       error
	processedIDs   []string
	markProcessErr error
}

func (m *mockSource) Name() string { return m.name }

func (m *mockSource) Fetch(ctx context.Context) ([]source.Notification, error) {
	return m.notifications, m.fetchErr
}

func (m *mockSource) MarkProcessed(ctx context.Context, n source.Notification) error {
	if id, ok := n.Metadata["id"].(string); ok {
		m.processedIDs = append(m.processedIDs, id)
	}
	return m.markProcessErr
}

func setupWatchEnv(t *testing.T) string {
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

	os.WriteFile(filepath.Join(promptsDir, "check-pr-status.md"), []byte(`---
description: PR check
auto: true
---
Check PR.
`), 0644)

	return tqDir
}

func TestWatch_NoNotifications(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupWatchEnv(t)

	src := &mockSource{name: "test-source"}
	cmd.SetWatchSourceFactory(func() (source.Source, error) {
		return src, nil
	})
	t.Cleanup(func() { cmd.SetWatchSourceFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"watch"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "fetched 0 notifications") {
		t.Errorf("output = %q, want to contain 'fetched 0 notifications'", out)
	}
	if !contains(out, "processed 0, failed 0") {
		t.Errorf("output = %q, want to contain 'processed 0, failed 0'", out)
	}
}

func TestWatch_WithNotifications(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupWatchEnv(t)

	src := &mockSource{
		name: "test-source",
		notifications: []source.Notification{
			{
				Source:  "test",
				Message: "PR review requested",
				Metadata: map[string]any{
					"id":           "123",
					"reason":       "review_requested",
					"subject_type": "PullRequest",
					"repo":         "immedioinc/immedio",
					"title":        "Fix bug",
					"url":          "https://github.com/immedioinc/immedio/pull/42",
				},
			},
		},
	}
	cmd.SetWatchSourceFactory(func() (source.Source, error) {
		return src, nil
	})
	t.Cleanup(func() { cmd.SetWatchSourceFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"watch"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "fetched 1 notifications") {
		t.Errorf("output = %q, want to contain 'fetched 1 notifications'", out)
	}
	if !contains(out, "processed 1, failed 0") {
		t.Errorf("output = %q, want to contain 'processed 1, failed 0'", out)
	}

	// Verify notification was marked processed
	if len(src.processedIDs) != 1 || src.processedIDs[0] != "123" {
		t.Errorf("processedIDs = %v, want [123]", src.processedIDs)
	}

	// Verify action was created in DB as pending
	actions, err := d.ListActions("pending", nil)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("pending action count = %d, want 1", len(actions))
	}
	action := actions[0]
	if action.PromptID != "classify-gh-notification" {
		t.Errorf("prompt_id = %q, want %q", action.PromptID, "classify-gh-notification")
	}
	if action.TaskID <= 0 {
		t.Errorf("task_id = %d, want > 0 (should be assigned to triage task)", action.TaskID)
	}

	// Verify metadata contains notification and existing_tasks
	var meta map[string]json.RawMessage
	if err := json.Unmarshal([]byte(action.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if _, ok := meta["notification"]; !ok {
		t.Error("metadata missing 'notification' key")
	}
	if _, ok := meta["existing_tasks"]; !ok {
		t.Error("metadata missing 'existing_tasks' key")
	}
	if !contains(string(meta["notification"]), "Fix bug") {
		t.Errorf("notification = %s, want to contain 'Fix bug'", meta["notification"])
	}
}

func TestWatch_FetchError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupWatchEnv(t)

	src := &mockSource{
		name:     "test-source",
		fetchErr: fmt.Errorf("network timeout"),
	}
	cmd.SetWatchSourceFactory(func() (source.Source, error) {
		return src, nil
	})
	t.Cleanup(func() { cmd.SetWatchSourceFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"watch"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "fetch notifications") {
		t.Errorf("error = %q, want to contain 'fetch notifications'", err.Error())
	}
}

func TestWatch_SourceCreateError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupWatchEnv(t)

	cmd.SetWatchSourceFactory(func() (source.Source, error) {
		return nil, fmt.Errorf("auth failed")
	})
	t.Cleanup(func() { cmd.SetWatchSourceFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"watch"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "create source") {
		t.Errorf("error = %q, want to contain 'create source'", err.Error())
	}
}

func TestWatch_MarkProcessedOnlyOnSuccess(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupWatchEnv(t)

	src := &mockSource{
		name: "test-source",
		notifications: []source.Notification{
			{
				Source:   "test",
				Message:  "good notification",
				Metadata: map[string]any{"id": "1", "title": "ok"},
			},
			{
				Source:   "test",
				Message:  "bad notification",
				Metadata: map[string]any{"id": "2", "title": "bad"},
			},
		},
	}
	cmd.SetWatchSourceFactory(func() (source.Source, error) {
		return src, nil
	})
	t.Cleanup(func() { cmd.SetWatchSourceFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"watch"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both notifications should succeed and be marked processed
	if len(src.processedIDs) != 2 {
		t.Errorf("processedIDs = %v, want 2 items", src.processedIDs)
	}

	actions, _ := d.ListActions("pending", nil)
	if len(actions) != 2 {
		t.Errorf("pending action count = %d, want 2", len(actions))
	}
}

func TestCreateClassifyAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)

	notifJSON := `{"type":"pull_request","action":"opened","repo":"test/repo"}`

	id, err := cmd.CreateClassifyAction(notifJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("action ID = %d, want > 0", id)
	}

	action, err := d.GetAction(id)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}

	if action.PromptID != "classify-gh-notification" {
		t.Errorf("prompt_id = %q, want %q", action.PromptID, "classify-gh-notification")
	}
	if action.Status != "pending" {
		t.Errorf("status = %q, want %q", action.Status, "pending")
	}
	if action.TaskID <= 0 {
		t.Errorf("task_id = %d, want > 0 (should be assigned to triage task)", action.TaskID)
	}

	var meta map[string]json.RawMessage
	if err := json.Unmarshal([]byte(action.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}

	if !contains(string(meta["notification"]), "test/repo") {
		t.Errorf("notification = %s, want to contain 'test/repo'", meta["notification"])
	}
	if _, ok := meta["existing_tasks"]; !ok {
		t.Error("metadata missing 'existing_tasks' key")
	}
}

func TestCreateClassifyAction_WithExistingTasks(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)

	// Create an open task
	_, err := d.InsertTask(1, "Test task", "", "{}", "")
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	id, err := cmd.CreateClassifyAction(`{"type":"issue"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := d.GetAction(id)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(action.Metadata), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}

	existingTasks, ok := meta["existing_tasks"].(string)
	if !ok {
		t.Fatal("existing_tasks is not a string")
	}
	if !contains(existingTasks, "Test task") {
		t.Errorf("existing_tasks = %q, want to contain 'Test task'", existingTasks)
	}
}
