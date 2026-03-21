//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var tqBin string

func TestMain(m *testing.M) {
	bin, err := os.CreateTemp("", "tq-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp file: %v\n", err)
		os.Exit(1)
	}
	bin.Close()
	tqBin = bin.Name()

	cmd := exec.Command("go", "build", "-o", tqBin, "..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build tq binary: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	os.Remove(tqBin)
	os.Exit(code)
}

// tqCmd runs the tq binary with an isolated HOME directory.
func tqCmd(t *testing.T, home string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(tqBin, args...)
	cmd.Env = filteredEnv(home)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// tqRun is a convenience wrapper that fails the test on error.
func tqRun(t *testing.T, home string, args ...string) string {
	t.Helper()
	stdout, stderr, err := tqCmd(t, home, args...)
	if err != nil {
		t.Fatalf("tq %s failed: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout, stderr)
	}
	return stdout
}

// newHome creates a temporary HOME directory with the prompts dir.
func newHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	promptsDir := filepath.Join(home, ".config", "tq", "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("create prompts dir: %v", err)
	}
	// Create a minimal prompt template for testing action creation
	prompt := `---
description: test prompt
mode: noninteractive
---
Test prompt for task: {{ .Task.Title }}
`
	if err := os.WriteFile(filepath.Join(promptsDir, "test-prompt.md"), []byte(prompt), 0o644); err != nil {
		t.Fatalf("write test prompt: %v", err)
	}
	return home
}

// filteredEnv builds an environment with HOME/XDG_CONFIG_HOME replaced.
func filteredEnv(home string) []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "XDG_CONFIG_HOME=") {
			continue
		}
		env = append(env, e)
	}
	return append(env,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
}

func parseJSONArray(t *testing.T, data string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(data), &rows); err != nil {
		t.Fatalf("parse JSON: %v\ndata: %s", err, data)
	}
	return rows
}

// --- Project CRUD ---

func TestProjectCRUD(t *testing.T) {
	home := newHome(t)

	// Create
	out := tqRun(t, home, "project", "create", "myapp", "/tmp/myapp")
	if !strings.Contains(out, "project #1 created") {
		t.Errorf("create output = %q, want 'project #1 created'", out)
	}

	// List
	out = tqRun(t, home, "project", "list")
	rows := parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 project, got %d", len(rows))
	}
	if rows[0]["name"] != "myapp" {
		t.Errorf("name = %v, want myapp", rows[0]["name"])
	}
	if rows[0]["work_dir"] != "/tmp/myapp" {
		t.Errorf("work_dir = %v, want /tmp/myapp", rows[0]["work_dir"])
	}

	// Create second
	tqRun(t, home, "project", "create", "other", "/tmp/other")
	out = tqRun(t, home, "project", "list")
	rows = parseJSONArray(t, out)
	if len(rows) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(rows))
	}

	// Delete
	out = tqRun(t, home, "project", "delete", "1")
	if !strings.Contains(out, "project #1 deleted") {
		t.Errorf("delete output = %q, want 'project #1 deleted'", out)
	}

	// List after delete
	out = tqRun(t, home, "project", "list")
	rows = parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 project after delete, got %d", len(rows))
	}
}

func TestProjectCreate_WithMeta(t *testing.T) {
	home := newHome(t)

	out := tqRun(t, home, "project", "create", "myapp", "/tmp/myapp", "--meta", `{"env":"prod"}`)
	if !strings.Contains(out, "project #1 created") {
		t.Errorf("output = %q, want 'project #1 created'", out)
	}

	out = tqRun(t, home, "project", "list")
	rows := parseJSONArray(t, out)
	if rows[0]["metadata"] != `{"env":"prod"}` {
		t.Errorf("metadata = %v, want {\"env\":\"prod\"}", rows[0]["metadata"])
	}
}

func TestProjectCreate_DuplicateName(t *testing.T) {
	home := newHome(t)

	tqRun(t, home, "project", "create", "dup", "/tmp/dup")
	_, stderr, err := tqCmd(t, home, "project", "create", "dup", "/tmp/dup2")
	if err == nil {
		t.Fatal("expected error for duplicate project name")
	}
	if !strings.Contains(stderr, "UNIQUE constraint") {
		t.Errorf("stderr = %q, want to contain 'UNIQUE constraint'", stderr)
	}
}

