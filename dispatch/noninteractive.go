package dispatch

import (
	"context"
	"time"

	tmpl "github.com/MH4GF/tq/template"
)

// NonInteractiveWorker runs `claude -p` for non-interactive actions.
type NonInteractiveWorker struct {
	Runner CommandRunner
	TQDir  string
}

func (w *NonInteractiveWorker) Execute(ctx context.Context, prompt string, cfg tmpl.Config, workDir string, actionID int64) (string, error) {
	args := []string{"-p", prompt, "--output-format", "json", "--allowedTools", cfg.AllowedTools}
	env := []string{"TQ_DIR=" + w.TQDir}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	output, err := w.Runner.Run(timeoutCtx, "claude", args, workDir, env)
	if err != nil {
		return "", err
	}
	return string(output), nil
}
