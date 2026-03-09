package dispatch

import (
	"context"
	"strings"
	"testing"

	"github.com/MH4GF/tq/prompt"
)

func TestRemoteWorker_Execute(t *testing.T) {
	runner := &mockRunner{
		output: []byte("https://console.anthropic.com/p/abc123\n"),
		failAt: -1,
	}

	w := &RemoteWorker{Runner: runner}
	result, err := w.Execute(context.Background(), "do the thing", prompt.Config{Mode: "remote"}, "/tmp/work", 42)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}

	c := runner.calls[0]
	if c.name != "claude" {
		t.Errorf("command = %q, want claude", c.name)
	}
	if c.args[0] != "--remote" {
		t.Errorf("args[0] = %q, want --remote", c.args[0])
	}
	if !strings.Contains(c.args[1], "tq-42-") {
		t.Errorf("prompt should contain branch naming rule with action ID, got: %s", c.args[1])
	}
	if c.dir != "/tmp/work" {
		t.Errorf("dir = %q, want /tmp/work", c.dir)
	}
	if result != "remote:session=https://console.anthropic.com/p/abc123" {
		t.Errorf("result = %q, want remote:session=https://console.anthropic.com/p/abc123", result)
	}
}

func TestRemoteWorker_ExecuteError(t *testing.T) {
	runner := &mockRunner{
		err:    context.DeadlineExceeded,
		failAt: 0,
	}

	w := &RemoteWorker{Runner: runner}
	_, err := w.Execute(context.Background(), "prompt", prompt.Config{Mode: "remote"}, ".", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSessionURL(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "URL on its own line",
			output: "Starting remote session...\nhttps://console.anthropic.com/p/abc123\n",
			want:   "https://console.anthropic.com/p/abc123",
		},
		{
			name:   "no URL",
			output: "session started",
			want:   "session started",
		},
		{
			name:   "URL with whitespace",
			output: "  https://example.com/session  \n",
			want:   "https://example.com/session",
		},
		{
			name:   "View: prefix",
			output: "View: https://claude.ai/code/session_01DRSiqedrMrewdvjRqtujYe?m=0\n",
			want:   "https://claude.ai/code/session_01DRSiqedrMrewdvjRqtujYe?m=0",
		},
		{
			name:   "full claude --remote output",
			output: "Created remote session: Test implementation\nView: https://claude.ai/code/session_01DRSiqedrMrewdvjRqtujYe?m=0\nResume with: claude --teleport session_01DRSiqedrMrewdvjRqtujYe\n",
			want:   "https://claude.ai/code/session_01DRSiqedrMrewdvjRqtujYe?m=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSessionURL(tt.output)
			if got != tt.want {
				t.Errorf("parseSessionURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
