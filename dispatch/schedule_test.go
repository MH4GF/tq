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
	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(1, &createdAt, nil)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions(db.ActionStatusPending, nil, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Title != "My Prompt" {
		t.Errorf("title = %q, want %q", actions[0].Title, "My Prompt")
	}
	if actions[0].Metadata != `{"instruction":"my-prompt","key":"val","schedule_id":"1"}` {
		t.Errorf("metadata = %q, want %q", actions[0].Metadata, `{"instruction":"my-prompt","key":"val","schedule_id":"1"}`)
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
	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:00:00")
	d.SetScheduleTimestampsForTest(1, &createdAt, nil)

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
	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(1, &createdAt, nil)

	// Insert an active action for the same task/prompt
	d.InsertAction("existing", taskID, `{"schedule_id":"1"}`, db.ActionStatusPending, nil)

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
	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(id, &createdAt, nil)
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

func TestCheckSchedules_InstructionBasedAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "/gh-ops:watch", "Watch notifications", "* * * * *", `{}`)
	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(1, &createdAt, nil)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions(db.ActionStatusPending, nil, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
}

func TestCheckSchedules_LastRunAtStoredAsUTC(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")
	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(1, &createdAt, nil)

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
	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-01 00:00:00")
	lastRunAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:00:00")
	d.SetScheduleTimestampsForTest(id, &createdAt, &lastRunAt)

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
