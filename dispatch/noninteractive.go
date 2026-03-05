package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tmpl "github.com/MH4GF/tq/template"
)

type claudeJSONOutput struct {
	Subtype string `json:"subtype"`
	Result  string `json:"result"`
}

// NonInteractiveWorker runs `claude -p` for non-interactive actions.
type NonInteractiveWorker struct {
	Runner CommandRunner
	TQDir  string
}

func (w *NonInteractiveWorker) Execute(ctx context.Context, prompt string, cfg tmpl.Config, workDir string, actionID int64) (string, error) {
	args := []string{"-p", prompt, "--output-format", "json"}
	if cfg.JSONSchema != "" {
		args = append(args, "--json-schema", strings.TrimSpace(cfg.JSONSchema))
	}
	args = append(args, "--allowedTools", cfg.AllowedTools)
	env := []string{"TQ_DIR=" + w.TQDir, fmt.Sprintf("TQ_ACTION_ID=%d", actionID)}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	output, err := w.Runner.Run(timeoutCtx, "claude", args, workDir, env)
	if err != nil {
		return "", err
	}

	if cfg.JSONSchema != "" {
		return extractStructuredOutput(output)
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

func extractStructuredOutput(output []byte) (string, error) {
	var envelope struct {
		Subtype          string          `json:"subtype"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		return "", fmt.Errorf("failed to parse claude JSON output: %w", err)
	}
	if envelope.Subtype != "success" {
		return "", fmt.Errorf("claude returned subtype %q: %s", envelope.Subtype, envelope.Result)
	}
	if len(envelope.StructuredOutput) == 0 {
		return "", fmt.Errorf("claude JSON output missing structured_output field")
	}
	return string(envelope.StructuredOutput), nil
}
