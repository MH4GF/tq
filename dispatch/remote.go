package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const RemoteSessionPrefix = "remote:session="

// RemoteWorker runs `claude --remote` for cloud-based execution.
// The session is fire-and-forget; completion is detected via PR branch naming.
type RemoteWorker struct {
	Runner CommandRunner
}

func (w *RemoteWorker) Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error) {
	remotePrompt := instruction + remoteRules(actionID)

	debugFile := filepath.Join(os.TempDir(), fmt.Sprintf("tq-remote-debug-%d.log", actionID))
	defer func() { _ = os.Remove(debugFile) }()

	// claude --remote requires a TTY (without one, claude switches to --print mode).
	// Use `script -q /dev/null` to allocate a PTY.
	args := []string{"-q", "/dev/null", "claude", "--remote", remotePrompt, "--debug-file", debugFile}
	env := buildTQEnv(actionID, taskID)

	output, err := w.Runner.Run(ctx, "script", args, workDir, env)
	cleanOutput := stripANSI(string(output))
	if err != nil {
		diag := extractDiagnosis(debugFile)
		return "", fmt.Errorf("claude --remote: %w\noutput: %s%s", err, cleanOutput, diag)
	}

	sessionURL := parseSessionURL(cleanOutput)
	return RemoteSessionPrefix + sessionURL, nil
}

func remoteRules(actionID int64) string {
	return fmt.Sprintf(`

## Remote Execution Rules
- Branch name MUST start with `+"`tq-%d-`"+` (e.g. tq-%d-add-feature)
- Create a Pull Request when work is complete — this is the completion signal
- /tq:done is NOT available in remote sessions`, actionID, actionID)
}

// extractDiagnosis reads the debug log file and extracts ERROR lines for diagnosis.
func extractDiagnosis(debugFile string) string {
	data, err := os.ReadFile(debugFile)
	if err != nil {
		return ""
	}

	var errors []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.Contains(line, "[ERROR]") {
			errors = append(errors, strings.TrimSpace(line))
		}
	}
	if len(errors) == 0 {
		return ""
	}
	return "\ndiagnosis:\n" + strings.Join(errors, "\n")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\[\?[0-9;]*[a-zA-Z]|\x1b\[<[a-zA-Z]|\r|\x04|\b`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func parseSessionURL(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "https://"); idx >= 0 {
			return line[idx:]
		}
	}
	return strings.TrimSpace(output)
}
