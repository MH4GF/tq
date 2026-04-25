package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

func TestResumeAction(t *testing.T) {
	tests := []struct {
		name            string
		parentMeta      map[string]any
		parentStatus    string
		opts            db.ResumeOptions
		wantErr         bool
		wantErrContains string
		wantMode        string
		wantInstruction string
	}{
		{
			name: "success failed parent inherits mode",
			parentMeta: map[string]any{
				"instruction":       "do x",
				"mode":              "noninteractive",
				"claude_session_id": "sess-abc",
			},
			parentStatus:    db.ActionStatusFailed,
			opts:            db.ResumeOptions{},
			wantMode:        "noninteractive",
			wantInstruction: "Continue the previous session.",
		},
		{
			name: "success done parent with custom message and mode",
			parentMeta: map[string]any{
				"instruction":       "do x",
				"mode":              "interactive",
				"claude_session_id": "sess-xyz",
			},
			parentStatus:    db.ActionStatusDone,
			opts:            db.ResumeOptions{Message: "next step", Mode: "noninteractive"},
			wantMode:        "noninteractive",
			wantInstruction: "next step",
		},
		{
			name: "success cancelled parent",
			parentMeta: map[string]any{
				"instruction":       "do x",
				"claude_session_id": "sess-c",
			},
			parentStatus:    db.ActionStatusCancelled,
			opts:            db.ResumeOptions{},
			wantMode:        dispatch.ModeInteractive,
			wantInstruction: "Continue the previous session.",
		},
		{
			name: "error: parent pending",
			parentMeta: map[string]any{
				"instruction":       "do x",
				"claude_session_id": "sess-abc",
			},
			parentStatus:    db.ActionStatusPending,
			wantErr:         true,
			wantErrContains: "only failed/cancelled/done",
		},
		{
			name: "error: parent running",
			parentMeta: map[string]any{
				"instruction":       "do x",
				"claude_session_id": "sess-abc",
			},
			parentStatus:    db.ActionStatusRunning,
			wantErr:         true,
			wantErrContains: "only failed/cancelled/done",
		},
		{
			name: "error: missing claude_session_id",
			parentMeta: map[string]any{
				"instruction": "do x",
			},
			parentStatus:    db.ActionStatusFailed,
			wantErr:         true,
			wantErrContains: "no claude_session_id",
		},
		{
			name: "error: empty claude_session_id",
			parentMeta: map[string]any{
				"instruction":       "do x",
				"claude_session_id": "",
			},
			parentStatus:    db.ActionStatusFailed,
			wantErr:         true,
			wantErrContains: "no claude_session_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "Test task", "{}", "")
			metaBytes, _ := json.Marshal(tc.parentMeta)
			parentID, _ := d.InsertAction("parent", taskID, string(metaBytes), db.ActionStatusPending, nil)

			// Move parent to desired status. ClaimPending → terminal transitions are valid.
			if tc.parentStatus != db.ActionStatusPending {
				if _, err := d.ClaimPending(context.Background(), parentID); err != nil {
					t.Fatalf("claim pending: %v", err)
				}
				switch tc.parentStatus {
				case db.ActionStatusFailed:
					_ = d.MarkFailed(parentID, "")
				case db.ActionStatusCancelled:
					_ = d.MarkCancelled(parentID, "")
				case db.ActionStatusDone:
					_ = d.MarkDone(parentID, "")
				case db.ActionStatusRunning:
					// already running after ClaimPending
				}
			}

			newID, err := d.ResumeAction(parentID, tc.opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			a, err := d.GetAction(newID)
			if err != nil {
				t.Fatalf("get new action: %v", err)
			}
			if a.Status != db.ActionStatusPending {
				t.Errorf("status = %q, want pending", a.Status)
			}
			if !strings.HasPrefix(a.Title, "resume #") {
				t.Errorf("title = %q, want prefix 'resume #'", a.Title)
			}

			var meta map[string]any
			if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if meta["instruction"] != tc.wantInstruction {
				t.Errorf("instruction = %v, want %q", meta["instruction"], tc.wantInstruction)
			}
			if meta["mode"] != tc.wantMode {
				t.Errorf("mode = %v, want %q", meta["mode"], tc.wantMode)
			}
			if meta["is_resume"] != true {
				t.Errorf("is_resume = %v, want true", meta["is_resume"])
			}
			if int64(meta["parent_action_id"].(float64)) != parentID {
				t.Errorf("parent_action_id = %v, want %d", meta["parent_action_id"], parentID)
			}
			args, _ := meta["claude_args"].([]any)
			if len(args) != 2 || args[0] != "--resume" {
				t.Errorf("claude_args = %v, want [--resume <id>]", args)
			}
			expectedSessionID, _ := tc.parentMeta["claude_session_id"].(string)
			if meta["claude_session_id"] != expectedSessionID {
				t.Errorf("claude_session_id = %v, want %q", meta["claude_session_id"], expectedSessionID)
			}
		})
	}
}