func TestProjectCreate_MissingArgs(t *testing.T) {
	home := newHome(t)

	_, stderr, err := tqCmd(t, home, "project", "create", "onlyname")
	if err == nil {
		t.Fatal("expected error for missing work_dir argument")
	}
	if !strings.Contains(stderr, "accepts 2 arg") {
		t.Errorf("stderr = %q, want to contain 'accepts 2 arg'", stderr)
	}
}

func TestProjectDelete_NotFound(t *testing.T) {
	home := newHome(t)

	_, stderr, err := tqCmd(t, home, "project", "delete", "999")
	if err == nil {
		t.Fatal("expected error for non-existent project")
	}
	if !strings.Contains(stderr, "no rows in result set") {
		t.Errorf("stderr = %q, want to contain 'no rows in result set'", stderr)
	}
}

func TestProjectList_Empty(t *testing.T) {
	home := newHome(t)

	out := tqRun(t, home, "project", "list")
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("output = %q, want '[]'", out)
	}
}

// --- Task Lifecycle ---

func TestTaskLifecycle(t *testing.T) {
	home := newHome(t)

	// Setup project
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")

	// Create task
	out := tqRun(t, home, "task", "create", "implement feature", "--project", "1")
	if !strings.Contains(out, "task #1 created") {
		t.Errorf("create output = %q, want 'task #1 created'", out)
	}

	// List tasks
	out = tqRun(t, home, "task", "list")
	rows := parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 task, got %d", len(rows))
	}
	if rows[0]["title"] != "implement feature" {
		t.Errorf("title = %v, want 'implement feature'", rows[0]["title"])
	}
	if rows[0]["status"] != "open" {
		t.Errorf("status = %v, want 'open'", rows[0]["status"])
	}

	// Update status
	out = tqRun(t, home, "task", "update", "1", "--status", "done")
	if !strings.Contains(out, "status: done") {
		t.Errorf("update output = %q, want 'status: done'", out)
	}

	// Verify updated status via list
	out = tqRun(t, home, "task", "list")
	rows = parseJSONArray(t, out)
	if rows[0]["status"] != "done" {
		t.Errorf("status after update = %v, want 'done'", rows[0]["status"])
	}
}

func TestTaskList_StatusFilter(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")

	tqRun(t, home, "task", "create", "open task", "--project", "1")
	tqRun(t, home, "task", "create", "done task", "--project", "1")
	tqRun(t, home, "task", "update", "2", "--status", "done")

	out := tqRun(t, home, "task", "list", "--status", "open")
	rows := parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 open task, got %d", len(rows))
	}
	if rows[0]["title"] != "open task" {
		t.Errorf("title = %v, want 'open task'", rows[0]["title"])
	}
}

func TestTaskList_ProjectFilter(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj1", "/tmp/p1")
	tqRun(t, home, "project", "create", "proj2", "/tmp/p2")

	tqRun(t, home, "task", "create", "task in p1", "--project", "1")
	tqRun(t, home, "task", "create", "task in p2", "--project", "2")

	out := tqRun(t, home, "task", "list", "--project", "1")
	rows := parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 task, got %d", len(rows))
	}
	if rows[0]["title"] != "task in p1" {
		t.Errorf("title = %v, want 'task in p1'", rows[0]["title"])
	}
}

func TestTaskCreate_WithMeta(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")

	tqRun(t, home, "task", "create", "meta task", "--project", "1", "--meta", `{"url":"https://example.com"}`)

	out := tqRun(t, home, "task", "list")
	rows := parseJSONArray(t, out)
	if rows[0]["metadata"] != `{"url":"https://example.com"}` {
		t.Errorf("metadata = %v", rows[0]["metadata"])
	}
}

func TestTaskCreate_MissingProject(t *testing.T) {
	home := newHome(t)

	_, stderr, err := tqCmd(t, home, "task", "create", "no project")
	if err == nil {
		t.Fatal("expected error for missing --project")
	}
	if !strings.Contains(stderr, "required flag") {
		t.Errorf("stderr = %q, want to contain 'required flag'", stderr)
	}
}

func TestTaskUpdate_MoveProject(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj1", "/tmp/p1")
	tqRun(t, home, "project", "create", "proj2", "/tmp/p2")
	tqRun(t, home, "task", "create", "movable", "--project", "1")

	out := tqRun(t, home, "task", "update", "1", "--project", "2")
	if !strings.Contains(out, "project: proj2") {
		t.Errorf("output = %q, want 'project: proj2'", out)
	}
}

func TestTaskList_Empty(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")

	out := tqRun(t, home, "task", "list")
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("output = %q, want '[]'", out)
	}
}

