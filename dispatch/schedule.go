package dispatch

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/MH4GF/tq/db"
)

var marshalMeta = json.Marshal

// scheduleDecision is the per-schedule outcome of the in-memory decision pass.
// Exactly one of insertSpec / errUpdate is set when the schedule contributes
// to bulk execution; both nil means silent skip (cron not due, task closed,
// active action already in flight, etc.).
type scheduleDecision struct {
	schedule   db.Schedule
	insertSpec *db.ActionInsertSpec
	successRun *db.ScheduleRunUpdate
	errUpdate  *db.ScheduleRunUpdate
}

func CheckSchedules(database db.Store, now time.Time) error {
	schedules, err := database.ListSchedules(0)
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}
	if len(schedules) == 0 {
		return nil
	}

	taskIDs := make([]int64, 0, len(schedules))
	scheduleIDs := make([]int64, 0, len(schedules))
	for _, s := range schedules {
		if !s.Enabled {
			continue
		}
		taskIDs = append(taskIDs, s.TaskID)
		scheduleIDs = append(scheduleIDs, s.ID)
	}
	if len(scheduleIDs) == 0 {
		return nil
	}

	taskMap, err := database.GetTasksByIDs(taskIDs)
	if err != nil {
		return fmt.Errorf("prefetch tasks: %w", err)
	}
	activeMap, err := database.HasActiveActionsForSchedules(scheduleIDs)
	if err != nil {
		return fmt.Errorf("prefetch active actions: %w", err)
	}

	decisions := decideSchedules(schedules, taskMap, activeMap, now)

	var (
		inserts         []db.ActionInsertSpec
		insertDecisions []*scheduleDecision
		errorRunUpdates []db.ScheduleRunUpdate
		successUpdates  []db.ScheduleRunUpdate
	)
	for i := range decisions {
		d := &decisions[i]
		if d.errUpdate != nil {
			errorRunUpdates = append(errorRunUpdates, *d.errUpdate)
		}
		if d.insertSpec != nil {
			inserts = append(inserts, *d.insertSpec)
			insertDecisions = append(insertDecisions, d)
			successUpdates = append(successUpdates, *d.successRun)
		}
	}

	// Record in-memory decision errors first; they are independent of bulk
	// insert success and must persist even if the insert phase fails.
	if len(errorRunUpdates) > 0 {
		if err := database.BulkUpdateScheduleRuns(errorRunUpdates); err != nil {
			return fmt.Errorf("bulk update schedule runs (errors): %w", err)
		}
	}

	if len(inserts) > 0 {
		ids, err := database.BulkInsertActions(inserts)
		if err != nil {
			// Do not write successUpdates; next tick will re-decide and retry.
			return fmt.Errorf("bulk insert actions: %w", err)
		}
		for i, id := range ids {
			d := insertDecisions[i]
			slog.Info("schedule: action created",
				"action_id", id,
				"schedule_id", d.schedule.ID,
				"instruction", d.schedule.Instruction,
				"task_id", d.schedule.TaskID)
		}
		if err := database.BulkUpdateScheduleRuns(successUpdates); err != nil {
			return fmt.Errorf("bulk update schedule runs (success): %w", err)
		}
	}

	return nil
}

// decideSchedules runs the in-memory tick decision for every enabled schedule.
// It performs no I/O; all DB facts come from taskMap and activeMap.
func decideSchedules(schedules []db.Schedule, taskMap map[int64]*db.Task, activeMap map[int64]bool, now time.Time) []scheduleDecision {
	out := make([]scheduleDecision, 0, len(schedules))
	nowUTC := now.UTC().Format(db.TimeLayout)

	for _, s := range schedules {
		if !s.Enabled {
			continue
		}

		task, ok := taskMap[s.TaskID]
		if !ok {
			slog.Warn("schedule: task lookup failed", "schedule_id", s.ID, "task_id", s.TaskID)
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
		next := sched.Next(base.In(now.Location()))
		if now.Before(next) {
			continue
		}

		if activeMap[s.ID] {
			slog.Debug("schedule: skipping, active action exists", "schedule_id", s.ID)
			continue
		}

		meta, err := ParseActionMetadata(s.Metadata)
		if err != nil {
			slog.Warn("schedule: parse metadata failed", "schedule_id", s.ID, "error", err)
			meta = make(map[string]any)
		}
		meta[MetaKeyInstruction] = s.Instruction
		meta[MetaKeyScheduleID] = fmt.Sprintf("%d", s.ID)

		if err := ValidateActionMetadata(meta); err != nil {
			slog.Warn("schedule: invalid metadata", "schedule_id", s.ID, "error", err)
			out = append(out, scheduleDecision{
				schedule:  s,
				errUpdate: &db.ScheduleRunUpdate{ID: s.ID, LastRunAt: nowUTC, LastError: fmt.Sprintf("invalid metadata: %v", err)},
			})
			continue
		}

		metaJSON, err := marshalMeta(meta)
		if err != nil {
			slog.Error("schedule: marshal metadata failed", "schedule_id", s.ID, "error", err)
			out = append(out, scheduleDecision{
				schedule:  s,
				errUpdate: &db.ScheduleRunUpdate{ID: s.ID, LastRunAt: nowUTC, LastError: fmt.Sprintf("marshal metadata: %v", err)},
			})
			continue
		}

		out = append(out, scheduleDecision{
			schedule:   s,
			insertSpec: &db.ActionInsertSpec{Title: s.Title, TaskID: s.TaskID, Metadata: string(metaJSON), Status: db.ActionStatusPending},
			successRun: &db.ScheduleRunUpdate{ID: s.ID, LastRunAt: nowUTC, LastError: ""},
		})
	}

	return out
}
