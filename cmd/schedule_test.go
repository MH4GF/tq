package cmd_test

import (
	"bytes"
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
	root.SetArgs([]string{"schedule", "create", "--prompt", "daily-review", "--task", "1", "--cron", "0 9 * * *", "--meta", "{invalid}"})

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
	root.SetArgs([]string{"schedule", "create", "--instruction", "/gh-notifications:watch", "--task", "1", "--cron", "*/10 * * * *", "--title", "Watch notifications"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(out.String(), "schedule #") {
		t.Errorf("output = %q, want to contain 'schedule #'", out.String())
	}

	schedules, _ := d.ListSchedules()
	if len(schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].PromptID != "" {
		t.Errorf("prompt_id = %q, want empty", schedules[0].PromptID)
	}
	if !contains(schedules[0].Metadata, `"instruction":"/gh-notifications:watch"`) {
		t.Errorf("metadata = %q, want instruction in metadata", schedules[0].Metadata)
	}
}

func TestScheduleCreate_NoPromptNoInstruction(t *testing.T) {
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
		t.Fatal("expected error when neither --prompt nor --instruction is provided")
	}
	if !contains(err.Error(), "at least one of --prompt or --instruction is required") {
		t.Errorf("error = %q, want to contain 'at least one of --prompt or --instruction is required'", err.Error())
	}
}

func TestScheduleUpdate_InvalidMeta(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertSchedule(taskID, "daily-review", "daily", "0 9 * * *", "{}")

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
