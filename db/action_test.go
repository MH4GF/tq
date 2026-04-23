package db_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestInsertAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	id, err := d.InsertAction("review-pr", taskID, `{"pr":123}`, db.ActionStatusPending, nil)
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
	if a.Title != "review-pr" {
		t.Errorf("expected title 'review-pr', got %s", a.Title)
	}
	if a.TaskID != taskID {
		t.Errorf("expected task_id %d, got %d", taskID, a.TaskID)
	}
}

func TestNextPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertAction("first", taskID, "{}", db.ActionStatusPending, nil)
	d.InsertAction("second", taskID, "{}", db.ActionStatusPending, nil)
	d.InsertAction("third", taskID, "{}", db.ActionStatusPending, nil)

	ctx := context.Background()

	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected action, got nil")
	}
	if a.Title != "first" {
		t.Errorf("expected first (lowest ID), got %s", a.Title)
	}
	if a.Status != db.ActionStatusRunning {
		t.Errorf("expected status running, got %s", a.Status)
	}

	// Verify it's persisted as running
	fetched, _ := d.GetAction(a.ID)
	if fetched.Status != db.ActionStatusRunning {
		t.Errorf("expected persisted status running, got %s", fetched.Status)
	}

	// Next should return second (next lowest ID)
	a2, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a2.Title != "second" {
		t.Errorf("expected second, got %s", a2.Title)
	}
}

func TestInsertAction_WithDispatchAfter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	after := "2099-12-31 23:59:59"
	id, err := d.InsertAction("scheduled", taskID, "{}", db.ActionStatusPending, &after)
	if err != nil {
		t.Fatal(err)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatal(err)
	}
	if !a.DispatchAfter.Valid {
		t.Fatal("expected dispatch_after to be set")
	}
	if a.DispatchAfter.String != after {
		t.Errorf("expected dispatch_after %q, got %q", after, a.DispatchAfter.String)
	}
}

func TestNextPending_DispatchAfter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")

	// Action with future dispatch_after should be skipped
	future := "2099-12-31 23:59:59"
	d.InsertAction("future", taskID, "{}", db.ActionStatusPending, &future)

	// Action with past dispatch_after should be picked up
	past := "2000-01-01 00:00:00"
	d.InsertAction("past", taskID, "{}", db.ActionStatusPending, &past)

	// Action with no dispatch_after should be picked up (immediate)
	d.InsertAction("immediate", taskID, "{}", db.ActionStatusPending, nil)

	ctx := context.Background()

	// First should be "past" (lower ID than "immediate", both eligible)
	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected action, got nil")
	}
	if a.Title != "past" {
		t.Errorf("expected 'past', got %q", a.Title)
	}

	// Second should be "immediate"
	a2, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a2 == nil {
		t.Fatal("expected action, got nil")
	}
	if a2.Title != "immediate" {
		t.Errorf("expected 'immediate', got %q", a2.Title)
	}

	// Third should be nil (only "future" remains, not eligible)
	a3, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a3 != nil {
		t.Errorf("expected nil (future action should be skipped), got %q", a3.Title)
	}
}

func TestCountPendingByDispatch_DispatchAfter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")

	// Immediate action
	d.InsertAction("now", taskID, "{}", db.ActionStatusPending, nil)

	// Future action (should not be counted)
	future := "2099-12-31 23:59:59"
	d.InsertAction("later", taskID, "{}", db.ActionStatusPending, &future)

	pc, err := d.CountPendingByDispatch()
	if err != nil {
		t.Fatal(err)
	}
	if pc.Dispatchable != 1 {
		t.Errorf("expected 1 dispatchable, got %d", pc.Dispatchable)
	}
	if pc.Total != 1 {
		t.Errorf("expected 1 total, got %d", pc.Total)
	}
}

