package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/MH4GF/tq/db"
)

func CheckSchedules(database db.Store, now time.Time) error {
	schedules, err := database.ListSchedules(0)
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
			slog.Debug("schedule: skipping for closed task", "schedule_id", s.ID, "task_id", s.TaskID, "task_status", task.Status)
			continue
		}

		sched, err := db.CronParser.Parse(s.CronExpr)
		if err != nil {
			slog.Warn("schedule: invalid cron expr", "schedule_id", s.ID, "cron_expr", s.CronExpr, "error", err)
			continue
		}

		baseTime := s.CreatedAt
		if s.LastRunAt.Valid {
			baseTime = s.LastRunAt.String
		}
		base, err := time.Parse(db.TimeLayout, baseTime)
		if err != nil {
			slog.Warn("schedule: parse base time failed", "schedule_id", s.ID, "base_time", baseTime, "error", err)
			continue
		}

		next := sched.Next(base)
		if now.Before(next) {
			continue
		}

		has, err := database.HasActiveActionWithMeta(s.TaskID, MetaKeyScheduleID, fmt.Sprintf("%d", s.ID))
		if err != nil {
			slog.Warn("schedule: active action check failed", "schedule_id", s.ID, "error", err)
			continue
		}
		if has {
			slog.Debug("schedule: skipping, active action exists", "schedule_id", s.ID)
			continue
		}

		meta, err := parseMetadata(s.Metadata)
		if err != nil {
			slog.Warn("schedule: parse metadata failed", "schedule_id", s.ID, "error", err)
			meta = make(map[string]any)
		}
		meta[MetaKeyInstruction] = s.Instruction
		meta[MetaKeyScheduleID] = fmt.Sprintf("%d", s.ID)
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			slog.Error("schedule: marshal metadata failed", "schedule_id", s.ID, "error", err)
			continue
		}

		id, err := database.InsertAction(s.Title, s.TaskID, string(metaJSON), db.ActionStatusPending)
		if err != nil {
			slog.Error("schedule: insert action failed", "schedule_id", s.ID, "error", err)
			continue
		}

		if err := database.UpdateScheduleLastRunAt(s.ID, now.UTC().Format(db.TimeLayout)); err != nil {
			slog.Error("schedule: update last_run_at failed", "schedule_id", s.ID, "error", err)
		}

		slog.Info("schedule: action created", "action_id", id, "schedule_id", s.ID, "instruction", s.Instruction, "task_id", s.TaskID)
	}

	return nil
}
