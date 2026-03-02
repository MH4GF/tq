package dispatch

import (
	"context"
	"os"
	"os/exec"

	tmpl "github.com/MH4GF/tq/template"
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
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

// Worker executes a rendered prompt.
type Worker interface {
	Execute(ctx context.Context, prompt string, cfg tmpl.Config, workDir string, actionID int64) (string, error)
}
