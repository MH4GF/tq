package db_test

import (
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestInsertSchedule(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	id, err := d.InsertSchedule(taskID, "inbox-zero", "Inbox Zero", "0 */3 * * *", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if id < 1 {
		t.Errorf("expected positive id, got %d", id)
	}

	s, err := d.GetSchedule(id)
	if err != nil {
		t.Fatal(err)
	}
	if s.Instruction != "inbox-zero" {
		t.Errorf("instruction = %q, want %q", s.Instruction, "inbox-zero")
	}
	if s.Title != "Inbox Zero" {
		t.Errorf("title = %q, want %q", s.Title, "Inbox Zero")
	}
	if s.CronExpr != "0 */3 * * *" {
		t.Errorf("cron_expr = %q, want %q", s.CronExpr, "0 */3 * * *")
	}
	if !s.Enabled {
		t.Error("expected enabled = true")
	}
	if s.LastRunAt.Valid {
		t.Error("expected last_run_at to be NULL")
	}
}

func TestListSchedules(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "a", "A", "* * * * *", "{}")
	d.InsertSchedule(taskID, "b", "B", "0 * * * *", "{}")

	schedules, err := d.ListSchedules(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(schedules))
	}

	limited, err := d.ListSchedules(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 {
		t.Errorf("expected 1 schedule with limit=1, got %d", len(limited))
	}
}

func TestUpdateScheduleEnabled(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "test", "Test", "* * * * *", "{}")

	if err := d.UpdateScheduleEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	s, _ := d.GetSchedule(id)
	if s.Enabled {
		t.Error("expected enabled = false")
	}

	if err := d.UpdateScheduleEnabled(id, true); err != nil {
		t.Fatal(err)
	}
	s, _ = d.GetSchedule(id)
	if !s.Enabled {
		t.Error("expected enabled = true")
	}
}

func TestUpdateScheduleLastRunAt(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "test", "Test", "* * * * *", "{}")

	if err := d.UpdateScheduleLastRunAt(id, "2026-03-12 10:00:00"); err != nil {
		t.Fatal(err)
	}
	s, _ := d.GetSchedule(id)
	if !s.LastRunAt.Valid || s.LastRunAt.String != "2026-03-12 10:00:00" {
		t.Errorf("last_run_at = %v, want 2026-03-12 10:00:00", s.LastRunAt)
	}
}

func TestDeleteSchedule(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "test", "Test", "* * * * *", "{}")

	if err := d.DeleteSchedule(id); err != nil {
		t.Fatal(err)
	}

	schedules, _ := d.ListSchedules(0)
	if len(schedules) != 0 {
		t.Errorf("expected 0 schedules after delete, got %d", len(schedules))
	}
}

func TestUpdateSchedule(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	taskID2, _ := d.InsertTask(1, "test2", "{}", "")
	id, _ := d.InsertSchedule(taskID, "test", "Original", "* * * * *", "{}")

	newTitle := "Updated"
	if err := d.UpdateSchedule(id, &newTitle, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	s, _ := d.GetSchedule(id)
	if s.Title != "Updated" {
		t.Errorf("title = %q, want %q", s.Title, "Updated")
	}
	if s.CronExpr != "* * * * *" {
		t.Errorf("cron_expr changed unexpectedly: %q", s.CronExpr)
	}

	newCron := "0 */3 * * *"
	newMeta := `{"key":"val"}`
	if err := d.UpdateSchedule(id, nil, &newCron, &newMeta, nil, &taskID2); err != nil {
		t.Fatal(err)
	}
	s, _ = d.GetSchedule(id)
	if s.CronExpr != "0 */3 * * *" {
		t.Errorf("cron_expr = %q, want %q", s.CronExpr, "0 */3 * * *")
	}
	if s.Metadata != `{"key":"val"}` {
		t.Errorf("metadata = %q, want %q", s.Metadata, `{"key":"val"}`)
	}
	if s.TaskID != taskID2 {
		t.Errorf("task_id = %d, want %d", s.TaskID, taskID2)
	}
	if s.Title != "Updated" {
		t.Errorf("title changed unexpectedly: %q", s.Title)
	}

	if err := d.UpdateSchedule(id, nil, nil, nil, nil, nil); err == nil {
		t.Error("expected error when no fields specified")
	}
}

func TestUpdateSchedule_Instruction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "old-instruction", "Test", "* * * * *", "{}")

	newInstruction := "new-instruction"
	if err := d.UpdateSchedule(id, nil, nil, nil, &newInstruction, nil); err != nil {
		t.Fatal(err)
	}
	s, _ := d.GetSchedule(id)
	if s.Instruction != "new-instruction" {
		t.Errorf("instruction = %q, want %q", s.Instruction, "new-instruction")
	}

	emptyInstruction := ""
	newMeta := `{"key":"val"}`
	if err := d.UpdateSchedule(id, nil, nil, &newMeta, &emptyInstruction, nil); err != nil {
		t.Fatal(err)
	}
	s, _ = d.GetSchedule(id)
	if s.Instruction != "" {
		t.Errorf("instruction = %q, want empty", s.Instruction)
	}
	if s.Metadata != `{"key":"val"}` {
		t.Errorf("metadata = %q, want %q", s.Metadata, `{"key":"val"}`)
	}
}

func TestGetSchedule_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	_, err := d.GetSchedule(999)
	if err == nil {
		t.Error("expected error for non-existent schedule")
	}
}
