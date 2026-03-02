package dispatch

import (
	"context"
	"os"
	"strings"
	"testing"

	tmpl "github.com/MH4GF/tq/template"
)

type mockCall struct {
	name string
	args []string
	dir  string
	env  []string
}

type mockRunner struct {
	calls  []mockCall
	output []byte
	err    error
	// failAt indicates which call index should return the error (0-based). -1 means no failure.
	failAt int
}

func (m *mockRunner) Run(_ context.Context, name string, args []string, dir string, env []string) ([]byte, error) {
	idx := len(m.calls)
	m.calls = append(m.calls, mockCall{name: name, args: args, dir: dir, env: env})
	if m.failAt >= 0 && idx == m.failAt {
		return m.output, m.err
	}
	return m.output, nil
}

func TestInteractiveWorker_Execute(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{
		Runner: runner,
		TQDir:  "/tmp/tq",
	}

	cfg := tmpl.Config{}

	result, err := w.Execute(context.Background(), "Fix the bug", cfg, "/work/dir", 42)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(result, "action=42") {
		t.Errorf("result = %q, want to contain action=42", result)
	}

	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 runner calls, got %d", len(runner.calls))
	}

	// Call 1: tmux new-window
	c := runner.calls[0]
	if c.name != "tmux" {
		t.Errorf("call[0] command = %q, want tmux", c.name)
	}
	wantArgs := []string{"new-window", "-t", "main", "-n", "tq-action-42", "-c", "/work/dir"}
	if strings.Join(c.args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("call[0] args = %v, want %v", c.args, wantArgs)
	}

	// Call 2: tmux send-keys (command text)
	c = runner.calls[1]
	if c.name != "tmux" {
		t.Errorf("call[1] command = %q, want tmux", c.name)
	}
	argsStr := strings.Join(c.args, " ")
	if !strings.Contains(argsStr, "send-keys") {
		t.Errorf("call[1] args = %v, want to contain send-keys", c.args)
	}
	if !strings.Contains(argsStr, "main:tq-action-42") {
		t.Errorf("call[1] args = %v, want to contain main:tq-action-42", c.args)
	}
	if !strings.Contains(argsStr, "claude --worktree") {
		t.Errorf("call[1] args = %v, want to contain 'claude --worktree'", c.args)
	}
	if !strings.Contains(argsStr, "TQ_DIR=/tmp/tq") {
		t.Errorf("call[1] args = %v, want to contain TQ_DIR=/tmp/tq", c.args)
	}
	if strings.Contains(argsStr, "--tmux") {
		t.Errorf("call[1] args = %v, must NOT contain --tmux", c.args)
	}

	// Call 3: tmux send-keys Enter
	c = runner.calls[2]
	if c.name != "tmux" {
		t.Errorf("call[2] command = %q, want tmux", c.name)
	}
	if len(c.args) < 4 || c.args[3] != "Enter" {
		t.Errorf("call[2] args = %v, want last arg to be Enter", c.args)
	}

	// Prompt file should exist
	promptFile := "/tmp/tq-prompt-42.txt"
	data, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("prompt file not found: %v", err)
	}
	if string(data) != "Fix the bug" {
		t.Errorf("prompt file content = %q, want %q", string(data), "Fix the bug")
	}
	os.Remove(promptFile)
}

func TestInteractiveWorker_NewWindowError(t *testing.T) {
	runner := &mockRunner{err: context.DeadlineExceeded, failAt: 0}
	w := &InteractiveWorker{
		Runner: runner,
		TQDir:  "/tmp/tq",
	}

	cfg := tmpl.Config{}
	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create tmux window") {
		t.Errorf("error = %q, want to contain 'create tmux window'", err.Error())
	}
	os.Remove("/tmp/tq-prompt-1.txt")
}

func TestInteractiveWorker_SendKeysError(t *testing.T) {
	runner := &mockRunner{err: context.DeadlineExceeded, failAt: 1}
	w := &InteractiveWorker{
		Runner: runner,
		TQDir:  "/tmp/tq",
	}

	cfg := tmpl.Config{}
	_, err := w.Execute(context.Background(), "test", cfg, "/work", 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "send claude command") {
		t.Errorf("error = %q, want to contain 'send claude command'", err.Error())
	}
	os.Remove("/tmp/tq-prompt-2.txt")
}
