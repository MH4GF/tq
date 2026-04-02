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

func setupTestSessionDir(t *testing.T, homeDir, cwd, sessionID string, logAge time.Duration) {
	t.Helper()

	// Create session metadata file
	sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := claudeSessionMeta{PID: 12345, SessionID: sessionID, Cwd: cwd}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(sessionsDir, "12345.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create session log file
	logDir := filepath.Join(homeDir, ".claude", "projects", encodeCwd(cwd))
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, sessionID+".jsonl")
	if err := os.WriteFile(logPath, []byte(`{"type":"user"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set modification time
	modTime := time.Now().Add(-logAge)
	if err := os.Chtimes(logPath, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestFileSessionLogChecker_ActiveSession(t *testing.T) {
	homeDir := t.TempDir()
	setupTestSessionDir(t, homeDir, "/test/project", "session-abc", 10*time.Second)

	checker := &FileSessionLogChecker{HomeDir: homeDir}
	active, sessionID, err := checker.IsSessionActive("/test/project", 120*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active {
		t.Error("expected active=true")
	}
	if sessionID != "session-abc" {
		t.Errorf("sessionID = %q, want %q", sessionID, "session-abc")
	}
}

func TestFileSessionLogChecker_StaleSession(t *testing.T) {
	homeDir := t.TempDir()
	setupTestSessionDir(t, homeDir, "/test/project", "session-abc", 300*time.Second)

	checker := &FileSessionLogChecker{HomeDir: homeDir}
	active, sessionID, err := checker.IsSessionActive("/test/project", 120*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected active=false for stale session")
	}
	if sessionID != "" {
		t.Errorf("sessionID = %q, want empty", sessionID)
	}
}

func TestFileSessionLogChecker_NoMatchingSession(t *testing.T) {
	homeDir := t.TempDir()
	setupTestSessionDir(t, homeDir, "/other/project", "session-abc", 10*time.Second)

	checker := &FileSessionLogChecker{HomeDir: homeDir}
	active, sessionID, err := checker.IsSessionActive("/test/project", 120*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected active=false for non-matching session")
	}
	if sessionID != "" {
		t.Errorf("sessionID = %q, want empty", sessionID)
	}
}

func TestFileSessionLogChecker_MissingLogFile(t *testing.T) {
	homeDir := t.TempDir()

	// Create session metadata but no log file
	sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := claudeSessionMeta{PID: 12345, SessionID: "session-abc", Cwd: "/test/project"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(sessionsDir, "12345.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	checker := &FileSessionLogChecker{HomeDir: homeDir}
	active, sessionID, err := checker.IsSessionActive("/test/project", 120*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected active=false when log file missing")
	}
	if sessionID != "" {
		t.Errorf("sessionID = %q, want empty", sessionID)
	}
}

func TestFileSessionLogChecker_WorktreePrefixMatch(t *testing.T) {
	homeDir := t.TempDir()
	worktreeCwd := "/test/project/.claude/worktrees/gentle-wobbling-rossum"
	setupTestSessionDir(t, homeDir, worktreeCwd, "session-wt", 10*time.Second)

	checker := &FileSessionLogChecker{HomeDir: homeDir}
	active, sessionID, err := checker.IsSessionActive("/test/project", 120*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active {
		t.Error("expected active=true for worktree prefix match")
	}
	if sessionID != "session-wt" {
		t.Errorf("sessionID = %q, want %q", sessionID, "session-wt")
	}
}

func TestFileSessionLogChecker_NoSessionsDir(t *testing.T) {
	homeDir := t.TempDir()

	checker := &FileSessionLogChecker{HomeDir: homeDir}
	active, sessionID, err := checker.IsSessionActive("/test/project", 120*time.Second)
	if err == nil {
		t.Error("expected error when sessions dir does not exist")
	}
	if active {
		t.Error("expected active=false on error")
	}
	if sessionID != "" {
		t.Errorf("sessionID = %q, want empty", sessionID)
	}
}
