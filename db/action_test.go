package db_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestInsertAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "", "{}")
	id, err := d.InsertAction("review-pr", &taskID, `{"pr":123}`, "pending", "auto")
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
	if a.PromptID != "review-pr" {
		t.Errorf("expected prompt_id 'review-pr', got %s", a.PromptID)
	}
}

func TestInsertAction_NilTaskID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, err := d.InsertAction("standalone", nil, "{}", "pending", "human")
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
	d.InsertAction("review-pr", &taskID, "{}", "pending", "auto")

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
	d.InsertAction("implement", &taskID, "{}", "waiting_human", "auto")

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
	d.InsertAction("implement", &taskID, "{}", "done", "auto")

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

	d.InsertAction("first", nil, "{}", "pending", "auto")
	d.InsertAction("second", nil, "{}", "pending", "auto")
	d.InsertAction("third", nil, "{}", "pending", "auto")

	ctx := context.Background()

	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected action, got nil")
	}
	if a.PromptID != "first" {
		t.Errorf("expected first (lowest ID), got %s", a.PromptID)
	}
	if a.Status != "running" {
		t.Errorf("expected status running, got %s", a.Status)
	}

	// Verify it's persisted as running
	fetched, _ := d.GetAction(a.ID)
	if fetched.Status != "running" {
		t.Errorf("expected persisted status running, got %s", fetched.Status)
	}

	// Next should return second (next lowest ID)
	a2, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a2.PromptID != "second" {
		t.Errorf("expected second, got %s", a2.PromptID)
	}
}

func TestNextPending_SkipsDisabledProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Disable project 1 (immedio)
	d.SetDispatchEnabled(1, false)

	taskID, _ := d.InsertTask(1, "disabled task", "", "{}")
	d.InsertAction("disabled-action", &taskID, "{}", "pending", "auto")

	taskID2, _ := d.InsertTask(2, "enabled task", "", "{}")
	d.InsertAction("enabled-action", &taskID2, "{}", "pending", "auto")

	ctx := context.Background()
	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected action, got nil")
	}
	if a.PromptID != "enabled-action" {
		t.Errorf("expected enabled-action, got %s", a.PromptID)
	}
}

func TestNextPending_IncludesNullTaskID(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Disable all projects
	d.SetAllDispatchEnabled(false)

	// Action with no task (task_id IS NULL) should still be dispatched
	d.InsertAction("standalone", nil, "{}", "pending", "auto")

	ctx := context.Background()
	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected action, got nil")
	}
	if a.PromptID != "standalone" {
		t.Errorf("expected standalone, got %s", a.PromptID)
	}
}

func TestNextPending_AllDisabled(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Disable all projects
	d.SetAllDispatchEnabled(false)

	taskID, _ := d.InsertTask(1, "disabled task", "", "{}")
	d.InsertAction("disabled-action", &taskID, "{}", "pending", "auto")

	ctx := context.Background()
	a, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if a != nil {
		t.Errorf("expected nil when all projects disabled, got action %s", a.PromptID)
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

	id, _ := d.InsertAction("test", nil, "{}", "running", "auto")
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

	id, _ := d.InsertAction("test", nil, "{}", "running", "auto")
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

	id, _ := d.InsertAction("test", nil, "{}", "running", "auto")
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
	d.InsertAction("a", &taskID1, "{}", "pending", "auto")
	d.InsertAction("b", &taskID2, "{}", "running", "auto")
	d.InsertAction("c", nil, "{}", "pending", "human")

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
	d.InsertAction("a", &taskID, "{}", "pending", "auto")
	d.InsertAction("b", &taskID, "{}", "pending", "auto")
	d.InsertAction("c", &taskID, "{}", "running", "auto")
	d.InsertAction("d", &taskID, "{}", "done", "auto")

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

func TestListRunningInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// running with session_id → should be returned
	d.InsertAction("a", nil, "{}", "running", "auto")
	d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1' WHERE id = 1")

	// running without session_id → should NOT be returned
	d.InsertAction("b", nil, "{}", "running", "auto")

	// pending → should NOT be returned
	d.InsertAction("c", nil, "{}", "pending", "auto")

	// done → should NOT be returned
	d.InsertAction("d", nil, "{}", "done", "auto")

	actions, err := d.ListRunningInteractive()
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].PromptID != "a" {
		t.Errorf("expected prompt_id 'a', got %s", actions[0].PromptID)
	}
	if !actions[0].SessionID.Valid || actions[0].SessionID.String != "main" {
		t.Errorf("expected session_id 'main', got %v", actions[0].SessionID)
	}
}

func TestCountRunningInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertAction("a", nil, "{}", "running", "auto")
	d.InsertAction("b", nil, "{}", "running", "auto")
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

	id, _ := d.InsertAction("a", nil, "{}", "running", "auto")
	d.Exec("UPDATE actions SET session_id = 'sess-1', tmux_pane = 'tq-action-1' WHERE id = ?", id)

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

	id, _ := d.InsertAction("test", nil, "{}", "running", "auto")

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

func TestClaimPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertAction("first", nil, "{}", "pending", "auto")
	d.InsertAction("second", nil, "{}", "pending", "auto")

	ctx := context.Background()

	a, err := d.ClaimPending(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if a.PromptID != "second" {
		t.Errorf("expected second, got %s", a.PromptID)
	}
	if a.Status != "running" {
		t.Errorf("expected status running, got %s", a.Status)
	}

	fetched, _ := d.GetAction(2)
	if fetched.Status != "running" {
		t.Errorf("expected persisted status running, got %s", fetched.Status)
	}

	// first should still be pending
	first, _ := d.GetAction(1)
	if first.Status != "pending" {
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

	tests := []struct {
		name   string
		status string
	}{
		{"running", "running"},
		{"done", "done"},
		{"failed", "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction("test-"+tt.status, nil, "{}", tt.status, "auto")
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

func TestUpdateActionStatus(t *testing.T) {
	tests := []struct {
		name    string
		to      string
		wantErr bool
	}{
		{"to pending", "pending", false},
		{"to running", "running", false},
		{"to failed", "failed", false},
		{"to waiting_human", "waiting_human", false},
		{"to done rejected", "done", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			id, _ := d.InsertAction("test", nil, "{}", "running", "auto")
			err := d.UpdateActionStatus(id, tc.to)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			a, _ := d.GetAction(id)
			if a.Status != tc.to {
				t.Errorf("status = %q, want %q", a.Status, tc.to)
			}

			switch tc.to {
			case "pending":
				if a.StartedAt.Valid {
					t.Error("started_at should be NULL after setting to pending")
				}
				if a.CompletedAt.Valid {
					t.Error("completed_at should be NULL after setting to pending")
				}
			case "running":
				if !a.StartedAt.Valid {
					t.Error("started_at should be set for running")
				}
			case "failed":
				if !a.CompletedAt.Valid {
					t.Error("completed_at should be set for failed")
				}
			}
		})
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
