package dispatch_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

func TestCheckSchedules(t *testing.T) {
	mustParse := func(s string) time.Time {
		ts, err := time.Parse("2006-01-02 15:04:05", s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return ts
	}

	createdDue := mustParse("2026-03-12 09:58:00")
	nowDue := mustParse("2026-03-12 10:00:00")
	createdRecent := mustParse("2026-03-12 09:00:00")
	nowEarly := mustParse("2026-03-12 09:30:00")
	createdOld := mustParse("2026-03-01 00:00:00")
	lastRunRecent := mustParse("2026-03-12 09:00:00")
	jst := time.FixedZone("JST", 9*3600)
	createdJSTYesterday := mustParse("2026-03-11 12:00:00")
	nowJST0900 := time.Date(2026, 3, 12, 9, 0, 0, 0, jst)

	tests := []struct {
		name                  string
		prompt                string
		title                 string
		cron                  string
		metadata              string
		createdAt             time.Time
		lastRunAt             *time.Time
		now                   time.Time
		insertDuplicateAction bool
		disableSchedule       bool
		wantActions           int
		wantTitle             string
		wantMetadata          string
		wantLastRunAtUTC      string
	}{
		{
			name:             "action created when cron is due",
			prompt:           "my-prompt",
			title:            "My Prompt",
			cron:             "* * * * *",
			metadata:         `{"key":"val"}`,
			createdAt:        createdDue,
			now:              nowDue,
			wantActions:      1,
			wantTitle:        "My Prompt",
			wantMetadata:     `{"instruction":"my-prompt","key":"val","schedule_id":"1"}`,
			wantLastRunAtUTC: "2026-03-12 10:00:00",
		},
		{
			name:        "not due yet",
			prompt:      "my-prompt",
			title:       "My Prompt",
			cron:        "0 */3 * * *",
			metadata:    "{}",
			createdAt:   createdRecent,
			now:         nowEarly,
			wantActions: 0,
		},
		{
			name:                  "duplicate active action skipped",
			prompt:                "my-prompt",
			title:                 "My Prompt",
			cron:                  "* * * * *",
			metadata:              "{}",
			createdAt:             createdDue,
			now:                   nowDue,
			insertDuplicateAction: true,
			wantActions:           1,
		},
		{
			name:            "disabled schedule skipped",
			prompt:          "my-prompt",
			title:           "My Prompt",
			cron:            "* * * * *",
			metadata:        "{}",
			createdAt:       createdDue,
			now:             nowDue,
			disableSchedule: true,
			wantActions:     0,
		},
		{
			name:        "instruction-based action created",
			prompt:      "/gh-ops:watch",
			title:       "Watch notifications",
			cron:        "* * * * *",
			metadata:    "{}",
			createdAt:   createdDue,
			now:         nowDue,
			wantActions: 1,
		},
		{
			name:             "last_run_at stored as UTC",
			prompt:           "my-prompt",
			title:            "My Prompt",
			cron:             "* * * * *",
			metadata:         "{}",
			createdAt:        createdDue,
			now:              time.Date(2026, 3, 12, 19, 0, 0, 0, jst),
			wantActions:      1,
			wantLastRunAtUTC: "2026-03-12 10:00:00",
		},
		{
			name:        "uses last_run_at over created_at",
			prompt:      "my-prompt",
			title:       "My Prompt",
			cron:        "0 */3 * * *",
			metadata:    "{}",
			createdAt:   createdOld,
			lastRunAt:   &lastRunRecent,
			now:         nowEarly,
			wantActions: 0,
		},
		{
			name:        "cron evaluated in now.Location(): 09:00 fires at 09:00 JST",
			prompt:      "my-prompt",
			title:       "My Prompt",
			cron:        "0 9 * * *",
			metadata:    "{}",
			createdAt:   createdJSTYesterday,
			now:         nowJST0900,
			wantActions: 1,
		},
		{
			name:        "cron evaluated in now.Location(): 18:00 does not fire at 09:00 JST",
			prompt:      "my-prompt",
			title:       "My Prompt",
			cron:        "0 18 * * *",
			metadata:    "{}",
			createdAt:   createdJSTYesterday,
			now:         nowJST0900,
			wantActions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, _ := d.InsertTask(1, "test", "{}", "")
			scheduleID, _ := d.InsertSchedule(taskID, tt.prompt, tt.title, tt.cron, tt.metadata)
			createdAt := tt.createdAt
			d.SetScheduleTimestampsForTest(scheduleID, &createdAt, tt.lastRunAt)
			if tt.insertDuplicateAction {
				d.InsertAction("existing", taskID, `{"schedule_id":"1"}`, db.ActionStatusPending, nil)
			}
			if tt.disableSchedule {
				d.UpdateScheduleEnabled(scheduleID, false)
			}

			if err := dispatch.CheckSchedules(d, tt.now); err != nil {
				t.Fatal(err)
			}

			actions, _ := d.ListActions("", nil, 0)
			if len(actions) != tt.wantActions {
				t.Fatalf("len(actions) = %d, want %d", len(actions), tt.wantActions)
			}

			if tt.wantTitle != "" && actions[0].Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", actions[0].Title, tt.wantTitle)
			}
			if tt.wantMetadata != "" && actions[0].Metadata != tt.wantMetadata {
				t.Errorf("metadata = %q, want %q", actions[0].Metadata, tt.wantMetadata)
			}

			if tt.wantLastRunAtUTC != "" {
				s, _ := d.GetSchedule(scheduleID)
				if !s.LastRunAt.Valid {
					t.Fatal("expected last_run_at to be set")
				}
				if s.LastRunAt.String != tt.wantLastRunAtUTC {
					t.Errorf("last_run_at = %q, want %q", s.LastRunAt.String, tt.wantLastRunAtUTC)
				}
			}
		})
	}
}

