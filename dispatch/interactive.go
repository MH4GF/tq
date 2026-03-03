package dispatch

import (
	"context"
	"fmt"
	"os"
	"strconv"

	tmpl "github.com/MH4GF/tq/template"
)

// InteractiveWorker opens a tmux window and launches `claude` via send-keys.
// The session reports back via `tq action done`.
type InteractiveWorker struct {
	Runner CommandRunner
	TQDir  string
}

func (w *InteractiveWorker) Execute(ctx context.Context, prompt string, cfg tmpl.Config, workDir string, actionID int64) (string, error) {
	windowName := fmt.Sprintf("tq-action-%d", actionID)

	// Write prompt to temp file to avoid shell escaping issues with send-keys
	promptFile := fmt.Sprintf("/tmp/tq-prompt-%d.txt", actionID)
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return "", fmt.Errorf("write prompt file: %w", err)
	}

	// 1. Create tmux window
	out, err := w.Runner.Run(ctx, "tmux", []string{
		"new-window", "-t", "main", "-n", windowName, "-c", workDir,
	}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("create tmux window: %w (output: %s)", err, string(out))
	}

	// 2. Send claude command text
	claudeCmd := fmt.Sprintf("TQ_DIR=%s claude \"$(cat %s)\"", w.TQDir, promptFile)
	out, err = w.Runner.Run(ctx, "tmux", []string{
		"send-keys", "-t", fmt.Sprintf("main:%s", windowName), claudeCmd,
	}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("send claude command: %w (output: %s)", err, string(out))
	}

	// 3. Send Enter separately (tmux-task convention)
	out, err = w.Runner.Run(ctx, "tmux", []string{
		"send-keys", "-t", fmt.Sprintf("main:%s", windowName), "Enter",
	}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("send enter key: %w (output: %s)", err, string(out))
	}

	return "interactive:action=" + strconv.FormatInt(actionID, 10), nil
}
