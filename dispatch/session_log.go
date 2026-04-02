package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionLogChecker checks if a Claude Code session is actively working.
type SessionLogChecker interface {
	IsSessionActive(workDir string, freshnessThreshold time.Duration) (active bool, sessionID string, err error)
}

// FileSessionLogChecker implements SessionLogChecker by scanning ~/.claude/sessions/*.json
// and checking the corresponding session log file's modification time.
type FileSessionLogChecker struct {
	HomeDir string // overrides home directory for testing
}

type claudeSessionMeta struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

func (c *FileSessionLogChecker) IsSessionActive(workDir string, freshnessThreshold time.Duration) (bool, string, error) {
	homeDir := c.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return false, "", fmt.Errorf("get home dir: %w", err)
		}
	}

	sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return false, "", fmt.Errorf("read sessions dir: %w", err)
	}

	var bestSessionID string
	var bestModTime time.Time

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

		modTime := info.ModTime()
		if time.Since(modTime) < freshnessThreshold {
			if bestModTime.IsZero() || modTime.After(bestModTime) {
				bestModTime = modTime
				bestSessionID = meta.SessionID
			}
		}
	}

	if bestSessionID != "" {
		return true, bestSessionID, nil
	}
	return false, "", nil
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

func encodeCwd(cwd string) string {
	r := strings.NewReplacer("/", "-", ".", "-")
	return r.Replace(cwd)
}

func sessionLogPath(homeDir, cwd, sessionID string) string {
	return filepath.Join(homeDir, ".claude", "projects", encodeCwd(cwd), sessionID+".jsonl")
}
