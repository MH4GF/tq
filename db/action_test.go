package db_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestInsertAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	id, err := d.InsertAction("review-pr", taskID, `{"pr":123}`, db.ActionStatusPending, nil, "")
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

func TestInsertAction_WithWorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")

	tests := []struct {
		name    string
		workDir string
		want    string
	}{
		{name: "empty work_dir", workDir: "", want: ""},
		{name: "absolute path", workDir: "/tmp/some/worktree", want: "/tmp/some/worktree"},
		{name: "tilde path", workDir: "~/projects/tq", want: "~/projects/tq"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := d.InsertAction("t", taskID, "{}", db.ActionStatusPending, nil, tt.workDir)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := d.GetAction(id)
			if a.WorkDir != tt.want {
				t.Errorf("work_dir = %q, want %q", a.WorkDir, tt.want)
			}
		})
	}
}

func TestUpdateAction_WorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name        string
		initial     string
		updateValue *string
		want        string
	}{
		{
			name:        "set work_dir on action with empty initial",
			initial:     "",
			updateValue: strPtr("/tmp/wt"),
			want:        "/tmp/wt",
		},
		{
			name:        "overwrite existing work_dir",
			initial:     "/tmp/old",
			updateValue: strPtr("/tmp/new"),
			want:        "/tmp/new",
		},
		{
			name:        "clear work_dir with empty string",
			initial:     "/tmp/old",
			updateValue: strPtr(""),
			want:        "",
		},
		{
			name:        "nil pointer keeps existing value",
			initial:     "/tmp/keep",
			updateValue: nil,
			want:        "/tmp/keep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction("t", taskID, "{}", db.ActionStatusPending, nil, tt.initial)
			// UpdateAction requires at least one non-nil field; pass a noop title when
			// only verifying nil-keeps-value semantics.
			var title *string
			if tt.updateValue == nil {
				v := "t"
				title = &v
			}
			if err := d.UpdateAction(id, title, nil, nil, tt.updateValue, nil); err != nil {
				t.Fatal(err)
			}
			a, _ := d.GetAction(id)
			if a.WorkDir != tt.want {
				t.Errorf("work_dir = %q, want %q", a.WorkDir, tt.want)
			}
		})
	}
}

func TestBulkInsertActions_WithWorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "test task", "{}", "")

	specs := []db.ActionInsertSpec{
		{Title: "a", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending, WorkDir: "/tmp/a"},
		{Title: "b", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending, WorkDir: ""},
		{Title: "c", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending, WorkDir: "~/c"},
	}
	ids, err := d.BulkInsertActions(specs)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		a, _ := d.GetAction(id)
		if a.WorkDir != specs[i].WorkDir {
			t.Errorf("specs[%d] work_dir = %q, want %q", i, a.WorkDir, specs[i].WorkDir)
		}
	}
}

func TestNextPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	d.InsertAction("first", taskID, "{}", db.ActionStatusPending, nil, "")
	d.InsertAction("second", taskID, "{}", db.ActionStatusPending, nil, "")
	d.InsertAction("third", taskID, "{}", db.ActionStatusPending, nil, "")

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
	id, err := d.InsertAction("scheduled", taskID, "{}", db.ActionStatusPending, &after, "")
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
	d.InsertAction("future", taskID, "{}", db.ActionStatusPending, &future, "")

	// Action with past dispatch_after should be picked up
	past := "2000-01-01 00:00:00"
	d.InsertAction("past", taskID, "{}", db.ActionStatusPending, &past, "")

	// Action with no dispatch_after should be picked up (immediate)
	d.InsertAction("immediate", taskID, "{}", db.ActionStatusPending, nil, "")

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
	d.InsertAction("now", taskID, "{}", db.ActionStatusPending, nil, "")

	// Future action (should not be counted)
	future := "2099-12-31 23:59:59"
	d.InsertAction("later", taskID, "{}", db.ActionStatusPending, &future, "")

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
	d.InsertAction("disabled-action", taskID, "{}", db.ActionStatusPending, nil, "")

	taskID2, _ := d.InsertTask(2, "enabled task", "{}", "")
	d.InsertAction("enabled-action", taskID2, "{}", db.ActionStatusPending, nil, "")

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
	projects, err := d.ListProjects(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if err := d.SetDispatchEnabled(p.ID, false); err != nil {
			t.Fatal(err)
		}
	}

	taskID, _ := d.InsertTask(1, "disabled task", "{}", "")
	d.InsertAction("disabled-action", taskID, "{}", db.ActionStatusPending, nil, "")

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
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil, "")
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
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil, "")
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
	d.InsertAction("a", taskID1, "{}", db.ActionStatusPending, nil, "")
	d.InsertAction("b", taskID2, "{}", db.ActionStatusRunning, nil, "")
	d.InsertAction("c", taskID1, "{}", db.ActionStatusPending, nil, "")

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
				d.InsertAction("a", task1, "{}", db.ActionStatusPending, nil, "")
				d.InsertAction("b", task1, "{}", db.ActionStatusPending, nil, "")
				task2, _ := d.InsertTask(2, "t2", "{}", "")
				d.InsertAction("c", task2, "{}", db.ActionStatusPending, nil, "")
				d.InsertAction("d", task2, "{}", db.ActionStatusPending, nil, "")
				d.InsertAction("e", task2, "{}", db.ActionStatusPending, nil, "")
				d.SetDispatchEnabled(2, false)
			},
			wantDispatchable: 2,
			wantTotal:        5,
		},
		{
			name: "all enabled",
			setup: func(d *db.DB) {
				task1, _ := d.InsertTask(1, "t1", "{}", "")
				d.InsertAction("a", task1, "{}", db.ActionStatusPending, nil, "")
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

func TestCountRunningInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("with-session", taskID, "{}", db.ActionStatusRunning, nil, "")
	d.InsertAction("default-no-session", taskID, "{}", db.ActionStatusRunning, nil, "")
	d.InsertAction("explicit-interactive", taskID, `{"mode":"interactive"}`, db.ActionStatusRunning, nil, "")
	d.InsertAction("noninteractive", taskID, `{"mode":"noninteractive"}`, db.ActionStatusRunning, nil, "")
	d.InsertAction("remote", taskID, `{"mode":"remote"}`, db.ActionStatusRunning, nil, "")
	d.InsertAction("pending", taskID, "{}", db.ActionStatusPending, nil, "")
	d.Exec("UPDATE actions SET tmux_session = 'sess-1' WHERE id = 1")

	count, err := d.CountRunningInteractive()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3 (with-session + default-no-session + explicit-interactive)", count)
	}
}

func TestCountRunningNonInteractive(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("ni-1", taskID, `{"mode":"noninteractive"}`, db.ActionStatusRunning, nil, "")
	d.InsertAction("ni-2", taskID, `{"mode":"noninteractive"}`, db.ActionStatusRunning, nil, "")
	d.InsertAction("interactive", taskID, `{"mode":"interactive"}`, db.ActionStatusRunning, nil, "")
	d.InsertAction("default-mode", taskID, "{}", db.ActionStatusRunning, nil, "")
	d.InsertAction("remote", taskID, `{"mode":"remote"}`, db.ActionStatusRunning, nil, "")
	d.InsertAction("ni-pending", taskID, `{"mode":"noninteractive"}`, db.ActionStatusPending, nil, "")
	d.InsertAction("ni-done", taskID, `{"mode":"noninteractive"}`, db.ActionStatusDone, nil, "")

	count, err := d.CountRunningNonInteractive()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (ni-1 + ni-2)", count)
	}
}

func TestResetToPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil, "")
	d.Exec("UPDATE actions SET tmux_session = 'sess-1', tmux_window = 'tq-action-1', dispatch_after = datetime('now', '+30 seconds') WHERE id = ?", id)

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
	if a.TmuxSession.Valid {
		t.Error("tmux_session should be NULL after reset")
	}
	if a.TmuxWindow.Valid {
		t.Error("tmux_window should be NULL after reset")
	}
	if a.DispatchAfter.Valid {
		t.Errorf("dispatch_after should be NULL after reset, got %q", a.DispatchAfter.String)
	}

	events, err := d.ListEvents("action", id)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no events emitted")
	}
	last := events[len(events)-1]
	if last.EventType != "action.status_changed" {
		t.Errorf("last event_type = %q, want action.status_changed", last.EventType)
	}
	if strings.Contains(last.Payload, `"dispatch_after"`) {
		t.Errorf("payload should not carry dispatch_after on reset, got %s", last.Payload)
	}
}

func TestDeferToPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil, "")
	d.Exec("UPDATE actions SET tmux_session = 'sess-1', tmux_window = 'tq-action-1' WHERE id = ?", id)

	before := time.Now().UTC()
	if err := d.DeferToPending(id, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	a, _ := d.GetAction(id)
	if a.Status != db.ActionStatusPending {
		t.Errorf("status = %q, want pending", a.Status)
	}
	if a.StartedAt.Valid {
		t.Error("started_at should be NULL after defer")
	}
	if a.TmuxSession.Valid || a.TmuxWindow.Valid {
		t.Error("tmux fields should be NULL after defer")
	}
	if !a.DispatchAfter.Valid {
		t.Fatal("dispatch_after should be set after defer")
	}
	got, err := time.Parse(db.TimeLayout, a.DispatchAfter.String)
	if err != nil {
		t.Fatalf("parse dispatch_after: %v", err)
	}
	expected := before.Add(30 * time.Second)
	delta := got.Sub(expected)
	if delta < -2*time.Second || delta > 2*time.Second {
		t.Errorf("dispatch_after = %v, want ~%v (delta %v)", got, expected, delta)
	}

	events, _ := d.ListEvents("action", id)
	last := events[len(events)-1]
	if last.EventType != "action.status_changed" {
		t.Errorf("last event_type = %q, want action.status_changed", last.EventType)
	}
	if !strings.Contains(last.Payload, `"from":"running"`) || !strings.Contains(last.Payload, `"to":"pending"`) {
		t.Errorf("payload missing from/to: %s", last.Payload)
	}
	if !strings.Contains(last.Payload, `"dispatch_after":`) {
		t.Errorf("payload missing dispatch_after: %s", last.Payload)
	}
}

func TestDeferToPending_RejectsNonRunning(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusPending, nil, "")

	err := d.DeferToPending(id, 10*time.Second)
	if err == nil {
		t.Fatal("expected error for non-running action, got nil")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("error = %v, want substring 'not running'", err)
	}

	a, _ := d.GetAction(id)
	if a.DispatchAfter.Valid {
		t.Errorf("dispatch_after should remain NULL on rejected defer, got %q", a.DispatchAfter.String)
	}
}

