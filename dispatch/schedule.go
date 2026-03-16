package dispatch

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func CheckSchedules(database db.Store, now time.Time) error {
	schedules, err := database.ListSchedules()
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}

	for _, s := range schedules {
		if !s.Enabled {
			continue
		}

		task, err := database.GetTask(s.TaskID)
		if err != nil {
			slog.Warn("schedule: task lookup failed", "schedule_id", s.ID, "task_id", s.TaskID, "error", err)
			continue
		}
		if task.Status == db.TaskStatusDone || task.Status == db.TaskStatusArchived {
			slog.Info("schedule: auto-disabling for closed task", "schedule_id", s.ID, "task_id", s.TaskID, "task_status", task.Status)
			if err := database.UpdateScheduleEnabled(s.ID, false); err != nil {
				slog.Error("schedule: auto-disable failed", "schedule_id", s.ID, "error", err)
			}
			continue
		}

		sched, err := cronParser.Parse(s.CronExpr)
		if err != nil {
			slog.Warn("schedule: invalid cron expr", "schedule_id", s.ID, "cron_expr", s.CronExpr, "error", err)
			continue
		}

		baseTime := s.CreatedAt
		if s.LastRunAt.Valid {
			baseTime = s.LastRunAt.String
		}
		base, err := time.Parse("2006-01-02 15:04:05", baseTime)
		if err != nil {
			slog.Warn("schedule: parse base time failed", "schedule_id", s.ID, "base_time", baseTime, "error", err)
			continue
		}

		next := sched.Next(base)
		if now.Before(next) {
			continue
		}

		has, err := database.HasActiveAction(s.TaskID, s.PromptID)
		if err != nil {
			slog.Warn("schedule: active action check failed", "schedule_id", s.ID, "error", err)
			continue
		}
		if has {
			slog.Debug("schedule: skipping, active action exists", "schedule_id", s.ID)
			continue
		}

		_, err = database.InsertAction(s.Title, s.PromptID, s.TaskID, s.Metadata, "pending")
		if err != nil {
			slog.Error("schedule: insert action failed", "schedule_id", s.ID, "error", err)
			continue
		}

		if err := database.UpdateScheduleLastRunAt(s.ID, now.Format("2006-01-02 15:04:05")); err != nil {
			slog.Error("schedule: update last_run_at failed", "schedule_id", s.ID, "error", err)
		}

		slog.Info("schedule: action created", "schedule_id", s.ID, "prompt_id", s.PromptID, "task_id", s.TaskID)
	}

	return nil
}
