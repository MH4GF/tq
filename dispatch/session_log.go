package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeSessionLogChecker checks if a Claude Code session is actively working.
type ClaudeSessionLogChecker interface {
	IsClaudeSessionActive(workDir string, freshnessThreshold time.Duration) (active bool, err error)
	// SessionLogExists reports whether a session log .jsonl exists anywhere
	// under ~/.claude/projects for the given claude session id. Unlike
	// IsClaudeSessionActive it does not depend on ~/.claude/sessions/*.json
	// (deleted on process exit), so it stays true after the session ends.
	SessionLogExists(sessionID string) (exists bool, err error)
}

// FileClaudeSessionLogChecker implements ClaudeSessionLogChecker by scanning
// ~/.claude/sessions/*.json and checking the corresponding session log file's
// modification time.
type FileClaudeSessionLogChecker struct {
	HomeDir string // overrides home directory for testing
}

// claudeSessionMeta mirrors the JSON shape of ~/.claude/sessions/*.json. The
// SessionID field maps to the raw "sessionId" key written by Claude Code.
type claudeSessionMeta struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

func (c *FileClaudeSessionLogChecker) IsClaudeSessionActive(workDir string, freshnessThreshold time.Duration) (bool, error) {
	homeDir := c.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return false, fmt.Errorf("get home dir: %w", err)
		}
	}

	sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return false, fmt.Errorf("read sessions dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}

		var meta claudeSessionMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		if !cwdPrefixMatch(meta.Cwd, workDir) {
			continue
		}

		logPath := sessionLogPath(homeDir, meta.Cwd, meta.SessionID)
		info, err := os.Stat(logPath)
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) < freshnessThreshold {
			return true, nil
		}
	}

	return false, nil
}

// cwdPrefixMatch handles worktree subdirectories
// (e.g., workDir=/repo matches session cwd=/repo/.claude/worktrees/xxx).
func cwdPrefixMatch(sessionCwd, workDir string) bool {
	return pathHasPrefix(sessionCwd, workDir) || pathHasPrefix(workDir, sessionCwd)
}

func pathHasPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

func (c *FileClaudeSessionLogChecker) SessionLogExists(sessionID string) (bool, error) {
	homeDir := c.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return false, fmt.Errorf("get home dir: %w", err)
		}
	}
	matches, err := filepath.Glob(filepath.Join(homeDir, ".claude", "projects", "*", sessionID+".jsonl"))
	if err != nil {
		return false, fmt.Errorf("glob session log: %w", err)
	}
	return len(matches) > 0, nil
}

func encodeCwd(cwd string) string {
	r := strings.NewReplacer("/", "-", ".", "-")
	return r.Replace(cwd)
}

func sessionLogPath(homeDir, cwd, sessionID string) string {
	return filepath.Join(homeDir, ".claude", "projects", encodeCwd(cwd), sessionID+".jsonl")
}