func TestDeferToPending_LastWriteWins(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil, "")

	if err := d.DeferToPending(id, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	a1, _ := d.GetAction(id)
	first := a1.DispatchAfter.String

	if err := d.SetActionStatusForTest(id, db.ActionStatusRunning); err != nil {
		t.Fatal(err)
	}

	if err := d.DeferToPending(id, 90*time.Second); err != nil {
		t.Fatal(err)
	}
	a2, _ := d.GetAction(id)
	second := a2.DispatchAfter.String

	if first == second {
		t.Errorf("dispatch_after should differ between defers: first=%q second=%q", first, second)
	}
	gotFirst, _ := time.Parse(db.TimeLayout, first)
	gotSecond, _ := time.Parse(db.TimeLayout, second)
	if !gotSecond.After(gotFirst) {
		t.Errorf("second dispatch_after (%v) should be after first (%v)", gotSecond, gotFirst)
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

func TestClaimPending(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertAction("first", taskID, "{}", db.ActionStatusPending, nil, "")
	d.InsertAction("second", taskID, "{}", db.ActionStatusPending, nil, "")

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
			id, _ := d.InsertAction("test-"+tt.status, taskID, "{}", tt.status, nil, "")
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

func TestListActionsByTaskIDsForView(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "task1", "{}", "")
	otherTaskID, _ := d.InsertTask(1, "task2", "{}", "")

	d.InsertAction("pending", taskID, "{}", db.ActionStatusPending, nil, "")
	d.InsertAction("running", taskID, "{}", db.ActionStatusRunning, nil, "")
	d.InsertAction("dispatched", taskID, "{}", db.ActionStatusDispatched, nil, "")

	doneTodayID, _ := d.InsertAction("done-today", taskID, "{}", db.ActionStatusRunning, nil, "")
	if err := d.MarkDone(doneTodayID, ""); err != nil {
		t.Fatal(err)
	}
	failedTodayID, _ := d.InsertAction("failed-today", taskID, "{}", db.ActionStatusRunning, nil, "")
	if err := d.MarkFailed(failedTodayID, ""); err != nil {
		t.Fatal(err)
	}

	d.InsertAction("other-pending", otherTaskID, "{}", db.ActionStatusPending, nil, "")

	// Manually set timestamps for the "old" terminal action to be 10 days ago.
	doneOldID, _ := d.InsertAction("done-old", taskID, "{}", db.ActionStatusRunning, nil, "")
	if err := d.MarkDone(doneOldID, ""); err != nil {
		t.Fatal(err)
	}
	oldUTC := time.Now().UTC().Add(-10 * 24 * time.Hour)
	if err := d.SetActionTimestampsForTest(doneOldID, &oldUTC, &oldUTC); err != nil {
		t.Fatal(err)
	}

	// Today as local date string (the SQL filter uses 'localtime').
	today := time.Now().Format("2006-01-02")

	titleSet := func(actions []db.Action) map[string]bool {
		s := make(map[string]bool, len(actions))
		for _, a := range actions {
			s[a.Title] = true
		}
		return s
	}

	t.Run("empty filter returns all actions including terminals", func(t *testing.T) {
		got, err := d.ListActionsByTaskIDsForView([]int64{taskID, otherTaskID}, "")
		if err != nil {
			t.Fatal(err)
		}
		titles := titleSet(got[taskID])
		for _, want := range []string{"pending", "running", "dispatched", "done-today", "failed-today", "done-old"} {
			if !titles[want] {
				t.Errorf("expected title %q in task1 actions, got %v", want, titles)
			}
		}
		if !titleSet(got[otherTaskID])["other-pending"] {
			t.Errorf("expected other-pending in task2 actions, got %v", got[otherTaskID])
		}
	})

	t.Run("date filter excludes old terminal actions", func(t *testing.T) {
		got, err := d.ListActionsByTaskIDsForView([]int64{taskID, otherTaskID}, today)
		if err != nil {
			t.Fatal(err)
		}
		titles := titleSet(got[taskID])
		// non-terminals always returned regardless of date
		for _, want := range []string{"pending", "running", "dispatched"} {
			if !titles[want] {
				t.Errorf("non-terminal %q should always be returned, got %v", want, titles)
			}
		}
		// today's terminals returned
		for _, want := range []string{"done-today", "failed-today"} {
			if !titles[want] {
				t.Errorf("today's terminal %q should be returned, got %v", want, titles)
			}
		}
		// old terminal excluded
		if titles["done-old"] {
			t.Errorf("old terminal action should be excluded from date-filtered view, got %v", titles)
		}
		// other task's pending still returned
		if !titleSet(got[otherTaskID])["other-pending"] {
			t.Errorf("other task pending should be returned, got %v", got[otherTaskID])
		}
	})

	t.Run("date filter matches by created_at when other dates miss", func(t *testing.T) {
		// done-created-today: created today, started/completed in the past
		id, _ := d.InsertAction("done-created-today", taskID, "{}", db.ActionStatusRunning, nil, "")
		if err := d.MarkDone(id, ""); err != nil {
			t.Fatal(err)
		}
		past := time.Now().UTC().Add(-30 * 24 * time.Hour)
		// SetActionTimestampsForTest takes (createdAt, completedAt). Pass nil for createdAt
		// to keep "today" and rewrite completed_at only.
		if err := d.SetActionTimestampsForTest(id, nil, &past); err != nil {
			t.Fatal(err)
		}

		got, err := d.ListActionsByTaskIDsForView([]int64{taskID}, today)
		if err != nil {
			t.Fatal(err)
		}
		if !titleSet(got[taskID])["done-created-today"] {
			t.Errorf("action with created_at=today should be matched, got %v", titleSet(got[taskID]))
		}
	})

	t.Run("empty taskIDs returns empty map", func(t *testing.T) {
		got, err := d.ListActionsByTaskIDsForView(nil, today)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})
}

func TestListActionsByTaskIDs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "task1", "{}", "")
	taskID2, _ := d.InsertTask(1, "task2", "{}", "")
	taskID3, _ := d.InsertTask(1, "task3 no actions", "{}", "")

	d.InsertAction("a1", taskID1, "{}", db.ActionStatusPending, nil, "")
	d.InsertAction("a2", taskID1, "{}", db.ActionStatusDone, nil, "")
	d.InsertAction("b1", taskID2, "{}", db.ActionStatusRunning, nil, "")

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
			_, err := d.InsertAction("test", taskID, "{}", tt.status, nil, "")
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
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil, "")

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
			id, _ := d.InsertAction("test-"+tt.status, taskID, "{}", tt.status, nil, "")
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
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil, "")
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
	id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusRunning, nil, "")
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
	id, err := d.InsertAction("Review PR #123", taskID, `{"pr":123}`, db.ActionStatusPending, nil, "")
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
			title:        strPtr("new"),
			taskID:       &taskID2,
			metadata:     strPtr(`{"b":"2"}`),
			wantTitle:    "new",
			wantTaskID:   taskID2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction(tt.initialTitle, taskID1, tt.initialMeta, db.ActionStatusPending, nil, "")
			if tt.markFailed {
				d.MarkFailed(id, "err")
			}
			if err := d.UpdateAction(id, tt.title, tt.taskID, tt.metadata, nil, nil); err != nil {
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
			id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusPending, nil, "")
			tt.setup(id)
			title := "nope"
			err := d.UpdateAction(id, &title, nil, nil, nil, nil)
			if err == nil {
				t.Fatalf("expected error for %s action", tt.status)
			}
			if !strings.Contains(err.Error(), "title/task/work-dir can only be updated on pending or failed") {
				t.Errorf("error = %q, want mention of status restriction", err.Error())
			}
		})
	}
}

