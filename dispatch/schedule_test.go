package dispatch_test

import (
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

func TestCheckSchedules_ActionCreated(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", `{"key":"val"}`)

	// Set created_at to 2 minutes ago so cron is due
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions(db.ActionStatusPending, nil, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].PromptID != "my-prompt" {
		t.Errorf("prompt_id = %q, want %q", actions[0].PromptID, "my-prompt")
	}
	if actions[0].Title != "My Prompt" {
		t.Errorf("title = %q, want %q", actions[0].Title, "My Prompt")
	}
	if actions[0].Metadata != `{"key":"val"}` {
		t.Errorf("metadata = %q, want %q", actions[0].Metadata, `{"key":"val"}`)
	}

	// Verify last_run_at was updated
	s, _ := d.GetSchedule(1)
	if !s.LastRunAt.Valid {
		t.Error("expected last_run_at to be set")
	}
}

func TestCheckSchedules_NotDueYet(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "my-prompt", "My Prompt", "0 */3 * * *", "{}")

	// created_at is now, next run is 3 hours later
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:00:00' WHERE id = 1")

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:30:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestCheckSchedules_DuplicateSkipped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")

	// Insert an active action for the same task/prompt
	d.InsertAction("existing", "my-prompt", taskID, "{}", db.ActionStatusPending)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions(db.ActionStatusPending, nil, 0)
	if len(actions) != 1 {
		t.Errorf("expected 1 action (existing only), got %d", len(actions))
	}
}

func TestCheckSchedules_DisabledSkipped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = ?", id)
	d.UpdateScheduleEnabled(id, false)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestCheckSchedules_TaskDoneAutoDisable(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.UpdateTask(taskID, db.TaskStatusDone, "")
	id, _ := d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = ?", id)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	// No action created
	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}

	// Schedule auto-disabled
	s, _ := d.GetSchedule(id)
	if s.Enabled {
		t.Error("expected schedule to be auto-disabled")
	}
}

func TestCheckSchedules_TaskArchivedAutoDisable(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.UpdateTask(taskID, db.TaskStatusArchived, "")
	id, _ := d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = ?", id)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	s, _ := d.GetSchedule(id)
	if s.Enabled {
		t.Error("expected schedule to be auto-disabled for archived task")
	}
}

func TestCheckSchedules_InstructionBasedAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "", "Watch notifications", "* * * * *", `{"instruction":"/gh-notifications:watch"}`)
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions(db.ActionStatusPending, nil, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].PromptID != "" {
		t.Errorf("prompt_id = %q, want empty", actions[0].PromptID)
	}
	if actions[0].Metadata != `{"instruction":"/gh-notifications:watch"}` {
		t.Errorf("metadata = %q, want instruction in metadata", actions[0].Metadata)
	}
}

func TestCheckSchedules_InstructionDuplicateSkipped(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "", "Watch notifications", "* * * * *", `{"instruction":"/gh-notifications:watch"}`)
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")

	d.InsertAction("existing", "", taskID, `{"instruction":"/gh-notifications:watch"}`, db.ActionStatusPending)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions(db.ActionStatusPending, nil, 0)
	if len(actions) != 1 {
		t.Errorf("expected 1 action (existing only), got %d", len(actions))
	}
}

func TestCheckSchedules_LastRunAtStoredAsUTC(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")
	d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")

	// Simulate calling CheckSchedules with a JST time (UTC+9)
	jst := time.FixedZone("JST", 9*3600)
	now := time.Date(2026, 3, 12, 19, 0, 0, 0, jst) // 19:00 JST = 10:00 UTC

	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	s, _ := d.GetSchedule(1)
	if !s.LastRunAt.Valid {
		t.Fatal("expected last_run_at to be set")
	}
	// last_run_at should be stored as UTC (10:00), not JST (19:00)
	if s.LastRunAt.String != "2026-03-12 10:00:00" {
		t.Errorf("last_run_at = %q, want UTC %q", s.LastRunAt.String, "2026-03-12 10:00:00")
	}
}

func TestCheckSchedules_UsesLastRunAt(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "my-prompt", "My Prompt", "0 */3 * * *", "{}")

	// Set created_at far in the past, last_run_at to recent
	d.Exec("UPDATE schedules SET created_at = '2026-03-01 00:00:00', last_run_at = '2026-03-12 09:00:00' WHERE id = ?", id)

	// now is 09:30, next run from 09:00 is 12:00 → should NOT trigger
	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:30:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions (not due yet from last_run_at), got %d", len(actions))
	}
}
