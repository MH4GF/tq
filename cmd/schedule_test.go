package cmd_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestScheduleCreate_InvalidMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schedule", "create", "--instruction", "daily-review", "--task", "1", "--cron", "0 9 * * *", "--meta", "{invalid}"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON meta")
	}
	if !contains(err.Error(), "invalid JSON for --meta (must be a JSON object)") {
		t.Errorf("error = %q, want to contain 'invalid JSON for --meta (must be a JSON object)'", err.Error())
	}
}

func TestScheduleCreate_WithInstruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schedule", "create", "--instruction", "/gh-ops:watch", "--task", "1", "--cron", "*/10 * * * *", "--title", "Watch notifications"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(out.String(), "schedule #") {
		t.Errorf("output = %q, want to contain 'schedule #'", out.String())
	}

	schedules, _ := d.ListSchedules(0)
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].Instruction != "/gh-ops:watch" {
		t.Errorf("instruction = %q, want %q", schedules[0].Instruction, "/gh-ops:watch")
	}
}

func TestScheduleCreate_NoInstruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schedule", "create", "--task", "1", "--cron", "0 9 * * *"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --instruction is not provided")
	}
	if !contains(err.Error(), "--instruction is required") {
		t.Errorf("error = %q, want to contain '--instruction is required'", err.Error())
	}
}

func TestScheduleUpdate_Success(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertSchedule(taskID, "/gh-ops:watch", "Watch", "* * * * *", "{}")

	root := cmd.GetRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schedule", "update", "1", "--title", "Updated Watch"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(out.String(), "schedule #1 updated") {
		t.Errorf("output = %q, want to contain 'schedule #1 updated'", out.String())
	}
}

func TestScheduleCreate_ClaudeArgsValid(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	d.InsertTask(1, "test task", "{}", "")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schedule", "create", "--instruction", "/gh-ops:watch", "--task", "1", "--cron", "* * * * *", "--meta", `{"claude_args":["--max-turns","5"]}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	schedules, _ := d.ListSchedules(0)
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(schedules[0].Metadata), &meta); err != nil {
		t.Fatalf("parse schedule metadata: %v", err)
	}
	rawArgs, ok := meta["claude_args"].([]any)
	if !ok {
		t.Fatalf("claude_args not found or wrong type in schedule metadata: %v", meta)
	}
	if len(rawArgs) != 2 || rawArgs[0] != "--max-turns" || rawArgs[1] != "5" {
		t.Errorf("claude_args = %v, want [--max-turns 5]", rawArgs)
	}
}

func TestScheduleCreate_ClaudeArgsInvalidType(t *testing.T) {
	tests := []struct {
		name    string
		meta    string
		wantErr string
	}{
		{
			name:    "string instead of array",
			meta:    `{"claude_args":"--max-turns 5"}`,
			wantErr: "claude_args must be a JSON array of strings",
		},
		{
			name:    "array with non-string element",
			meta:    `{"claude_args":["--max-turns",5]}`,
			wantErr: "claude_args[1] must be a string",
		},
		{
			name:    "blocked flag",
			meta:    `{"claude_args":["--output-format","text"]}`,
			wantErr: "claude_args cannot include",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()

			d.InsertTask(1, "test task", "{}", "")

			root := cmd.GetRootCmd()
			root.SetOut(new(bytes.Buffer))
			root.SetErr(new(bytes.Buffer))
			root.SetArgs([]string{"schedule", "create", "--instruction", "/gh-ops:watch", "--task", "1", "--cron", "* * * * *", "--meta", tc.meta})

			err := root.Execute()
			if err == nil {
				t.Fatal("expected error for invalid claude_args")
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestScheduleUpdate_ClaudeArgsValid(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertSchedule(taskID, "/gh-ops:watch", "Watch", "* * * * *", "{}")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schedule", "update", "1", "--meta", `{"claude_args":["--max-turns","3"]}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, err := d.GetSchedule(1)
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(s.Metadata), &meta); err != nil {
		t.Fatalf("parse schedule metadata: %v", err)
	}
	rawArgs, ok := meta["claude_args"].([]any)
	if !ok {
		t.Fatalf("claude_args not found or wrong type: %v", meta)
	}
	if len(rawArgs) != 2 || rawArgs[0] != "--max-turns" || rawArgs[1] != "3" {
		t.Errorf("claude_args = %v, want [--max-turns 3]", rawArgs)
	}
}

func TestScheduleUpdate_InvalidMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertSchedule(taskID, "/daily-review", "daily", "0 9 * * *", "{}")

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"schedule", "update", "1", "--meta", "{invalid}"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON meta")
	}
	if !contains(err.Error(), "invalid JSON for --meta (must be a JSON object)") {
		t.Errorf("error = %q, want to contain 'invalid JSON for --meta (must be a JSON object)'", err.Error())
	}
}
