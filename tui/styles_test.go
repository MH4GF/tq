package tui

import "testing"

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
