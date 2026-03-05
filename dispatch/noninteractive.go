package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tmpl "github.com/MH4GF/tq/template"
)

const defaultTimeout = 300

type claudeJSONOutput struct {
	Subtype string `json:"subtype"`
	Result  string `json:"result"`
}

// NonInteractiveWorker runs `claude -p` for non-interactive actions.
type NonInteractiveWorker struct {
	Runner CommandRunner
}

func (w *NonInteractiveWorker) Execute(ctx context.Context, prompt string, cfg tmpl.Config, workDir string, actionID int64) (string, error) {
	args := []string{"-p", prompt, "--output-format", "json"}
	env := []string{fmt.Sprintf("TQ_ACTION_ID=%d", actionID)}

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
