package tui

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
)

func TestRenderDetailView(t *testing.T) {
	tests := []struct {
		name        string
		action      db.Action
		wantContain []string
	}{
		{
			name: "done with instruction and result",
			action: db.Action{
				ID: 1, Title: "do it", Status: db.ActionStatusDone,
				Metadata: `{"instruction":"the instruction body"}`,
				Result:   sql.NullString{String: "the result body", Valid: true},
			},
			wantContain: []string{"the instruction body", "the result body"},
		},
		{
			name: "pending with instruction, no result",
			action: db.Action{
				ID: 2, Title: "pending", Status: db.ActionStatusPending,
				Metadata: `{"instruction":"check the logs"}`,
			},
			wantContain: []string{"check the logs", "(no result yet)"},
		},
		{
			name: "empty metadata does not panic",
			action: db.Action{
				ID: 3, Title: "bare", Status: db.ActionStatusRunning, Metadata: "{}",
			},
			wantContain: []string{"(no instruction)", "(no result yet)"},
		},
		{
			name: "dispatch metadata shown",
			action: db.Action{
				ID: 4, Title: "dispatched", Status: db.ActionStatusRunning,
				WorkDir: "/tmp/work",
				Metadata: `{"instruction":"go","mode":"interactive","executor":"local",` +
					`"claude_args":["--worktree","scope-name","--effort","xhigh"],` +
					`"claude_session_id":"sess-abc","daemon_short":"6983ea7f","parent_action_id":42}`,
			},
			wantContain: []string{
				"Mode", "interactive",
				"Executor", "local",
				"Work dir", "/tmp/work",
				"Args", "--worktree", "scope-name", "--effort", "xhigh",
				"Session", "sess-abc",
				"Daemon", "6983ea7f",
				"Parent", "#42",
			},
		},
		{
			name: "timestamps and tmux fields",
			action: db.Action{
				ID: 5, Title: "tmux", Status: db.ActionStatusRunning,
				CreatedAt:     "2026-05-23 10:00:00",
				StartedAt:     sql.NullString{String: "2026-05-23 10:05:00", Valid: true},
				CompletedAt:   sql.NullString{String: "2026-05-23 10:30:00", Valid: true},
				DispatchAfter: sql.NullString{String: "2026-05-23 11:00:00", Valid: true},
				TmuxSession:   sql.NullString{String: "tq", Valid: true},
				TmuxWindow:    sql.NullString{String: "5", Valid: true},
				Metadata:      `{}`,
			},
			wantContain: []string{
				"Created",
				"Started",
				"Completed",
				"After",
				"Tmux", "tq:5",
			},
		},
		{
			name: "long claude_args wraps without truncation",
			action: db.Action{
				ID: 6, Title: "wrap", Status: db.ActionStatusPending,
				Metadata: `{"claude_args":["--worktree","very-long-scope-name-that-might-overflow","--effort","xhigh","--permission-mode","plan"]}`,
			},
			wantContain: []string{
				"--worktree",
				"very-long-scope-name-that-might-overflow",
				"--permission-mode",
				"plan",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderDetailView(&tt.action, nil, 0, 100, 60)
			for _, want := range tt.wantContain {
				if !strings.Contains(out, want) {
					t.Errorf("RenderDetailView output missing %q\ngot:\n%s", want, out)
				}
			}
		})
	}
}

func TestTruncateResult(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "newlines replaced with spaces",
			input:  "line1\nline2\nline3",
			maxLen: 100,
			want:   "line1 line2 line3",
		},
		{
			name:   "long string truncated with ellipsis",
			input:  "abcdefghij",
			maxLen: 5,
			want:   "abcde...",
		},
		{
			name:   "exact length unchanged",
			input:  "12345",
			maxLen: 5,
			want:   "12345",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateResult(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateResult(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
