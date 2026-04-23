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

// Guards tmux send-keys; its internal buffer rejects larger inputs with "command too long".
const maxInstructionBytes = 16 * 1024

func (w *InteractiveWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	if len(instruction) > maxInstructionBytes {
		return "", fmt.Errorf("instruction too long (%d bytes, limit %d); shorten via generator or split action", len(instruction), maxInstructionBytes)
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

	// 2. Send claude command text
	tmuxTarget := fmt.Sprintf("%s:%s", session, windowName)
	escapedPrompt := strings.ReplaceAll(instruction, "'", "'\\''")
	envPrefix := fmt.Sprintf("TQ_ACTION_ID=%d TQ_TASK_ID=%d", actionID, taskID)
	var claudeArgsBuf strings.Builder
	for _, arg := range cfg.ClaudeArgs {
		escaped := strings.ReplaceAll(arg, "'", "'\\''")
		claudeArgsBuf.WriteString(" '")
		claudeArgsBuf.WriteString(escaped)
		claudeArgsBuf.WriteByte('\'')
	}
	claudeCmd := fmt.Sprintf("%s claude '%s'%s", envPrefix, escapedPrompt, claudeArgsBuf.String())
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
