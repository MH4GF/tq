package db_test

import (
	"database/sql"
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestInsertTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "test task", "", "https://example.com", "{}")
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
	if task.Status != "open" {
		t.Errorf("expected status 'open', got %s", task.Status)
	}
}

func TestUpdateTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "task to update", "", "", "{}")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.UpdateTask(id, "done"); err != nil {
		t.Fatal(err)
	}

	task, err := d.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "done" {
		t.Errorf("expected status 'done', got %s", task.Status)
	}
	if !task.UpdatedAt.Valid {
		t.Error("expected updated_at to be set")
	}
}

func TestUpdateTaskProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "task to move", "", "", "{}")
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
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestListTasks(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertTask(1, "task A", "", "", "{}")
	id2, _ := d.InsertTask(1, "task B", "", "", "{}")
	d.InsertTask(2, "task C", "", "", "{}")
	d.UpdateTask(id2, "done")

	t.Run("no filter", func(t *testing.T) {
		tasks, err := d.ListTasks(0, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 3 {
			t.Errorf("expected 3 tasks, got %d", len(tasks))
		}
	})

	t.Run("filter by project", func(t *testing.T) {
		tasks, err := d.ListTasks(1, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks for project 1, got %d", len(tasks))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		tasks, err := d.ListTasks(0, "open")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 open tasks, got %d", len(tasks))
		}
	})

	t.Run("filter by project and status", func(t *testing.T) {
		tasks, err := d.ListTasks(1, "done")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 1 {
			t.Errorf("expected 1 done task for project 1, got %d", len(tasks))
		}
	})
}

func TestListTasksByProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertTask(1, "task A", "", "", "{}")
	d.InsertTask(1, "task B", "", "", "{}")
	d.InsertTask(2, "task C", "", "", "{}")

	tasks, err := d.ListTasksByProject(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for project 1, got %d", len(tasks))
	}
}

func TestListTasksByStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id1, _ := d.InsertTask(1, "open task", "", "", "{}")
	id2, _ := d.InsertTask(1, "done task", "", "", "{}")
	_ = id1
	d.UpdateTask(id2, "done")

	tasks, err := d.ListTasksByStatus("open")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 open task, got %d", len(tasks))
	}

	tasks, err = d.ListTasksByStatus("done")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 done task, got %d", len(tasks))
	}
}
