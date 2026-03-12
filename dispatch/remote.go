package dispatch

import (
	"context"
	"fmt"
	"strings"

	"github.com/MH4GF/tq/prompt"
)

const RemoteSessionPrefix = "remote:session="

// RemoteWorker runs `claude --remote` for cloud-based execution.
// The session is fire-and-forget; completion is detected via PR branch naming.
type RemoteWorker struct {
	Runner CommandRunner
}

func (w *RemoteWorker) Execute(ctx context.Context, prompt string, cfg prompt.Config, workDir string, actionID int64, taskID int64) (string, error) {
	remotePrompt := prompt + remoteRules(actionID)

	// claude --remote requires a TTY (stdout must be a terminal).
	// Without a TTY, claude auto-switches to --print mode and fails.
	// Use `script -q /dev/null` to allocate a PTY.
	args := []string{"-q", "/dev/null", "claude", "--remote", remotePrompt}
	env := buildTQEnv(actionID, taskID)

	output, err := w.Runner.Run(ctx, "script", args, workDir, env)
	if err != nil {
		return "", fmt.Errorf("claude --remote: %w\noutput: %s", err, string(output))
	}

	sessionURL := parseSessionURL(string(output))
	return RemoteSessionPrefix + sessionURL, nil
}

func remoteRules(actionID int64) string {
	return fmt.Sprintf(`

## Remote Execution Rules
- Branch name MUST start with `+"`tq-%d-`"+` (e.g. tq-%d-add-feature)
- Create a Pull Request when work is complete — this is the completion signal
- /tq:done is NOT available in remote sessions`, actionID, actionID)
}

func parseSessionURL(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "https://"); idx >= 0 {
			return line[idx:]
		}
	}
	return strings.TrimSpace(output)
}
