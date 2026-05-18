package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBgWorker_Execute(t *testing.T) {
	const sampleSuccess = "backgrounded · 239007b1\n" +
		"  claude agents             list sessions\n" +
		"  claude attach 239007b1    open in this terminal\n" +
		"  claude logs 239007b1      show recent output\n" +
		"  claude stop 239007b1      stop this session\n"

	tests := []struct {
		name          string
		runnerOutput  []byte
		runnerErr     error
		cfg           ActionConfig
		instruction   string
		wantShort     string
		wantErrSubstr string
		wantArgs      []string
	}{
		{
			name:         "parses short id from success banner",
			runnerOutput: []byte(sampleSuccess),
			cfg:          ActionConfig{},
			instruction:  "Fix the bug",
			wantShort:    "239007b1",
			wantArgs:     []string{"--bg", "Fix the bug"},
		},
		{
			name:         "claude_args passthrough after instruction",
			runnerOutput: []byte(sampleSuccess),
			cfg:          ActionConfig{ClaudeArgs: []string{"--effort", "xhigh", "--worktree"}},
			instruction:  "Fix the bug",
			wantShort:    "239007b1",
			wantArgs:     []string{"--bg", "Fix the bug", "--effort", "xhigh", "--worktree"},
		},
		{
			// Real reproduction from action #5211: claude --bg wrapped the
			// short id in ANSI SGR cyan (\x1b[36m ... \x1b[39m), which broke
			// the parser and falsely marked a healthy session failed.
			name: "parses short id wrapped in ANSI color",
			runnerOutput: []byte("backgrounded · \x1b[36meb7a86bf\x1b[39m\n" +
				"  claude agents             list sessions\n" +
				"  claude attach eb7a86bf    open in this terminal\n" +
				"  claude logs eb7a86bf      show recent output\n" +
				"  claude stop eb7a86bf      stop this session\n"),
			cfg:         ActionConfig{},
			instruction: "Fix the bug",
			wantShort:   "eb7a86bf",
			wantArgs:    []string{"--bg", "Fix the bug"},
		},
		{
			name:          "agent view disabled marker surfaces verbatim",
			runnerOutput:  []byte("'--bg' is not enabled. If this is unexpected, retry in a moment.\n"),
			cfg:           ActionConfig{},
			instruction:   "Anything",
			wantErrSubstr: "'--bg' is not enabled",
		},
		{
			name:          "unrecognised output yields parse error with raw output",
			runnerOutput:  []byte("welcome to claude\nsomething went sideways\n"),
			cfg:           ActionConfig{},
			instruction:   "Anything",
			wantErrSubstr: "could not parse bg short id",
		},
		{
			name:          "runner error is wrapped with stdout payload",
			runnerOutput:  []byte("boom"),
			runnerErr:     context.DeadlineExceeded,
			cfg:           ActionConfig{},
			instruction:   "Anything",
			wantErrSubstr: "boom",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &mockRunner{output: tc.runnerOutput, err: tc.runnerErr, failAt: 0}
			if tc.runnerErr == nil {
				runner.failAt = -1
			}
			w := &BgWorker{Runner: runner}

			short, err := w.Execute(context.Background(), tc.instruction, tc.cfg, "/work", 42, 7)

			if tc.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got short=%q", tc.wantErrSubstr, short)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Errorf("error %q must contain %q", err.Error(), tc.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if short != tc.wantShort {
				t.Errorf("short = %q, want %q", short, tc.wantShort)
			}
			if len(runner.calls) != 1 {
				t.Fatalf("expected exactly 1 runner call, got %d", len(runner.calls))
			}
			got := runner.calls[0]
			if got.name != "claude" {
				t.Errorf("invoked %q, want %q", got.name, "claude")
			}
			if got.dir != "/work" {
				t.Errorf("dir = %q, want %q", got.dir, "/work")
			}
			if got.env != nil {
				t.Errorf("env = %v, want nil (bg launcher needs no TQ_* env: daemon does not propagate)", got.env)
			}
			if tc.wantArgs != nil {
				if !equalStringSlices(got.args, tc.wantArgs) {
					t.Errorf("args = %v, want %v", got.args, tc.wantArgs)
				}
			}
		})
	}
}

func TestBgWorker_RunnerErrorWrapsOriginal(t *testing.T) {
	sentinel := errors.New("network down")
	runner := &mockRunner{output: []byte("partial"), err: sentinel, failAt: 0}
	w := &BgWorker{Runner: runner}

	_, err := w.Execute(context.Background(), "hi", ActionConfig{}, "/work", 1, 1)
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v must wrap sentinel via errors.Is", err)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