func TestCheckSchedules_MarshalFailureUpdatesLastRunAt(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	scheduleID, _ := d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")

	createdAt, _ := time.Parse(db.TimeLayout, "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(scheduleID, &createdAt, nil)

	restore := dispatch.SetMarshalMeta(func(any) ([]byte, error) {
		return nil, errors.New("simulated marshal failure")
	})
	t.Cleanup(restore)

	now, _ := time.Parse(db.TimeLayout, "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions on marshal failure, got %d", len(actions))
	}

	s, _ := d.GetSchedule(scheduleID)
	if !s.LastRunAt.Valid {
		t.Fatal("expected last_run_at to be set after marshal failure (throttle retries)")
	}
	if s.LastRunAt.String != "2026-03-12 10:00:00" {
		t.Errorf("last_run_at = %q, want %q", s.LastRunAt.String, "2026-03-12 10:00:00")
	}
	if !s.LastError.Valid {
		t.Fatal("expected last_error to be populated after marshal failure")
	}
	if !strings.Contains(s.LastError.String, "marshal metadata") {
		t.Errorf("last_error = %q, want to contain %q", s.LastError.String, "marshal metadata")
	}
}

func TestCheckSchedules_InvalidMetadataRecordsLastError(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	scheduleID, _ := d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", `{"mode":"bogus"}`)

	createdAt, _ := time.Parse(db.TimeLayout, "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(scheduleID, &createdAt, nil)

	now, _ := time.Parse(db.TimeLayout, "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions on invalid metadata, got %d", len(actions))
	}

	s, _ := d.GetSchedule(scheduleID)
	if !s.LastRunAt.Valid {
		t.Fatal("expected last_run_at to be set after invalid metadata (throttle retries)")
	}
	if !s.LastError.Valid {
		t.Fatal("expected last_error to be populated on invalid metadata")
	}
	if !strings.Contains(s.LastError.String, "invalid metadata") {
		t.Errorf("last_error = %q, want to contain %q", s.LastError.String, "invalid metadata")
	}
}

