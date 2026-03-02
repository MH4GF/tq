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
	cmd.SetTQDir(tqDir)

	templatesDir := filepath.Join(tqDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	os.WriteFile(filepath.Join(templatesDir, "classify.md"), []byte(`---
description: classify
auto: true
interactive: false
timeout: 10
---
Classify: {{index .Action.Meta "notification"}}
Tasks: {{index .Action.Meta "existing_tasks"}}
`), 0644)

	os.WriteFile(filepath.Join(templatesDir, "check-pr-status.md"), []byte(`---
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

	classifyResult := struct {
		Task struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		} `json:"task"`
		Actions []struct {
			TemplateID string `json:"template_id"`
			Priority   int    `json:"priority"`
		} `json:"actions"`
	}{
		Task: struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		}{
			ProjectName: "immedio",
			Title:       "Fix bug",
			URL:         "https://github.com/immedioinc/immedio/pull/42",
		},
		Actions: []struct {
			TemplateID string `json:"template_id"`
			Priority   int    `json:"priority"`
		}{
			{TemplateID: "check-pr-status", Priority: 5},
		},
	}
	resultBytes, _ := json.Marshal(classifyResult)

	cmd.SetWorkerFactory(func(tqDir string) dispatch.Worker {
		return &mockWorker{result: string(resultBytes)}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

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
	if !contains(out, "task #1 created") {
		t.Errorf("output = %q, want to contain 'task #1 created'", out)
	}

	// Verify notification was marked processed
	if len(src.processedIDs) != 1 || src.processedIDs[0] != "123" {
		t.Errorf("processedIDs = %v, want [123]", src.processedIDs)
	}

	// Verify task was created in DB
	task, err := d.GetTask(1)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Title != "Fix bug" {
		t.Errorf("title = %q, want %q", task.Title, "Fix bug")
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

func TestWatch_AllClassifyFail(t *testing.T) {
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
				Message: "notification 1",
				Metadata: map[string]any{
					"id":    "1",
					"title": "test",
				},
			},
		},
	}
	cmd.SetWatchSourceFactory(func() (source.Source, error) {
		return src, nil
	})
	t.Cleanup(func() { cmd.SetWatchSourceFactory(nil) })

	cmd.SetWorkerFactory(func(tqDir string) dispatch.Worker {
		return &mockWorker{err: fmt.Errorf("classify failed")}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"watch"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when all notifications fail, got nil")
	}
	if !contains(err.Error(), "all") {
		t.Errorf("error = %q, want to contain 'all'", err.Error())
	}
}
