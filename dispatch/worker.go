package dispatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, dir string, env []string) ([]byte, error)
}

// ExecRunner is the real implementation using os/exec.
type ExecRunner struct{}

// execWaitDelay bounds how long Cmd.Wait blocks after ctx cancel before
// pipes are forcibly closed. Without this, grandchildren that inherit
// stdout/stderr (e.g. MCP servers spawned by claude) keep the pipes open
// after SIGKILL and cmd.Wait hangs indefinitely.
const execWaitDelay = 30 * time.Second

func (r *ExecRunner) Run(ctx context.Context, name string, args []string, dir string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(filteredEnv(), env...)
	cmd.WaitDelay = execWaitDelay
	return cmd.CombinedOutput()
}

// filteredEnv returns os.Environ() excluding CLAUDECODE.
func filteredEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func buildTQEnv(actionID, taskID int64) []string {
	return []string{
		fmt.Sprintf("TQ_ACTION_ID=%d", actionID),
		fmt.Sprintf("TQ_TASK_ID=%d", taskID),
	}
}

// Worker executes an instruction.
type Worker interface {
	Execute(ctx context.Context, instruction string, cfg ActionConfig, workDir string, actionID, taskID int64) (string, error)
}
