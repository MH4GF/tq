package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEncodeCwd(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/Users/user/project", "-Users-user-project"},
		{"/", "-"},
		{"/a/b/c", "-a-b-c"},
		{"/Users/user/ghq/github.com/org/repo", "-Users-user-ghq-github-com-org-repo"},
		{"/path/to/.claude/worktrees/name", "-path-to--claude-worktrees-name"},
	}
	for _, tt := range tests {
		got := encodeCwd(tt.input)
		if got != tt.want {
			t.Errorf("encodeCwd(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCwdPrefixMatch(t *testing.T) {
	tests := []struct {
		sessionCwd string
		workDir    string
		want       bool
	}{
		{"/repo", "/repo", true},
		{"/repo/.claude/worktrees/xxx", "/repo", true},
		{"/repo", "/repo/.claude/worktrees/xxx", true},
		{"/other/repo", "/repo", false},
		{"/repo-other", "/repo", false},
	}
	for _, tt := range tests {
		got := cwdPrefixMatch(tt.sessionCwd, tt.workDir)
		if got != tt.want {
			t.Errorf("cwdPrefixMatch(%q, %q) = %v, want %v", tt.sessionCwd, tt.workDir, got, tt.want)
		}
	}
}

func writeSessionMeta(t *testing.T, homeDir, cwd, sessionID string) {
	t.Helper()
	sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := claudeSessionMeta{PID: 12345, SessionID: sessionID, Cwd: cwd}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(sessionsDir, "12345.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSessionLog(t *testing.T, homeDir, cwd, sessionID string, logAge time.Duration) {
	t.Helper()
	logDir := filepath.Join(homeDir, ".claude", "projects", encodeCwd(cwd))
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, sessionID+".jsonl")
	if err := os.WriteFile(logPath, []byte(`{"type":"user"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modTime := time.Now().Add(-logAge)
	if err := os.Chtimes(logPath, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestFileSessionLogChecker(t *testing.T) {
	tests := []struct {
		name          string
		sessionCwd    string
		sessionID     string
		logAge        time.Duration
		skipLog       bool
		cwd           string
		wantActive    bool
		wantSessionID string
		wantErr       bool
	}{
		{
			name:          "active session",
			sessionCwd:    "/test/project",
			sessionID:     "session-abc",
			logAge:        10 * time.Second,
			cwd:           "/test/project",
			wantActive:    true,
			wantSessionID: "session-abc",
		},
		{
			name:       "stale session",
			sessionCwd: "/test/project",
			sessionID:  "session-abc",
			logAge:     300 * time.Second,
			cwd:        "/test/project",
		},
		{
			name:       "no matching session",
			sessionCwd: "/other/project",
			sessionID:  "session-abc",
			logAge:     10 * time.Second,
			cwd:        "/test/project",
		},
		{
			name:       "missing log file",
			sessionCwd: "/test/project",
			sessionID:  "session-abc",
			skipLog:    true,
			cwd:        "/test/project",
		},
		{
			name:          "worktree prefix match",
			sessionCwd:    "/test/project/.claude/worktrees/gentle-wobbling-rossum",
			sessionID:     "session-wt",
			logAge:        10 * time.Second,
			cwd:           "/test/project",
			wantActive:    true,
			wantSessionID: "session-wt",
		},
		{
			name:    "no sessions dir",
			cwd:     "/test/project",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			if tt.sessionCwd != "" {
				writeSessionMeta(t, homeDir, tt.sessionCwd, tt.sessionID)
				if !tt.skipLog {
					writeSessionLog(t, homeDir, tt.sessionCwd, tt.sessionID, tt.logAge)
				}
			}

			checker := &FileSessionLogChecker{HomeDir: homeDir}
			active, sessionID, err := checker.IsSessionActive(tt.cwd, 120*time.Second)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if active != tt.wantActive {
				t.Errorf("active = %v, want %v", active, tt.wantActive)
			}
			if sessionID != tt.wantSessionID {
				t.Errorf("sessionID = %q, want %q", sessionID, tt.wantSessionID)
			}
		})
	}
}