func TestActionResumeCmd(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(d db.Store) int64 // returns parent action id
		args            func(parentID int64) []string
		workerResult    string
		workerErr       error
		wantErr         bool
		wantOutContains string
		wantNewStatus   string
	}{
		{
			name: "success noninteractive",
			setup: func(d db.Store) int64 {
				taskID, _ := d.InsertTask(1, "t", "{}", "")
				meta := `{"instruction":"orig","mode":"noninteractive","claude_session_id":"sess-1"}`
				id, _ := d.InsertAction("orig", taskID, meta, db.ActionStatusPending, nil)
				_, _ = d.ClaimPending(context.Background(), id)
				_ = d.MarkFailed(id, "boom")
				return id
			},
			args:            func(id int64) []string { return []string{"action", "resume", intToStr(id)} },
			workerResult:    `{"ok":true}`,
			wantOutContains: "resumed from",
			wantNewStatus:   db.ActionStatusDone,
		},
		{
			name: "claude failure marks new action failed",
			setup: func(d db.Store) int64 {
				taskID, _ := d.InsertTask(1, "t", "{}", "")
				meta := `{"instruction":"orig","mode":"noninteractive","claude_session_id":"sess-2"}`
				id, _ := d.InsertAction("orig", taskID, meta, db.ActionStatusPending, nil)
				_, _ = d.ClaimPending(context.Background(), id)
				_ = d.MarkFailed(id, "boom")
				return id
			},
			args:            func(id int64) []string { return []string{"action", "resume", intToStr(id)} },
			workerErr:       context.DeadlineExceeded,
			wantOutContains: "failed",
			wantNewStatus:   db.ActionStatusFailed,
		},
		{
			name: "error: parent missing session_id",
			setup: func(d db.Store) int64 {
				taskID, _ := d.InsertTask(1, "t", "{}", "")
				meta := `{"instruction":"orig","mode":"noninteractive"}`
				id, _ := d.InsertAction("orig", taskID, meta, db.ActionStatusPending, nil)
				_, _ = d.ClaimPending(context.Background(), id)
				_ = d.MarkFailed(id, "boom")
				return id
			},
			args:    func(id int64) []string { return []string{"action", "resume", intToStr(id)} },
			wantErr: true,
		},
		{
			name: "error: parent still pending",
			setup: func(d db.Store) int64 {
				taskID, _ := d.InsertTask(1, "t", "{}", "")
				meta := `{"instruction":"orig","claude_session_id":"sess-3"}`
				id, _ := d.InsertAction("orig", taskID, meta, db.ActionStatusPending, nil)
				return id
			},
			args:    func(id int64) []string { return []string{"action", "resume", intToStr(id)} },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()
			cmd.SetConfigDir(t.TempDir())

			parentID := tt.setup(d)

			worker := &mockWorker{result: tt.workerResult, err: tt.workerErr}
			cmd.SetWorkerFactory(func() dispatch.Worker { return worker })
			t.Cleanup(func() { cmd.SetWorkerFactory(nil) })

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tt.args(parentID))

			err := root.Execute()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantOutContains != "" && !strings.Contains(buf.String(), tt.wantOutContains) {
				t.Errorf("output = %q, want to contain %q", buf.String(), tt.wantOutContains)
			}

			// New action should be parentID+1 (only two actions exist in the test DB).
			newAction, err := d.GetAction(parentID + 1)
			if err != nil {
				t.Fatalf("get new action: %v", err)
			}
			if tt.wantNewStatus != "" && newAction.Status != tt.wantNewStatus {
				t.Errorf("new action status = %q, want %q", newAction.Status, tt.wantNewStatus)
			}
		})
	}
}

func intToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}
