package pty_test

import (
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/testutil/pty"
)

func TestScriptPTY_Echo(t *testing.T) {
	pty.Require(t)

	p := &pty.ScriptPTY{}
	result := p.Run(t, 5*time.Second, "echo", "hello from pty")

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello from pty") {
		t.Errorf("stdout = %q, want to contain 'hello from pty'", result.Stdout)
	}
}

func TestScriptPTY_ExitCode(t *testing.T) {
	pty.Require(t)

	p := &pty.ScriptPTY{}
	result := p.Run(t, 5*time.Second, "sh", "-c", "exit 42")

	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestScriptPTY_Env(t *testing.T) {
	pty.Require(t)

	p := &pty.ScriptPTY{
		Env: []string{"TQ_TEST_VAR=hello123"},
	}
	result := p.Run(t, 5*time.Second, "sh", "-c", "echo $TQ_TEST_VAR")

	if !strings.Contains(result.Stdout, "hello123") {
		t.Errorf("stdout = %q, want to contain 'hello123'", result.Stdout)
	}
}

func TestScriptPTY_IsATTY(t *testing.T) {
	pty.Require(t)

	// Verify that the child process sees a TTY on stdout.
	// `test -t 1` returns 0 if fd 1 (stdout) is a terminal.
	p := &pty.ScriptPTY{}
	result := p.Run(t, 5*time.Second, "sh", "-c", "test -t 1 && echo IS_TTY || echo NOT_TTY")

	if !strings.Contains(result.Stdout, "IS_TTY") {
		t.Errorf("expected IS_TTY in output, got %q", result.Stdout)
	}
}
