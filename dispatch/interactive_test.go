package dispatch

import (
	"context"
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
	if !strings.Contains(argsStr, "claude") {
		t.Errorf("call[1] args = %v, want to contain 'claude'", c.args)
	}
	if strings.Contains(argsStr, "--worktree") {
		t.Errorf("call[1] args = %v, must NOT contain --worktree", c.args)
	}
	if strings.Contains(argsStr, "TQ_DIR") {
		t.Errorf("call[1] args = %v, must NOT contain TQ_DIR", c.args)
	}
	if !strings.Contains(argsStr, "TQ_ACTION_ID=42") {
		t.Errorf("call[1] args = %v, want to contain TQ_ACTION_ID=42", c.args)
	}
	if strings.Contains(argsStr, "--tmux") {
		t.Errorf("call[1] args = %v, must NOT contain --tmux", c.args)
	}
	if !strings.Contains(argsStr, "Fix the bug") {
		t.Errorf("call[1] args = %v, want to contain prompt text 'Fix the bug'", c.args)
	}

	// Call 3: tmux send-keys Enter
	c = runner.calls[2]
	if c.name != "tmux" {
		t.Errorf("call[2] command = %q, want tmux", c.name)
	}
	if len(c.args) < 4 || c.args[3] != "Enter" {
		t.Errorf("call[2] args = %v, want last arg to be Enter", c.args)
	}
}

func TestInteractiveWorker_NewWindowError(t *testing.T) {
	runner := &mockRunner{err: context.DeadlineExceeded, failAt: 0}
	w := &InteractiveWorker{
		Runner: runner,
	}

	cfg := tmpl.Config{}
	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create tmux window") {
		t.Errorf("error = %q, want to contain 'create tmux window'", err.Error())
	}
}

func TestInteractiveWorker_SendKeysError(t *testing.T) {
	runner := &mockRunner{err: context.DeadlineExceeded, failAt: 1}
	w := &InteractiveWorker{
		Runner: runner,
	}

	cfg := tmpl.Config{}
	_, err := w.Execute(context.Background(), "test", cfg, "/work", 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "send claude command") {
		t.Errorf("error = %q, want to contain 'send claude command'", err.Error())
	}
}

func TestInteractiveWorker_SingleQuoteEscape(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{
		Runner: runner,
	}

	cfg := tmpl.Config{}
	_, err := w.Execute(context.Background(), "it's a test", cfg, "/work/dir", 99)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// send-keys command should contain the escaped single quote
	argsStr := strings.Join(runner.calls[1].args, " ")
	if !strings.Contains(argsStr, "it'\\''s a test") {
		t.Errorf("call[1] args = %v, want to contain escaped single quote", runner.calls[1].args)
	}
}
