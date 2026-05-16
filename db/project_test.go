package db_test

import (
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

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

func TestGetProjectsByIDs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	tests := []struct {
		name    string
		ids     []int64
		wantIDs []int64
	}{
		{name: "all existing", ids: []int64{1, 2, 3}, wantIDs: []int64{1, 2, 3}},
		{name: "subset", ids: []int64{1, 3}, wantIDs: []int64{1, 3}},
		{name: "missing IDs absent", ids: []int64{1, 9999}, wantIDs: []int64{1}},
		{name: "all missing returns empty map", ids: []int64{9998, 9999}, wantIDs: nil},
		{name: "empty input returns empty map", ids: nil, wantIDs: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := d.GetProjectsByIDs(tc.ids)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.wantIDs) {
				t.Errorf("len(result) = %d, want %d", len(got), len(tc.wantIDs))
			}
			for _, id := range tc.wantIDs {
				p, ok := got[id]
				if !ok {
					t.Errorf("missing id %d", id)
					continue
				}
				if p.ID != id {
					t.Errorf("result[%d].ID = %d, want %d", id, p.ID, id)
				}
			}
		})
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
	tests := []struct {
		name            string
		setup           func(t *testing.T, d *db.DB) (projectID, taskID int64)
		cascade         bool
		wantErr         bool
		wantErrContains string
		wantProjectKept bool
	}{
		{
			name: "deletes project without tasks",
			setup: func(t *testing.T, d *db.DB) (int64, int64) {
				t.Helper()
				id, err := d.InsertProject("todelete", "/tmp/del", "{}")
				if err != nil {
					t.Fatal(err)
				}
				return id, 0
			},
		},
		{
			name: "not found returns error",
			setup: func(t *testing.T, _ *db.DB) (int64, int64) {
				t.Helper()
				return 999, 0
			},
			wantErr: true,
		},
		{
			name: "with tasks and cascade=false fails",
			setup: func(t *testing.T, d *db.DB) (int64, int64) {
				t.Helper()
				pid, err := d.InsertProject("haswork", "/tmp/hw", "{}")
				if err != nil {
					t.Fatal(err)
				}
				tid, err := d.InsertTask(pid, "some task", "{}", "")
				if err != nil {
					t.Fatal(err)
				}
				return pid, tid
			},
			wantErr:         true,
			wantErrContains: "cannot delete without cascade",
			wantProjectKept: true,
		},
		{
			name: "cascade deletes project and tasks",
			setup: func(t *testing.T, d *db.DB) (int64, int64) {
				t.Helper()
				pid, err := d.InsertProject("cascademe", "/tmp/cm", "{}")
				if err != nil {
					t.Fatal(err)
				}
				tid, err := d.InsertTask(pid, "task1", "{}", "")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := d.InsertAction("act1", tid, "{}", "pending", nil, ""); err != nil {
					t.Fatal(err)
				}
				if _, err := d.InsertSchedule(tid, "do stuff", "sched1", "0 * * * *", "{}"); err != nil {
					t.Fatal(err)
				}
				return pid, tid
			},
			cascade: true,
		},
		{
			name: "cascade with no tasks",
			setup: func(t *testing.T, d *db.DB) (int64, int64) {
				t.Helper()
				pid, err := d.InsertProject("empty", "/tmp/e", "{}")
				if err != nil {
					t.Fatal(err)
				}
				return pid, 0
			},
			cascade: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			projectID, taskID := tt.setup(t, d)

			err := d.DeleteProject(projectID, tt.cascade)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error should contain %q, got: %s", tt.wantErrContains, err)
				}
				if tt.wantProjectKept {
					if _, err := d.GetProjectByID(projectID); err != nil {
						t.Errorf("project should still exist: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, err := d.GetProjectByID(projectID); err == nil {
				t.Error("project should be deleted")
			}
			if taskID != 0 {
				if _, err := d.GetTask(taskID); err == nil {
					t.Error("task should be deleted")
				}
			}
		})
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

func TestSetWorkDir_EmitsEvent(t *testing.T) {
	d := testutil.NewTestDB(t)

	id, err := d.InsertProject("test", "/tmp/old", "{}")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.SetWorkDir(id, "/tmp/new"); err != nil {
		t.Fatal(err)
	}

	events, err := d.ListEvents("project", id)
	if err != nil {
		t.Fatal(err)
	}

	var got *db.Event
	for i := range events {
		if events[i].EventType == "project.work_dir_changed" {
			got = &events[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected project.work_dir_changed event, got %d events", len(events))
	}
	if !strings.Contains(got.Payload, `"work_dir":"/tmp/new"`) {
		t.Errorf("payload missing work_dir=/tmp/new: %s", got.Payload)
	}
}

func TestSetWorkDir_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)

	err := d.SetWorkDir(999, "/tmp/nope")
	if err == nil {
		t.Error("expected error for non-existent project")
	}
}
