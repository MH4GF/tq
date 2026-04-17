package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNonInteractiveWorker_Execute(t *testing.T) {
	tests := []struct {
		name           string
		cfg            ActionConfig
		prompt         string
		output         string
		wantArgs       []string
		wantResult     string
		wantDenialsLen int
	}{
		{
			name:       "basic",
			cfg:        ActionConfig{},
			prompt:     "do something",
			output:     `{"type":"result","subtype":"success","result":"{\"result\":\"ok\"}","cost_usd":0.01}`,
			wantArgs:   []string{"-p", "do something", "--output-format", "json"},
			wantResult: `{"result":"ok"}`,
		},
		{
			name:       "permission mode",
			cfg:        ActionConfig{PermissionMode: "plan"},
			prompt:     "plan something",
			output:     `{"type":"result","subtype":"success","result":"ok"}`,
			wantArgs:   []string{"-p", "plan something", "--output-format", "json", "--permission-mode", "plan"},
			wantResult: "ok",
		},
		{
			name:       "worktree",
			cfg:        ActionConfig{Worktree: true},
			prompt:     "do something",
			output:     `{"type":"result","subtype":"success","result":"ok"}`,
			wantArgs:   []string{"-p", "do something", "--output-format", "json", "--worktree"},
			wantResult: "ok",
		},
		{
			name:       "claude_args",
			cfg:        ActionConfig{ClaudeArgs: []string{"--max-turns", "5", "--model", "opus"}},
			prompt:     "do something",
			output:     `{"type":"result","subtype":"success","result":"ok"}`,
			wantArgs:   []string{"-p", "do something", "--output-format", "json", "--max-turns", "5", "--model", "opus"},
			wantResult: "ok",
		},
		{
			name:       "claude_args with permission mode",
			cfg:        ActionConfig{PermissionMode: "plan", ClaudeArgs: []string{"--max-turns", "3"}},
			prompt:     "plan",
			output:     `{"type":"result","subtype":"success","result":"ok"}`,
			wantArgs:   []string{"-p", "plan", "--output-format", "json", "--permission-mode", "plan", "--max-turns", "3"},
			wantResult: "ok",
		},
		{
			name:       "complex output",
			cfg:        ActionConfig{},
			prompt:     "process data",
			output:     `{"type":"result","subtype":"success","result":"{\"status\":\"success\",\"data\":[1,2,3]}"}`,
			wantArgs:   []string{"-p", "process data", "--output-format", "json"},
			wantResult: `{"status":"success","data":[1,2,3]}`,
		},
		{
			name:           "permission denials present",
			cfg:            ActionConfig{},
			prompt:         "fetch user",
			output:         `{"type":"result","subtype":"success","result":"done","permission_denials":[{"tool_name":"Bash","tool_use_id":"toolu_1","tool_input":{"command":"gh api user","description":"fetch"}},{"tool_name":"Bash","tool_use_id":"toolu_2","tool_input":{"command":"gh api notifications","description":"list"}}]}`,
			wantArgs:       []string{"-p", "fetch user", "--output-format", "json"},
			wantResult:     "done\n\n⚠️ permission_denials: 2件\n- Bash: gh api user\n- Bash: gh api notifications\n",
			wantDenialsLen: 2,
		},
		{
			name:       "permission denials empty array",
			cfg:        ActionConfig{},
			prompt:     "do something",
			output:     `{"type":"result","subtype":"success","result":"ok","permission_denials":[]}`,
			wantArgs:   []string{"-p", "do something", "--output-format", "json"},
			wantResult: "ok",
		},
		{
			name:       "permission denials malformed schema",
			cfg:        ActionConfig{},
			prompt:     "do something",
			output:     `{"type":"result","subtype":"success","result":"ok","permission_denials":"unexpected-string"}`,
			wantArgs:   []string{"-p", "do something", "--output-format", "json"},
			wantResult: "ok",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{output: []byte(tc.output), failAt: -1}
			w := &NonInteractiveWorker{Runner: runner}

			result, err := w.Execute(context.Background(), tc.prompt, tc.cfg, "/work", 1, 10)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tc.wantResult {
				t.Errorf("result = %q, want %q", result, tc.wantResult)
			}

			if got := len(w.LastDenials()); got != tc.wantDenialsLen {
				t.Errorf("LastDenials len = %d, want %d", got, tc.wantDenialsLen)
			}

			c := runner.calls[0]
			if c.name != "claude" {
				t.Errorf("name = %q, want %q", c.name, "claude")
			}
			if len(c.args) != len(tc.wantArgs) {
				t.Fatalf("args len = %d, want %d: %v", len(c.args), len(tc.wantArgs), c.args)
			}
			for i, a := range tc.wantArgs {
				if c.args[i] != a {
					t.Errorf("args[%d] = %q, want %q", i, c.args[i], a)
				}
			}
		})
	}
}

