package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BgWorker dispatches via `claude --bg "<prompt>" [args...]`. The command
// returns immediately after the daemon supervisor registers the session;
// lifecycle (working / done / failed) is tracked by the queue worker reaper
// reading ~/.claude/jobs/<short>/state.json.
type BgWorker struct {
	Runner CommandRunner
}

// bgShortRe matches the daemon-assigned short id printed on the first line of
// `claude --bg` output, e.g. `backgrounded · 239007b1`.
var bgShortRe = regexp.MustCompile(`(?m)^backgrounded · ([a-f0-9]{8})\b`)

// bgDisabledMarker is emitted by `claude --bg` when the agent view is
// disabled (settings.disableAgentView or CLAUDE_CODE_DISABLE_AGENT_VIEW=1)
// or when the running Claude Code version predates the feature.
const bgDisabledMarker = "'--bg' is not enabled"

func (w *BgWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, _, _ int64) (string, error) {
	args := append([]string{"--bg", instruction}, cfg.ClaudeArgs...)

	output, err := w.Runner.Run(ctx, "claude", args, workDir, nil)
	if err != nil {
		return "", appendOutput(err, output)
	}

	// claude --bg may wrap the short id (and other text) in ANSI SGR color
	// codes. Strip them before matching so a colorized banner like
	// "backgrounded · \x1b[36m239007b1\x1b[39m" still parses; otherwise a
	// healthy session is falsely marked failed because the reaper never
	// learns its short id.
	s := stripANSI(string(output))
	if strings.Contains(s, bgDisabledMarker) {
		return "", fmt.Errorf("claude --bg refused: %s", strings.TrimSpace(s))
	}
	m := bgShortRe.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("could not parse bg short id from claude --bg output: %s", truncate(output, 2000))
	}
	return m[1], nil
}

// BgStateReader reads the daemon-maintained state.json for a bg session.
// Implementations MUST propagate os.IsNotExist so the reaper can distinguish
// "job dir missing" (mark failed after grace period) from transient I/O
// errors (skip this tick).
type BgStateReader interface {
	ReadState(short string) ([]byte, error)
}

// FileBgStateReader reads ~/.claude/jobs/<short>/state.json from disk. Used
// in production by the queue worker reaper.
type FileBgStateReader struct{}

func (FileBgStateReader) ReadState(short string) ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate home dir: %w", err)
	}
	return os.ReadFile(filepath.Join(home, ".claude", "jobs", short, "state.json"))
}
