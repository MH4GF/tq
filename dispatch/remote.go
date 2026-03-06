package dispatch

import (
	"context"
	"fmt"
	"strings"

	"github.com/MH4GF/tq/prompt"
)

// RemoteWorker runs `claude --remote` for cloud-based execution.
// The session is fire-and-forget; completion is detected via PR branch naming.
type RemoteWorker struct {
	Runner CommandRunner
}

func (w *RemoteWorker) Execute(ctx context.Context, prompt string, cfg prompt.Config, workDir string, actionID int64) (string, error) {
	remotePrompt := prompt + remoteRules(actionID)

	args := []string{"--remote", remotePrompt}
	env := []string{fmt.Sprintf("TQ_ACTION_ID=%d", actionID)}

	output, err := w.Runner.Run(ctx, "claude", args, workDir, env)
	if err != nil {
		return "", fmt.Errorf("claude --remote: %w\noutput: %s", err, string(output))
	}

	sessionURL := parseSessionURL(string(output))
	return fmt.Sprintf("remote:session=%s", sessionURL), nil
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
		if strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return strings.TrimSpace(output)
}
