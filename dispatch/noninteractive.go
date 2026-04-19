package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	defaultTimeout                = 600
	nonInteractiveStaleMultiplier = 2
)

type claudeJSONOutput struct {
	Subtype           string          `json:"subtype"`
	Result            string          `json:"result"`
	PermissionDenials json.RawMessage `json:"permission_denials"`
}

// PermissionDenial describes a tool-use denial reported by `claude -p`.
type PermissionDenial struct {
	ToolName string
	Input    map[string]any
}

// Summary returns a one-line description of the denial, e.g. `Bash: gh api user`.
func (d PermissionDenial) Summary() string {
	if cmd, ok := d.Input["command"].(string); ok && cmd != "" {
		return d.ToolName + ": " + cmd
	}
	if fp, ok := d.Input["file_path"].(string); ok && fp != "" {
		return d.ToolName + ": " + fp
	}
	return d.ToolName
}

// NonInteractiveWorker runs `claude -p` for non-interactive actions.
// LastDenials reflects only the most recent Execute call and is reset on each call.
type NonInteractiveWorker struct {
	Runner      CommandRunner
	lastDenials []PermissionDenial
}

// LastDenials returns permission denials observed during the most recent Execute call.
func (w *NonInteractiveWorker) LastDenials() []PermissionDenial {
	return w.lastDenials
}

func (w *NonInteractiveWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	w.lastDenials = nil

	args := []string{"-p", instruction, "--output-format", "json"}
	if len(cfg.ClaudeArgs) > 0 {
		args = append(args, cfg.ClaudeArgs...)
	}
	env := buildTQEnv(actionID, taskID)

	timeoutCtx, cancel := context.WithTimeout(ctx, defaultTimeout*time.Second)
	defer cancel()

	output, err := w.Runner.Run(timeoutCtx, "claude", args, workDir, env)
	if err != nil {
		if len(output) > 0 {
			return "", fmt.Errorf("%w\noutput: %s", err, truncate(output, 2000))
		}
		return "", err
	}

	var wrapper claudeJSONOutput
	if err := json.Unmarshal(output, &wrapper); err != nil {
		return "", fmt.Errorf("failed to parse claude JSON output: %w", err)
	}
	if wrapper.Subtype != "success" {
		return "", fmt.Errorf("claude returned subtype %q: %s", wrapper.Subtype, wrapper.Result)
	}

	w.lastDenials = parsePermissionDenials(wrapper.PermissionDenials)
	if len(w.lastDenials) > 0 {
		return wrapper.Result + formatDenialsWarning(w.lastDenials), nil
	}
	return wrapper.Result, nil
}

func parsePermissionDenials(raw json.RawMessage) []PermissionDenial {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		slog.Warn("failed to parse permission_denials, schema may have changed",
			"error", err, "raw_size_bytes", len(raw))
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	out := make([]PermissionDenial, 0, len(entries))
	for _, e := range entries {
		d := PermissionDenial{}
		if name, ok := e["tool_name"].(string); ok {
			d.ToolName = name
		}
		if input, ok := e["tool_input"].(map[string]any); ok {
			d.Input = input
		}
		if d.ToolName == "" && len(d.Input) == 0 {
			continue
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func truncate(b []byte, max int) []byte {
	if len(b) <= max {
		return b
	}
	return b[:max]
}

func formatDenialsWarning(denials []PermissionDenial) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\n⚠️ permission_denials: %d件\n", len(denials))
	for _, d := range denials {
		b.WriteString("- ")
		b.WriteString(d.Summary())
		b.WriteString("\n")
	}
	return b.String()
}