func TestCheckSchedules_LastErrorClearedOnSuccess(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	scheduleID, _ := d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", "{}")
	if err := d.UpdateScheduleRun(scheduleID, "2026-03-12 09:58:00", "stale failure"); err != nil {
		t.Fatalf("seed last_error: %v", err)
	}

	createdAt, _ := time.Parse(db.TimeLayout, "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(scheduleID, &createdAt, nil)

	now, _ := time.Parse(db.TimeLayout, "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	s, _ := d.GetSchedule(scheduleID)
	if s.LastError.Valid {
		t.Errorf("expected last_error to be cleared after success, got %q", s.LastError.String)
	}
}

func TestCheckSchedules_MultipleSchedulesFiredAndRecorded(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")

	createdAt, _ := time.Parse(db.TimeLayout, "2026-03-12 09:58:00")
	scheduleIDs := make([]int64, 0, 3)
	for i := 1; i <= 3; i++ {
		id, err := d.InsertSchedule(taskID, "prompt", fmt.Sprintf("Title %d", i), "* * * * *", "{}")
		if err != nil {
			t.Fatalf("seed schedule %d: %v", i, err)
		}
		d.SetScheduleTimestampsForTest(id, &createdAt, nil)
		scheduleIDs = append(scheduleIDs, id)
	}

	now, _ := time.Parse(db.TimeLayout, "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 3 {
		t.Fatalf("len(actions) = %d, want 3", len(actions))
	}

	for _, id := range scheduleIDs {
		s, err := d.GetSchedule(id)
		if err != nil {
			t.Fatalf("GetSchedule(%d): %v", id, err)
		}
		if !s.LastRunAt.Valid {
			t.Errorf("schedule %d: last_run_at not set", id)
		}
		if s.LastError.Valid {
			t.Errorf("schedule %d: unexpected last_error = %q", id, s.LastError.String)
		}
	}
}

func TestCheckSchedules_MixedValidAndInvalidMetadataPartialSuccess(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")

	createdAt, _ := time.Parse(db.TimeLayout, "2026-03-12 09:58:00")
	specs := []struct {
		title    string
		metadata string
	}{
		{title: "Title 1", metadata: "{}"},
		{title: "Title 2", metadata: `{"mode":"bogus"}`},
		{title: "Title 3", metadata: "{}"},
	}
	scheduleIDs := make([]int64, 0, len(specs))
	for _, sp := range specs {
		id, err := d.InsertSchedule(taskID, "prompt", sp.title, "* * * * *", sp.metadata)
		if err != nil {
			t.Fatalf("seed schedule %s: %v", sp.title, err)
		}
		d.SetScheduleTimestampsForTest(id, &createdAt, nil)
		scheduleIDs = append(scheduleIDs, id)
	}

	now, _ := time.Parse(db.TimeLayout, "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions("", nil, 0)
	if len(actions) != 2 {
		t.Fatalf("len(actions) = %d, want 2 (schedule #2 invalid metadata)", len(actions))
	}

	titles := map[string]bool{}
	for _, a := range actions {
		titles[a.Title] = true
	}
	if !titles["Title 1"] || !titles["Title 3"] {
		t.Errorf("expected actions for Title 1 and Title 3, got %v", titles)
	}
	if titles["Title 2"] {
		t.Errorf("did not expect action for Title 2 (invalid metadata)")
	}

	good := []int64{scheduleIDs[0], scheduleIDs[2]}
	for _, id := range good {
		s, _ := d.GetSchedule(id)
		if !s.LastRunAt.Valid {
			t.Errorf("schedule %d: last_run_at not set on success", id)
		}
		if s.LastError.Valid {
			t.Errorf("schedule %d: unexpected last_error = %q", id, s.LastError.String)
		}
	}

	bad, _ := d.GetSchedule(scheduleIDs[1])
	if !bad.LastRunAt.Valid {
		t.Errorf("invalid-metadata schedule: last_run_at not set (throttle should still record)")
	}
	if !bad.LastError.Valid || !strings.Contains(bad.LastError.String, "invalid metadata") {
		t.Errorf("invalid-metadata schedule: last_error = %v, want containing %q", bad.LastError, "invalid metadata")
	}
}

func TestCheckSchedules_ClaudeArgsPropagated(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "{}", "")
	d.InsertSchedule(taskID, "my-prompt", "My Prompt", "* * * * *", `{"claude_args":["--max-turns","5"]}`)

	createdAt, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 09:58:00")
	d.SetScheduleTimestampsForTest(1, &createdAt, nil)

	now, _ := time.Parse("2006-01-02 15:04:05", "2026-03-12 10:00:00")
	if err := dispatch.CheckSchedules(d, now); err != nil {
		t.Fatal(err)
	}

	actions, _ := d.ListActions(db.ActionStatusPending, nil, 0)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	want := `{"claude_args":["--max-turns","5"],"instruction":"my-prompt","schedule_id":"1"}`
	if actions[0].Metadata != want {
		t.Errorf("metadata = %q, want %q", actions[0].Metadata, want)
	}
}
