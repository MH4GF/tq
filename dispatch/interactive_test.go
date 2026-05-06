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
				`claude "$(tq action prompt 42)"`,
				"TQ_ACTION_ID=42", "TQ_TASK_ID=10",
			},
			wantNotContain: []string{
				"TQ_DIR", "--tmux",
				"Fix the bug", // raw instruction must not be inlined (MAX_CANON guard)
				`'\''`,        // legacy single-quote escape must not appear
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
				`claude "$(tq action prompt 7)"`,
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
				`claude "$(tq action prompt 42)" '--max-turns' '5'`,
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
				`claude "$(tq action prompt 42)" '--worktree'`,
			},
		},
		{
			name:     "claude_args with special characters still escaped",
			cfg:      ActionConfig{ClaudeArgs: []string{"--system-prompt", "you're a helper"}},
			prompt:   "Fix the bug",
			actionID: 42,
			taskID:   10,
			wantContains: []string{
				"'you'\\''re a helper'",
			},
		},
		{
			name:     "instruction with apostrophe is not inlined",
			cfg:      ActionConfig{},
			prompt:   "it's a test",
			actionID: 99,
			wantContains: []string{
				`claude "$(tq action prompt 99)"`,
			},
			wantNotContain: []string{
				"it's a test",
				`it'\''s`,
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

// MAX_CANON regression: even multi-KB instructions must produce a short
// send-keys payload. The instruction lives in the DB; the worker only
// emits a `$(tq action prompt N)` substitution.
func TestInteractiveWorker_LongInstructionStaysShortInSendKeys(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{Runner: runner}

	long := strings.Repeat("a", 5000)
	_, err := w.Execute(context.Background(), long, ActionConfig{}, "/work", 4242, 99)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 runner calls, got %d", len(runner.calls))
	}
	sendKeys := strings.Join(runner.calls[1].args, " ")
	// Payload size must be far below macOS pty MAX_CANON (1024 bytes).
	if len(sendKeys) > 256 {
		t.Errorf("send-keys payload too large: %d bytes; want <= 256 (MAX_CANON guard)", len(sendKeys))
	}
	if strings.Contains(sendKeys, long) {
		t.Error("send-keys must NOT inline the raw instruction")
	}
	want := `claude "$(tq action prompt 4242)"`
	if !strings.Contains(sendKeys, want) {
		t.Errorf("send-keys = %q, want to contain %q", sendKeys, want)
	}
}

func TestInteractiveWorker_TooLong(t *testing.T) {
	runner := &mockRunner{output: []byte("ok"), failAt: -1}
	w := &InteractiveWorker{Runner: runner}

	longInstruction := strings.Repeat("a", 17*1024)
	_, err := w.Execute(context.Background(), longInstruction, ActionConfig{}, "/work", 1, 0)
	if err == nil {
		t.Fatal("expected error for too-long instruction")
	}
	if !strings.Contains(err.Error(), "instruction too long") {
		t.Errorf("error = %q, want to contain 'instruction too long'", err.Error())
	}
	if len(runner.calls) != 0 {
		t.Errorf("tmux should not be invoked; got %d calls", len(runner.calls))
	}
}

func TestInteractiveWorker_ControlCharacters(t *testing.T) {
	tests := []struct {
		name        string
		instruction string
		wantErr     bool
	}{
		{name: "newline allowed", instruction: "first line\nrm -rf /", wantErr: false},
		{name: "carriage return", instruction: "first\rsecond", wantErr: true},
		{name: "crlf", instruction: "first\r\nsecond", wantErr: true},
		{name: "escape", instruction: "abc\x1bdef", wantErr: true},
		{name: "backspace", instruction: "abc\bdef", wantErr: true},
		{name: "null", instruction: "abc\x00def", wantErr: true},
		{name: "tab allowed", instruction: "col1\tcol2", wantErr: false},
		{name: "RenderPrompt postamble passthrough", instruction: "user instruction body\n\n---\n\n## tq action context\n\nYou are executing **action #1** (task #2).\n", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{output: []byte("ok"), failAt: -1}
			w := &InteractiveWorker{Runner: runner}

			_, err := w.Execute(context.Background(), tc.instruction, ActionConfig{}, "/work", 1, 0)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error for instruction with control character")
				}
				if !strings.Contains(err.Error(), "forbidden control character") {
					t.Errorf("error = %q, want to contain 'forbidden control character'", err.Error())
				}
				if len(runner.calls) != 0 {
					t.Errorf("tmux should not be invoked; got %d calls", len(runner.calls))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(runner.calls) != 3 {
				t.Errorf("expected 3 runner calls, got %d", len(runner.calls))
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
