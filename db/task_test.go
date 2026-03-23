package db_test

import (
	"database/sql"
	"errors"
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

	t.Run("no filter", func(t *testing.T) {
		tasks, err := d.ListTasks(0, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 3 {
			t.Errorf("expected 3 tasks, got %d", len(tasks))
		}
	})

	t.Run("filter by project", func(t *testing.T) {
		tasks, err := d.ListTasks(1, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks for project 1, got %d", len(tasks))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		tasks, err := d.ListTasks(0, db.TaskStatusOpen, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 open tasks, got %d", len(tasks))
		}
	})

	t.Run("filter by project and status", func(t *testing.T) {
		tasks, err := d.ListTasks(1, db.TaskStatusDone, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 1 {
			t.Errorf("expected 1 done task for project 1, got %d", len(tasks))
		}
	})

	t.Run("limit", func(t *testing.T) {
		tasks, err := d.ListTasks(0, "", 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks with limit=2, got %d", len(tasks))
		}
		if tasks[0].ID < tasks[1].ID {
			t.Errorf("expected DESC order: first ID %d should be > second ID %d", tasks[0].ID, tasks[1].ID)
		}
	})
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

	t.Run("merge new key", func(t *testing.T) {
		id, err := d.InsertTask(1, "task", `{"existing":"value"}`, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := d.MergeTaskMetadata(id, map[string]any{"url": "https://example.com"}); err != nil {
			t.Fatal(err)
		}
		task, err := d.GetTask(id)
		if err != nil {
			t.Fatal(err)
		}
		if task.Metadata != `{"existing":"value","url":"https://example.com"}` {
			t.Errorf("expected merged metadata, got %s", task.Metadata)
		}
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		id, err := d.InsertTask(1, "task", `{"existing":"value","url":"https://example.com"}`, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := d.MergeTaskMetadata(id, map[string]any{"existing": "new"}); err != nil {
			t.Fatal(err)
		}
		task, err := d.GetTask(id)
		if err != nil {
			t.Fatal(err)
		}
		if task.Metadata != `{"existing":"new","url":"https://example.com"}` {
			t.Errorf("expected overwritten key, got %s", task.Metadata)
		}
	})

	t.Run("merge into empty metadata", func(t *testing.T) {
		id, err := d.InsertTask(1, "task2", "{}", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := d.MergeTaskMetadata(id, map[string]any{"key": "val"}); err != nil {
			t.Fatal(err)
		}
		task, err := d.GetTask(id)
		if err != nil {
			t.Fatal(err)
		}
		if task.Metadata != `{"key":"val"}` {
			t.Errorf("expected metadata on empty, got %s", task.Metadata)
		}
	})
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
		{"review allowed with active schedule", "review", nil, false},
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
