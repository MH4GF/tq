package db_test

import (
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestListEvents(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")

	events, err := d.ListEvents("task", taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event from InsertTask")
	}
	if events[0].EventType != "task.created" {
		t.Errorf("expected event_type 'task.created', got %s", events[0].EventType)
	}
	if events[0].EntityID != taskID {
		t.Errorf("expected entity_id %d, got %d", taskID, events[0].EntityID)
	}
}

func TestListEvents_FiltersByEntity(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "task1", "{}", "")
	taskID2, _ := d.InsertTask(1, "task2", "{}", "")

	events1, _ := d.ListEvents("task", taskID1)
	events2, _ := d.ListEvents("task", taskID2)

	if len(events1) != 1 {
		t.Errorf("expected 1 event for task1, got %d", len(events1))
	}
	if len(events2) != 1 {
		t.Errorf("expected 1 event for task2, got %d", len(events2))
	}
}

func TestListRecentEvents(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertTask(1, "task1", "{}", "")
	d.InsertTask(1, "task2", "{}", "")
	d.InsertTask(1, "task3", "{}", "")

	events, err := d.ListRecentEvents(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
	// Most recent first
	if events[0].ID <= events[1].ID {
		t.Error("expected descending order")
	}
}
