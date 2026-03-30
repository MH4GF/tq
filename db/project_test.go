package db_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestGetProjectByName(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "immedio" {
		t.Errorf("expected name immedio, got %s", p.Name)
	}
	if p.WorkDir != "~/ghq/github.com/immedioinc/immedio" {
		t.Errorf("unexpected work_dir: %s", p.WorkDir)
	}
}

func TestGetProjectByName_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)

	_, err := d.GetProjectByName("nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestListProjects(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	projects, err := d.ListProjects(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
	}

	limited, err := d.ListProjects(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 2 {
		t.Errorf("expected 2 projects with limit=2, got %d", len(limited))
	}
	if limited[0].ID < limited[1].ID {
		t.Errorf("expected DESC order: first ID %d should be > second ID %d", limited[0].ID, limited[1].ID)
	}
}

func TestInsertProject(t *testing.T) {
	d := testutil.NewTestDB(t)

	id, err := d.InsertProject("myapp", "/tmp/myapp", `{"key":"val"}`)
	if err != nil {
		t.Fatal(err)
	}
	if id < 1 {
		t.Errorf("expected positive id, got %d", id)
	}

	p, err := d.GetProjectByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "myapp" {
		t.Errorf("name = %q, want %q", p.Name, "myapp")
	}
	if p.WorkDir != "/tmp/myapp" {
		t.Errorf("work_dir = %q, want %q", p.WorkDir, "/tmp/myapp")
	}
	if p.Metadata != `{"key":"val"}` {
		t.Errorf("metadata = %q, want %q", p.Metadata, `{"key":"val"}`)
	}
}

func TestDeleteProject(t *testing.T) {
	d := testutil.NewTestDB(t)

	id, err := d.InsertProject("todelete", "/tmp/del", "{}")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.DeleteProject(id, false); err != nil {
		t.Fatal(err)
	}

	_, err = d.GetProjectByID(id)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestDeleteProject_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)

	err := d.DeleteProject(999, false)
	if err == nil {
		t.Error("expected error for non-existent project")
	}
}

func TestDeleteProject_WithTasks_NoCascade(t *testing.T) {
	d := testutil.NewTestDB(t)

	pid, err := d.InsertProject("haswork", "/tmp/hw", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.InsertTask(pid, "some task", "{}", ""); err != nil {
		t.Fatal(err)
	}

	err = d.DeleteProject(pid, false)
	if err == nil {
		t.Fatal("expected error when tasks exist and cascade=false")
	}
	if !strings.Contains(err.Error(), "cannot delete without cascade") {
		t.Errorf("error should mention cascade requirement, got: %s", err)
	}

	// project should still exist
	if _, err := d.GetProjectByID(pid); err != nil {
		t.Errorf("project should still exist: %v", err)
	}
}

func TestDeleteProject_Cascade(t *testing.T) {
	d := testutil.NewTestDB(t)

	pid, err := d.InsertProject("cascademe", "/tmp/cm", "{}")
	if err != nil {
		t.Fatal(err)
	}
	tid, err := d.InsertTask(pid, "task1", "{}", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.InsertAction("act1", tid, "{}", "pending"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.InsertSchedule(tid, "do stuff", "sched1", "0 * * * *", "{}"); err != nil {
		t.Fatal(err)
	}

	if err := d.DeleteProject(pid, true); err != nil {
		t.Fatal(err)
	}

	if _, err := d.GetProjectByID(pid); err == nil {
		t.Error("project should be deleted")
	}
	if _, err := d.GetTask(tid); err == nil {
		t.Error("task should be deleted")
	}
}

func TestDeleteProject_Cascade_NoTasks(t *testing.T) {
	d := testutil.NewTestDB(t)

	pid, err := d.InsertProject("empty", "/tmp/e", "{}")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.DeleteProject(pid, true); err != nil {
		t.Fatal(err)
	}

	if _, err := d.GetProjectByID(pid); err == nil {
		t.Error("project should be deleted")
	}
}

func TestProject_DispatchEnabledDefault(t *testing.T) {
	d := testutil.NewTestDB(t)

	id, err := d.InsertProject("test", "/tmp/test", "{}")
	if err != nil {
		t.Fatal(err)
	}

	p, err := d.GetProjectByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if !p.DispatchEnabled {
		t.Error("expected dispatch_enabled to default to true")
	}
}

func TestSetDispatchEnabled(t *testing.T) {
	d := testutil.NewTestDB(t)

	id, err := d.InsertProject("test", "/tmp/test", "{}")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.SetDispatchEnabled(id, false); err != nil {
		t.Fatal(err)
	}

	p, err := d.GetProjectByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.DispatchEnabled {
		t.Error("expected dispatch_enabled to be false")
	}

	if err := d.SetDispatchEnabled(id, true); err != nil {
		t.Fatal(err)
	}

	p, err = d.GetProjectByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if !p.DispatchEnabled {
		t.Error("expected dispatch_enabled to be true")
	}
}

func TestSetDispatchEnabled_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)

	err := d.SetDispatchEnabled(999, false)
	if err == nil {
		t.Error("expected error for non-existent project")
	}
}

func TestSetWorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)

	id, err := d.InsertProject("test", "/tmp/old", "{}")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.SetWorkDir(id, "/tmp/new"); err != nil {
		t.Fatal(err)
	}

	p, err := d.GetProjectByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.WorkDir != "/tmp/new" {
		t.Errorf("work_dir = %q, want %q", p.WorkDir, "/tmp/new")
	}
}

func TestSetWorkDir_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)

	err := d.SetWorkDir(999, "/tmp/nope")
	if err == nil {
		t.Error("expected error for non-existent project")
	}
}

func TestEnsureProject(t *testing.T) {
	d := testutil.NewTestDB(t)

	id1, err := d.EnsureProject("test-proj")
	if err != nil {
		t.Fatal(err)
	}
	if id1 < 1 {
		t.Errorf("expected positive id, got %d", id1)
	}

	id2, err := d.EnsureProject("test-proj")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("EnsureProject returned different IDs: %d vs %d", id1, id2)
	}

	projects, err := d.ListProjects(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}

func TestSetAllDispatchEnabled(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	if err := d.SetAllDispatchEnabled(false); err != nil {
		t.Fatal(err)
	}

	projects, err := d.ListProjects(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if p.DispatchEnabled {
			t.Errorf("project %s should be disabled", p.Name)
		}
	}

	if err := d.SetAllDispatchEnabled(true); err != nil {
		t.Fatal(err)
	}

	projects, err = d.ListProjects(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if !p.DispatchEnabled {
			t.Errorf("project %s should be enabled", p.Name)
		}
	}
}
