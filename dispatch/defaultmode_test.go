package dispatch

import (
	"strings"
	"testing"
)

func TestResolveDefaultMode(t *testing.T) {
	tests := []struct {
		name          string
		actionMeta    map[string]any
		globalDefault string
		want          string
		wantErrSub    string
	}{
		{
			name:          "explicit action mode wins, nothing stamped",
			actionMeta:    map[string]any{MetaKeyMode: ModeInteractive},
			globalDefault: ModeBg,
			want:          "",
		},
		{
			name:          "no explicit mode, global default adopted",
			actionMeta:    map[string]any{},
			globalDefault: ModeBg,
			want:          ModeBg,
		},
		{
			name:          "empty explicit mode is treated as unset",
			actionMeta:    map[string]any{MetaKeyMode: ""},
			globalDefault: ModeNonInteractive,
			want:          ModeNonInteractive,
		},
		{
			name:          "nothing configured, nothing stamped",
			actionMeta:    map[string]any{},
			globalDefault: "",
			want:          "",
		},
		{
			name:          "invalid global default errors",
			actionMeta:    map[string]any{},
			globalDefault: "plan",
			wantErrSub:    `configured default mode "plan" is invalid`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveDefaultMode(tc.actionMeta, tc.globalDefault)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolveDefaultMode = %q, want %q", got, tc.want)
			}
		})
	}
}