// --- Action Workflow ---

func TestActionWorkflow(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")

	// Create action
	out := tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "Do something")
	if !strings.Contains(out, "action #1 created") {
		t.Errorf("create output = %q, want 'action #1 created'", out)
	}

	// List actions
	out = tqRun(t, home, "action", "list", "--task", "1")
	rows := parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 action, got %d", len(rows))
	}
	if rows[0]["status"] != "pending" {
		t.Errorf("status = %v, want 'pending'", rows[0]["status"])
	}
	if rows[0]["title"] != "Do something" {
		t.Errorf("title = %v, want 'Do something'", rows[0]["title"])
	}

	// Mark done
	out = tqRun(t, home, "action", "done", "1", "completed successfully")
	if !strings.Contains(out, "action #1 done") {
		t.Errorf("done output = %q, want 'action #1 done'", out)
	}

	// Verify done status
	out = tqRun(t, home, "action", "list", "--task", "1")
	rows = parseJSONArray(t, out)
	if rows[0]["status"] != "done" {
		t.Errorf("status after done = %v, want 'done'", rows[0]["status"])
	}
	if rows[0]["result"] != "completed successfully" {
		t.Errorf("result = %v, want 'completed successfully'", rows[0]["result"])
	}
}

func TestActionCancel(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")
	tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "To cancel")

	out := tqRun(t, home, "action", "cancel", "1", "no longer needed")
	if !strings.Contains(out, "action #1 cancelled") {
		t.Errorf("cancel output = %q, want 'action #1 cancelled'", out)
	}

	out = tqRun(t, home, "action", "list", "--task", "1")
	rows := parseJSONArray(t, out)
	if rows[0]["status"] != "cancelled" {
		t.Errorf("status = %v, want 'cancelled'", rows[0]["status"])
	}
}

func TestActionCancel_AlreadyDone(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")
	tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "Already done")
	tqRun(t, home, "action", "done", "1")

	_, stderr, err := tqCmd(t, home, "action", "cancel", "1")
	if err == nil {
		t.Fatal("expected error cancelling done action")
	}
	if !strings.Contains(stderr, "already") {
		t.Errorf("stderr = %q, want to contain 'already'", stderr)
	}
}

func TestActionReset(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")
	tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "To reset", "--status", "failed")

	out := tqRun(t, home, "action", "reset", "1")
	if !strings.Contains(out, "action #1 reset") {
		t.Errorf("reset output = %q, want 'action #1 reset'", out)
	}

	out = tqRun(t, home, "action", "list", "--task", "1")
	rows := parseJSONArray(t, out)
	if rows[0]["status"] != "pending" {
		t.Errorf("status after reset = %v, want 'pending'", rows[0]["status"])
	}
}

func TestActionCreate_DuplicateBlocked(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")
	tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "First")

	_, stderr, err := tqCmd(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "Second")
	if err == nil {
		t.Fatal("expected error for duplicate action")
	}
	if !strings.Contains(stderr, "blocked") {
		t.Errorf("stderr = %q, want to contain 'blocked'", stderr)
	}
}

func TestActionCreate_ForceOverride(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")
	tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "First")

	out := tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "Second", "--force")
	if !strings.Contains(out, "action #2 created") {
		t.Errorf("output = %q, want 'action #2 created'", out)
	}
}

func TestActionCreate_MissingTitle(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")

	_, stderr, err := tqCmd(t, home, "action", "create", "test-prompt", "--task", "1")
	if err == nil {
		t.Fatal("expected error for missing --title")
	}
	if !strings.Contains(stderr, "--title is required") {
		t.Errorf("stderr = %q, want to contain '--title is required'", stderr)
	}
}

func TestActionList_Empty(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "test task", "--project", "1")

	out := tqRun(t, home, "action", "list", "--task", "1")
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("output = %q, want '[]'", out)
	}
}

// --- JSON Output Validation ---

