package dispatch

import (
	"time"

	"github.com/MH4GF/tq/db"
)

// SetDirExists replaces the dirExists function for testing and returns a restore function.
func SetDirExists(fn func(string) bool) func() {
	orig := dirExists
	dirExists = fn
	return func() { dirExists = orig }
}

// SetMarshalMeta replaces the marshalMeta function for testing and returns a restore function.
func SetMarshalMeta(fn func(any) ([]byte, error)) func() {
	orig := marshalMeta
	marshalMeta = fn
	return func() { marshalMeta = orig }
}

// DecideSchedulesInvariant drives decideSchedules with the same prefetch that
// CheckSchedules performs and reports, for every decision that yields an action
// insert, whether the paired success run update is populated and targets the
// same schedule. ok is false iff the insert/successRun invariant is broken.
func DecideSchedulesInvariant(database db.Store, now time.Time) (insertCount int, ok bool, err error) {
	schedules, err := database.ListSchedules(0)
	if err != nil {
		return 0, false, err
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
	taskMap, err := database.GetTasksByIDs(taskIDs)
	if err != nil {
		return 0, false, err
	}
	activeMap, err := database.HasActiveActionsForSchedules(scheduleIDs)
	if err != nil {
		return 0, false, err
	}

	for _, d := range decideSchedules(schedules, taskMap, activeMap, now) {
		if d.insert == nil {
			continue
		}
		insertCount++
		if d.insert.successRun.ID != d.schedule.ID || d.insert.successRun.LastRunAt == "" {
			return insertCount, false, nil
		}
	}
	return insertCount, true, nil
}