func TestNextPending_SkipsDisabledProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Disable project 1 (immedio)
	d.SetDispatchEnabled(1, false)

	taskID, _ := d.InsertTask(1, "disabled task", "{}", "")
	d.InsertAction("disabled-action", taskID, "{}", db.ActionStatusPending, nil)

	taskID2, _ := d.InsertTask(2, "enabled task", "{}", "")
	d.InsertAction("enabled-action", taskID2, "{}", db.ActionStatusPending, nil)

	ctx := context.Background()
	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected action, got nil")
	}
	if a.Title != "enabled-action" {
		t.Errorf("expected enabled-action, got %s", a.Title)
	}
}

func TestNextPending_AllDisabled(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Disable all projects
	d.SetAllDispatchEnabled(false)

	taskID, _ := d.InsertTask(1, "disabled task", "{}", "")
	d.InsertAction("disabled-action", taskID, "{}", db.ActionStatusPending, nil)

	ctx := context.Background()
	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a != nil {
		t.Errorf("expected nil when all projects disabled, got action %s", a.Title)
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

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)
	if err := d.MarkDone(id, "success"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusDone {
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

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)
	if err := d.MarkFailed(id, "error occurred"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusFailed {
		t.Errorf("expected status failed, got %s", a.Status)
	}
	if !a.Result.Valid || a.Result.String != "error occurred" {
		t.Errorf("expected result 'error occurred', got %v", a.Result)
	}
}

func TestListActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "task1", "{}", "")
	taskID2, _ := d.InsertTask(1, "task2", "{}", "")
	d.InsertAction("a", taskID1, "{}", db.ActionStatusPending, nil)
	d.InsertAction("b", taskID2, "{}", db.ActionStatusRunning, nil)
	d.InsertAction("c", taskID1, "{}", db.ActionStatusPending, nil)

	tests := []struct {
		name    string
		status  string
		taskID  *int64
		limit   int
		wantLen int
	}{
		{"no filter", "", nil, 0, 3},
		{"status filter", db.ActionStatusPending, nil, 0, 2},
		{"task filter", "", &taskID1, 0, 2},
		{"both filters", db.ActionStatusPending, &taskID1, 0, 2},
		{"limit", "", nil, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.ListActions(tt.status, tt.taskID, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.limit > 0 && len(got) >= 2 && got[0].ID < got[1].ID {
				t.Errorf("expected DESC order: first ID %d should be > second ID %d", got[0].ID, got[1].ID)
			}
		})
	}
}

func TestCountPendingByDispatch(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(d *db.DB)
		wantDispatchable int
		wantTotal        int
	}{
		{
			name: "some disabled",
			setup: func(d *db.DB) {
				task1, _ := d.InsertTask(1, "t1", "{}", "")
				d.InsertAction("a", task1, "{}", db.ActionStatusPending, nil)
				d.InsertAction("b", task1, "{}", db.ActionStatusPending, nil)
				task2, _ := d.InsertTask(2, "t2", "{}", "")
				d.InsertAction("c", task2, "{}", db.ActionStatusPending, nil)
				d.InsertAction("d", task2, "{}", db.ActionStatusPending, nil)
				d.InsertAction("e", task2, "{}", db.ActionStatusPending, nil)
				d.SetDispatchEnabled(2, false)
			},
			wantDispatchable: 2,
			wantTotal:        5,
		},
		{
			name: "all enabled",
			setup: func(d *db.DB) {
				task1, _ := d.InsertTask(1, "t1", "{}", "")
				d.InsertAction("a", task1, "{}", db.ActionStatusPending, nil)
			},
			wantDispatchable: 1,
			wantTotal:        1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			tt.setup(d)

			pc, err := d.CountPendingByDispatch()
			if err != nil {
				t.Fatal(err)
			}
			if pc.Dispatchable != tt.wantDispatchable {
				t.Errorf("Dispatchable = %d, want %d", pc.Dispatchable, tt.wantDispatchable)
			}
			if pc.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", pc.Total, tt.wantTotal)
			}
		})
	}
}

func TestListRunningInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")

	// running with session_id → should be returned
	d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil)
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1' WHERE id = 1")

	// running without session_id → should NOT be returned
	d.InsertAction("b", taskID, "{}", db.ActionStatusRunning, nil)

	// pending → should NOT be returned
	d.InsertAction("c", taskID, "{}", db.ActionStatusPending, nil)

	// done → should NOT be returned
	d.InsertAction("d", taskID, "{}", db.ActionStatusDone, nil)

	actions, err := d.ListRunningInteractive()
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Title != "a" {
		t.Errorf("expected title 'a', got %s", actions[0].Title)
	}
	if !actions[0].SessionID.Valid || actions[0].SessionID.String != "main" {
		t.Errorf("expected session_id 'main', got %v", actions[0].SessionID)
	}
}

func TestListRunningNonInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")

	d.InsertAction("ni-action", taskID, `{"instruction":"check","mode":"noninteractive"}`, db.ActionStatusRunning, nil)
	d.InsertAction("worktree-action", taskID, `{"instruction":"fix","claude_args":["--permission-mode","plan","--worktree"]}`, db.ActionStatusRunning, nil)
	d.InsertAction("interactive-action", taskID, `{}`, db.ActionStatusRunning, nil)
	d.InsertAction("pending-ni", taskID, `{"mode":"noninteractive"}`, db.ActionStatusPending, nil)

	actions, err := d.ListRunningNonInteractive()
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Title != "ni-action" {
		t.Errorf("expected title 'ni-action', got %s", actions[0].Title)
	}
}

func TestCountRunningInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil)
	d.InsertAction("b", taskID, "{}", db.ActionStatusRunning, nil)
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

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil)
	d.Exec("UPDATE actions SET session_id = 'sess-1', tmux_pane = 'tq-action-1' WHERE id = ?", id)

	if err := d.ResetToPending(id); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusPending {
		t.Errorf("status = %q, want pending", a.Status)
	}
	if a.StartedAt.Valid {
		t.Error("started_at should be NULL after reset")
	}
	if a.SessionID.Valid {
		t.Error("session_id should be NULL after reset")
	}
	if a.TmuxPane.Valid {
		t.Error("tmux_pane should be NULL after reset")
	}
}

func TestSetSessionInfo(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)

	if err := d.SetSessionInfo(id, "main", "tq-action-1"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if !a.SessionID.Valid || a.SessionID.String != "main" {
		t.Errorf("session_id = %v, want 'main'", a.SessionID)
	}
	if !a.TmuxPane.Valid || a.TmuxPane.String != "tq-action-1" {
		t.Errorf("tmux_pane = %v, want 'tq-action-1'", a.TmuxPane)
	}
}

func localDate(utcStr string) string {
	return db.FormatLocal(utcStr)[:10]
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
			date: localDate("2026-01-15 10:00:00"),
			want: true,
		},
		{
			name: "match by started_at",
			a: db.Action{
				CreatedAt: "2026-01-01 00:00:00",
				StartedAt: sql.NullString{String: "2026-02-20 09:00:00", Valid: true},
			},
			date: localDate("2026-02-20 09:00:00"),
			want: true,
		},
		{
			name: "match by completed_at",
			a: db.Action{
				CreatedAt:   "2026-01-01 00:00:00",
				CompletedAt: sql.NullString{String: "2026-03-01 18:00:00", Valid: true},
			},
			date: localDate("2026-03-01 18:00:00"),
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
			date: localDate("2026-01-15 10:00:00"),
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

func TestAction_MatchesDate_UTCLocalConversion(t *testing.T) {
	_, offset := time.Now().Zone()
	if offset == 0 {
		t.Skip("test only meaningful in non-UTC timezone")
	}

	// UTC timestamp near midnight; in a positive-offset timezone this is the next day
	utcStr := "2026-03-16 23:30:00"
	localDate := db.FormatLocal(utcStr)[:10]

	a := db.Action{CreatedAt: utcStr}
	if !a.MatchesDate(localDate) {
		t.Errorf("MatchesDate(%q) = false for CreatedAt=%q, expected true after local conversion", localDate, utcStr)
	}
}

func TestFilterForOpenTask(t *testing.T) {
	targetDate := localDate("2026-03-04 10:00:00")
	actions := []db.Action{
		{ID: 1, Status: db.ActionStatusPending, CreatedAt: "2026-01-01 00:00:00"},
		{ID: 2, Status: db.ActionStatusRunning, CreatedAt: "2026-01-01 00:00:00"},
		{ID: 3, Status: db.ActionStatusDispatched, CreatedAt: "2026-01-01 00:00:00"},
		{ID: 4, Status: db.ActionStatusDone, CreatedAt: "2026-01-01 00:00:00", CompletedAt: sql.NullString{String: "2026-03-04 10:00:00", Valid: true}},
		{ID: 5, Status: db.ActionStatusFailed, CreatedAt: "2026-01-01 00:00:00", CompletedAt: sql.NullString{String: "2026-01-01 12:00:00", Valid: true}},
		{ID: 6, Status: db.ActionStatusDone, CreatedAt: "2026-03-04 09:00:00"},
	}

	filtered := db.FilterForOpenTask(actions, targetDate)

	ids := make(map[int64]bool)
	for _, a := range filtered {
		ids[a.ID] = true
	}

	// pending/running/dispatched always included
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
		{ID: 1, Status: db.ActionStatusDone, CreatedAt: "2026-01-01 00:00:00"},
	}
	filtered := db.FilterForOpenTask(actions, "")
	if len(filtered) != 1 {
		t.Errorf("expected all actions returned for empty date, got %d", len(filtered))
	}
}

func TestClaimPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("first", taskID, "{}", db.ActionStatusPending, nil)
	d.InsertAction("second", taskID, "{}", db.ActionStatusPending, nil)

	ctx := context.Background()

	a, err := d.ClaimPending(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "second" {
		t.Errorf("expected second, got %s", a.Title)
	}
	if a.Status != db.ActionStatusRunning {
		t.Errorf("expected status running, got %s", a.Status)
	}

	fetched, _ := d.GetAction(2)
	if fetched.Status != db.ActionStatusRunning {
		t.Errorf("expected persisted status running, got %s", fetched.Status)
	}

	// first should still be pending
	first, _ := d.GetAction(1)
	if first.Status != db.ActionStatusPending {
		t.Errorf("expected first to remain pending, got %s", first.Status)
	}
}

func TestClaimPending_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	_, err := d.ClaimPending(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for non-existent action")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err)
	}
}

func TestClaimPending_NotPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")

	tests := []struct {
		name   string
		status string
	}{
		{"running", db.ActionStatusRunning},
		{"done", db.ActionStatusDone},
		{"failed", db.ActionStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction("test-"+tt.status, taskID, "{}", tt.status, nil)
			_, err := d.ClaimPending(context.Background(), id)
			if err == nil {
				t.Fatal("expected error for non-pending action")
			}
			if !strings.Contains(err.Error(), "not pending") {
				t.Errorf("error = %q, want to contain 'not pending'", err)
			}
			if !strings.Contains(err.Error(), tt.status) {
				t.Errorf("error = %q, want to contain status %q", err, tt.status)
			}
		})
	}
}

func TestListActionsByTaskIDs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "task1", "{}", "")
	taskID2, _ := d.InsertTask(1, "task2", "{}", "")
	taskID3, _ := d.InsertTask(1, "task3 no actions", "{}", "")

	d.InsertAction("a1", taskID1, "{}", db.ActionStatusPending, nil)
	d.InsertAction("a2", taskID1, "{}", db.ActionStatusDone, nil)
	d.InsertAction("b1", taskID2, "{}", db.ActionStatusRunning, nil)

	t.Run("multiple tasks", func(t *testing.T) {
		result, err := d.ListActionsByTaskIDs([]int64{taskID1, taskID2, taskID3})
		if err != nil {
			t.Fatal(err)
		}
		if len(result[taskID1]) != 2 {
			t.Errorf("task1 actions = %d, want 2", len(result[taskID1]))
		}
		if len(result[taskID2]) != 1 {
			t.Errorf("task2 actions = %d, want 1", len(result[taskID2]))
		}
		if len(result[taskID3]) != 0 {
			t.Errorf("task3 actions = %d, want 0", len(result[taskID3]))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result, err := d.ListActionsByTaskIDs([]int64{})
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})
}

