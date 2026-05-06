package cmd_test

import (
	"bytes"
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

func TestActionPromptCmd_RendersWrappedPrompt(t *testing.T) {
	tests := []struct {
		name            string
		mode            string
		wantTerminate   bool // mode != remote → /tq:done /tq:failed appears
		wantInstruction string
	}{
		{name: "interactive", mode: dispatch.ModeInteractive, wantTerminate: true, wantInstruction: "Fix the bug"},
		{name: "noninteractive", mode: dispatch.ModeNonInteractive, wantTerminate: true, wantInstruction: "Fix the bug"},
		{name: "remote", mode: dispatch.ModeRemote, wantTerminate: false, wantInstruction: "Fix the bug"},
		{name: "default mode (empty -> interactive)", mode: "", wantTerminate: true, wantInstruction: "Fix the bug"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, _ := d.InsertTask(1, "task", "{}", "")
			metaParts := `{"instruction":"Fix the bug"`
			if tc.mode != "" {
				metaParts += `,"mode":"` + tc.mode + `"`
			}
			metaParts += `}`
			actionID, _ := d.InsertAction("a", taskID, metaParts, db.ActionStatusPending, nil)

			out, err := runActionPrompt(t, d, intToStr(actionID))
			if err != nil {
				t.Fatalf("execute: %v", err)
			}

			effectiveMode := tc.mode
			if effectiveMode == "" {
				effectiveMode = dispatch.ModeInteractive
			}
			want := dispatch.RenderPrompt(tc.wantInstruction, actionID, taskID, effectiveMode, false)
			if !strings.HasSuffix(want, "\n") {
				want += "\n"
			}
			if out != want {
				t.Errorf("output mismatch.\n got: %q\nwant: %q", out, want)
			}

			if !strings.HasPrefix(out, tc.wantInstruction) {
				t.Errorf("output should start with the raw instruction; got prefix %q", out[:min(len(out), 40)])
			}
			if !strings.Contains(out, "## tq action context") {
				t.Error("output should contain postamble heading")
			}
			if !strings.Contains(out, "Required first step") {
				t.Error("output should always contain 'Required first step' (isResume hardcoded false)")
			}
			if tc.wantTerminate && !strings.Contains(out, "/tq:done") {
				t.Error("output should contain /tq:done for non-remote modes")
			}
			if !tc.wantTerminate && strings.Contains(out, "/tq:done") {
				t.Error("output should NOT contain /tq:done for remote mode")
			}
			if !strings.HasSuffix(out, "\n") {
				t.Error("output must end with a single trailing LF")
			}
			if strings.HasSuffix(out, "\n\n") {
				t.Error("output must end with exactly one trailing LF, not two")
			}
		})
	}
}

func TestActionPromptCmd_ResumeActionStillIncludesFirstStep(t *testing.T) {
	// Documents the current scope decision: the CLI hardcodes isResume=false,
	// so even resume actions render with the "Required first step" postamble.
	// A follow-up PR will drop the isResume parameter entirely.
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")
	meta := `{"instruction":"Continue the previous session.","mode":"interactive","is_resume":true,"claude_session_id":"sess-x"}`
	actionID, _ := d.InsertAction("resume", taskID, meta, db.ActionStatusPending, nil)

	out, err := runActionPrompt(t, d, intToStr(actionID))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "Required first step") {
		t.Error("resume action should still include 'Required first step' (isResume hardcoded false)")
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
	meta := `{"instruction":"   ","mode":"interactive"}`
	actionID, _ := d.InsertAction("a", taskID, meta, db.ActionStatusPending, nil)

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
