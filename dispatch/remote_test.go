package dispatch

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRemoteWorker_Execute(t *testing.T) {
	runner := &mockRunner{
		output: []byte("https://console.anthropic.com/p/abc123\n"),
		failAt: -1,
	}

	w := &RemoteWorker{Runner: runner}
	result, err := w.Execute(context.Background(), "do the thing", ActionConfig{Mode: "remote"}, "/tmp/work", 42, 10)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}

	c := runner.calls[0]
	if c.name != "script" {
		t.Errorf("command = %q, want script", c.name)
	}
	// args: -q /dev/null claude --remote <prompt> --debug-file <path>
	if c.args[0] != "-q" || c.args[1] != "/dev/null" {
		t.Errorf("args[0:2] = %v, want [-q /dev/null]", c.args[0:2])
	}
	if c.args[2] != "claude" || c.args[3] != "--remote" {
		t.Errorf("args[2:4] = %v, want [claude --remote]", c.args[2:4])
	}
	if !strings.Contains(c.args[4], "tq-42-") {
		t.Errorf("prompt should contain branch naming rule with action ID, got: %s", c.args[4])
	}
	if c.args[5] != "--debug-file" {
		t.Errorf("args[5] = %q, want --debug-file", c.args[5])
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
	_, err := w.Execute(context.Background(), "prompt", ActionConfig{Mode: "remote"}, ".", 1, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractDiagnosis(t *testing.T) {
	tests := []struct {
		name       string
		content    string // empty means no file created
		useNoFile  bool
		wantEmpty  bool
		wantSubstr []string
		wantAbsent []string
	}{
		{
			name: "extracts ERROR lines from debug log",
			content: `2026-03-16T09:17:43.463Z [DEBUG] Error log sink initialized
2026-03-16T09:17:44.029Z [ERROR] AxiosError: [url=https://api.anthropic.com/api/oauth/account/settings,status=403,body=OAuth token does not meet scope requirement user:profile] Error
2026-03-16T09:17:45.487Z [DEBUG] [teleportToRemote] Git source: github.com/MH4GF/tq
2026-03-16T09:17:46.239Z [ERROR] AxiosError: [url=https://api.anthropic.com/v1/sessions,status=401,body=Authentication failed] Error
`,
			wantSubstr: []string{"diagnosis:", "status=401", "status=403"},
			wantAbsent: []string{"[DEBUG]"},
		},
		{
			name:      "returns empty for missing file",
			useNoFile: true,
			wantEmpty: true,
		},
		{
			name:      "returns empty when no ERROR lines",
			content:   "2026-03-16T09:17:43.463Z [DEBUG] all good\n",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var debugFile string
			if tt.useNoFile {
				debugFile = "/nonexistent/file"
			} else {
				debugFile = t.TempDir() + "/debug.log"
				if err := os.WriteFile(debugFile, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			diag := extractDiagnosis(debugFile)
			if tt.wantEmpty {
				if diag != "" {
					t.Errorf("want empty, got: %s", diag)
				}
				return
			}
			for _, s := range tt.wantSubstr {
				if !strings.Contains(diag, s) {
					t.Errorf("should contain %q, got: %s", s, diag)
				}
			}
			for _, s := range tt.wantAbsent {
				if strings.Contains(diag, s) {
					t.Errorf("should not contain %q, got: %s", s, diag)
				}
			}
		})
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
