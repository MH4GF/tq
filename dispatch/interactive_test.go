package dispatch

import (
	"context"
	"strings"
	"testing"

	"github.com/MH4GF/tq/prompt"
)

func TestInteractiveWorker_Execute(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{
		Runner: runner,
	}

	cfg := prompt.Config{}

	result, err := w.Execute(context.Background(), "Fix the bug", cfg, "/work/dir", 42, 10)
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
	if !strings.Contains(argsStr, "TQ_TASK_ID=10") {
		t.Errorf("call[1] args = %v, want to contain TQ_TASK_ID=10", c.args)
	}
	if strings.Contains(argsStr, "--tmux") {
		t.Errorf("call[1] args = %v, must NOT contain --tmux", c.args)
	}
	if strings.Contains(argsStr, "--permission-mode") {
		t.Errorf("call[1] args = %v, must NOT contain --permission-mode when not configured", c.args)
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

	cfg := prompt.Config{}
	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1, 0)
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

	cfg := prompt.Config{}
	_, err := w.Execute(context.Background(), "test", cfg, "/work", 2, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "send claude command") {
		t.Errorf("error = %q, want to contain 'send claude command'", err.Error())
	}
}

func TestInteractiveWorker_CustomSession(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{
		Runner:  runner,
		Session: "work",
	}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "Fix the bug", cfg, "/work/dir", 7, 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Call 1: tmux new-window -t work
	c := runner.calls[0]
	wantArgs := []string{"new-window", "-t", "work", "-n", "tq-action-7", "-c", "/work/dir"}
	if strings.Join(c.args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("call[0] args = %v, want %v", c.args, wantArgs)
	}

	// Call 2: send-keys -t work:tq-action-7
	argsStr := strings.Join(runner.calls[1].args, " ")
	if !strings.Contains(argsStr, "work:tq-action-7") {
		t.Errorf("call[1] args = %v, want to contain work:tq-action-7", runner.calls[1].args)
	}

	// Call 3: send-keys Enter -t work:tq-action-7
	argsStr = strings.Join(runner.calls[2].args, " ")
	if !strings.Contains(argsStr, "work:tq-action-7") {
		t.Errorf("call[2] args = %v, want to contain work:tq-action-7", runner.calls[2].args)
	}
}

func TestInteractiveWorker_PermissionMode(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{
		Runner: runner,
	}

	cfg := prompt.Config{PermissionMode: "plan"}

	_, err := w.Execute(context.Background(), "Plan the feature", cfg, "/work/dir", 50, 10)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	argsStr := strings.Join(runner.calls[1].args, " ")
	if !strings.Contains(argsStr, "--permission-mode 'plan'") {
		t.Errorf("call[1] args = %v, want to contain \"--permission-mode 'plan'\"", runner.calls[1].args)
	}
}

func TestInteractiveWorker_SingleQuoteEscape(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{
		Runner: runner,
	}

	cfg := prompt.Config{}
	_, err := w.Execute(context.Background(), "it's a test", cfg, "/work/dir", 99, 0)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// send-keys command should contain the escaped single quote
	argsStr := strings.Join(runner.calls[1].args, " ")
	if !strings.Contains(argsStr, "it'\\''s a test") {
		t.Errorf("call[1] args = %v, want to contain escaped single quote", runner.calls[1].args)
	}
}