func TestInsertAction_InvalidStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")

	tests := []struct {
		name   string
		status string
	}{
		{"open is invalid", "open"},
		{"arbitrary string", "bogus"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := d.InsertAction("test", taskID, "{}", tt.status, nil)
			if err == nil {
				t.Fatalf("expected error for invalid status %q, got nil", tt.status)
			}
			if !strings.Contains(err.Error(), "invalid action status") {
				t.Errorf("error = %q, want to contain 'invalid action status'", err)
			}
		})
	}
}

func TestMarkDispatched(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)

	if err := d.MarkDispatched(id); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusDispatched {
		t.Errorf("status = %q, want dispatched", a.Status)
	}
}

func TestMarkDispatched_NotRunning(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")

	tests := []struct {
		name   string
		status string
	}{
		{"pending", db.ActionStatusPending},
		{"done", db.ActionStatusDone},
		{"failed", db.ActionStatusFailed},
		{"cancelled", db.ActionStatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction("test-"+tt.status, taskID, "{}", tt.status, nil)
			err := d.MarkDispatched(id)
			if err == nil {
				t.Fatalf("expected error for status %q", tt.status)
			}
			if !strings.Contains(err.Error(), "not running") {
				t.Errorf("error = %q, want to contain 'not running'", err)
			}
		})
	}
}

func TestMarkDone_FromDispatched(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)
	d.MarkDispatched(id)

	if err := d.MarkDone(id, "pr merged"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusDone {
		t.Errorf("status = %q, want done", a.Status)
	}
}

func TestMarkCancelled_FromDispatched(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil)
	d.MarkDispatched(id)

	if err := d.MarkCancelled(id, "no longer needed"); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusCancelled {
		t.Errorf("status = %q, want cancelled", a.Status)
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

func TestInsertAction_Title(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	id, err := d.InsertAction("Review PR #123", taskID, `{"pr":123}`, db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "Review PR #123" {
		t.Errorf("title = %q, want %q", a.Title, "Review PR #123")
	}
}

func TestUpdateAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "task 1", "{}", "")
	taskID2, _ := d.InsertTask(1, "task 2", "{}", "")
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name         string
		initialTitle string
		initialMeta  string
		markFailed   bool
		title        *string
		taskID       *int64
		metadata     *string
		wantTitle    string
		wantTaskID   int64
		metaContains []string
	}{
		{
			name:         "update title",
			initialTitle: "original",
			initialMeta:  `{"k":"v"}`,
			title:        strPtr("updated title"),
			wantTitle:    "updated title",
			wantTaskID:   taskID1,
		},
		{
			name:         "update task_id",
			initialTitle: "test",
			initialMeta:  "{}",
			taskID:       &taskID2,
			wantTitle:    "test",
			wantTaskID:   taskID2,
		},
		{
			name:         "merge metadata",
			initialTitle: "test",
			initialMeta:  `{"existing":"value"}`,
			metadata:     strPtr(`{"new":"data"}`),
			wantTitle:    "test",
			wantTaskID:   taskID1,
			metaContains: []string{`"existing":"value"`, `"new":"data"`},
		},
		{
			name:         "update failed action",
			initialTitle: "test",
			initialMeta:  "{}",
			markFailed:   true,
			title:        strPtr("fixed"),
			wantTitle:    "fixed",
			wantTaskID:   taskID1,
		},
		{
			name:         "multiple fields",
			initialTitle: "test",
			initialMeta:  `{"a":"1"}`,
			title:      strPtr("new"),
			taskID:     &taskID2,
			metadata:   strPtr(`{"b":"2"}`),
			wantTitle:  "new",
			wantTaskID: taskID2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction(tt.initialTitle, taskID1, tt.initialMeta, db.ActionStatusPending, nil)
			if tt.markFailed {
				d.MarkFailed(id, "err")
			}
			if err := d.UpdateAction(id, tt.title, tt.taskID, tt.metadata); err != nil {
				t.Fatal(err)
			}
			a, _ := d.GetAction(id)
			if a.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", a.Title, tt.wantTitle)
			}
			if a.TaskID != tt.wantTaskID {
				t.Errorf("task_id = %d, want %d", a.TaskID, tt.wantTaskID)
			}
			for _, want := range tt.metaContains {
				if !strings.Contains(a.Metadata, want) {
					t.Errorf("metadata %s should contain %s", a.Metadata, want)
				}
			}
		})
	}
}

