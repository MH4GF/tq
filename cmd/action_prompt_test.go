package cmd_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

func runActionPrompt(t *testing.T, d db.Store, args ...string) (string, error) {
	t.Helper()
	cmd.SetDB(d)
	cmd.ResetForTest()
	cmd.SetConfigDir(t.TempDir())

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(append([]string{"action", "prompt"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestActionPromptCmd_RendersWrappedPrompt(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		isResume      bool
		wantTerminate bool // mode != remote → /tq:done /tq:failed appears
	}{
		{name: "interactive", mode: dispatch.ModeInteractive, wantTerminate: true},
		{name: "noninteractive", mode: dispatch.ModeNonInteractive, wantTerminate: true},
		{name: "remote drops terminate block", mode: dispatch.ModeRemote, wantTerminate: false},
		{name: "default mode (empty -> interactive)", mode: "", wantTerminate: true},
		// is_resume metadata has no effect on rendered prompt — the
		// "Required first step" block is always emitted.
		{name: "resume metadata is ignored", mode: dispatch.ModeInteractive, isResume: true, wantTerminate: true},
	}

	const instruction = "Fix the bug"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, _ := d.InsertTask(1, "task", "{}", "")
			meta := map[string]any{dispatch.MetaKeyInstruction: instruction}
			if tc.mode != "" {
				meta[dispatch.MetaKeyMode] = tc.mode
			}
			if tc.isResume {
				meta[dispatch.MetaKeyIsResume] = true
			}
			actionID, _ := d.InsertAction("a", taskID, mustMarshal(t, meta), db.ActionStatusPending, nil, "")

			out, err := runActionPrompt(t, d, intToStr(actionID))
			if err != nil {
				t.Fatalf("execute: %v", err)
			}

			effectiveMode := tc.mode
			if effectiveMode == "" {
				effectiveMode = dispatch.ModeInteractive
			}
			want := dispatch.RenderPrompt(instruction, actionID, taskID, effectiveMode)
			if !strings.HasSuffix(want, "\n") {
				want += "\n"
			}
			if out != want {
				t.Errorf("output mismatch.\n got: %q\nwant: %q", out, want)
			}

			if !strings.HasPrefix(out, instruction) {
				t.Errorf("output should start with the raw instruction; got prefix %q", out[:min(len(out), 40)])
			}
			if !strings.Contains(out, "Required first step") {
				t.Error("output should always contain 'Required first step'")
			}
			if tc.wantTerminate && !strings.Contains(out, "/tq:done") {
				t.Error("output should contain /tq:done for non-remote modes")
			}
			if !tc.wantTerminate && strings.Contains(out, "/tq:done") {
				t.Error("output should NOT contain /tq:done for remote mode")
			}
			if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
				t.Errorf("output must end with exactly one trailing LF, got: %q", out[max(0, len(out)-3):])
			}
		})
	}
}

func TestActionPromptCmd_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	_, err := runActionPrompt(t, d, "9999")
	if err == nil {
		t.Fatal("expected error for missing action")
	}
	if !strings.Contains(err.Error(), "get action") {
		t.Errorf("error = %q, want to contain 'get action'", err.Error())
	}
}

func TestActionPromptCmd_EmptyInstruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")
	meta := map[string]any{
		dispatch.MetaKeyInstruction: "   ",
		dispatch.MetaKeyMode:        dispatch.ModeInteractive,
	}
	actionID, _ := d.InsertAction("a", taskID, mustMarshal(t, meta), db.ActionStatusPending, nil, "")

	_, err := runActionPrompt(t, d, intToStr(actionID))
	if err == nil {
		t.Fatal("expected error for empty instruction")
	}
	if !strings.Contains(err.Error(), "empty instruction") {
		t.Errorf("error = %q, want to contain 'empty instruction'", err.Error())
	}
}

func TestActionPromptCmd_InvalidID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	_, err := runActionPrompt(t, d, "not-a-number")
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}
