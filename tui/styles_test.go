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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderDetailView(&tt.action, 0, 100, 40)
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