func TestUpdateAction_StatusRestriction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	tests := []struct {
		name   string
		setup  func(int64)
		status string
	}{
		{"running", func(id int64) { d.ClaimPending(context.Background(), id) }, "running"},
		{"done", func(id int64) { d.MarkDone(id, "ok") }, "done"},
		{"cancelled", func(id int64) { d.MarkCancelled(id, "no") }, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusPending, nil)
			tt.setup(id)
			title := "nope"
			err := d.UpdateAction(id, &title, nil, nil)
			if err == nil {
				t.Fatalf("expected error for %s action", tt.status)
			}
			if !strings.Contains(err.Error(), "only pending or failed") {
				t.Errorf("error = %q, want mention of status restriction", err.Error())
			}
		})
	}
}

func TestIsActionDispatchEnabled(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	actionID, _ := d.InsertAction("test", taskID, "{}", "pending", nil)

	tests := []struct {
		name     string
		setup    func()
		expected bool
	}{
		{
			name:     "focused project returns true",
			setup:    func() {},
			expected: true,
		},
		{
			name: "unfocused project returns false",
			setup: func() {
				if err := d.SetDispatchEnabled(1, false); err != nil {
					t.Fatal(err)
				}
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := d.IsActionDispatchEnabled(actionID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("IsActionDispatchEnabled(%d) = %v, want %v", actionID, got, tt.expected)
			}
		})
	}
}

func TestUpdateAction_NotFound(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	title := "nope"
	err := d.UpdateAction(999, &title, nil, nil)
	if err == nil {
		t.Fatal("expected error for non-existent action")
	}
}

func TestMarkTerminalSkipsTerminalState(t *testing.T) {
	tests := []struct {
		name       string
		initial    string
		markAs     string
		wantStatus string
	}{
		{"done_to_failed", db.ActionStatusDone, db.ActionStatusFailed, db.ActionStatusDone},
		{"failed_to_done", db.ActionStatusFailed, db.ActionStatusDone, db.ActionStatusFailed},
		{"cancelled_to_failed", db.ActionStatusCancelled, db.ActionStatusFailed, db.ActionStatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "test", "{}", "")
			id, err := d.InsertAction("act", taskID, "{}", db.ActionStatusRunning, nil)
			if err != nil {
				t.Fatal(err)
			}

			switch tt.initial {
			case db.ActionStatusDone:
				err = d.MarkDone(id, "original result")
			case db.ActionStatusFailed:
				err = d.MarkFailed(id, "original result")
			case db.ActionStatusCancelled:
				err = d.MarkCancelled(id, "original result")
			}
			if err != nil {
				t.Fatal(err)
			}

			switch tt.markAs {
			case db.ActionStatusDone:
				err = d.MarkDone(id, "overwrite attempt")
			case db.ActionStatusFailed:
				err = d.MarkFailed(id, "overwrite attempt")
			}
			if err != nil {
				t.Fatalf("markTerminal returned error: %v", err)
			}

			a, err := d.GetAction(id)
			if err != nil {
				t.Fatal(err)
			}
			if a.Status != tt.wantStatus {
				t.Errorf("status = %s, want %s", a.Status, tt.wantStatus)
			}
			if a.Result.String != "original result" {
				t.Errorf("result = %s, want 'original result'", a.Result.String)
			}
		})
	}
}