func TestUpdateAction_Result(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	tests := []struct {
		name      string
		setup     func(int64)
		wantError bool
	}{
		{"pending", func(int64) {}, false},
		{"failed", func(id int64) { d.MarkFailed(id, "old") }, false},
		{"done", func(id int64) { d.MarkDone(id, "old") }, false},
		{"cancelled", func(id int64) { d.MarkCancelled(id, "old") }, false},
		{"running", func(id int64) { d.ClaimPending(context.Background(), id) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction("test", taskID, "{}", db.ActionStatusPending, nil, "")
			tt.setup(id)

			amended := "outcome: recovered"
			err := d.UpdateAction(id, nil, nil, nil, nil, &amended)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error amending result on %s action", tt.name)
				}
				if !strings.Contains(err.Error(), "result can only be amended") {
					t.Errorf("error = %q, want mention of result amend restriction", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			a, _ := d.GetAction(id)
			if !a.Result.Valid || a.Result.String != amended {
				t.Errorf("result = %v, want %q", a.Result, amended)
			}
		})
	}
}

func TestUpdateAction_Metadata(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	tests := []struct {
		name      string
		setup     func(int64)
		wantError bool
	}{
		{"pending", func(int64) {}, false},
		{"failed", func(id int64) { d.MarkFailed(id, "old") }, false},
		{"done", func(id int64) { d.MarkDone(id, "old") }, false},
		{"cancelled", func(id int64) { d.MarkCancelled(id, "old") }, false},
		{"running", func(id int64) { d.ClaimPending(context.Background(), id) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, _ := d.InsertAction("test", taskID, `{"existing":"keep"}`, db.ActionStatusPending, nil, "")
			tt.setup(id)

			newMeta := `{"claude_session_id":"01a2b3ed-bc16-4577-8ad5-e0ee40f1f39c"}`
			err := d.UpdateAction(id, nil, nil, &newMeta, nil, nil)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error amending metadata on %s action", tt.name)
				}
				if !strings.Contains(err.Error(), "metadata can only be amended") {
					t.Errorf("error = %q, want mention of metadata amend restriction", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			a, _ := d.GetAction(id)
			if !strings.Contains(a.Metadata, `"claude_session_id":"01a2b3ed-bc16-4577-8ad5-e0ee40f1f39c"`) {
				t.Errorf("metadata missing claude_session_id: %s", a.Metadata)
			}
			if !strings.Contains(a.Metadata, `"existing":"keep"`) {
				t.Errorf("merge clobbered existing key: %s", a.Metadata)
			}
		})
	}
}

func TestIsActionDispatchEnabled(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test task", "{}", "")
	actionID, _ := d.InsertAction("test", taskID, "{}", "pending", nil, "")

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
	err := d.UpdateAction(999, &title, nil, nil, nil, nil)
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
			id, err := d.InsertAction("act", taskID, "{}", db.ActionStatusRunning, nil, "")
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

			events, err := d.ListEvents("action", id)
			if err != nil {
				t.Fatal(err)
			}
			var statusChanges int
			for _, e := range events {
				if e.EventType == "action.status_changed" {
					statusChanges++
				}
			}
			if statusChanges != 1 {
				t.Errorf("status_changed events = %d, want 1 (only the initial transition)", statusChanges)
			}
		})
	}
}

