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

	pid := int64(2)
	if err := d.UpdateTaskFields(id, db.TaskFieldChanges{ProjectID: &pid}); err != nil {
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

func TestListTasksByProjectIDs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertTask(1, "p1-A", "{}", "")
	d.InsertTask(1, "p1-B", "{}", "")
	d.InsertTask(2, "p2-A", "{}", "")

	tests := []struct {
		name           string
		projectIDs     []int64
		wantPerProject map[int64]int
	}{
		{
			name:           "multiple projects grouped",
			projectIDs:     []int64{1, 2, 3},
			wantPerProject: map[int64]int{1: 2, 2: 1},
		},
		{
			name:           "single project",
			projectIDs:     []int64{1},
			wantPerProject: map[int64]int{1: 2},
		},
		{
			name:           "project with no tasks omitted",
			projectIDs:     []int64{3},
			wantPerProject: map[int64]int{},
		},
		{
			name:           "empty projectIDs returns empty map",
			projectIDs:     nil,
			wantPerProject: map[int64]int{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := d.ListTasksByProjectIDs(tc.projectIDs)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.wantPerProject) {
				t.Errorf("len(map) = %d, want %d (got=%v)", len(got), len(tc.wantPerProject), got)
			}
			for pid, wantCount := range tc.wantPerProject {
				if len(got[pid]) != wantCount {
					t.Errorf("project %d tasks = %d, want %d", pid, len(got[pid]), wantCount)
				}
			}
		})
	}

	t.Run("id DESC order", func(t *testing.T) {
		got, err := d.ListTasksByProjectIDs([]int64{1})
		if err != nil {
			t.Fatal(err)
		}
		tasks := got[1]
		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks for project 1, got %d", len(tasks))
		}
		if tasks[0].ID < tasks[1].ID {
			t.Errorf("expected id DESC: tasks[0].ID=%d, tasks[1].ID=%d", tasks[0].ID, tasks[1].ID)
		}
	})
}

func TestGetTasksByIDs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	t1, _ := d.InsertTask(1, "task-1", "{}", "")
	t2, _ := d.InsertTask(1, "task-2", "{}", "")
	t3, _ := d.InsertTask(2, "task-3", "{}", "")

	tests := []struct {
		name    string
		ids     []int64
		wantIDs []int64
	}{
		{name: "all existing", ids: []int64{t1, t2, t3}, wantIDs: []int64{t1, t2, t3}},
		{name: "subset", ids: []int64{t1, t3}, wantIDs: []int64{t1, t3}},
		{name: "missing IDs absent from result", ids: []int64{t1, 9999}, wantIDs: []int64{t1}},
		{name: "all missing returns empty map", ids: []int64{9998, 9999}, wantIDs: nil},
		{name: "empty input returns empty map", ids: nil, wantIDs: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := d.GetTasksByIDs(tc.ids)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.wantIDs) {
				t.Errorf("len(result) = %d, want %d", len(got), len(tc.wantIDs))
			}
			for _, id := range tc.wantIDs {
				task, ok := got[id]
				if !ok {
					t.Errorf("missing id %d in result", id)
					continue
				}
				if task.ID != id {
					t.Errorf("result[%d].ID = %d, want %d", id, task.ID, id)
				}
			}
		})
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

	wd := "/new/path"
	if err := d.UpdateTaskFields(id, db.TaskFieldChanges{WorkDir: &wd}); err != nil {
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

func TestUpdateTaskFields_MetadataMerge(t *testing.T) {
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
			if err := d.UpdateTaskFields(id, db.TaskFieldChanges{Metadata: tc.merge}); err != nil {
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

func TestUpdateTaskFields_AtomicRollback(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "task", `{"keep":"me"}`, "/orig/dir")
	if err != nil {
		t.Fatal(err)
	}

	// project_id REFERENCES projects(id) with foreign_keys=ON, so a
	// non-existent project makes the project write fail mid-transaction.
	missing := int64(99999)
	wd := "/new/dir"
	err = d.UpdateTaskFields(id, db.TaskFieldChanges{
		WorkDir:   &wd,
		Metadata:  map[string]any{"added": "x"},
		ProjectID: &missing,
	})
	if err == nil {
		t.Fatal("expected error from FK violation, got nil")
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.ProjectID != 1 {
		t.Errorf("project_id changed despite rollback: got %d, want 1", task.ProjectID)
	}
	if task.WorkDir != "/orig/dir" {
		t.Errorf("work_dir changed despite rollback: got %q, want %q", task.WorkDir, "/orig/dir")
	}
	if task.Metadata != `{"keep":"me"}` {
		t.Errorf("metadata changed despite rollback: got %s", task.Metadata)
	}
}

func TestUpdateTaskFields_AllFieldsAtomicSuccess(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "task", `{"keep":"me"}`, "/orig/dir")
	if err != nil {
		t.Fatal(err)
	}

	pid := int64(2)
	wd := "/new/dir"
	status := db.TaskStatusDone
	if err := d.UpdateTaskFields(id, db.TaskFieldChanges{
		ProjectID: &pid,
		WorkDir:   &wd,
		Metadata:  map[string]any{"added": "x"},
		Status:    &status,
		Reason:    "all at once",
	}); err != nil {
		t.Fatal(err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.ProjectID != 2 {
		t.Errorf("project_id: got %d, want 2", task.ProjectID)
	}
	if task.WorkDir != "/new/dir" {
		t.Errorf("work_dir: got %q, want %q", task.WorkDir, "/new/dir")
	}
	if task.Metadata != `{"added":"x","keep":"me"}` {
		t.Errorf("metadata: got %s", task.Metadata)
	}
	if task.Status != db.TaskStatusDone {
		t.Errorf("status: got %s, want done", task.Status)
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

	tests := []struct {
		name                string
		initialActionStatus string
		markDispatched      bool
		finalActionMark     bool
		targetStatus        string
		wantErr             bool
	}{
		{"pending action blocks done", db.ActionStatusPending, false, false, db.TaskStatusDone, true},
		{"pending action blocks archived", db.ActionStatusPending, false, false, db.TaskStatusArchived, true},
		{"running action blocks done", db.ActionStatusRunning, false, false, db.TaskStatusDone, true},
		{"dispatched action blocks done", db.ActionStatusRunning, true, false, db.TaskStatusDone, true},
		{"dispatched action blocks archived", db.ActionStatusRunning, true, false, db.TaskStatusArchived, true},
		{"cancelled action does not block done", db.ActionStatusRunning, false, true, db.TaskStatusDone, false},
		{"cancelled action does not block archived", db.ActionStatusPending, false, true, db.TaskStatusArchived, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taskID, _ := d.InsertTask(1, "test", "{}", "")
			actionID, _ := d.InsertAction(tc.name, taskID, "{}", tc.initialActionStatus, nil, "")
			if tc.markDispatched {
				if err := d.MarkDispatched(actionID); err != nil {
					t.Fatalf("MarkDispatched: %v", err)
				}
			}
			if tc.finalActionMark {
				if err := d.MarkCancelled(actionID, ""); err != nil {
					t.Fatalf("MarkCancelled: %v", err)
				}
			}
			err := d.UpdateTask(taskID, tc.targetStatus, "")
			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateTask(%q) error = %v, wantErr %v", tc.targetStatus, err, tc.wantErr)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "pending/running/dispatched action(s)") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
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
