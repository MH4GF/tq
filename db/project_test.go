package db_test

import (
	"database/sql"
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
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestListProjects(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	projects, err := d.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
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

	if err := d.DeleteProject(id); err != nil {
		t.Fatal(err)
	}

	_, err = d.GetProjectByID(id)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestDeleteProject_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)

	err := d.DeleteProject(999)
	if err == nil {
		t.Error("expected error for non-existent project")
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

func TestSetAllDispatchEnabled(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	if err := d.SetAllDispatchEnabled(false); err != nil {
		t.Fatal(err)
	}

	projects, err := d.ListProjects()
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

	projects, err = d.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if !p.DispatchEnabled {
			t.Errorf("project %s should be enabled", p.Name)
		}
	}
}