func TestMarkTerminalNoEventWhenAlreadyTerminal(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id, err := d.InsertAction("act", taskID, "{}", db.ActionStatusRunning, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.MarkDone(id, "first"); err != nil {
		t.Fatal(err)
	}

	before, err := d.ListEvents("action", id)
	if err != nil {
		t.Fatal(err)
	}

	if err := d.MarkFailed(id, "second"); err != nil {
		t.Fatalf("MarkFailed on terminal action returned error: %v", err)
	}
	if err := d.MarkCancelled(id, "third"); err != nil {
		t.Fatalf("MarkCancelled on terminal action returned error: %v", err)
	}
	if err := d.MarkDone(id, "fourth"); err != nil {
		t.Fatalf("MarkDone on terminal action returned error: %v", err)
	}

	after, err := d.ListEvents("action", id)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("event count after no-op marks = %d, want %d (no new events)", len(after), len(before))
	}
}

func TestBulkInsertActions(t *testing.T) {
	t.Run("empty input is no-op", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		ids, err := d.BulkInsertActions(nil)
		if err != nil {
			t.Errorf("nil specs returned err: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("len(ids) = %d, want 0", len(ids))
		}
	})

	t.Run("normal: 3 actions inserted with returned IDs in input order", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")

		specs := []db.ActionInsertSpec{
			{Title: "first", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending},
			{Title: "second", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending},
			{Title: "third", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending},
		}
		ids, err := d.BulkInsertActions(specs)
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 3 {
			t.Fatalf("len(ids) = %d, want 3", len(ids))
		}
		for i, id := range ids {
			a, err := d.GetAction(id)
			if err != nil {
				t.Fatalf("GetAction(%d): %v", id, err)
			}
			if a.Title != specs[i].Title {
				t.Errorf("ids[%d] title = %q, want %q (RETURNING did not preserve input order)", i, a.Title, specs[i].Title)
			}
		}
	})

	t.Run("invalid status fails fast (no rows inserted)", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")

		specs := []db.ActionInsertSpec{
			{Title: "ok", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending},
			{Title: "bogus", TaskID: taskID, Metadata: "{}", Status: "garbage"},
		}
		ids, err := d.BulkInsertActions(specs)
		if err == nil {
			t.Fatal("expected error for invalid status")
		}
		if ids != nil {
			t.Errorf("ids = %v, want nil on error", ids)
		}
		actions, _ := d.ListActions("", nil, 0)
		if len(actions) != 0 {
			t.Errorf("expected 0 inserted actions on validation failure, got %d", len(actions))
		}
	})

	t.Run("FK violation rolls back entire batch", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")

		specs := []db.ActionInsertSpec{
			{Title: "good", TaskID: taskID, Metadata: "{}", Status: db.ActionStatusPending},
			{Title: "bad", TaskID: 99999, Metadata: "{}", Status: db.ActionStatusPending},
		}
		_, err := d.BulkInsertActions(specs)
		if err == nil {
			t.Fatal("expected FK violation error")
		}
		actions, _ := d.ListActions("", nil, 0)
		if len(actions) != 0 {
			t.Errorf("expected 0 actions after rollback, got %d (atomicity broken)", len(actions))
		}
	})
}

