package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	defaultTimeout                = 600
	nonInteractiveStaleMultiplier = 2
)

type claudeJSONOutput struct {
	Subtype string `json:"subtype"`
	Result  string `json:"result"`
}

// NonInteractiveWorker runs `claude -p` for non-interactive actions.
type NonInteractiveWorker struct {
	Runner CommandRunner
}

func (w *NonInteractiveWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	args := []string{"-p", instruction, "--output-format", "json"}
	if cfg.PermissionMode != "" {
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}
	if cfg.Worktree {
		args = append(args, "--worktree")
	}
	env := buildTQEnv(actionID, taskID)

	timeoutCtx, cancel := context.WithTimeout(ctx, defaultTimeout*time.Second)
	defer cancel()

	output, err := w.Runner.Run(timeoutCtx, "claude", args, workDir, env)
	if err != nil {
		return "", err
	}

	var wrapper claudeJSONOutput
	if err := json.Unmarshal(output, &wrapper); err != nil {
		return "", fmt.Errorf("failed to parse claude JSON output: %w", err)
	}
	if wrapper.Subtype != "success" {
		return "", fmt.Errorf("claude returned subtype %q: %s", wrapper.Subtype, wrapper.Result)
	}
	return wrapper.Result, nil
}
