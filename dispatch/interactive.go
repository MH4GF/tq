package dispatch

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/MH4GF/tq/prompt"
)

// InteractiveWorker opens a tmux window and launches `claude` via send-keys.
// The session reports back via `tq action done`.
type InteractiveWorker struct {
	Runner  CommandRunner
	Session string
}

func (w *InteractiveWorker) Execute(ctx context.Context, prompt string, cfg prompt.Config, workDir string, actionID int64, taskID *int64) (string, error) {
	session := w.Session
	if session == "" {
		session = "main"
	}
	windowName := fmt.Sprintf("tq-action-%d", actionID)

	// 1. Create tmux window
	out, err := w.Runner.Run(ctx, "tmux", []string{
		"new-window", "-t", session, "-n", windowName, "-c", workDir,
	}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("create tmux window: %w (output: %s)", err, string(out))
	}

	// 2. Send claude command text
	tmuxTarget := fmt.Sprintf("%s:%s", session, windowName)
	escapedPrompt := strings.ReplaceAll(prompt, "'", "'\\''")
	envPrefix := fmt.Sprintf("TQ_ACTION_ID=%d", actionID)
	if taskID != nil {
		envPrefix += fmt.Sprintf(" TQ_TASK_ID=%d", *taskID)
	}
	claudeCmd := fmt.Sprintf("%s claude '%s'", envPrefix, escapedPrompt)
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
