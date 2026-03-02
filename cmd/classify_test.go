package cmd_test

import (
	"bytes"
	"encoding/json"
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

	os.WriteFile(filepath.Join(templatesDir, "respond-review.md"), []byte(`---
description: Respond review
auto: false
---
Respond.
`), 0644)

	buf := new(bytes.Buffer)
	return tqDir, buf
}

func TestClassify_NewTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	tqDir, buf := setupClassifyEnv(t)
	_ = tqDir

	classifyResult := cmd.ClassifyResult{}
	classifyResult.Task.ID = 0
	classifyResult.Task.ProjectName = "immedio"
	classifyResult.Task.Title = "Fix login bug"
	classifyResult.Task.URL = "https://github.com/immedioinc/immedio/pull/42"

	type actionEntry struct {
		TemplateID string `json:"template_id"`
		Priority   int    `json:"priority"`
	}
	resultJSON := struct {
		Task struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		} `json:"task"`
		Actions []actionEntry `json:"actions"`
	}{
		Task: classifyResult.Task,
		Actions: []actionEntry{
			{TemplateID: "check-pr-status", Priority: 5},
		},
	}
	resultBytes, _ := json.Marshal(resultJSON)

	cmd.SetWorkerFactory(func(tqDir string) dispatch.Worker {
		return &mockWorker{result: string(resultBytes)}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"pull_request","action":"opened"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "task #1 created") {
		t.Errorf("output = %q, want to contain 'task #1 created'", out)
	}
	if !contains(out, "project: immedio") {
		t.Errorf("output = %q, want to contain 'project: immedio'", out)
	}
	if !contains(out, "action #1 created") {
		t.Errorf("output = %q, want to contain 'action #1 created'", out)
	}
	if !contains(out, "status: pending") {
		t.Errorf("output = %q, want to contain 'status: pending'", out)
	}

	task, err := d.GetTask(1)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Title != "Fix login bug" {
		t.Errorf("title = %q, want %q", task.Title, "Fix login bug")
	}
	if task.URL != "https://github.com/immedioinc/immedio/pull/42" {
		t.Errorf("url = %q, want %q", task.URL, "https://github.com/immedioinc/immedio/pull/42")
	}

	action, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if action.TemplateID != "check-pr-status" {
		t.Errorf("template_id = %q, want %q", action.TemplateID, "check-pr-status")
	}
	if action.Priority != 5 {
		t.Errorf("priority = %d, want 5", action.Priority)
	}
	if action.Source != "classify" {
		t.Errorf("source = %q, want %q", action.Source, "classify")
	}
}

func TestClassify_ExistingTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyEnv(t)

	taskID, _ := d.InsertTask(1, "Existing PR", "https://github.com/test/1", "{}")

	type actionEntry struct {
		TemplateID string `json:"template_id"`
		Priority   int    `json:"priority"`
	}
	resultJSON := struct {
		Task struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		} `json:"task"`
		Actions []actionEntry `json:"actions"`
	}{
		Task: struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		}{
			ID: taskID,
		},
		Actions: []actionEntry{
			{TemplateID: "check-pr-status", Priority: 3},
		},
	}
	resultBytes, _ := json.Marshal(resultJSON)

	cmd.SetWorkerFactory(func(tqDir string) dispatch.Worker {
		return &mockWorker{result: string(resultBytes)}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"review"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "linked to existing task #1") {
		t.Errorf("output = %q, want to contain 'linked to existing task #1'", out)
	}
	if !contains(out, "action #1 created") {
		t.Errorf("output = %q, want to contain 'action #1 created'", out)
	}

	// Verify no new tasks were created (only the one we inserted)
	tasks, _ := d.ListTasksByStatus("open")
	if len(tasks) != 1 {
		t.Errorf("task count = %d, want 1", len(tasks))
	}

	action, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if !action.TaskID.Valid || action.TaskID.Int64 != taskID {
		t.Errorf("task_id = %v, want %d", action.TaskID, taskID)
	}
}

func TestClassify_DuplicateAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyEnv(t)

	taskID, _ := d.InsertTask(1, "Existing PR", "https://github.com/test/1", "{}")
	d.InsertAction("check-pr-status", &taskID, "{}", "pending", 5, "classify")

	type actionEntry struct {
		TemplateID string `json:"template_id"`
		Priority   int    `json:"priority"`
	}
	resultJSON := struct {
		Task struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		} `json:"task"`
		Actions []actionEntry `json:"actions"`
	}{
		Task: struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		}{
			ID: taskID,
		},
		Actions: []actionEntry{
			{TemplateID: "check-pr-status", Priority: 5},
		},
	}
	resultBytes, _ := json.Marshal(resultJSON)

	cmd.SetWorkerFactory(func(tqDir string) dispatch.Worker {
		return &mockWorker{result: string(resultBytes)}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"review"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "skipped duplicate action: check-pr-status for task #1") {
		t.Errorf("output = %q, want to contain 'skipped duplicate action'", out)
	}

	// Verify only the original action exists
	actions, _ := d.ListActions("", nil)
	if len(actions) != 1 {
		t.Errorf("action count = %d, want 1", len(actions))
	}
}

func TestClassify_AutoFalseTemplate(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	setupClassifyEnv(t)

	type actionEntry struct {
		TemplateID string `json:"template_id"`
		Priority   int    `json:"priority"`
	}
	resultJSON := struct {
		Task struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		} `json:"task"`
		Actions []actionEntry `json:"actions"`
	}{
		Task: struct {
			ID          int64  `json:"id"`
			ProjectName string `json:"project_name"`
			Title       string `json:"title"`
			URL         string `json:"url"`
		}{
			ProjectName: "works",
			Title:       "Review needed",
			URL:         "https://github.com/MH4GF/works/pull/1",
		},
		Actions: []actionEntry{
			{TemplateID: "respond-review", Priority: 7},
		},
	}
	resultBytes, _ := json.Marshal(resultJSON)

	cmd.SetWorkerFactory(func(tqDir string) dispatch.Worker {
		return &mockWorker{result: string(resultBytes)}
	})
	t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"classify", `{"type":"review_requested"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "status: waiting_human") {
		t.Errorf("output = %q, want to contain 'status: waiting_human'", out)
	}

	action, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if action.Status != "waiting_human" {
		t.Errorf("status = %q, want %q", action.Status, "waiting_human")
	}
}