func TestBulkMarkFailed(t *testing.T) {
	t.Run("empty input is no-op", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		if err := d.BulkMarkFailed(nil); err != nil {
			t.Errorf("nil updates returned err: %v", err)
		}
	})

	t.Run("normal: 3 running actions marked failed", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		var ids []int64
		for range 3 {
			id, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil, "")
			ids = append(ids, id)
		}

		updates := []db.ActionFailureUpdate{
			{ID: ids[0], Reason: "stale-1"},
			{ID: ids[1], Reason: "stale-2"},
			{ID: ids[2], Reason: "stale-3"},
		}
		if err := d.BulkMarkFailed(updates); err != nil {
			t.Fatal(err)
		}
		for i, id := range ids {
			a, _ := d.GetAction(id)
			if a.Status != db.ActionStatusFailed {
				t.Errorf("action %d status = %q, want failed", id, a.Status)
			}
			if !a.Result.Valid || a.Result.String != updates[i].Reason {
				t.Errorf("action %d result = %v, want %q", id, a.Result, updates[i].Reason)
			}
		}
	})

	t.Run("already-terminal actions skipped without error", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		doneID, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil, "")
		_ = d.MarkDone(doneID, "ok")
		runningID, _ := d.InsertAction("b", taskID, "{}", db.ActionStatusRunning, nil, "")

		err := d.BulkMarkFailed([]db.ActionFailureUpdate{
			{ID: doneID, Reason: "should be skipped"},
			{ID: runningID, Reason: "real failure"},
		})
		if err != nil {
			t.Fatal(err)
		}

		stillDone, _ := d.GetAction(doneID)
		if stillDone.Status != db.ActionStatusDone {
			t.Errorf("done action status = %q, want unchanged %q", stillDone.Status, db.ActionStatusDone)
		}
		nowFailed, _ := d.GetAction(runningID)
		if nowFailed.Status != db.ActionStatusFailed {
			t.Errorf("running action status = %q, want failed", nowFailed.Status)
		}
	})

	t.Run("emitted event payload carries failure reason", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		var ids []int64
		for range 2 {
			id, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil, "")
			ids = append(ids, id)
		}

		updates := []db.ActionFailureUpdate{
			{ID: ids[0], Reason: "boom-0"},
			{ID: ids[1], Reason: "boom-1"},
		}
		if err := d.BulkMarkFailed(updates); err != nil {
			t.Fatal(err)
		}

		for i, id := range ids {
			events, err := d.ListEvents("action", id)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) == 0 {
				t.Fatalf("action %d: no events emitted", id)
			}
			last := events[len(events)-1]
			if last.EventType != "action.status_changed" {
				t.Errorf("action %d: last event_type = %q, want action.status_changed", id, last.EventType)
			}
			if !strings.Contains(last.Payload, `"from":"running"`) || !strings.Contains(last.Payload, `"to":"failed"`) {
				t.Errorf("action %d: payload missing from/to: %s", id, last.Payload)
			}
			wantResult := `"result":"` + updates[i].Reason + `"`
			if !strings.Contains(last.Payload, wantResult) {
				t.Errorf("action %d: payload missing failure reason %s: %s", id, wantResult, last.Payload)
			}
		}
	})

	t.Run("missing ID rolls back entire batch", func(t *testing.T) {
		d := testutil.NewTestDB(t)
		testutil.SeedTestProjects(t, d)
		taskID, _ := d.InsertTask(1, "test", "{}", "")
		runningID, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusRunning, nil, "")

		err := d.BulkMarkFailed([]db.ActionFailureUpdate{
			{ID: runningID, Reason: "real"},
			{ID: 99999, Reason: "ghost"},
		})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		a, _ := d.GetAction(runningID)
		if a.Status != db.ActionStatusRunning {
			t.Errorf("real action status = %q, want unchanged %q (atomicity broken)", a.Status, db.ActionStatusRunning)
		}
	})
}

func TestMergeActionMetadata_ConcurrentMerges(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, err := d.InsertTask(1, "task", "{}", "")
	if err != nil {
		t.Fatal(err)
	}
	id, err := d.InsertAction("test", taskID, "{}", db.ActionStatusPending, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	const n = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			key := fmt.Sprintf("k%d", idx)
			if err := d.MergeActionMetadata(id, map[string]any{key: idx}); err != nil {
				errs <- err
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("MergeActionMetadata returned error: %v", err)
	}

	a, err := d.GetAction(id)
	if err != nil {
		t.Fatal(err)
	}
	merged := map[string]any{}
	if err := json.Unmarshal([]byte(a.Metadata), &merged); err != nil {
		t.Fatalf("parse final metadata %s: %v", a.Metadata, err)
	}
	if len(merged) != n {
		t.Fatalf("expected %d keys, got %d: %s", n, len(merged), a.Metadata)
	}
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("k%d", i)
		if _, ok := merged[key]; !ok {
			t.Errorf("missing key %q in merged metadata: %s", key, a.Metadata)
		}
	}
}

func TestActionInstruction(t *testing.T) {
	tests := []struct {
		name     string
		metadata string
		want     string
	}{
		{"valid", `{"instruction":"do the thing","mode":"x"}`, "do the thing"},
		{"empty string", "", ""},
		{"empty object", "{}", ""},
		{"no instruction key", `{"mode":"x"}`, ""},
		{"invalid json", `{not json`, ""},
		{"instruction not string", `{"instruction":42}`, ""},
		{"multiline", "{\"instruction\":\"line1\\nline2\"}", "line1\nline2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := db.Action{Metadata: tt.metadata}
			if got := a.Instruction(); got != tt.want {
				t.Errorf("Instruction() = %q, want %q", got, tt.want)
			}
		})
	}
}