func TestNonInteractiveWorker_Execute_DenialsResetBetweenCalls(t *testing.T) {
	outputs := [][]byte{
		[]byte(`{"type":"result","subtype":"success","result":"ok","permission_denials":[{"tool_name":"Bash","tool_input":{"command":"x"}}]}`),
		[]byte(`{"type":"result","subtype":"success","result":"ok"}`),
	}
	runner := &sequenceRunner{outputs: outputs}
	w := &NonInteractiveWorker{Runner: runner}

	if _, err := w.Execute(context.Background(), "first", ActionConfig{}, "/work", 1, 10); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if got := len(w.LastDenials()); got != 1 {
		t.Errorf("after first call: LastDenials len = %d, want 1", got)
	}

	if _, err := w.Execute(context.Background(), "second", ActionConfig{}, "/work", 1, 10); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if got := w.LastDenials(); got != nil {
		t.Errorf("after second call: LastDenials = %v, want nil", got)
	}
}

func TestPermissionDenial_Summary(t *testing.T) {
	tests := []struct {
		name string
		d    PermissionDenial
		want string
	}{
		{
			name: "Bash with command",
			d:    PermissionDenial{ToolName: "Bash", Input: map[string]any{"command": "gh api user"}},
			want: "Bash: gh api user",
		},
		{
			name: "Edit with file_path",
			d:    PermissionDenial{ToolName: "Edit", Input: map[string]any{"file_path": "/tmp/foo.go"}},
			want: "Edit: /tmp/foo.go",
		},
		{
			name: "tool only",
			d:    PermissionDenial{ToolName: "WebFetch", Input: map[string]any{}},
			want: "WebFetch",
		},
		{
			name: "nil input",
			d:    PermissionDenial{ToolName: "Bash"},
			want: "Bash",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.d.Summary(); got != tc.want {
				t.Errorf("Summary() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNonInteractiveWorker_Execute_Env(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"type":"result","subtype":"success","result":"ok"}`),
		failAt: -1,
	}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := ActionConfig{}

	_, err := w.Execute(context.Background(), "do something", cfg, "/work", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := runner.calls[0]
	if c.dir != "/work" {
		t.Errorf("dir = %q, want %q", c.dir, "/work")
	}

	foundActionID := false
	foundTaskID := false
	for _, e := range c.env {
		if e == "TQ_ACTION_ID=1" {
			foundActionID = true
		}
		if e == "TQ_TASK_ID=10" {
			foundTaskID = true
		}
	}
	if !foundActionID {
		t.Errorf("env missing TQ_ACTION_ID=1, got %v", c.env)
	}
	if !foundTaskID {
		t.Errorf("env missing TQ_TASK_ID=10, got %v", c.env)
	}
}

func TestNonInteractiveWorker_Execute_Error(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		runnerErr   error
		failAt      int
		wantErrText string
	}{
		{
			name:        "runner error",
			runnerErr:   errors.New("command failed"),
			failAt:      0,
			wantErrText: "command failed",
		},
		{
			name:        "runner error with output",
			output:      "Error: API key invalid",
			runnerErr:   errors.New("exit status 1"),
			failAt:      0,
			wantErrText: "output: Error: API key invalid",
		},
		{
			name:        "error subtype",
			output:      `{"type":"result","subtype":"error","result":"model refused"}`,
			failAt:      -1,
			wantErrText: `claude returned subtype "error": model refused`,
		},
		{
			name:        "malformed JSON",
			output:      `not json at all`,
			failAt:      -1,
			wantErrText: "failed to parse claude JSON output",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{output: []byte(tc.output), err: tc.runnerErr, failAt: tc.failAt}
			w := &NonInteractiveWorker{Runner: runner}

			_, err := w.Execute(context.Background(), "fail", ActionConfig{}, "/work", 1, 0)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErrText) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrText)
			}
		})
	}
}

func TestNonInteractiveWorker_Execute_Timeout(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"type":"result","subtype":"success","result":"ok"}`),
		failAt: -1,
	}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := ActionConfig{}

	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline, ok := runner.calls[0].ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}

	expected := time.Now().Add(defaultTimeout * time.Second)
	diff := deadline.Sub(expected)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("deadline diff from expected = %v, want within 2s", diff)
	}
}