func TestTaskList_JSONStructure(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "json test", "--project", "1", "--meta", `{"key":"value"}`)

	out := tqRun(t, home, "task", "list")
	rows := parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	row := rows[0]

	// Verify required fields exist
	requiredFields := []string{"id", "project_id", "title", "status", "metadata", "created_at", "actions"}
	for _, f := range requiredFields {
		if _, ok := row[f]; !ok {
			t.Errorf("missing field %q in task JSON", f)
		}
	}

	// Verify field values
	if row["id"] != float64(1) {
		t.Errorf("id = %v, want 1", row["id"])
	}
	if row["status"] != "open" {
		t.Errorf("status = %v, want 'open'", row["status"])
	}
	if row["metadata"] != `{"key":"value"}` {
		t.Errorf("metadata = %v", row["metadata"])
	}

	// actions should be empty array, not null
	actions, ok := row["actions"].([]any)
	if !ok {
		t.Fatalf("actions is not array: %v", row["actions"])
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestTaskList_WithActionsJSON(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "task with action", "--project", "1")
	tqRun(t, home, "action", "create", "test-prompt",
		"--task", "1", "--title", "Sub action")

	out := tqRun(t, home, "task", "list")
	rows := parseJSONArray(t, out)
	actions, ok := rows[0]["actions"].([]any)
	if !ok {
		t.Fatalf("actions is not array: %v", rows[0]["actions"])
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action, ok := actions[0].(map[string]any)
	if !ok {
		t.Fatalf("action is not object: %v", actions[0])
	}
	if action["prompt_id"] != "test-prompt" {
		t.Errorf("prompt_id = %v, want 'test-prompt'", action["prompt_id"])
	}
	if action["status"] != "pending" {
		t.Errorf("status = %v, want 'pending'", action["status"])
	}
}

func TestProjectList_JSONStructure(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj", "--meta", `{"env":"test"}`)

	out := tqRun(t, home, "project", "list")
	rows := parseJSONArray(t, out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	requiredFields := []string{"id", "name", "work_dir", "metadata", "created_at"}
	for _, f := range requiredFields {
		if _, ok := rows[0][f]; !ok {
			t.Errorf("missing field %q in project JSON", f)
		}
	}
}

// --- Error Cases ---

func TestInvalidCommand(t *testing.T) {
	home := newHome(t)

	_, stderr, err := tqCmd(t, home, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr = %q, want to contain 'unknown command'", stderr)
	}
}

func TestProjectCreate_InvalidMeta(t *testing.T) {
	home := newHome(t)

	_, stderr, err := tqCmd(t, home, "project", "create", "proj", "/tmp/proj", "--meta", "not-json")
	if err == nil {
		t.Fatal("expected error for invalid meta JSON")
	}
	if !strings.Contains(stderr, "invalid JSON") {
		t.Errorf("stderr = %q, want to contain 'invalid JSON'", stderr)
	}
}

func TestTaskCreate_InvalidMeta(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")

	_, stderr, err := tqCmd(t, home, "task", "create", "task", "--project", "1", "--meta", "{bad}")
	if err == nil {
		t.Fatal("expected error for invalid meta JSON")
	}
	if !strings.Contains(stderr, "invalid JSON") {
		t.Errorf("stderr = %q, want to contain 'invalid JSON'", stderr)
	}
}

func TestTaskUpdate_NoFlags(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "task", "--project", "1")

	_, stderr, err := tqCmd(t, home, "task", "update", "1")
	if err == nil {
		t.Fatal("expected error when no update flags given")
	}
	if !strings.Contains(stderr, "at least one of") {
		t.Errorf("stderr = %q, want to contain 'at least one of'", stderr)
	}
}

func TestActionCreate_UnknownPrompt(t *testing.T) {
	home := newHome(t)
	tqRun(t, home, "project", "create", "proj", "/tmp/proj")
	tqRun(t, home, "task", "create", "task", "--project", "1")

	_, stderr, err := tqCmd(t, home, "action", "create", "nonexistent-prompt",
		"--task", "1", "--title", "Test")
	if err == nil {
		t.Fatal("expected error for unknown prompt template")
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q, want to contain 'not found'", stderr)
	}
}

func TestActionDone_NotFound(t *testing.T) {
	home := newHome(t)

	_, stderr, err := tqCmd(t, home, "action", "done", "999")
	if err == nil {
		t.Fatal("expected error for non-existent action")
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q, want to contain 'not found'", stderr)
	}
}

func TestActionCancel_NotFound(t *testing.T) {
	home := newHome(t)

	_, stderr, err := tqCmd(t, home, "action", "cancel", "999")
	if err == nil {
		t.Fatal("expected error for non-existent action")
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q, want to contain 'not found'", stderr)
	}
}

// --- Version ---

func TestVersion(t *testing.T) {
	home := newHome(t)

	out := tqRun(t, home, "--version")
	if !strings.Contains(out, "tq version") {
		t.Errorf("output = %q, want to contain 'tq version'", out)
	}
}
