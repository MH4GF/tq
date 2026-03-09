package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/testutil/pty"
)

// TestPTY_VersionCommand verifies that `tq --version` works under a PTY.
func TestPTY_VersionCommand(t *testing.T) {
	pty.Require(t)

	bin := pty.BuildBinary(t, "..")

	p := &pty.ScriptPTY{}
	result := p.Run(t, 10*time.Second, bin, "--version")

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", result.ExitCode, result.Stdout)
	}
	if !strings.Contains(result.Stdout, "tq") {
		t.Errorf("version output should contain 'tq', got %q", result.Stdout)
	}
}

// TestPTY_HelpCommand verifies that `tq --help` works under a PTY.
func TestPTY_HelpCommand(t *testing.T) {
	pty.Require(t)

	bin := pty.BuildBinary(t, "..")

	p := &pty.ScriptPTY{}
	result := p.Run(t, 10*time.Second, bin, "--help")

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", result.ExitCode, result.Stdout)
	}
	if !strings.Contains(result.Stdout, "Task Queue CLI") {
		t.Errorf("help output should contain 'Task Queue CLI', got %q", result.Stdout)
	}
}

// TestPTY_ActionListEmpty verifies that `tq action list` works under a PTY
// with a fresh database.
func TestPTY_ActionListEmpty(t *testing.T) {
	pty.Require(t)

	bin := pty.BuildBinary(t, "..")

	// Use a temp config dir so we get a fresh DB
	configDir := t.TempDir()
	p := &pty.ScriptPTY{
		Env: []string{"HOME=" + configDir},
	}

	// Create the config directory structure
	tqDir := filepath.Join(configDir, ".config", "tq")
	if err := os.MkdirAll(tqDir, 0o755); err != nil {
		t.Fatal(err)
	}

	result := p.Run(t, 10*time.Second, bin, "action", "list")

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", result.ExitCode, result.Stdout)
	}
	if !strings.Contains(result.Stdout, "no actions found") {
		t.Errorf("output should contain 'no actions found', got %q", result.Stdout)
	}
}

// TestPTY_UIQuit verifies that `tq ui` starts under a PTY and can be
// terminated. The TUI starting without panic is the success criteria.
func TestPTY_UIQuit(t *testing.T) {
	pty.Require(t)

	bin := pty.BuildBinary(t, "..")

	configDir := t.TempDir()
	tqDir := filepath.Join(configDir, ".config", "tq")
	if err := os.MkdirAll(tqDir, 0o755); err != nil {
		t.Fatal(err)
	}

	p := &pty.ScriptPTY{
		Env: []string{"HOME=" + configDir},
	}

	// Use sh -c to send 'q' after a brief delay to quit the TUI
	result := p.Run(t, 10*time.Second, "sh", "-c",
		"echo q | "+bin+" ui")

	// The TUI should have started and exited via the 'q' key or pipe close.
	// Any exit code is acceptable as long as it doesn't panic.
	if strings.Contains(result.Stdout, "panic:") {
		t.Errorf("TUI panicked:\n%s", result.Stdout)
	}
}
