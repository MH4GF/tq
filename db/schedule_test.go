package db_test

import (
	"encoding/json"
	"testing"

	"github.com/MH4GF/tq/db"
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

func TestUpdateScheduleRun(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "test", "Test", "* * * * *", "{}")

	if err := d.UpdateScheduleRun(id, "2026-03-12 10:00:00", "boom"); err != nil {
		t.Fatal(err)
	}
	s, _ := d.GetSchedule(id)
	if !s.LastRunAt.Valid || s.LastRunAt.String != "2026-03-12 10:00:00" {
		t.Errorf("last_run_at = %v, want 2026-03-12 10:00:00", s.LastRunAt)
	}
	if !s.LastError.Valid || s.LastError.String != "boom" {
		t.Errorf("last_error = %v, want %q", s.LastError, "boom")
	}

	if err := d.UpdateScheduleRun(id, "2026-03-12 10:05:00", ""); err != nil {
		t.Fatal(err)
	}
	s, _ = d.GetSchedule(id)
	if s.LastError.Valid {
		t.Errorf("last_error = %q, want NULL after success run", s.LastError.String)
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

func TestUpdateSchedule_EmitsEvent(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertSchedule(taskID, "instr", "Original", "* * * * *", "{}")

	newTitle := "Updated"
	newCron := "0 */3 * * *"
	if err := d.UpdateSchedule(id, &newTitle, &newCron, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	events, err := d.ListEvents("schedule", id)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (created + updated), got %d", len(events))
	}
	updated := events[1]
	if updated.EventType != "schedule.updated" {
		t.Fatalf("event_type = %q, want schedule.updated", updated.EventType)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(updated.Payload), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if payload["title"] != "Updated" {
		t.Errorf("payload[title] = %v, want %q", payload["title"], "Updated")
	}
	if payload["cron_expr"] != "0 */3 * * *" {
		t.Errorf("payload[cron_expr] = %v, want %q", payload["cron_expr"], "0 */3 * * *")
	}
	for _, k := range []string{"metadata", "instruction", "task_id"} {
		if _, ok := payload[k]; ok {
			t.Errorf("payload should not include unchanged field %q: %v", k, payload)
		}
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

func TestEnabledScheduleIDs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "test", "{}", "")
	schedID, _ := d.InsertSchedule(taskID, "p", "t", "* * * * *", "{}")

	tests := []struct {
		name    string
		setup   func()
		wantLen int
		wantID  int64
	}{
		{
			name:    "enabled schedule returns its ID",
			setup:   func() {},
			wantLen: 1,
			wantID:  schedID,
		},
		{
			name:    "disabled schedule returns empty",
			setup:   func() { d.UpdateScheduleEnabled(schedID, false) },
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			ids, err := d.EnabledScheduleIDs(taskID)
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) != tc.wantLen {
				t.Errorf("got %d IDs, want %d: %v", len(ids), tc.wantLen, ids)
			}
			if tc.wantLen == 1 && ids[0] != tc.wantID {
				t.Errorf("got ID %d, want %d", ids[0], tc.wantID)
			}
		})
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

func TestBulkUpdateScheduleRuns(t *testing.T) {
	t.Run("empty input is no-op", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		if err := d.BulkUpdateScheduleRuns(nil); err != nil {
			t.Errorf("nil updates returned err: %v", err)
		}
		if err := d.BulkUpdateScheduleRuns([]db.ScheduleRunUpdate{}); err != nil {
			t.Errorf("empty updates returned err: %v", err)
		}
	})

	t.Run("normal: 3 schedules updated atomically", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")

		var ids []int64
		for range 3 {
			id, _ := d.InsertSchedule(taskID, "p", "T", "* * * * *", "{}")
			ids = append(ids, id)
		}

		updates := []db.ScheduleRunUpdate{
			{ID: ids[0], LastRunAt: "2026-01-01 00:00:00", LastError: ""},
			{ID: ids[1], LastRunAt: "2026-01-01 00:00:00", LastError: "boom"},
			{ID: ids[2], LastRunAt: "2026-01-01 00:00:00", LastError: ""},
		}
		if err := d.BulkUpdateScheduleRuns(updates); err != nil {
			t.Fatal(err)
		}

		s0, _ := d.GetSchedule(ids[0])
		if !s0.LastRunAt.Valid || s0.LastError.Valid {
			t.Errorf("schedule 0: got run=%v err=%v, want run set + err clear", s0.LastRunAt, s0.LastError)
		}
		s1, _ := d.GetSchedule(ids[1])
		if !s1.LastError.Valid || s1.LastError.String != "boom" {
			t.Errorf("schedule 1: last_error = %v, want \"boom\"", s1.LastError)
		}
		s2, _ := d.GetSchedule(ids[2])
		if !s2.LastRunAt.Valid {
			t.Errorf("schedule 2: last_run_at not set")
		}
	})

	t.Run("invalid ID does not error (UPDATE matches 0 rows)", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		updates := []db.ScheduleRunUpdate{
			{ID: 9999, LastRunAt: "2026-01-01 00:00:00", LastError: ""},
		}
		if err := d.BulkUpdateScheduleRuns(updates); err != nil {
			t.Errorf("unexpected err on missing id: %v", err)
		}
	})
}

func TestHasActiveActionsForSchedules(t *testing.T) {
	t.Run("empty input returns empty map", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		got, err := d.HasActiveActionsForSchedules(nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("active action present", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		_, _ = d.InsertAction("a1", taskID, `{"schedule_id":"42"}`, "pending", nil, "")

		got, err := d.HasActiveActionsForSchedules([]int64{42, 99})
		if err != nil {
			t.Fatal(err)
		}
		if !got[42] {
			t.Errorf("schedule 42 should be active")
		}
		if got[99] {
			t.Errorf("schedule 99 should be absent (no active action)")
		}
	})

	t.Run("done/failed/cancelled actions are not active", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		// Insert a done action — should not count as active.
		aid, _ := d.InsertAction("a1", taskID, `{"schedule_id":"7"}`, "pending", nil, "")
		_ = d.MarkDone(aid, "ok")

		got, err := d.HasActiveActionsForSchedules([]int64{7})
		if err != nil {
			t.Fatal(err)
		}
		if got[7] {
			t.Errorf("done action should not count as active")
		}
	})

	t.Run("running and dispatched both count as active", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		_, _ = d.InsertAction("a1", taskID, `{"schedule_id":"1"}`, "running", nil, "")
		_, _ = d.InsertAction("a2", taskID, `{"schedule_id":"2"}`, "dispatched", nil, "")

		got, err := d.HasActiveActionsForSchedules([]int64{1, 2, 3})
		if err != nil {
			t.Fatal(err)
		}
		if !got[1] || !got[2] {
			t.Errorf("expected schedules 1 and 2 active, got %v", got)
		}
		if got[3] {
			t.Errorf("schedule 3 should be absent")
		}
	})
}
