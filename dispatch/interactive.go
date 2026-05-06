package dispatch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// InteractiveWorker opens a tmux window and launches `claude` via send-keys.
// The session reports back via `tq action done`.
type InteractiveWorker struct {
	Runner  CommandRunner
	Session string
}

// Sanity cap on the raw instruction; the prompt is fetched at claude launch
// time via `$(tq action prompt <id>)` so this no longer guards tmux send-keys
// directly, but oversized prompts still degrade the claude session.
const maxInstructionBytes = 16 * 1024

func (w *InteractiveWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	if len(instruction) > maxInstructionBytes {
		return "", fmt.Errorf("instruction too long (%d bytes, limit %d); shorten via generator or split action", len(instruction), maxInstructionBytes)
	}
	// \n is allowed: RenderPrompt's postamble always starts with one. Other C0
	// bytes would corrupt the prompt downstream when claude reads the
	// substituted argv.
	for i := 0; i < len(instruction); i++ {
		b := instruction[i]
		if b < 0x20 && b != '\t' && b != '\n' {
			return "", fmt.Errorf("instruction contains forbidden control character (byte 0x%02x at offset %d); strip control bytes before submitting", b, i)
		}
	}

	session := w.Session
	if session == "" {
		session = "main"
	}
	windowName := WindowName(actionID)

	// 1. Create tmux window
	out, err := w.Runner.Run(ctx, "tmux", []string{
		"new-window", "-t", session, "-n", windowName, "-c", workDir,
	}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("create tmux window: %w (output: %s)", err, string(out))
	}

	// 2. Send claude command text. Prompt rendering is deferred to
	// claude-launch time via `$(tq action prompt <id>)`; inlining the wrapped
	// instruction here would trip macOS pty canonical-mode MAX_CANON (1024
	// bytes) and silently truncate long prompts.
	tmuxTarget := fmt.Sprintf("%s:%s", session, windowName)
	envPrefix := fmt.Sprintf("TQ_ACTION_ID=%d TQ_TASK_ID=%d", actionID, taskID)
	var claudeArgsBuf strings.Builder
	for _, arg := range cfg.ClaudeArgs {
		escaped := strings.ReplaceAll(arg, "'", "'\\''")
		claudeArgsBuf.WriteString(" '")
		claudeArgsBuf.WriteString(escaped)
		claudeArgsBuf.WriteByte('\'')
	}
	claudeCmd := fmt.Sprintf(`%s claude "$(tq action prompt %d)"%s`, envPrefix, actionID, claudeArgsBuf.String())
	out, err = w.Runner.Run(ctx, "tmux", []string{
		"send-keys", "-t", tmuxTarget, claudeCmd,
	}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("send claude command: %w (output: %s)", err, string(out))
	}

	// 3. Send Enter separately (tmux-task convention)
	out, err = w.Runner.Run(ctx, "tmux", []string{
		"send-keys", "-t", tmuxTarget, "Enter",
	}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("send enter key: %w (output: %s)", err, string(out))
	}

	return "interactive:action=" + strconv.FormatInt(actionID, 10), nil
}
