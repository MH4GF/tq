package pty

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// ScriptPTY runs a command inside a PTY allocated by the `script` command.
// This is useful for integration-testing programs that require a terminal
// (e.g., Bubble Tea TUI, isatty checks).
type ScriptPTY struct {
	// Env holds additional environment variables for the child process.
	Env []string
	// Dir is the working directory. Empty means current directory.
	Dir string
}

// Result holds the output and exit information from a script PTY run.
type Result struct {
	Stdout   string
	ExitCode int
}

// Run executes the given command inside a PTY using `script`.
// On Linux: script -qec "command" /dev/null
// On macOS: script -q /dev/null command args...
func (s *ScriptPTY) Run(t *testing.T, timeout time.Duration, name string, args ...string) Result {
	t.Helper()

	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script command not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	var scriptArgs []string
	switch runtime.GOOS {
	case "linux":
		cmdStr := name
		for _, a := range args {
			cmdStr += " " + shellQuote(a)
		}
		scriptArgs = []string{"-qec", cmdStr, "/dev/null"}
	case "darwin":
		scriptArgs = append([]string{"-q", "/dev/null", name}, args...)
	default:
		t.Skipf("script PTY not supported on %s", runtime.GOOS)
	}

	cmd := exec.CommandContext(ctx, "script", scriptArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	if s.Dir != "" {
		cmd.Dir = s.Dir
	}
	cmd.Env = append(cmd.Environ(), s.Env...)

	err := cmd.Run()

	result := Result{
		Stdout: stdout.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			result.ExitCode = -1
		} else {
			t.Fatalf("script command failed: %v", err)
		}
	}

	return result
}

func shellQuote(s string) string {
	return "'" + shellEscape(s) + "'"
}

func shellEscape(s string) string {
	var buf bytes.Buffer
	for _, c := range s {
		if c == '\'' {
			buf.WriteString("'\\''")
		} else {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

// Require skips the test if script PTY support is not available.
func Require(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skipf("script PTY not supported on %s", runtime.GOOS)
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script command not available")
	}
}

// BuildBinary builds the Go binary at the given package path and returns
// the path to the compiled binary. The binary is placed in t.TempDir()
// and cleaned up automatically.
func BuildBinary(t *testing.T, pkg string) string {
	t.Helper()

	binPath := fmt.Sprintf("%s/tq-test", t.TempDir())
	cmd := exec.Command("go", "build", "-o", binPath, pkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
}
