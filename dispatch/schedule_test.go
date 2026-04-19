package dispatch_test

import (
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
