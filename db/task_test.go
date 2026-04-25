package db_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestInsertTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "test task", `{"url":"https://example.com"}`, "")
	if err != nil {
		t.Fatal(err)
	}
	if id < 1 {
		t.Errorf("expected positive id, got %d", id)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "test task" {
		t.Errorf("expected title 'test task', got %s", task.Title)
	}
	if task.Status != db.TaskStatusOpen {
		t.Errorf("expected status 'open', got %s", task.Status)
	}
}

func TestUpdateTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "task to update", "{}", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.UpdateTask(id, db.TaskStatusDone, ""); err != nil {
		t.Fatal(err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != db.TaskStatusDone {
		t.Errorf("expected status 'done', got %s", task.Status)
	}
	if !task.UpdatedAt.Valid {
		t.Error("expected updated_at to be set")
	}
}

func TestUpdateTaskProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "task to move", "{}", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.UpdateTaskProject(id, 2); err != nil {
		t.Fatal(err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.ProjectID != 2 {
		t.Errorf("expected project_id 2, got %d", task.ProjectID)
	}
	if !task.UpdatedAt.Valid {
		t.Error("expected updated_at to be set")
	}
}

func TestGetTask_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)

	_, err := d.GetTask(999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestListTasks(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertTask(1, "task A", "{}", "")
	id2, _ := d.InsertTask(1, "task B", "{}", "")
	d.InsertTask(2, "task C", "{}", "")
	d.UpdateTask(id2, db.TaskStatusDone, "")

	tests := []struct {
		name      string
		projectID int64
		status    string
		limit     int
		wantLen   int
	}{
		{"no filter", 0, "", 0, 3},
		{"filter by project", 1, "", 0, 2},
		{"filter by status", 0, db.TaskStatusOpen, 0, 2},
		{"filter by project and status", 1, db.TaskStatusDone, 0, 1},
		{"limit", 0, "", 2, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tasks, err := d.ListTasks(tc.projectID, tc.status, tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != tc.wantLen {
				t.Errorf("expected %d tasks, got %d", tc.wantLen, len(tasks))
			}
			if tc.limit > 0 && len(tasks) > 1 && tasks[0].ID < tasks[1].ID {
				t.Errorf("expected DESC order: first ID %d should be > second ID %d", tasks[0].ID, tasks[1].ID)
			}
		})
	}
}

func TestListTasksByProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertTask(1, "task A", "{}", "")
	d.InsertTask(1, "task B", "{}", "")
	d.InsertTask(2, "task C", "{}", "")

	tasks, err := d.ListTasksByProject(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for project 1, got %d", len(tasks))
	}
}

func TestInsertTaskWithWorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "worktree task", "{}", "/tmp/worktree")
	if err != nil {
		t.Fatal(err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.WorkDir != "/tmp/worktree" {
		t.Errorf("expected work_dir '/tmp/worktree', got %s", task.WorkDir)
	}
}

func TestInsertTaskDefaultWorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "default workdir task", "{}", "")
	if err != nil {
		t.Fatal(err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.WorkDir != "" {
		t.Errorf("expected empty work_dir, got %s", task.WorkDir)
	}
}

func TestUpdateTaskWorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "task", "{}", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.UpdateTaskWorkDir(id, "/new/path"); err != nil {
		t.Fatal(err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.WorkDir != "/new/path" {
		t.Errorf("expected work_dir '/new/path', got %s", task.WorkDir)
	}
	if !task.UpdatedAt.Valid {
		t.Error("expected updated_at to be set")
	}
}

func TestMergeTaskMetadata(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tests := []struct {
		name        string
		initialMeta string
		merge       map[string]any
		wantMeta    string
	}{
		{"merge new key", `{"existing":"value"}`, map[string]any{"url": "https://example.com"}, `{"existing":"value","url":"https://example.com"}`},
		{"overwrite existing key", `{"existing":"value","url":"https://example.com"}`, map[string]any{"existing": "new"}, `{"existing":"new","url":"https://example.com"}`},
		{"merge into empty metadata", "{}", map[string]any{"key": "val"}, `{"key":"val"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := d.InsertTask(1, "task", tc.initialMeta, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := d.MergeTaskMetadata(id, tc.merge); err != nil {
				t.Fatal(err)
			}
			task, err := d.GetTask(id)
			if err != nil {
				t.Fatal(err)
			}
			if task.Metadata != tc.wantMeta {
				t.Errorf("expected metadata %s, got %s", tc.wantMeta, task.Metadata)
			}
		})
	}
}

func TestEnsureTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id1, err := d.EnsureTask(1, "my task")
	if err != nil {
		t.Fatal(err)
	}
	if id1 < 1 {
		t.Errorf("expected positive id, got %d", id1)
	}

	id2, err := d.EnsureTask(1, "my task")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("EnsureTask returned different IDs: %d vs %d", id1, id2)
	}

	// Closing the task should cause a new one to be created
	if err := d.UpdateTask(id1, db.TaskStatusDone, ""); err != nil {
		t.Fatal(err)
	}
	id3, err := d.EnsureTask(1, "my task")
	if err != nil {
		t.Fatal(err)
	}
	if id3 == id1 {
		t.Errorf("expected new task after closing, got same ID %d", id3)
	}
}

func TestUpdateTask_BlockedByActiveSchedule(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "test", "{}", "")
	schedID, _ := d.InsertSchedule(taskID, "p", "t", "* * * * *", "{}")

	tests := []struct {
		name    string
		status  string
		setup   func()
		wantErr bool
	}{
		{"done blocked by active schedule", db.TaskStatusDone, nil, true},
		{"archived blocked by active schedule", db.TaskStatusArchived, nil, true},
		{"invalid status rejected", "review", nil, true},
		{"done allowed after disable", db.TaskStatusDone, func() { d.UpdateScheduleEnabled(schedID, false) }, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			err := d.UpdateTask(taskID, tc.status, "")
			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateTask(%q) error = %v, wantErr %v", tc.status, err, tc.wantErr)
			}
		})
	}
}

func TestUpdateTask_BlockedByActiveActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	t.Run("pending action blocks done", func(t *testing.T) {
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		d.InsertAction("pending", taskID, "{}", db.ActionStatusPending, nil)

		err := d.UpdateTask(taskID, db.TaskStatusDone, "")
		if err == nil {
			t.Fatal("expected error when completing task with pending action")
		}
		if !strings.Contains(err.Error(), "pending/running action(s)") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("pending action blocks archived", func(t *testing.T) {
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		d.InsertAction("pending", taskID, "{}", db.ActionStatusPending, nil)

		err := d.UpdateTask(taskID, db.TaskStatusArchived, "")
		if err == nil {
			t.Fatal("expected error when archiving task with pending action")
		}
	})

	t.Run("running action blocks done", func(t *testing.T) {
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		d.InsertAction("running", taskID, "{}", db.ActionStatusRunning, nil)

		err := d.UpdateTask(taskID, db.TaskStatusDone, "")
		if err == nil {
			t.Fatal("expected error when completing task with running action")
		}
	})

	t.Run("failed action does not block done", func(t *testing.T) {
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		actionID, _ := d.InsertAction("to-fail", taskID, "{}", db.ActionStatusRunning, nil)
		d.MarkFailed(actionID, "failed")

		err := d.UpdateTask(taskID, db.TaskStatusDone, "")
		if err != nil {
			t.Errorf("done should succeed with only failed actions, got: %v", err)
		}
	})

	t.Run("done/cancelled actions do not block", func(t *testing.T) {
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		doneID, _ := d.InsertAction("to-done", taskID, "{}", db.ActionStatusRunning, nil)
		d.MarkDone(doneID, "ok")
		cancelID, _ := d.InsertAction("to-cancel", taskID, "{}", db.ActionStatusPending, nil)
		d.MarkCancelled(cancelID, "")

		err := d.UpdateTask(taskID, db.TaskStatusDone, "")
		if err != nil {
			t.Errorf("done should succeed with only done/cancelled actions, got: %v", err)
		}
	})

	t.Run("after cancelling all active actions", func(t *testing.T) {
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		actionID, _ := d.InsertAction("pending", taskID, "{}", db.ActionStatusPending, nil)

		if err := d.MarkCancelled(actionID, ""); err != nil {
			t.Fatalf("cancel action: %v", err)
		}
		err := d.UpdateTask(taskID, db.TaskStatusArchived, "")
		if err != nil {
			t.Errorf("archived should succeed after cancelling actions, got: %v", err)
		}
	})
}

func TestRecordTaskNote(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task with notes", "{}", "")

	tests := []struct {
		name    string
		kind    string
		reason  string
		meta    map[string]any
		wantErr bool
	}{
		{"basic note", "triage_keep", "PR review pending", nil, false},
		{"note with metadata", "triage_keep", "snooze", map[string]any{"snooze_until": "2026-05-02"}, false},
		{"missing kind", "", "reason", nil, true},
		{"missing reason", "triage_keep", "", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := d.RecordTaskNote(taskID, tc.kind, tc.reason, tc.meta)
			if (err != nil) != tc.wantErr {
				t.Errorf("RecordTaskNote err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}

	t.Run("nonexistent task errors", func(t *testing.T) {
		if err := d.RecordTaskNote(9999, "triage_keep", "x", nil); err == nil {
			t.Error("expected error for missing task")
		}
	})
}

func TestTaskNotes(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	if err := d.RecordTaskNote(taskID, "triage_keep", "first reason", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordTaskNote(taskID, "observation", "second reason", map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordTaskNote(taskID, "triage_keep", "third reason", map[string]any{"snooze_until": "2026-05-02"}); err != nil {
		t.Fatal(err)
	}

	t.Run("all notes in order", func(t *testing.T) {
		notes, err := d.TaskNotes(taskID, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(notes) != 3 {
			t.Fatalf("expected 3 notes, got %d", len(notes))
		}
		if notes[0].Reason != "first reason" || notes[2].Reason != "third reason" {
			t.Errorf("unexpected order: %+v", notes)
		}
		if notes[2].Metadata["snooze_until"] != "2026-05-02" {
			t.Errorf("metadata not preserved: %+v", notes[2].Metadata)
		}
	})

	t.Run("kind filter", func(t *testing.T) {
		notes, err := d.TaskNotes(taskID, "triage_keep")
		if err != nil {
			t.Fatal(err)
		}
		if len(notes) != 2 {
			t.Fatalf("expected 2 triage_keep notes, got %d", len(notes))
		}
		for _, n := range notes {
			if n.Kind != "triage_keep" {
				t.Errorf("got kind %q, want triage_keep", n.Kind)
			}
		}
	})

	t.Run("empty for unknown kind", func(t *testing.T) {
		notes, err := d.TaskNotes(taskID, "nonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if len(notes) != 0 {
			t.Errorf("expected 0 notes, got %d", len(notes))
		}
	})

	t.Run("nil metadata round trip", func(t *testing.T) {
		notes, err := d.TaskNotes(taskID, "triage_keep")
		if err != nil {
			t.Fatal(err)
		}
		if notes[0].Metadata != nil {
			t.Errorf("expected nil metadata for first note, got %+v", notes[0].Metadata)
		}
	})
}

func TestLatestTaskNotes(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	t1, _ := d.InsertTask(1, "task 1", "{}", "")
	t2, _ := d.InsertTask(1, "task 2", "{}", "")
	t3, _ := d.InsertTask(1, "task 3", "{}", "")

	if err := d.RecordTaskNote(t1, "triage_keep", "old", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordTaskNote(t1, "triage_keep", "newest", map[string]any{"snooze_until": "2026-05-02"}); err != nil {
		t.Fatal(err)
	}
	if err := d.RecordTaskNote(t2, "observation", "obs", nil); err != nil {
		t.Fatal(err)
	}
	// t3 has no notes

	t.Run("filter by kind picks latest matching", func(t *testing.T) {
		got, err := d.LatestTaskNotes([]int64{t1, t2, t3}, "triage_keep")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 entry, got %d: %+v", len(got), got)
		}
		entry, ok := got[t1]
		if !ok {
			t.Fatalf("expected entry for t1, got %+v", got)
		}
		if entry.Reason != "newest" {
			t.Errorf("got reason %q, want newest", entry.Reason)
		}
		if entry.Metadata["snooze_until"] != "2026-05-02" {
			t.Errorf("metadata = %+v", entry.Metadata)
		}
	})

	t.Run("no filter picks latest of any kind", func(t *testing.T) {
		got, err := d.LatestTaskNotes([]int64{t1, t2, t3}, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
		}
		if got[t1].Reason != "newest" {
			t.Errorf("t1 latest = %q, want newest", got[t1].Reason)
		}
		if got[t2].Reason != "obs" {
			t.Errorf("t2 latest = %q, want obs", got[t2].Reason)
		}
		if _, ok := got[t3]; ok {
			t.Errorf("t3 should not appear (no notes)")
		}
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		got, err := d.LatestTaskNotes(nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty map, got %+v", got)
		}
	})
}

func TestListTasksByStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	_, _ = d.InsertTask(1, "open task", "{}", "")
	id2, _ := d.InsertTask(1, "done task", "{}", "")
	d.UpdateTask(id2, db.TaskStatusDone, "")

	tasks, err := d.ListTasksByStatus(db.TaskStatusOpen)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 open task, got %d", len(tasks))
	}

	tasks, err = d.ListTasksByStatus(db.TaskStatusDone)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 done task, got %d", len(tasks))
	}
}
