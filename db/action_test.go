package db_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestInsertAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	id, err := d.InsertAction("review-pr", &taskID, `{"pr":123}`, "pending", 5, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if id < 1 {
		t.Errorf("expected positive id, got %d", id)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.TemplateID != "review-pr" {
		t.Errorf("expected template_id 'review-pr', got %s", a.TemplateID)
	}
	if a.Priority != 5 {
		t.Errorf("expected priority 5, got %d", a.Priority)
	}
}

func TestInsertAction_NilTaskID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertAction("standalone", nil, "{}", "pending", 0, "human")
	if err != nil {
		t.Fatal(err)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.TaskID.Valid {
		t.Error("expected task_id to be NULL")
	}
}

func TestHasActiveAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", 0, "auto")

	has, err := d.HasActiveAction(taskID, "review-pr")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected true for pending action")
	}

	has, err = d.HasActiveAction(taskID, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected false for non-existent template")
	}
}

func TestHasActiveAction_WaitingHuman(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "", "{}")
	d.InsertAction("implement", &taskID, "{}", "waiting_human", 0, "auto")

	has, err := d.HasActiveAction(taskID, "implement")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected true for waiting_human action")
	}
}

func TestHasActiveAction_DoneNotActive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "", "{}")
	d.InsertAction("implement", &taskID, "{}", "done", 0, "auto")

	has, err := d.HasActiveAction(taskID, "implement")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected false for done action")
	}
}

func TestNextPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertAction("low-priority", nil, "{}", "pending", 1, "auto")
	d.InsertAction("high-priority", nil, "{}", "pending", 10, "auto")
	d.InsertAction("high-priority-2", nil, "{}", "pending", 10, "auto")

	ctx := context.Background()

	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected action, got nil")
	}
	if a.TemplateID != "high-priority" {
		t.Errorf("expected high-priority first, got %s", a.TemplateID)
	}
	if a.Status != "running" {
		t.Errorf("expected status running, got %s", a.Status)
	}

	// Verify it's persisted as running
	fetched, _ := d.GetAction(a.ID)
	if fetched.Status != "running" {
		t.Errorf("expected persisted status running, got %s", fetched.Status)
	}

	// Next should return high-priority-2 (same priority, lower ID)
	a2, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a2.TemplateID != "high-priority-2" {
		t.Errorf("expected high-priority-2, got %s", a2.TemplateID)
	}
}

func TestNextPending_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	a, err := d.NextPending(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a != nil {
		t.Errorf("expected nil, got action %d", a.ID)
	}
}

func TestMarkDone(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("test", nil, "{}", "running", 0, "auto")
	if err := d.MarkDone(id, "success"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != "done" {
		t.Errorf("expected status done, got %s", a.Status)
	}
	if !a.CompletedAt.Valid {
		t.Error("expected completed_at to be set")
	}
	if !a.Result.Valid || a.Result.String != "success" {
		t.Errorf("expected result 'success', got %v", a.Result)
	}
}

func TestMarkFailed(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("test", nil, "{}", "running", 0, "auto")
	if err := d.MarkFailed(id, "error occurred"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != "failed" {
		t.Errorf("expected status failed, got %s", a.Status)
	}
	if !a.Result.Valid || a.Result.String != "error occurred" {
		t.Errorf("expected result 'error occurred', got %v", a.Result)
	}
}

func TestMarkWaitingHuman(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("test", nil, "{}", "running", 0, "auto")
	if err := d.MarkWaitingHuman(id, "needs approval"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != "waiting_human" {
		t.Errorf("expected status waiting_human, got %s", a.Status)
	}
}

func TestListActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "task1", "", "{}")
	taskID2, _ := d.InsertTask(1, "task2", "", "{}")
	d.InsertAction("a", &taskID1, "{}", "pending", 0, "auto")
	d.InsertAction("b", &taskID2, "{}", "running", 0, "auto")
	d.InsertAction("c", nil, "{}", "pending", 0, "human")

	// No filter
	all, err := d.ListActions("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 actions, got %d", len(all))
	}

	// Status filter
	pending, err := d.ListActions("pending", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending actions, got %d", len(pending))
	}

	// Task filter
	task1Actions, err := d.ListActions("", &taskID1)
	if err != nil {
		t.Fatal(err)
	}
	if len(task1Actions) != 1 {
		t.Errorf("expected 1 action for task1, got %d", len(task1Actions))
	}

	// Both filters
	both, err := d.ListActions("pending", &taskID1)
	if err != nil {
		t.Fatal(err)
	}
	if len(both) != 1 {
		t.Errorf("expected 1 pending action for task1, got %d", len(both))
	}
}

func TestCountByStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "", "{}")
	d.InsertAction("a", &taskID, "{}", "pending", 0, "auto")
	d.InsertAction("b", &taskID, "{}", "pending", 0, "auto")
	d.InsertAction("c", &taskID, "{}", "running", 0, "auto")
	d.InsertAction("d", &taskID, "{}", "done", 0, "auto")

	counts, err := d.CountByStatus()
	if err != nil {
		t.Fatal(err)
	}
	if counts["pending"] != 2 {
		t.Errorf("pending = %d, want 2", counts["pending"])
	}
	if counts["running"] != 1 {
		t.Errorf("running = %d, want 1", counts["running"])
	}
	if counts["done"] != 1 {
		t.Errorf("done = %d, want 1", counts["done"])
	}
}

func TestCountRunningInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertAction("a", nil, "{}", "running", 0, "auto")
	d.InsertAction("b", nil, "{}", "running", 0, "auto")
	d.Exec("UPDATE actions SET session_id = 'sess-1' WHERE id = 1")

	count, err := d.CountRunningInteractive()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (only one has session_id)", count)
	}
}

func TestResetToPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("a", nil, "{}", "running", 0, "auto")
	if err := d.ResetToPending(id); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != "pending" {
		t.Errorf("status = %q, want pending", a.Status)
	}
	if a.StartedAt.Valid {
		t.Error("started_at should be NULL after reset")
	}
}

func TestAction_MatchesDate(t *testing.T) {
	tests := []struct {
		name string
		a    db.Action
		date string
		want bool
	}{
		{
			name: "match by created_at",
			a:    db.Action{CreatedAt: "2026-01-15 10:00:00"},
			date: "2026-01-15",
			want: true,
		},
		{
			name: "match by started_at",
			a: db.Action{
				CreatedAt: "2026-01-01 00:00:00",
				StartedAt: sql.NullString{String: "2026-02-20 09:00:00", Valid: true},
			},
			date: "2026-02-20",
			want: true,
		},
		{
			name: "match by completed_at",
			a: db.Action{
				CreatedAt:   "2026-01-01 00:00:00",
				CompletedAt: sql.NullString{String: "2026-03-01 18:00:00", Valid: true},
			},
			date: "2026-03-01",
			want: true,
		},
		{
			name: "no match",
			a:    db.Action{CreatedAt: "2026-01-01 00:00:00"},
			date: "2026-12-25",
			want: false,
		},
		{
			name: "null started_at and completed_at",
			a:    db.Action{CreatedAt: "2026-01-15 10:00:00"},
			date: "2026-01-15",
			want: true,
		},
		{
			name: "empty date matches all",
			a:    db.Action{CreatedAt: "2026-01-15 10:00:00"},
			date: "",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.MatchesDate(tt.date)
			if got != tt.want {
				t.Errorf("MatchesDate(%q) = %v, want %v", tt.date, got, tt.want)
			}
		})
	}
}

func TestFilterForOpenTask(t *testing.T) {
	actions := []db.Action{
		{ID: 1, Status: "pending", CreatedAt: "2026-01-01 00:00:00"},
		{ID: 2, Status: "running", CreatedAt: "2026-01-01 00:00:00"},
		{ID: 3, Status: "waiting_human", CreatedAt: "2026-01-01 00:00:00"},
		{ID: 4, Status: "done", CreatedAt: "2026-01-01 00:00:00", CompletedAt: sql.NullString{String: "2026-03-04 10:00:00", Valid: true}},
		{ID: 5, Status: "failed", CreatedAt: "2026-01-01 00:00:00", CompletedAt: sql.NullString{String: "2026-01-01 12:00:00", Valid: true}},
		{ID: 6, Status: "done", CreatedAt: "2026-03-04 09:00:00"},
	}

	filtered := db.FilterForOpenTask(actions, "2026-03-04")

	ids := make(map[int64]bool)
	for _, a := range filtered {
		ids[a.ID] = true
	}

	// pending/running/waiting_human always included
	for _, id := range []int64{1, 2, 3} {
		if !ids[id] {
			t.Errorf("expected action %d (non-terminal) to be included", id)
		}
	}
	// done matching date
	if !ids[4] {
		t.Error("expected action 4 (done, date match) to be included")
	}
	// done matching by created_at
	if !ids[6] {
		t.Error("expected action 6 (done, created_at match) to be included")
	}
	// failed not matching date
	if ids[5] {
		t.Error("expected action 5 (failed, no date match) to be excluded")
	}
}

func TestFilterForOpenTask_EmptyDate(t *testing.T) {
	actions := []db.Action{
		{ID: 1, Status: "done", CreatedAt: "2026-01-01 00:00:00"},
	}
	filtered := db.FilterForOpenTask(actions, "")
	if len(filtered) != 1 {
		t.Errorf("expected all actions returned for empty date, got %d", len(filtered))
	}
}

func TestGetAction_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	_, err := d.GetAction(999)
	if err == nil {
		t.Error("expected error for non-existent action")
	}
}
