package dispatch

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/MH4GF/tq/prompt"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, dir string, env []string) ([]byte, error)
}

// ExecRunner is the real implementation using os/exec.
type ExecRunner struct{}

func (r *ExecRunner) Run(ctx context.Context, name string, args []string, dir string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(filteredEnv(), env...)
	return cmd.CombinedOutput()
}

// filteredEnv returns os.Environ() without CLAUDECODE to avoid
// Claude Code's nested session detection blocking child processes.
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

// Worker executes a rendered prompt.
type Worker interface {
	Execute(ctx context.Context, prompt string, cfg prompt.Config, workDir string, actionID int64) (string, error)
}
