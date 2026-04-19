package dispatch

import (
	"context"
	"strings"
	"testing"
)

func TestInteractiveWorker_Execute(t *testing.T) {
	tests := []struct {
		name           string
		session        string
		cfg            ActionConfig
		prompt         string
		actionID       int64
		taskID         int64
		wantContains   []string
		wantNotContain []string
		wantNewWindow  []string
	}{
		{
			name:     "basic",
			cfg:      ActionConfig{},
			prompt:   "Fix the bug",
			actionID: 42,
			taskID:   10,
			wantContains: []string{
				"claude", "Fix the bug",
				"TQ_ACTION_ID=42", "TQ_TASK_ID=10",
			},
			wantNotContain: []string{
				"TQ_DIR", "--tmux",
			},
			wantNewWindow: []string{"new-window", "-t", "main", "-n", "tq-action-42", "-c", "/work/dir"},
		},
		{
			name:     "custom session",
			session:  "work",
			cfg:      ActionConfig{},
			prompt:   "Fix the bug",
			actionID: 7,
			wantContains: []string{
				"work:tq-action-7",
			},
			wantNewWindow: []string{"new-window", "-t", "work", "-n", "tq-action-7", "-c", "/work/dir"},
		},
		{
			name:     "claude_args",
			cfg:      ActionConfig{ClaudeArgs: []string{"--max-turns", "5"}},
			prompt:   "Fix the bug",
			actionID: 42,
			taskID:   10,
			wantContains: []string{
				"'--max-turns' '5'",
			},
		},
		{
			name:     "claude_args permission-mode passthrough",
			cfg:      ActionConfig{ClaudeArgs: []string{"--permission-mode", "plan"}},
			prompt:   "Plan the feature",
			actionID: 50,
			taskID:   10,
			wantContains: []string{
				"'--permission-mode' 'plan'",
			},
		},
		{
			name:     "claude_args worktree passthrough",
			cfg:      ActionConfig{ClaudeArgs: []string{"--worktree"}},
			prompt:   "Fix the bug",
			actionID: 42,
			taskID:   10,
			wantContains: []string{
				"'Fix the bug' '--worktree'",
			},
		},
		{
			name:     "claude_args with special characters",
			cfg:      ActionConfig{ClaudeArgs: []string{"--system-prompt", "you're a helper"}},
			prompt:   "Fix the bug",
			actionID: 42,
			taskID:   10,
			wantContains: []string{
				"'you'\\''re a helper'",
			},
		},
		{
			name:     "single quote escape",
			cfg:      ActionConfig{},
			prompt:   "it's a test",
			actionID: 99,
			wantContains: []string{
				"it'\\''s a test",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{output: []byte("ok"), failAt: -1}
			w := &InteractiveWorker{
				Runner:  runner,
				Session: tc.session,
			}

			result, err := w.Execute(context.Background(), tc.prompt, tc.cfg, "/work/dir", tc.actionID, tc.taskID)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}

			if !strings.Contains(result, "action=") {
				t.Errorf("result = %q, want to contain action=", result)
			}

			if len(runner.calls) != 3 {
				t.Fatalf("expected 3 runner calls, got %d", len(runner.calls))
			}

			if tc.wantNewWindow != nil {
				c := runner.calls[0]
				if strings.Join(c.args, " ") != strings.Join(tc.wantNewWindow, " ") {
					t.Errorf("new-window args = %v, want %v", c.args, tc.wantNewWindow)
				}
			}

			argsStr := strings.Join(runner.calls[1].args, " ")
			for _, want := range tc.wantContains {
				if !strings.Contains(argsStr, want) {
					t.Errorf("send-keys args = %q, want to contain %q", argsStr, want)
				}
			}
			for _, notWant := range tc.wantNotContain {
				if strings.Contains(argsStr, notWant) {
					t.Errorf("send-keys args = %q, must NOT contain %q", argsStr, notWant)
				}
			}

			enterCall := runner.calls[2]
			if len(enterCall.args) < 4 || enterCall.args[3] != "Enter" {
				t.Errorf("call[2] args = %v, want last arg to be Enter", enterCall.args)
			}
		})
	}
}

func TestInteractiveWorker_Error(t *testing.T) {
	tests := []struct {
		name        string
		failAt      int
		wantErrText string
	}{
		{
			name:        "new window error",
			failAt:      0,
			wantErrText: "create tmux window",
		},
		{
			name:        "send keys error",
			failAt:      1,
			wantErrText: "send claude command",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{err: context.DeadlineExceeded, failAt: tc.failAt}
			w := &InteractiveWorker{Runner: runner}

			_, err := w.Execute(context.Background(), "test", ActionConfig{}, "/work", 1, 0)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErrText) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrText)
			}
		})
	}
}
