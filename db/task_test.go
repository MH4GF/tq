package db_test

import (
	"database/sql"
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestInsertTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertTask(1, "test task", "https://example.com", "{}")
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

	id, err := d.InsertTask(1, "task to update", "", "{}")
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

	id, err := d.InsertTask(1, "task to move", "", "{}")
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

	d.InsertTask(1, "task A", "", "{}")
	id2, _ := d.InsertTask(1, "task B", "", "{}")
	d.InsertTask(2, "task C", "", "{}")
	d.UpdateTask(id2, "done")

	t.Run("no filter", func(t *testing.T) {
		tasks, err := d.ListTasks("", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 3 {
			t.Errorf("expected 3 tasks, got %d", len(tasks))
		}
	})

	t.Run("filter by project", func(t *testing.T) {
		tasks, err := d.ListTasks("immedio", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 tasks for immedio, got %d", len(tasks))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		tasks, err := d.ListTasks("", "open")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 2 {
			t.Errorf("expected 2 open tasks, got %d", len(tasks))
		}
	})

	t.Run("filter by project and status", func(t *testing.T) {
		tasks, err := d.ListTasks("immedio", "done")
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 1 {
			t.Errorf("expected 1 done task for immedio, got %d", len(tasks))
		}
	})
}

func TestListTasksByProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertTask(1, "task A", "", "{}")
	d.InsertTask(1, "task B", "", "{}")
	d.InsertTask(2, "task C", "", "{}")

	tasks, err := d.ListTasksByProject(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for project 1, got %d", len(tasks))
	}
}

func TestInsertTask_SpacesAndLongMetadata(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tests := []struct {
		name     string
		title    string
		url      string
		metadata string
	}{
		{
			name:     "title with multiple spaces",
			title:    "review pull request for feature branch",
			url:      "https://github.com/org/repo/pull/42",
			metadata: `{"type":"pr","number":42}`,
		},
		{
			name:     "title with leading and trailing spaces",
			title:    "  spaced out title  ",
			url:      "",
			metadata: `{}`,
		},
		{
			name:  "long metadata with prompt content",
			title: "task with long prompt",
			url:   "",
			metadata: `{"prompt":"You are a helpful assistant. Please review the following code changes carefully and provide detailed feedback on correctness, performance, and style. Pay special attention to edge cases and potential security issues. Make sure to check for proper error handling and resource cleanup.","labels":["review","security","performance"],"context":{"repo":"org/repo","branch":"feature/new-auth","files_changed":15}}`,
		},
		{
			name:     "title with special whitespace",
			title:    "fix\tbug\nin\tmodule",
			url:      "https://example.com/issues/1",
			metadata: `{"description":"a task with\ttabs and\nnewlines in metadata"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := d.InsertTask(1, tt.title, tt.url, tt.metadata)
			if err != nil {
				t.Fatalf("InsertTask failed: %v", err)
			}

			task, err := d.GetTask(id)
			if err != nil {
				t.Fatalf("GetTask failed: %v", err)
			}
			if task.Title != tt.title {
				t.Errorf("title mismatch: got %q, want %q", task.Title, tt.title)
			}
			if task.URL != tt.url {
				t.Errorf("url mismatch: got %q, want %q", task.URL, tt.url)
			}
			if task.Metadata != tt.metadata {
				t.Errorf("metadata mismatch: got %q, want %q", task.Metadata, tt.metadata)
			}
		})
	}
}

func TestListTasksByStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id1, _ := d.InsertTask(1, "open task", "", "{}")
	id2, _ := d.InsertTask(1, "done task", "", "{}")
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
